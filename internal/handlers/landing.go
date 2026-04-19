package handlers

import (
	"net/http"

	"github.com/adammcgrogan/fader/internal/middleware"
)

func ServeLanding(w http.ResponseWriter, r *http.Request) {
	_, loggedIn := middleware.GetUserID(r)
	renderTemplate(w, "landing.html", map[string]any{
		"LoggedIn": loggedIn,
	})
}
