package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	gotrue "github.com/supabase-community/gotrue-go"
	"github.com/supabase-community/gotrue-go/types"
)

type SettingsHandler struct {
	db   *db.Queries
	auth gotrue.Client
}

func NewSettingsHandler(q *db.Queries, auth gotrue.Client) *SettingsHandler {
	return &SettingsHandler{db: q, auth: auth}
}

func (h *SettingsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	sub, _ := h.db.GetSubscriptionByUserID(r.Context(), userID) // nil for free users
	renderTemplate(w, "settings.html", map[string]any{
		"User":         user,
		"Subscription": sub,
		"Success":      r.URL.Query().Get("success"),
	})
}

func (h *SettingsHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Redirect(w, r, "/settings?success=", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		renderTemplate(w, "settings.html", map[string]any{"User": user, "PasswordError": "bad request"})
		return
	}

	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	if len(password) < 8 {
		renderTemplate(w, "settings.html", map[string]any{"User": user, "PasswordError": "password must be at least 8 characters"})
		return
	}
	if password != confirm {
		renderTemplate(w, "settings.html", map[string]any{"User": user, "PasswordError": "passwords do not match"})
		return
	}

	cookie, err := r.Cookie("sb-token")
	if err != nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	if _, err := h.auth.WithToken(cookie.Value).UpdateUser(types.UpdateUserRequest{Password: &password}); err != nil {
		log.Printf("change password error for %s: %v", user.Email, err)
		renderTemplate(w, "settings.html", map[string]any{"User": user, "PasswordError": "could not update password — please try again"})
		return
	}

	http.Redirect(w, r, "/settings?success=password", http.StatusSeeOther)
}

func (h *SettingsHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		renderTemplate(w, "settings.html", map[string]any{"User": user, "DeleteError": "bad request"})
		return
	}

	if strings.TrimSpace(r.FormValue("confirm")) != "DELETE" {
		renderTemplate(w, "settings.html", map[string]any{"User": user, "DeleteError": `type "DELETE" exactly to confirm`})
		return
	}

	if err := h.db.DeleteUserData(r.Context(), userID); err != nil {
		log.Printf("delete account error for %s: %v", user.Email, err)
		renderTemplate(w, "settings.html", map[string]any{"User": user, "DeleteError": "could not delete account — please contact support"})
		return
	}

	secure := !strings.Contains(BaseDomain, "localhost")
	cookieDomain := cookieDomainFor(BaseDomain)
	for _, name := range []string{"sb-token", "sb-refresh"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   cookieDomain,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
