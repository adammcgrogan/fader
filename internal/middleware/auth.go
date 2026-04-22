package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	gotrue "github.com/supabase-community/gotrue-go"
)

const userIDKey contextKey = "userID"

type AuthMiddleware struct {
	jwks       keyfunc.Keyfunc
	auth       gotrue.Client
	baseDomain string
}

func NewAuthMiddleware(supabaseURL string, auth gotrue.Client, baseDomain string) (*AuthMiddleware, error) {
	jwksURL := strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"
	k, err := keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
	}
	log.Printf("auth: loaded JWKS from %s", jwksURL)
	return &AuthMiddleware{jwks: k, auth: auth, baseDomain: baseDomain}, nil
}

func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := a.extractUserID(w, r)
		if !ok {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := a.extractUserID(w, r); ok {
			r = r.WithContext(context.WithValue(r.Context(), userIDKey, id))
		}
		next.ServeHTTP(w, r)
	})
}

func GetUserID(r *http.Request) (uuid.UUID, bool) {
	v, ok := r.Context().Value(userIDKey).(uuid.UUID)
	return v, ok
}

func (a *AuthMiddleware) extractUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	tokenStr := tokenFromCookieOrHeader(r)
	if tokenStr == "" {
		return uuid.Nil, false
	}

	token, err := jwt.Parse(tokenStr, a.jwks.Keyfunc, jwt.WithLeeway(30*time.Second))
	if err == nil && token.Valid {
		return userIDFromClaims(token)
	}

	// Token invalid for a reason other than expiry — clear cookies and bail.
	if !isExpiredError(err) {
		a.clearAuthCookies(w)
		return uuid.Nil, false
	}

	// Token is expired — try to refresh silently.
	refreshCookie, cookieErr := r.Cookie("sb-refresh")
	if cookieErr != nil {
		a.clearAuthCookies(w)
		return uuid.Nil, false
	}

	resp, refreshErr := a.auth.RefreshToken(refreshCookie.Value)
	if refreshErr != nil {
		log.Printf("auth: refresh failed: %v", refreshErr)
		a.clearAuthCookies(w)
		return uuid.Nil, false
	}

	a.setAuthCookies(w, resp.AccessToken, resp.RefreshToken)

	newToken, parseErr := jwt.Parse(resp.AccessToken, a.jwks.Keyfunc, jwt.WithLeeway(30*time.Second))
	if parseErr != nil || !newToken.Valid {
		return uuid.Nil, false
	}
	return userIDFromClaims(newToken)
}

func userIDFromClaims(token *jwt.Token) (uuid.UUID, bool) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, false
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func isExpiredError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "token is expired")
}

func (a *AuthMiddleware) cookieDomain() string {
	host := strings.Split(a.baseDomain, ":")[0]
	if host == "localhost" {
		return ""
	}
	return "." + host
}

func (a *AuthMiddleware) secure() bool {
	return !strings.Contains(a.baseDomain, "localhost")
}

func (a *AuthMiddleware) setAuthCookies(w http.ResponseWriter, access, refresh string) {
	domain := a.cookieDomain()
	secure := a.secure()
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-token",
		Value:    access,
		Path:     "/",
		Domain:   domain,
		MaxAge:   60 * 60 * 24 * 7,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-refresh",
		Value:    refresh,
		Path:     "/",
		Domain:   domain,
		MaxAge:   60 * 60 * 24 * 30,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *AuthMiddleware) clearAuthCookies(w http.ResponseWriter) {
	domain := a.cookieDomain()
	secure := a.secure()
	for _, name := range []string{"sb-token", "sb-refresh"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   domain,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func tokenFromCookieOrHeader(r *http.Request) string {
	if c, err := r.Cookie("sb-token"); err == nil {
		return c.Value
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// RequireAuth and OptionalAuth as standalone funcs kept for backwards compat.
func RequireAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}
func OptionalAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}
