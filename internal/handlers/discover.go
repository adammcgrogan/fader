package handlers

import (
	"net/http"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
)

type DiscoverHandler struct {
	db *db.Queries
}

func NewDiscoverHandler(q *db.Queries) *DiscoverHandler {
	return &DiscoverHandler{db: q}
}

func (h *DiscoverHandler) Show(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.db.ListDiscoverProfiles(r.Context())
	if err != nil {
		http.Error(w, "could not load profiles", http.StatusInternalServerError)
		return
	}
	_, loggedIn := middleware.GetUserID(r)
	renderTemplate(w, "discover.html", map[string]any{
		"Profiles": profiles,
		"LoggedIn": loggedIn,
	})
}
