package handlers

import "net/http"

func ServeLanding(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "landing.html", nil)
}
