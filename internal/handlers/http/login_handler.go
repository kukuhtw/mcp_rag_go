// internal/handlers/http/login_handler.go
package http

import (
	"encoding/json"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"
	"mcp-oilgas/internal/middleware"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"` // epoch seconds
	User      string `json:"user"`
	Role      string `json:"role"`
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	envUser := os.Getenv("ADMIN_USER")
	envHash := os.Getenv("ADMIN_PASS_HASH")
	if envUser == "" || envHash == "" {
		http.Error(w, "admin not configured", http.StatusForbidden)
		return
	}

	if in.Username != envUser {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(envHash), []byte(in.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, exp, err := middleware.GenerateAdminToken()
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResp{
		Token:     token,
		ExpiresAt: exp,
		User:      envUser,
		Role:      "admin",
	})
}
