package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/validate"
	gotrue "github.com/supabase-community/gotrue-go"
	"github.com/supabase-community/gotrue-go/types"
)

type AuthHandler struct {
	db   *db.Queries
	auth gotrue.Client
}

// CheckHandle returns an HTMX-friendly fragment indicating whether a handle is
// available. Called from the register/settings forms as the user types.
func (h *AuthHandler) CheckHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("handle")))
	if raw == "" {
		w.Write([]byte(`<span class="text-xs text-zinc-600">&nbsp;</span>`))
		return
	}
	if err := validate.Handle(raw); err != nil {
		w.Write([]byte(`<span class="text-xs text-amber-400">` + htmlEscape(err.Error()) + `</span>`))
		return
	}
	exists, err := h.db.HandleExists(r.Context(), raw)
	if err != nil {
		w.Write([]byte(`<span class="text-xs text-zinc-600">&nbsp;</span>`))
		return
	}
	if exists {
		w.Write([]byte(`<span class="text-xs text-red-400">` + htmlEscape(raw) + ` is taken</span>`))
		return
	}
	w.Write([]byte(`<span class="text-xs text-green-400">` + htmlEscape(raw) + `.fader.bio is available</span>`))
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func (h *AuthHandler) ShowForgotPassword(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "auth_forgot_password.html", nil)
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		renderTemplate(w, "auth_forgot_password.html", map[string]any{"Error": "email is required"})
		return
	}
	if err := h.auth.Recover(types.RecoverRequest{Email: email}); err != nil {
		log.Printf("recover error for %s: %v", email, err)
	}
	// Always show success to prevent email enumeration
	renderTemplate(w, "auth_forgot_password.html", map[string]any{"Success": true})
}

func (h *AuthHandler) ShowResetPassword(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "auth_reset_password.html", nil)
}

func (h *AuthHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token := r.FormValue("access_token")
	password := r.FormValue("password")
	if token == "" || len(password) < 8 {
		renderTemplate(w, "auth_reset_password.html", map[string]any{"Error": "password must be at least 8 characters"})
		return
	}
	if _, err := h.auth.WithToken(token).UpdateUser(types.UpdateUserRequest{Password: &password}); err != nil {
		log.Printf("set password error: %v", err)
		renderTemplate(w, "auth_reset_password.html", map[string]any{"Error": "could not update password — the link may have expired"})
		return
	}
	setAuthCookie(w, token)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
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
	if err != nil {
		renderTemplate(w, "auth_register.html", map[string]any{"Error": "database error — have you run migrations?", "Handle": handle})
		return
	}
	if exists {
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
		log.Printf("login error for %s: %v", r.FormValue("email"), err)
		renderTemplate(w, "auth_login.html", map[string]any{"Error": "invalid credentials: " + err.Error()})
		return
	}

	if resp.Session.AccessToken == "" {
		log.Printf("login: empty access token for %s", r.FormValue("email"))
		renderTemplate(w, "auth_login.html", map[string]any{"Error": "login succeeded but no session returned — check email confirmation is disabled in Supabase"})
		return
	}

	log.Printf("login ok for %s, userID=%s", r.FormValue("email"), resp.User.ID)
	setAuthCookie(w, resp.Session.AccessToken)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	secure := !strings.Contains(BaseDomain, "localhost")
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func setAuthCookie(w http.ResponseWriter, token string) {
	secure := !strings.Contains(BaseDomain, "localhost")
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-token",
		Value:    token,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 7,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
