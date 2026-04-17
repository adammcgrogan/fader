package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const userIDKey contextKey = "userID"

func RequireAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := extractUserID(r, jwtSecret)
			if !ok {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if id, ok := extractUserID(r, jwtSecret); ok {
				r = r.WithContext(context.WithValue(r.Context(), userIDKey, id))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GetUserID(r *http.Request) (uuid.UUID, bool) {
	v, ok := r.Context().Value(userIDKey).(uuid.UUID)
	return v, ok
}

func extractUserID(r *http.Request, secret string) (uuid.UUID, bool) {
	tokenStr := tokenFromCookieOrHeader(r)
	if tokenStr == "" {
		return uuid.Nil, false
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
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
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
