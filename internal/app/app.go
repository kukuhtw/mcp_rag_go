// internal/app/app.go
package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"

	hh "mcp-oilgas/internal/handlers/http"
	mcphandlers "mcp-oilgas/internal/handlers/mcp"
	ragh "mcp-oilgas/internal/handlers/rag" // RAG hybrid (BM25 + cosine)
	"mcp-oilgas/internal/mcp"
	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
	searchrepo "mcp-oilgas/internal/repositories/search"
	"mcp-oilgas/pkg/vector"
)


// App menampung router utama
type App struct {
	Router *mux.Router
}

// New membuat instance App + registrasi semua routes (HTTP & MCP)
func New() *App {
	r := mux.NewRouter()

	// === init DB ===
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = os.Getenv("DB_DSN_DOCKER")
	}

	var (
		db  *sql.DB
		err error
	)

	if dsn != "" {
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Printf("[WARN] open mysql failed: %v", err)
		} else {
			db.SetMaxOpenConns(20)
			db.SetMaxIdleConns(10)
			db.SetConnMaxLifetime(30 * time.Minute)

			// retry ping agar tahan saat container DB baru up
			var pingErr error
			for i := 0; i < 20; i++ {
				pingErr = db.Ping()
				if pingErr == nil {
					break
				}
				log.Printf("[WARN] ping mysql failed (try %d): %v", i+1, pingErr)
				time.Sleep(3 * time.Second)
			}

			if pingErr != nil {
				log.Printf("[ERROR] mysql not ready after retries: %v", pingErr)
			} else {
				// === Inject repos ke handler MCP ===
				poRepo := &mysqlrepo.PORepo{DB: db}

				mcphandlers.SetPORepo(poRepo)        // legacy count-by-status
				mcphandlers.SetPOCompareRepo(poRepo) // vendor_compare mode
				mcphandlers.SetPOStatRepo(poRepo)    // statistik lain (jika dipakai)
				mcphandlers.SetTimeseriesRepo(&mysqlrepo.TimeseriesRepo{DB: db})
				mcphandlers.SetWorkOrderRepo(&mysqlrepo.WorkOrderRepo{DB: db})
				mcphandlers.SetProductionRepo(&mysqlrepo.ProductionRepo{DB: db})
				mcphandlers.SetDrillingRepo(&mysqlrepo.DrillingRepo{DB: db})

				// NEW: untuk tool get_po_top_amount (Top N by amount)
				mcphandlers.SetPOLister(poRepo)
			}
		}
	} else {
		log.Printf("[WARN] DB_DSN/DB_DSN_DOCKER empty; skipping DB init")
	}

	// ==== Inisialisasi RAG repo untuk /ask & SSE (pipeline existing) ====
	var ragRepo searchrepo.RAGRepo
	if db != nil {
		embedClient, e := vector.NewOpenAIClientFromEnv()
		if e != nil {
			log.Printf("[WARN] init embeddings client: %v", e)
		} else {
			ragRepo = searchrepo.NewRAGRepo(db, embedClient, "text-embedding-3-small", 200)
		}
	}
	// share ke SSE handler (opsional)
	hh.SetRAGRepo(ragRepo)

	// ---- HTTP routes (UI/API biasa) ----
	RegisterRoutesWithDeps(r, RegisterDeps{RAGRepo: ragRepo})

	// ---- RAG Hybrid (BM25 + Cosine) terhadap doc_chunks.embedding (JSON) ----
	// Endpoint ini langsung memakai repo MySQL-native tanpa memanggil OpenAI di query-time.
	if db != nil {
		rv2 := &ragh.HandlerV2{RAG: &mysqlrepo.RAGRepo{DB: db}}

		// GET untuk debug (pakai ?q=...), POST untuk payload JSON {query, query_embedding, top_k, alpha}
		r.HandleFunc("/rag/search_v2", rv2.SearchV2).Methods(http.MethodGet, http.MethodPost)

		// Wire "answer_with_docs" agar auto-retrieve via hybrid /rag/search_v2 (in-process)
		mcphandlers.RegisterRetriever(func(ctx context.Context, q string, topK int) ([]mcphandlers.DocChunkRef, error) {
			if topK <= 0 || topK > 50 {
				topK = 10
			}

			payload := map[string]any{
				"query": q,
				"top_k": topK,
				"alpha": 0.6, // BM25:cosine blend; sama seperti yang dipakai di normalizer
			}
			b, _ := json.Marshal(payload)

			// re-use handler SearchV2 in-process (tanpa HTTP nyata)
			req := httptest.NewRequest(http.MethodPost, "/rag/search_v2", bytes.NewReader(b)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			rv2.SearchV2(rr, req)

			if rr.Code >= 400 {
				return nil, fmt.Errorf("rag search_v2 error: %s", rr.Body.String())
			}

			// Bentuk respons SearchV2 -> ubah ke DocChunkRef utk answer_with_docs
			// ✅ Decode ke objek dengan field retrieved_chunks (sesuai output /rag/search_v2)
var resp struct {
    RetrievedChunks []struct {
        DocID   string `json:"doc_id"`
        Title   string `json:"title"`
        URL     string `json:"url"`
        Snippet string `json:"snippet"`
        PageNo  int    `json:"page_no"`
    } `json:"retrieved_chunks"`
}
if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
    return nil, fmt.Errorf("decode rag response: %w; body=%s", err, rr.Body.String())
}

out := make([]mcphandlers.DocChunkRef, 0, len(resp.RetrievedChunks))
for _, h := range resp.RetrievedChunks {
    out = append(out, mcphandlers.DocChunkRef{
        DocID:   h.DocID,
        Title:   h.Title,
        URL:     h.URL,
        Snippet: h.Snippet,
        PageNo:  h.PageNo,
    })
}
return out, nil

		})
	}

	// ---- MCP (Model Context Protocol) ----
	registerMCPTools()

	// Endpoint router MCP (LLM-based intent lama; tetap ada untuk kompatibilitas)
	r.HandleFunc("/mcp/route", mcp.RouterHandler).Methods(http.MethodPost)

	// Endpoint HTTP langsung (opsional, memudahkan debug/manual curl)
	r.HandleFunc("/mcp/get_po_vendor_summary", mcphandlers.GetPOVendorSummaryHandler).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/mcp/get_po_top_amount", mcphandlers.GetPOTopAmountHandler).Methods(http.MethodGet, http.MethodPost) // NEW (opsional)

	return &App{Router: r}
}

