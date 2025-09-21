// internal/handlers/http/cors_handler.go
package http

import "net/http"

// PreflightHandler mengembalikan 204 untuk OPTIONS.
func PreflightHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
