// internal/handlers/mcp/rag_search_docs.go
// MCP Tool: rag_search_docs - pencarian dokumen berbasis embedding
// internal/handlers/mcp/rag_search_docs.go
package mcp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"mcp-oilgas/internal/repositories/search"
)

type DocChunk struct {
	DocID   string `json:"doc_id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
	PageNo  int    `json:"page_no"`
	Score   string `json:"score,omitempty"`
}

// ====== GLOBAL (opsional) agar bisa diregister langsung di app.go ======
var DefaultRAGRepo search.RAGRepo

// Setter opsional, bisa dipanggil dari lapisan wiring ketika repo siap
func SetDefaultRAGRepo(r search.RAGRepo) { DefaultRAGRepo = r }

// Handler FUNCTION yang diminta app.go
func RagSearchDocsHandler(w http.ResponseWriter, r *http.Request) {
	if DefaultRAGRepo == nil {
		http.Error(w, "RAG repo not configured", http.StatusServiceUnavailable)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "missing q", http.StatusBadRequest)
		return
	}
	topK := 10
	if s := r.URL.Query().Get("k"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 50 {
			topK = v
		}
	}

	hits, err := DefaultRAGRepo.Retrieve(r.Context(), q, topK)
	if err != nil {
		http.Error(w, "search error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]DocChunk, 0, len(hits))
	for _, hi := range hits {
		resp = append(resp, DocChunk{
			DocID:   hi.DocID,
			Title:   hi.Title,
			Snippet: hi.Snippet,
			URL:     hi.URL,
			PageNo:  hi.Page,
			Score:   strconv.FormatFloat(hi.Score, 'f', 4, 64),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ====== VERSI BERBASIS DEPS (tetap dipertahankan, kalau nanti mau DI/wiring) ======
type RagDeps interface {
	RAG() search.RAGRepo
}
type ragHandler struct{ deps RagDeps }
func NewRagSearchDocsHandler(deps RagDeps) http.Handler { return &ragHandler{deps: deps} }

func (h *ragHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "missing q", http.StatusBadRequest)
		return
	}
	topK := 10
	if s := r.URL.Query().Get("k"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 50 {
			topK = v
		}
	}
	hits, err := h.deps.RAG().Retrieve(r.Context(), q, topK)
	if err != nil {
		http.Error(w, "search error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	resp := make([]DocChunk, 0, len(hits))
	for _, hi := range hits {
		resp = append(resp, DocChunk{
			DocID:   hi.DocID,
			Title:   hi.Title,
			Snippet: hi.Snippet,
			URL:     hi.URL,
			PageNo:  hi.Page,
			Score:   strconv.FormatFloat(hi.Score, 'f', 4, 64),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
