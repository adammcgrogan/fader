package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/adammcgrogan/fader/internal/config"
	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/webhook"
)

type StripeHandler struct {
	db  *db.Queries
	cfg *config.Config
}

func NewStripeHandler(q *db.Queries, cfg *config.Config) *StripeHandler {
	stripe.Key = cfg.StripeSecretKey
	return &StripeHandler{db: q, cfg: cfg}
}

func (h *StripeHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	// Get or create Stripe customer
	customerID := ""
	if user.StripeCustomerID != nil {
		customerID = *user.StripeCustomerID
	} else {
		c, err := customer.New(&stripe.CustomerParams{
			Email: stripe.String(user.Email),
		})
		if err != nil {
			http.Error(w, "could not create customer", http.StatusInternalServerError)
			return
		}
		customerID = c.ID
		h.db.SetStripeCustomerID(r.Context(), userID, customerID)
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(h.cfg.StripePriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String("https://fader.bio/billing/success"),
		CancelURL:  stripe.String("https://fader.bio/dashboard"),
	}

	s, err := checkoutsession.New(params)
	if err != nil {
		http.Error(w, "could not create checkout session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (h *StripeHandler) Portal(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil || user.StripeCustomerID == nil {
		http.Redirect(w, r, "/billing/checkout", http.StatusSeeOther)
		return
	}

	s, err := session.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*user.StripeCustomerID),
		ReturnURL: stripe.String("https://fader.bio/dashboard"),
	})
	if err != nil {
		http.Error(w, "could not create portal session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (h *StripeHandler) BillingSuccess(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "billing_success.html", nil)
}

func (h *StripeHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.cfg.StripeWebhookSecret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			break
		}
		u, err := h.db.GetUserByStripeCustomerID(r.Context(), cs.Customer.ID)
		if err != nil {
			log.Printf("stripe webhook: user not found for customer %s", cs.Customer.ID)
			break
		}
		h.db.SetUserTier(r.Context(), u.ID, "pro")
		if cs.Subscription != nil {
			h.db.UpsertSubscription(r.Context(), u.ID, cs.Subscription.ID, string(cs.Subscription.Status))
		}

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			break
		}
		u, err := h.db.GetUserByStripeCustomerID(r.Context(), sub.Customer.ID)
		if err != nil {
			break
		}
		h.db.SetUserTier(r.Context(), u.ID, "free")
		h.db.UpsertSubscription(r.Context(), u.ID, sub.ID, string(sub.Status))

	case "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			break
		}
		u, err := h.db.GetUserByStripeCustomerID(r.Context(), sub.Customer.ID)
		if err != nil {
			break
		}
		tier := "free"
		if sub.Status == stripe.SubscriptionStatusActive || sub.Status == stripe.SubscriptionStatusTrialing {
			tier = "pro"
		}
		h.db.SetUserTier(r.Context(), u.ID, tier)
		h.db.UpsertSubscription(r.Context(), u.ID, sub.ID, string(sub.Status))
	}

	w.WriteHeader(http.StatusOK)
}
