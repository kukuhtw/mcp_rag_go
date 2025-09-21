// internal/server/router.go
package server

import (
	"database/sql"
	"net/http"

	raghandler "mcp-oilgas/internal/handlers/rag"
	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

func NewMux(db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()

	// Healthcheck (biar gampang cek port/path)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// RAG v2 (hybrid)
	ragRepo := &mysqlrepo.RAGRepo{DB: db}
	ragV2 := &raghandler.HandlerV2{RAG: ragRepo}
	mux.HandleFunc("/rag/search_v2", ragV2.SearchV2)

	return mux
}
