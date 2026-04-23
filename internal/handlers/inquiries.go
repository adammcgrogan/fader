package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	"github.com/google/uuid"
)

const (
	inquiryRateLimitCount  = 5
	inquiryRateLimitWindow = 60
)

type InquiriesHandler struct {
	db *db.Queries
}

func NewInquiriesHandler(q *db.Queries) *InquiriesHandler {
	return &InquiriesHandler{db: q}
}

func (h *InquiriesHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	if !user.IsPro() {
		http.Error(w, "upgrade to Pro to view inquiries", http.StatusForbidden)
		return
	}

	profileIDStr := r.URL.Query().Get("profile")
	var profileID uuid.UUID
	if profileIDStr != "" {
		profileID, err = uuid.Parse(profileIDStr)
		if err != nil {
			http.Error(w, "invalid profile", http.StatusBadRequest)
			return
		}
	} else {
		profiles, err := h.db.GetProfilesByUserID(r.Context(), userID)
		if err != nil || len(profiles) == 0 {
			http.Error(w, "no profiles", http.StatusInternalServerError)
			return
		}
		profileID = profiles[0].ID
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	inquiries, err := h.db.ListInquiriesByProfileID(r.Context(), profileID)
	if err != nil {
		http.Error(w, "could not load inquiries", http.StatusInternalServerError)
		return
	}
	allProfiles, _ := h.db.GetProfilesByUserID(r.Context(), userID)
	unreadCount, _ := h.db.CountUnreadInquiriesByProfileID(r.Context(), profileID)

	if unreadCount > 0 {
		_ = h.db.MarkInquiriesReadByProfileID(r.Context(), profileID)
	}

	renderTemplate(w, "inquiries.html", map[string]any{
		"Profile":     profile,
		"Inquiries":   inquiries,
		"AllProfiles": allProfiles,
		"UnreadCount": unreadCount,
		"User":        user,
	})
}

func (h *InquiriesHandler) Submit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeInquiryJSON(w, http.StatusBadRequest, false, "Bad request.")
		return
	}

	profileID, err := uuid.Parse(strings.TrimSpace(r.FormValue("profile_id")))
	if err != nil {
		writeInquiryJSON(w, http.StatusBadRequest, false, "Invalid profile.")
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil {
		writeInquiryJSON(w, http.StatusNotFound, false, "Profile not found.")
		return
	}
	owner, err := h.db.GetUserByID(r.Context(), profile.UserID)
	if err != nil || !owner.IsPro() {
		writeInquiryJSON(w, http.StatusForbidden, false, "Booking inquiries are unavailable for this profile.")
		return
	}

	if strings.TrimSpace(r.FormValue("company")) != "" {
		writeInquiryJSON(w, http.StatusOK, true, "Message sent.")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	message := strings.TrimSpace(r.FormValue("message"))

	if name == "" || message == "" {
		writeInquiryJSON(w, http.StatusBadRequest, false, "Name and message are required.")
		return
	}
	if email == "" && phone == "" {
		writeInquiryJSON(w, http.StatusBadRequest, false, "Add an email address or phone number.")
		return
	}

	ipHash := hashIP(r.RemoteAddr)
	recentCount, err := h.db.CountRecentInquiriesByIP(r.Context(), profileID, ipHash, inquiryRateLimitWindow)
	if err != nil {
		writeInquiryJSON(w, http.StatusInternalServerError, false, "Could not process inquiry.")
		return
	}
	if recentCount >= inquiryRateLimitCount {
		writeInquiryJSON(w, http.StatusTooManyRequests, false, "Too many inquiries from this device. Please try again later.")
		return
	}

	var emailPtr, phonePtr *string
	if email != "" {
		emailPtr = &email
	}
	if phone != "" {
		phonePtr = &phone
	}

	if err := h.db.CreateInquiry(r.Context(), profileID, name, emailPtr, phonePtr, message, ipHash); err != nil {
		writeInquiryJSON(w, http.StatusInternalServerError, false, "Could not save inquiry.")
		return
	}

	writeInquiryJSON(w, http.StatusOK, true, "Message sent.")
}

func writeInquiryJSON(w http.ResponseWriter, status int, ok bool, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      ok,
		"message": message,
	})
}