// Run menjalankan server HTTP
func (a *App) Run(addr string) {
	log.Printf("server running on %s", addr)
	if err := http.ListenAndServe(addr, a.Router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// ----------------- MCP Wiring -----------------

// registerMCPTools mendaftarkan semua tool MCP ke registry.
func registerMCPTools() {
	// RAG (jawab berbasis dokumen)
	mcp.Register("answer_with_docs", http.HandlerFunc(mcphandlers.AnswerWithDocsHandler))

	// Time series & analitik
	mcp.Register("get_timeseries", http.HandlerFunc(mcphandlers.GetTimeseriesHandler))
	mcp.Register("detect_anomalies_and_correlate", http.HandlerFunc(mcphandlers.DetectAnomaliesHandler))

	// Drilling events
	mcp.Register("get_drilling_events", http.HandlerFunc(mcphandlers.GetDrillingEventsHandler))
	mcp.Register("drilling_events:list", http.HandlerFunc(mcphandlers.GetDrillingEventsHandler)) // alias

	// Domain lain
	mcp.Register("get_po_status", http.HandlerFunc(mcphandlers.GetPOStatusHandler))
	mcp.Register("get_po_vendor_compare", http.HandlerFunc(mcphandlers.GetPOVendorCompareHandler))
	mcp.Register("get_po_vendor_summary", http.HandlerFunc(mcphandlers.GetPOVendorSummaryHandler))
	mcp.Register("get_production", http.HandlerFunc(mcphandlers.GetProductionHandler))
	mcp.Register("search_work_orders", http.HandlerFunc(mcphandlers.SearchWorkOrdersHandler))
	mcp.Register("summarize_npt_events", http.HandlerFunc(mcphandlers.SummarizeNPTEventsHandler))

	// NEW: Top N PO berdasarkan amount
	mcp.Register("get_po_top_amount", http.HandlerFunc(mcphandlers.GetPOTopAmountHandler))
}
