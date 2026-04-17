package middleware

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const subdomainKey contextKey = "subdomain"

func SubdomainFromHeader(baseDomain string) func(http.Handler) http.Handler {
	// Strip port from baseDomain for comparison (e.g. "localhost:8080" -> "localhost")
	baseDomainHost := strings.Split(baseDomain, ":")[0]

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sub := r.Header.Get("X-Fader-Subdomain")

			// In local dev the Cloudflare Worker isn't running, so fall back to Host header.
			// e.g. Host: synq.localhost:8080 -> subdomain "synq"
			if sub == "" {
				host := r.Host
				if h, _, err := splitHost(host); err == nil {
					host = h
				}
				if strings.HasSuffix(host, "."+baseDomainHost) {
					sub = strings.TrimSuffix(host, "."+baseDomainHost)
				}
			}

			ctx := context.WithValue(r.Context(), subdomainKey, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func splitHost(hostport string) (host, port string, err error) {
	// net.SplitHostPort but we don't want to import net just for this
	if i := strings.LastIndex(hostport, ":"); i >= 0 {
		return hostport[:i], hostport[i+1:], nil
	}
	return hostport, "", nil
}

func GetSubdomain(r *http.Request) string {
	if v, ok := r.Context().Value(subdomainKey).(string); ok {
		return v
	}
	return ""
}
