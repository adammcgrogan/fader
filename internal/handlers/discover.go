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
	seen := map[string]bool{}
	var allGenres []string
	for _, p := range profiles {
		for _, g := range p.Genres {
			if g != "" && !seen[g] {
				seen[g] = true
				allGenres = append(allGenres, g)
			}
		}
	}

	_, loggedIn := middleware.GetUserID(r)
	renderTemplate(w, "discover.html", map[string]any{
		"Profiles":  profiles,
		"AllGenres": allGenres,
		"LoggedIn":  loggedIn,
	})
}
