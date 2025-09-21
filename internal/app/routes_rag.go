// internal/app/routes_rag.go
package app

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	raghandler "mcp-oilgas/internal/handlers/rag"
	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

// RegisterRAGRouters dipanggil dari New() setelah router & DB siap.
func RegisterRAGRouters(r chi.Router, db *sql.DB) {
	ragRepo := &mysqlrepo.RAGRepo{DB: db}
	ragV2 := &raghandler.HandlerV2{RAG: ragRepo}

	r.Route("/rag", func(cr chi.Router) {
		// GET untuk debug manual di browser (pakai ?q=...), POST untuk payload JSON
		cr.Get("/search_v2", ragV2.SearchV2)
		cr.Post("/search_v2", ragV2.SearchV2)
	})
}
