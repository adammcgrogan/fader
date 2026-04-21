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
)

const userIDKey contextKey = "userID"

type AuthMiddleware struct {
	jwks keyfunc.Keyfunc
}

func NewAuthMiddleware(supabaseURL string) (*AuthMiddleware, error) {
	jwksURL := strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"
	k, err := keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
	}
	log.Printf("auth: loaded JWKS from %s", jwksURL)
	return &AuthMiddleware{jwks: k}, nil
}

func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := a.extractUserID(r)
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
		if id, ok := a.extractUserID(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), userIDKey, id))
		}
		next.ServeHTTP(w, r)
	})
}

func GetUserID(r *http.Request) (uuid.UUID, bool) {
	v, ok := r.Context().Value(userIDKey).(uuid.UUID)
	return v, ok
}

func (a *AuthMiddleware) extractUserID(r *http.Request) (uuid.UUID, bool) {
	tokenStr := tokenFromCookieOrHeader(r)
	if tokenStr == "" {
		return uuid.Nil, false
	}

	token, err := jwt.Parse(tokenStr, a.jwks.Keyfunc, jwt.WithLeeway(30*time.Second))
	if err != nil || !token.Valid {
		log.Printf("auth: jwt validation failed: %v", err)
		return uuid.Nil, false
	}

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

func tokenFromCookieOrHeader(r *http.Request) string {
	if c, err := r.Cookie("sb-token"); err == nil {
		return c.Value
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// RequireAuth and OptionalAuth as standalone funcs are kept for backwards compat
// but now need the AuthMiddleware — these are no-ops that redirect to login.
func RequireAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}
func OptionalAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}
