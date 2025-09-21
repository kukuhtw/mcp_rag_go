// internal/middleware/admin_auth.go
package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func AdminBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := os.Getenv("ADMIN_USER")
		hash := os.Getenv("ADMIN_PASS_HASH")
		if user == "" || hash == "" {
			http.Error(w, "admin auth not configured", http.StatusForbidden)
			return
		}
		h := r.Header.Get("Authorization")
		const pfx = "Basic "
		if len(h) < len(pfx) || h[:len(pfx)] != pfx {
			w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
			http.Error(w, "auth required", http.StatusUnauthorized)
			return
		}
		raw, _ := base64.StdEncoding.DecodeString(h[len(pfx):])
		// raw = "username:password"
		var u, p string
		for i := range raw {
			if raw[i] == ':' {
				u = string(raw[:i])
				p = string(raw[i+1:])
				break
			}
		}
		if subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
