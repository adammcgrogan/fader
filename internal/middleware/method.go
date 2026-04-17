package middleware

import "net/http"

// MethodOverride allows HTML forms to send DELETE/PUT via _method field.
func MethodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if m := r.FormValue("_method"); m == "DELETE" || m == "PUT" || m == "PATCH" {
				r.Method = m
			}
		}
		next.ServeHTTP(w, r)
	})
}
