// middleware/auth.go
// Middleware untuk cek API key

package middleware

import (
	"net/http"
	"os"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		expected := os.Getenv("API_KEY")
		if expected != "" && apiKey != expected {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
