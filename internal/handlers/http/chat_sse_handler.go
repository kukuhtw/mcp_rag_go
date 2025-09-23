// internal/handlers/http/chat_sse_handler.go
package http

import (
	"bytes" // ← untuk hybrid RAG call
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"mcp-oilgas/internal/config"
	mcps "mcp-oilgas/internal/mcp"
	"mcp-oilgas/internal/mcp/llm"
	search "mcp-oilgas/internal/repositories/search"
)

// ----------------- Wiring RAGRepo -----------------
var ragRepo search.RAGRepo

// SetRAGRepo dipanggil dari app.go setelah RAGRepo siap (DB & embeddings client OK).
func SetRAGRepo(r search.RAGRepo) { ragRepo = r }

// ----------------- Request Models -----------------
type sseAskRequest struct {
	Question string                 `json:"question"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// ----------------- Helpers -----------------
func setSSEHeaders(w http.ResponseWriter) (http.Flusher, bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return nil, false
	}
	return flusher, true
}

func sseEvent(w http.ResponseWriter, flusher http.Flusher, event string, v any) {
	var data string
	switch t := v.(type) {
	case string:
		data = t
	default:
		b, _ := json.Marshal(v)
		data = string(b)
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// Heuristik ringan untuk mendeteksi bahasa Inggris
func looksEnglish(s string) bool {
	ls := strings.ToLower(s)
	enHint := []string{
		"the", "and", "of", "for", "what", "how", "why", "please", "show",
		"compare", "top", "amount", "vendor", "status", "drilling", "timeseries",
	}
	for _, w := range enHint {
		if strings.Contains(ls, w) {
			return true
		}
	}
	// fallback berdasarkan rasio huruf latin
	letters := 0
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			letters++
		}
	}
	return letters >= int(float64(len(s))*0.35)
}

func systemPromptByLang(lang string) string {
	if lang == "en" {
		return `You are a technical assistant.
- Use the "sources" data to answer the question.
- Be concise and accurate; call out key numbers and conclusions.
- If data is insufficient, state the limitation.
- Write in natural English for the user.
- If time series or tabular data is present, the client UI will render charts/tables in a separate section; you don't need to reformat them.`
	}
	// default: Indonesian
	return `Anda adalah asisten teknis.
- Gunakan data pada "sources" untuk menjawab pertanyaan.
- Tulis ringkas, akurat, sebutkan angka/kesimpulan penting.
- Jika data kurang, sebutkan keterbatasannya.
- Balas dengan bahasa Indonesia yang alami.
- Jika ada time series atau tabel, UI klien akan menampilkan chart/tabel di panel terpisah; Anda tidak perlu memformat ulang.`
}

// ----------------- Handler -----------------

