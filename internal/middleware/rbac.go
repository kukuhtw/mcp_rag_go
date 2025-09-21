// middleware/rbac.go
// Middleware RBAC sederhana

package middleware

import (
	"net/http"
)

func RBAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// stub: semua role diizinkan
		// tambahkan rule sesuai kebutuhan
		next.ServeHTTP(w, r)
	})
}
