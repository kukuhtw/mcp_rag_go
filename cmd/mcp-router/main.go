// cmd/mcp-router/main.go
package main

import (
	"log"
	"net/http"
	"os"

	"mcp-oilgas/internal/mcp"
)

func main() {
	port := getenv("MCP_PORT", "8090")
	http.HandleFunc("/route", mcp.RouterHandler)
	log.Printf("MCP Router listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
