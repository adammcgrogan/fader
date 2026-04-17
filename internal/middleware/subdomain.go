package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const subdomainKey contextKey = "subdomain"

func SubdomainFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub := r.Header.Get("X-Fader-Subdomain")
		ctx := context.WithValue(r.Context(), subdomainKey, sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetSubdomain(r *http.Request) string {
	if v, ok := r.Context().Value(subdomainKey).(string); ok {
		return v
	}
	return ""
}