// ChatSSEHandler: Orkestrasi (Planner LLM → Eksekusi Routes MCP/RAG → Synth LLM Streaming).
func ChatSSEHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := setSSEHeaders(w)
	if !ok {
		return
	}

	// Identitas server per request (untuk verifikasi biner/instance aktif)
	sseEvent(w, flusher, "server_info", map[string]any{
		"build": config.BuildVersion,
		"pid":   os.Getpid(),
		"time":  time.Now().Format(time.RFC3339),
	})

	// 1) Ambil pertanyaan (GET q=... atau POST {question,...})
	var body sseAskRequest
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" && r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&body)
		q = strings.TrimSpace(body.Question)
	}
	if q == "" {
		sseEvent(w, flusher, "error", `{"message":"question required"}`)
		return
	}

	// --- NEW: tentukan language ---
	lang := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang")))
	if lang == "" && body.Params != nil {
		if v, ok := body.Params["lang"].(string); ok {
			lang = strings.ToLower(strings.TrimSpace(v))
		}
	}
	if lang != "en" && lang != "id" {
		// heuristik ringan
		if looksEnglish(q) {
			lang = "en"
		} else {
			lang = "id"
		}
	}
	sseEvent(w, flusher, "meta", map[string]string{"lang": lang})

	// 2) Init LLM + Planner
	client, err := llm.NewFromEnv()
	if err != nil {
		sseEvent(w, flusher, "error", map[string]string{"message": "LLM init error: " + err.Error()})
		return
	}
	planner, err := llm.NewRoutePlannerFromEnv()
	if err != nil {
		sseEvent(w, flusher, "warn", map[string]string{"message": "Planner init failed, fallback RAG"})
	}

	// Deadlines
	ctx := r.Context()
	if _, has := ctx.Deadline(); !has {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(r.Context(), 75*time.Second)
		defer cancel()
	}

	// 3) Planning
	sseEvent(w, flusher, "phase", `"plan_start"`)

	// default fallback plan = RAG
	plan := mcps.Plan{
		Mode:     "rag",
		Fallback: true,
		Routes:   []mcps.Route{{Kind: mcps.RouteRAG, Query: q, TopK: 10}},
		Reason:   "default fallback",
	}

	// gunakan planner berbasis SCHEMA (bukan katalog statik)
	schemaDir := os.Getenv("MCP_SCHEMAS_DIR")
	if schemaDir == "" {
		schemaDir = "schemas/mcp"
	}
	sseEvent(w, flusher, "planner_info", map[string]any{
		"schema_dir": schemaDir,
	})

	// Muat tools dari folder schema dan expose ke FE
	if planner != nil {
		tools, loadErr := llm.LoadToolsFromSchemaDir(schemaDir)
		if loadErr != nil {
			log.Println("[planner] LoadToolsFromSchemaDir error:", loadErr)
			sseEvent(w, flusher, "warn", map[string]any{
				"message": "planner load error",
				"dir":     schemaDir,
				"error":   loadErr.Error(),
			})
		} else {
			names := make([]string, 0, len(tools))
			for _, t := range tools {
				names = append(names, t.Name)
			}
			sseEvent(w, flusher, "planner_tools", map[string]any{
				"dir":   schemaDir,
				"count": len(tools),
				"names": names,
			})

			// Panggil planner JSON-mode dengan tools yang sudah dimuat
			raw, perr := planner.PlanRaw(ctx, tools, q)
			if perr != nil {
				log.Println("[planner] AnswerJSON error:", perr)
				sseEvent(w, flusher, "warn", map[string]any{
					"message": "planner AnswerJSON error",
					"error":   perr.Error(),
				})
			} else if strings.TrimSpace(raw) == "" {
				log.Println("[planner] empty JSON returned")
				sseEvent(w, flusher, "warn", map[string]any{
					"message": "planner returned empty JSON",
				})
			} else {
				var p mcps.Plan
				if uerr := json.Unmarshal([]byte(raw), &p); uerr != nil || len(p.Routes) == 0 {
					log.Println("[planner] unmarshal fail or no routes; raw:", raw, "err:", uerr)
					sseEvent(w, flusher, "warn", map[string]any{
						"message": "planner unmarshal fail or no routes",
						"raw":     raw,
						"error":   fmt.Sprintf("%v", uerr),
					})
				} else {
					plan = p
				}
			}
		}
	} // planner nil → sudah dikirim warn di atas

	plan = mcps.NormalizePlan(ctx, q, plan)
	// Expose rencana ke FE
	sseEvent(w, flusher, "plan", plan)

	// 4) Eksekusi Routes
	sseEvent(w, flusher, "phase", `"exec_start"`)

	ragFn := func(ctx context.Context, query string, topK int) ([]map[string]any, error) {
		// 1) Coba pakai hybrid endpoint /rag/search_v2 (BM25+cosine) – tidak butuh OpenAI di query-time
		payload := map[string]any{
			"query": query,
			"top_k": topK,
			"alpha": 0.6,
		}
		b, _ := json.Marshal(payload)

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/rag/search_v2", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")

		if resp, err := http.DefaultClient.Do(req); err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()
			var r struct {
				RetrievedChunks []struct {
					DocID   string `json:"doc_id"`
					Title   string `json:"title"`
					URL     string `json:"url"`
					Snippet string `json:"snippet"`
					PageNo  int    `json:"page_no"`
					Score   any    `json:"score"`
				} `json:"retrieved_chunks"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
				return nil, fmt.Errorf("decode rag response: %w", err)
			}
			out := make([]map[string]any, 0, len(r.RetrievedChunks))
			for _, h := range r.RetrievedChunks {
				out = append(out, map[string]any{
					"doc_id":  h.DocID,
					"title":   h.Title,
					"url":     h.URL,
					"snippet": h.Snippet,
					"page_no": h.PageNo, // ← penting: page_no (bukan "page")
					"score":   h.Score,
				})
			}
			return out, nil
		}

		// 2) Fallback: kalau hybrid gagal, coba repo embeddings (kalau ada)
		if ragRepo == nil {
			return nil, fmt.Errorf("RAG hybrid & embeddings repo unavailable")
		}
		hits, err := ragRepo.Retrieve(ctx, query, topK)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(hits))
		for _, h := range hits {
			out = append(out, map[string]any{
				"doc_id":  h.DocID,
				"title":   h.Title,
				"url":     h.URL,
				"snippet": h.Snippet,
				"page_no": h.Page, // konsistenkan ke page_no
				"score":   h.Score,
			})
		}
		return out, nil
	}

	sources, _ := mcps.ExecuteRoutes(ctx, plan.Routes, ragFn)
	sseEvent(w, flusher, "sources", sources)
	sseEvent(w, flusher, "phase", `"exec_done"`)

	// 5) Synthesizer (Streaming)
	sys := systemPromptByLang(lang)

	payload := struct {
		Question string            `json:"question"`
		Sources  []mcps.ExecResult `json:"sources"`
	}{
		Question: q,
		Sources:  sources,
	}
	pb, _ := json.Marshal(payload)

	sseEvent(w, flusher, "phase", `"llm_start"`)

	final, err := client.AnswerStream(ctx, sys, string(pb), func(delta string) error {
		sseEvent(w, flusher, "delta", map[string]string{"delta": delta})
		return nil
	})
	if err != nil {
		sseEvent(w, flusher, "error", map[string]string{"message": "stream error: " + err.Error()})
		return
	}

	// 6) Selesai
	sseEvent(w, flusher, "done", map[string]string{"final": final})
	time.Sleep(50 * time.Millisecond)
}
