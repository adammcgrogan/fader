package handlers

import (
	"net/http"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/validate"
	gotrue "github.com/supabase-community/gotrue-go"
	"github.com/supabase-community/gotrue-go/types"
)

type AuthHandler struct {
	db   *db.Queries
	auth gotrue.Client
}

func NewAuthHandler(q *db.Queries, auth gotrue.Client) *AuthHandler {
	return &AuthHandler{db: q, auth: auth}
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "auth_login.html", nil)
}

func (h *AuthHandler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	handle := r.URL.Query().Get("handle")
	renderTemplate(w, "auth_register.html", map[string]any{"Handle": handle})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	handle := r.FormValue("handle")

	if err := validate.Handle(handle); err != nil {
		renderTemplate(w, "auth_register.html", map[string]any{"Error": err.Error(), "Handle": handle})
		return
	}

	exists, err := h.db.HandleExists(r.Context(), handle)
	if err != nil || exists {
		renderTemplate(w, "auth_register.html", map[string]any{"Error": "that handle is already taken", "Handle": handle})
		return
	}

	resp, err := h.auth.Signup(types.SignupRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		renderTemplate(w, "auth_register.html", map[string]any{"Error": "registration failed: " + err.Error()})
		return
	}

	userID := resp.User.ID
	if _, err := h.db.CreateUser(r.Context(), userID, email); err != nil {
		renderTemplate(w, "auth_register.html", map[string]any{"Error": "could not create account"})
		return
	}

	if _, err := h.db.CreateProfile(r.Context(), userID, handle, handle); err != nil {
		renderTemplate(w, "auth_register.html", map[string]any{"Error": "could not create profile"})
		return
	}

	setAuthCookie(w, resp.Session.AccessToken)
	http.Redirect(w, r, "/edit", http.StatusSeeOther)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	resp, err := h.auth.SignInWithEmailPassword(r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		renderTemplate(w, "auth_login.html", map[string]any{"Error": "invalid credentials"})
		return
	}

	setAuthCookie(w, resp.Session.AccessToken)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-token",
		Value:    token,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 7,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
