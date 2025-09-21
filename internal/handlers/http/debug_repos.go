// internal/handlers/http/debug_repos.go
package http

import (
	"encoding/json"
	"net/http"

	mcphandlers "mcp-oilgas/internal/handlers/mcp"
)

func ReposStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mcphandlers.ReposStatus())
}
