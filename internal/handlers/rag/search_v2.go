// internal/handlers/rag/search_v2.go
package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

type HandlerV2 struct {
	RAG *mysqlrepo.RAGRepo
}

type searchV2Req struct {
	Query          string    `json:"query,omitempty"`
	QueryEmbedding []float32 `json:"query_embedding,omitempty"` // biasanya 1536 dim (text-embedding-3-small)
	TopK           int       `json:"top_k,omitempty"`
	Alpha          float64   `json:"alpha,omitempty"` // 0..1
}

type chunkDTO struct {
	DocID   string   `json:"doc_id,omitempty"`
	Title   string   `json:"title,omitempty"`
	URL     string   `json:"url,omitempty"`
	PageNo  *int64   `json:"page_no,omitempty"`
	Snippet string   `json:"snippet,omitempty"`
	Score   *float64 `json:"score,omitempty"` // skor final hybrid
}

type searchV2Resp struct {
	Query           string     `json:"query,omitempty"`
	Alpha           float64    `json:"alpha"`
	Count           int        `json:"count"`
	RetrievedChunks []chunkDTO `json:"retrieved_chunks"`
}

func expectedDim() int {
	if v := strings.TrimSpace(os.Getenv("EMBED_DIM")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 1536 // default text-embedding-3-small
}

func (h *HandlerV2) SearchV2(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.RAG == nil {
		http.Error(w, "rag repo not configured", http.StatusServiceUnavailable)
		return
	}

	var req searchV2Req
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") && r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
	} else {
		// GET fallback (teks saja)
		req.Query = strings.TrimSpace(r.URL.Query().Get("q"))
		if v := r.URL.Query().Get("top_k"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.TopK = n
			}
		}
		if v := r.URL.Query().Get("alpha"); v != "" {
			if a, err := strconv.ParseFloat(v, 64); err == nil {
				req.Alpha = a
			}
		}
	}

	if req.TopK <= 0 || req.TopK > 100 {
		req.TopK = 10
	}
	if req.Alpha < 0 || req.Alpha > 1 {
		req.Alpha = 0.5
	}
	if req.Query == "" && len(req.QueryEmbedding) == 0 {
		http.Error(w, "missing query or query_embedding", http.StatusBadRequest)
		return
	}
	if n := len(req.QueryEmbedding); n > 0 && n != expectedDim() {
		http.Error(w, "query_embedding dimension mismatch", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results, err := h.RAG.SearchHybrid(ctx, req.Query, req.QueryEmbedding, req.Alpha, req.TopK)
	if err != nil {
		http.Error(w, "search error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]chunkDTO, 0, len(results))
	for _, c := range results {
		var pagePtr *int64
		if c.PageNo.Valid {
			p := c.PageNo.Int64
			pagePtr = &p
		}
		var scorePtr *float64
		if c.Score.Valid {
			s := c.Score.Float64
			scorePtr = &s
		}
		out = append(out, chunkDTO{
			DocID:   c.DocID.String,
			Title:   c.Title.String,
			URL:     c.URL.String,
			PageNo:  pagePtr,
			Snippet: c.Snippet.String,
			Score:   scorePtr,
		})
	}

	resp := searchV2Resp{
		Query:           req.Query,
		Alpha:           req.Alpha,
		Count:           len(out),
		RetrievedChunks: out,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
