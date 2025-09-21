// internal/middleware/admin_jwt.go
package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func AdminJWTAuth(next http.Handler) http.Handler {
	secret := os.Getenv("ADMIN_JWT_SECRET")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			http.Error(w, "admin jwt not configured", http.StatusForbidden)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GenerateAdminToken membuat JWT 24 jam untuk user admin dari ENV
func GenerateAdminToken() (string, int64, error) {
	secret := os.Getenv("ADMIN_JWT_SECRET")
	user := os.Getenv("ADMIN_USER")
	exp := time.Now().Add(24 * time.Hour).Unix()

	claims := jwt.MapClaims{
		"user": user,
		"exp":  exp,
		"role": "admin",
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	return signed, exp, err
}
