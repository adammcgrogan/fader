package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/adammcgrogan/fader/internal/config"
	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	stripeSubscription "github.com/stripe/stripe-go/v76/subscription"
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

func (h *StripeHandler) baseURL() string {
	if strings.Contains(h.cfg.BaseDomain, "localhost") {
		return "http://" + h.cfg.BaseDomain
	}
	return "https://" + h.cfg.BaseDomain
}

func (h *StripeHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	if h.cfg.StripeSecretKey == "" {
		http.Error(w, "payments not yet configured", http.StatusServiceUnavailable)
		return
	}
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	// Already Pro — send straight to portal
	if user.IsPro() {
		http.Redirect(w, r, "/billing/portal", http.StatusSeeOther)
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
		SuccessURL: stripe.String(h.baseURL() + "/billing/success"),
		CancelURL:  stripe.String(h.baseURL() + "/dashboard"),
	}

	s, err := checkoutsession.New(params)
	if err != nil {
		http.Error(w, "could not create checkout session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (h *StripeHandler) Portal(w http.ResponseWriter, r *http.Request) {
	if h.cfg.StripeSecretKey == "" {
		http.Error(w, "payments not yet configured", http.StatusServiceUnavailable)
		return
	}
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil || user.StripeCustomerID == nil {
		http.Redirect(w, r, "/billing/checkout", http.StatusSeeOther)
		return
	}

	s, err := session.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*user.StripeCustomerID),
		ReturnURL: stripe.String(h.baseURL() + "/settings"),
	})
	if err != nil {
		http.Error(w, "could not create portal session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (h *StripeHandler) BillingSuccess(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	user, _ := h.db.GetUserByID(r.Context(), userID)
	renderTemplate(w, "billing_success.html", map[string]any{"User": user})
}

func (h *StripeHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if h.cfg.StripeSecretKey == "" {
		http.Error(w, "payments not configured", http.StatusServiceUnavailable)
		return
	}
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil || !user.IsPro() {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	sub, err := h.db.GetSubscriptionByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "subscription not found", http.StatusInternalServerError)
		return
	}

	updated, err := stripeSubscription.Update(sub.StripeSubscriptionID, &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	})
	if err != nil {
		log.Printf("stripe cancel error for user %s: %v", userID, err)
		http.Error(w, "could not cancel subscription", http.StatusInternalServerError)
		return
	}

	// Update our DB immediately so settings page reflects the change at once
	periodEnd := time.Unix(updated.CurrentPeriodEnd, 0)
	h.db.UpsertSubscription(r.Context(), userID, updated.ID, string(updated.Status), &periodEnd, updated.CancelAtPeriodEnd)

	http.Redirect(w, r, "/settings?success=cancelled", http.StatusSeeOther)
}

func (h *StripeHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		log.Printf("stripe webhook: failed to read body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(body, sig, h.cfg.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		log.Printf("stripe webhook: signature verification failed: %v", err)
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
			// Fetch the full subscription to get period end and status
			fullSub, err := stripeSubscription.Get(cs.Subscription.ID, nil)
			if err == nil {
				periodEnd := time.Unix(fullSub.CurrentPeriodEnd, 0)
				h.db.UpsertSubscription(r.Context(), u.ID, fullSub.ID, string(fullSub.Status), &periodEnd, fullSub.CancelAtPeriodEnd)
			} else {
				h.db.UpsertSubscription(r.Context(), u.ID, cs.Subscription.ID, "active", nil, false)
			}
		}

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
		if sub.Status == stripe.SubscriptionStatusActive ||
			sub.Status == stripe.SubscriptionStatusTrialing ||
			sub.Status == stripe.SubscriptionStatusPastDue {
			tier = "pro"
		}
		h.db.SetUserTier(r.Context(), u.ID, tier)
		periodEnd := time.Unix(sub.CurrentPeriodEnd, 0)
		h.db.UpsertSubscription(r.Context(), u.ID, sub.ID, string(sub.Status), &periodEnd, sub.CancelAtPeriodEnd)

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
		periodEnd := time.Unix(sub.CurrentPeriodEnd, 0)
		h.db.UpsertSubscription(r.Context(), u.ID, sub.ID, string(sub.Status), &periodEnd, sub.CancelAtPeriodEnd)
	}

	w.WriteHeader(http.StatusOK)
}
