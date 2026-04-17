package handlers

import (
	"net/http"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
)

type DashboardHandler struct {
	db *db.Queries
}

func NewDashboardHandler(q *db.Queries) *DashboardHandler {
	return &DashboardHandler{db: q}
}

func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	profiles, err := h.db.GetProfilesByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "could not load profiles", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "dashboard.html", map[string]any{
		"User":     user,
		"Profiles": profiles,
	})
}
