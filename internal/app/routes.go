// internal/app/routes.go
/*


*/
package app

import (
	"net/http"

	"github.com/gorilla/mux"
	hh "mcp-oilgas/internal/handlers/http"
	mcphandlers "mcp-oilgas/internal/handlers/mcp"
	"mcp-oilgas/internal/middleware"
	search "mcp-oilgas/internal/repositories/search"
)

type RegisterDeps struct {
	RAGRepo search.RAGRepo
}

// RegisterRoutesWithDeps menambahkan route HTTP biasa (non-MCP).
func RegisterRoutesWithDeps(r *mux.Router, deps RegisterDeps) {
	// --- no prefix ---
	r.HandleFunc("/healthz", hh.HealthHandler).Methods(http.MethodGet)
	r.HandleFunc("/readyz", hh.HealthHandler).Methods(http.MethodGet)
	r.HandleFunc("/metrics", hh.MetricsHandler).Methods(http.MethodGet)
	r.HandleFunc("/chat/stream", hh.ChatSSEHandler).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	r.HandleFunc("/login", hh.LoginHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/debug/repos", hh.ReposStatusHandler).Methods(http.MethodGet)

	// --- /api prefix (supaya FE bisa pakai /api/...) ---
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/healthz", hh.HealthHandler).Methods(http.MethodGet)
	api.HandleFunc("/readyz", hh.HealthHandler).Methods(http.MethodGet)
	api.HandleFunc("/metrics", hh.MetricsHandler).Methods(http.MethodGet)
	api.HandleFunc("/chat/stream", hh.ChatSSEHandler).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	api.HandleFunc("/login", hh.LoginHandler).Methods(http.MethodPost, http.MethodOptions)
    api.HandleFunc("/po/vendor-compare", mcphandlers.GetPOVendorCompareHandler).
    Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	// Orchestrator Q&A
	if deps.RAGRepo != nil {
		api.HandleFunc("/ask", hh.NewAskHandler(hh.AskDeps{RAGRepo: deps.RAGRepo})).
			Methods(http.MethodPost, http.MethodOptions)
	}

	// Domain endpoints (MCP tools exposed via HTTP)
	api.HandleFunc("/drilling-events", mcphandlers.GetDrillingEventsHandler).
		Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	api.HandleFunc("/timeseries", mcphandlers.GetTimeseriesHandler).
		Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	api.HandleFunc("/timeseries/anomalies", mcphandlers.DetectAnomaliesHandler).
		Methods(http.MethodPost, http.MethodOptions)

	api.HandleFunc("/po/status", mcphandlers.GetPOStatusHandler).
		Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	api.HandleFunc("/production", mcphandlers.GetProductionHandler).
		Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	api.HandleFunc("/work-orders/search", mcphandlers.SearchWorkOrdersHandler).
		Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	api.HandleFunc("/npt/summarize", mcphandlers.SummarizeNPTEventsHandler).
		Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	api.HandleFunc("/answer-with-docs", mcphandlers.AnswerWithDocsHandler).
		Methods(http.MethodPost, http.MethodOptions)

	// Preflight catch-all
	api.PathPrefix("/").Methods(http.MethodOptions).HandlerFunc(hh.PreflightHandler)

	// Admin (JWT protected)
	adminJWT := r.PathPrefix("/admin").Subrouter()
	adminJWT.Use(middleware.AdminJWTAuth)
	adminJWT.HandleFunc("/docs", hh.AdminListDocs).Methods(http.MethodGet)
	adminJWT.HandleFunc("/docs/upload", hh.AdminUploadDoc).Methods(http.MethodPost)
}
