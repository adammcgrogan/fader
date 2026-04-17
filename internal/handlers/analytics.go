package handlers

import (
	"net/http"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	"github.com/google/uuid"
)

type AnalyticsHandler struct {
	db *db.Queries
}

func NewAnalyticsHandler(q *db.Queries) *AnalyticsHandler {
	return &AnalyticsHandler{db: q}
}

func (h *AnalyticsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	if !user.IsPro() {
		renderTemplate(w, "analytics_upgrade.html", map[string]any{"User": user})
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

	summary, err := h.db.GetAnalyticsSummary(r.Context(), profileID)
	if err != nil {
		http.Error(w, "could not load analytics", http.StatusInternalServerError)
		return
	}

	allProfiles, _ := h.db.GetProfilesByUserID(r.Context(), userID)

	renderTemplate(w, "analytics.html", map[string]any{
		"Profile":     profile,
		"Summary":     summary,
		"AllProfiles": allProfiles,
		"User":        user,
	})
}
