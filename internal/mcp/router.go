// internal/mcp/router.go
// Router MCP: menerima request lalu memilih & mengeksekusi tool.

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mcp-oilgas/internal/mcp/llm"
)

// ====== Structured log payload ======

type mcpLog struct {
	At              string `json:"@t,omitempty"`         // RFC3339 timestamp
	Level           string `json:"level,omitempty"`      // info|warn|error
	Event           string `json:"event,omitempty"`      // mcp.route
	RequestID       string `json:"request_id,omitempty"` // X-Request-ID jika ada
	Question        string `json:"question,omitempty"`
	RequestTool     string `json:"request_tool,omitempty"`
	ChosenTool      string `json:"chosen_tool,omitempty"`
	DecisionBy      string `json:"decision_by,omitempty"` // explicit|llm|keyword|default|explicit-plan
	CatalogCount    int    `json:"catalog_count,omitempty"`
	RegisteredCount int    `json:"registered_count,omitempty"`
	HasAPIKey       bool   `json:"has_api_key"`
	DurationMS      int64  `json:"duration_ms,omitempty"`
	Error           string `json:"error,omitempty"`
}

func logJSON(l mcpLog) {
	l.At = time.Now().Format(time.RFC3339Nano)
	if l.Level == "" {
		l.Level = "info"
	}
	b, _ := json.Marshal(l)
	log.Println(string(b))
}

// ====== Multi-route plan support ======
//
// NOTE: Tipe Route & Plan didefinisikan di internal/mcp/plan.go.

var maxRoutes = func() int {
	if v := os.Getenv("PLAN_MAX_ROUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8
}()

// simple recorder untuk menangkap output handler per-route
type respRecorder struct {
	status int
	hdr    http.Header
	buf    []byte
}

func (r *respRecorder) Header() http.Header {
	if r.hdr == nil {
		r.hdr = http.Header{}
	}
	return r.hdr
}
func (r *respRecorder) WriteHeader(code int) { r.status = code }
func (r *respRecorder) Write(b []byte) (int, error) {
	r.buf = append(r.buf, b...)
	return len(b), nil
}

// ====== Regex heuristik khusus ======
//
// rePOCompare: frasa “bandingkan/compare … PO … vendor”
var rePOCompare = regexp.MustCompile(`\b(bandingkan|compare)\b.*\bpo\b.*\b(vendor|halliburton|nov|weatherford)\b`)

// rePONumber: deteksi konteks nomor PO spesifik
var rePONumber = regexp.MustCompile(`\b(po[-_\s]*number|nomor\s*po|po\s*#)\b`)

// ====== Router Handler ======

func RouterHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body error", http.StatusBadRequest)
		logJSON(mcpLog{
			Level:     "error",
			Event:     "mcp.route",
			RequestID: r.Header.Get("X-Request-ID"),
			Error:     fmt.Sprintf("read body: %v", err),
		})
		return
	}
	defer r.Body.Close()

	var req ToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		logJSON(mcpLog{
			Level:     "error",
			Event:     "mcp.route",
			RequestID: r.Header.Get("X-Request-ID"),
			Error:     fmt.Sprintf("unmarshal: %v", err),
		})
		return
	}

	// ===== 0) Jika ada plan.routes (atau routes di root), eksekusi multi-route =====
	var planWrapper struct {
		Plan   *Plan   `json:"plan"`
		Routes []Route `json:"routes"`
	}
	_ = json.Unmarshal(raw, &planWrapper)

	var routes []Route
	if planWrapper.Plan != nil && len(planWrapper.Plan.Routes) > 0 {
		routes = planWrapper.Plan.Routes
	} else if len(planWrapper.Routes) > 0 {
		routes = planWrapper.Routes
	}

	if len(routes) > 0 {
		// === NORMALIZE EXPLICIT PLAN ===
		// Bangun Plan dari payload lalu panggil NormalizePlan agar:
		// - route kind:"rag" -> di-rewrite ke rag_search_v2
		// - fallback detect_anomalies -> RAG jika payload salah & query tampak dokumen
		// - perbaiki kasus "top N PO by amount"
		var p Plan
		if planWrapper.Plan != nil {
			p = *planWrapper.Plan
			// pastikan rute dari planWrapper dipakai
			if len(routes) > 0 {
				p.Routes = routes
			}
		} else {
			p = Plan{Mode: "mcp", Routes: routes}
		}
		// pertanyaan untuk normalizer (ambil dari req.Params/question bila ada)
		qForNorm := extractQuestion(req.Params)
		if strings.TrimSpace(qForNorm) == "" && planWrapper.Plan != nil {
			// fallback lain: gunakan Reason atau query di route pertama kalau ada
			if s := strings.TrimSpace(planWrapper.Plan.Reason); s != "" {
				qForNorm = s
			} else if len(p.Routes) > 0 && strings.TrimSpace(p.Routes[0].Query) != "" {
				qForNorm = p.Routes[0].Query
			}
		}
		p = NormalizePlan(r.Context(), qForNorm, p)

		// batasi jumlah rute setelah normalisasi
		if len(p.Routes) > maxRoutes {
			p.Routes = p.Routes[:maxRoutes]
		}
		routes = p.Routes

		// --- Guardrail tambahan (legacy): betulkan route yang salah pilih tool ---
		normRoutes := make([]Route, 0, len(routes))
		for _, rt := range routes {
			kind := strings.ToLower(strings.TrimSpace(string(rt.Kind)))
			// default mcp jika kosong
			if kind == "" {
				kind = "mcp"
				rt.Kind = RouteKind("mcp")
			}

			// Jika planner keliru pakai get_po_status utk "top amount",
			// deteksi param sort/limit → switch ke get_po_top_amount
			if strings.EqualFold(rt.Tool, "get_po_status") {
				type shallow struct {
					SortBy string `json:"sort_by"`
					Limit  int    `json:"limit"`
					Status string `json:"status"`
					Mode   string `json:"mode"`
				}
				var sp shallow
				_ = json.Unmarshal(rt.Params, &sp)
				if (strings.EqualFold(sp.SortBy, "amount") || sp.Limit > 0) && sp.Status == "" && sp.Mode == "" {
					rt.Tool = "get_po_top_amount"
					if sp.Limit <= 0 {
						sp.Limit = 3
					}
					rt.Params, _ = json.Marshal(map[string]any{"limit": sp.Limit})
				}
			}

			normRoutes = append(normRoutes, rt)
		}

		// Eksekusi semua rute
		results := make([]map[string]any, 0, len(normRoutes))
		for _, rt := range normRoutes {
			kind := strings.ToLower(strings.TrimSpace(string(rt.Kind)))
			switch kind {
			case "mcp":
				h, ok := Get(rt.Tool)
				if !ok {
					results = append(results, map[string]any{
						"route": rt, "error": "tool not found: " + rt.Tool,
					})
					continue
				}

				// forward body = params route (fallback ke raw jika kosong)
				fwd := rt.Params
				if len(fwd) == 0 {
					fwd = raw
				}
				r2 := r.Clone(r.Context())
				r2.Body = io.NopCloser(bytes.NewReader(fwd))
				r2.Header.Set("Content-Type", "application/json") // ensure JSON

				rr := &respRecorder{}
				h.ServeHTTP(rr, r2)

				var out any
				if len(rr.buf) > 0 {
					if err := json.Unmarshal(rr.buf, &out); err != nil {
						out = string(rr.buf) // fallback non-JSON
					}
				}
				status := rr.status
				if status == 0 {
					status = http.StatusOK
				}
				item := map[string]any{
					"route":  rt,
					"status": status,
				}
				if status >= 400 {
					item["error"] = string(rr.buf)
				} else {
					item["result"] = out
				}
				results = append(results, item)

			case "rag":
				// Eksekusi RAG via tool MCP "answer_with_docs"
				h, ok := Get("answer_with_docs")
				if !ok {
					results = append(results, map[string]any{
						"route": rt, "error": "tool not found: answer_with_docs",
					})
					continue
				}

				// TopK defensif
				topk := rt.TopK
				if topk <= 0 || topk > 50 {
					topk = 10
				}

				// --- Ambil optional params untuk meneruskan filters, dsb. ---
				// Skema umum: { "question": string, "top_k": number, "filters": { ... } }
				var pm map[string]any
				if len(rt.Params) > 0 {
					_ = json.Unmarshal(rt.Params, &pm)
				}

				// Gunakan "question", bukan "query"
				payload := map[string]any{
					"question": rt.Query,
					"top_k":    topk,
				}

				// Fallback question dari params jika rt.Query kosong
				if strings.TrimSpace(rt.Query) == "" && pm != nil {
					if qv, ok := pm["question"].(string); ok && strings.TrimSpace(qv) != "" {
						payload["question"] = qv
					} else if qv, ok := pm["query"].(string); ok && strings.TrimSpace(qv) != "" {
						payload["question"] = qv
					}
				}

				// Teruskan filters & opsi lain jika ada
				if pm != nil {
					if f, ok := pm["filters"]; ok && f != nil {
						payload["filters"] = f
					}
					if hl, ok := pm["highlight"]; ok {
						payload["highlight"] = hl
					}
					if lang, ok := pm["lang"]; ok {
						payload["lang"] = lang
					}
				}

				buf, _ := json.Marshal(payload)

				r2 := r.Clone(r.Context())
				r2.Body = io.NopCloser(bytes.NewReader(buf))
				r2.Header.Set("Content-Type", "application/json")

				rr := &respRecorder{}
				h.ServeHTTP(rr, r2)

				var out any
				if len(rr.buf) > 0 {
					if err := json.Unmarshal(rr.buf, &out); err != nil {
						out = string(rr.buf) // fallback non-JSON
					}
				}
				status := rr.status
				if status == 0 {
					status = http.StatusOK
				}
				item := map[string]any{
					"route":  rt,
					"status": status,
				}
				if status >= 400 {
					item["error"] = string(rr.buf)
				} else {
					item["result"] = out
				}
				results = append(results, item)

			default:
				results = append(results, map[string]any{
					"route": rt, "error": "unsupported kind: " + string(rt.Kind),
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode":            "mcp",
			"routes_executed": len(normRoutes),
			"items":           results,
		})

		logJSON(mcpLog{
			Event:      "mcp.route",
			RequestID:  r.Header.Get("X-Request-ID"),
			DecisionBy: "explicit-plan",
			DurationMS: time.Since(start).Milliseconds(),
		})
		return
	}

	// ===== Observability: catalog & registry =====
	defs, _ := LoadToolDefs()
	regNames := List()
	hasAPIKey := os.Getenv("OPENAI_API_KEY") != ""

	// 1) Explicit tool?
	tool := strings.TrimSpace(req.Tool)
	decision := "explicit"

	// 2) LLM choose if empty (dengan heuristik cepat sebelum LLM)
	var question string
	if tool == "" {
		decision = ""
		question = extractQuestion(req.Params)
		if q := strings.TrimSpace(question); q != "" {
			// Heuristik cepat (deterministik untuk pola populer)
			qq := strings.ToLower(q)
			if rePOCompare.MatchString(qq) {
				tool = "get_po_vendor_compare"
				decision = "keyword"
			}
		}
		// Jika belum terpilih oleh heuristik, baru coba LLM
		if tool == "" && strings.TrimSpace(question) != "" {
			if chosen := chooseToolWithLLM(r.Context(), question); chosen != "" {
				tool = chosen
				decision = "llm"
			}
		}
	}

	// 3) Keyword fallback (agar tetap jalan tanpa LLM)
	if tool == "" {
		q := strings.ToLower(question)

		switch {
		case strings.Contains(q, "timeseries") || strings.Contains(q, "grafik") || strings.Contains(q, "trend"):
			tool = "get_timeseries"

		case strings.Contains(q, "drilling") || strings.Contains(q, "npt"):
			tool = "get_drilling_events"

		case rePOCompare.MatchString(q):
			tool = "get_po_vendor_compare"

		// FIXED: Improved top amount detection
		case (strings.Contains(q, "tertinggi") || strings.Contains(q, "top")) &&
			(strings.Contains(q, "po") || strings.Contains(q, "purchase order")) &&
			(strings.Contains(q, "amount") || strings.Contains(q, "nilai")):
			tool = "get_po_top_amount"

		// Also catch numeric patterns like "3 po" with "tertinggi"
		case strings.Contains(q, "po") &&
			(strings.Contains(q, "tertinggi") || strings.Contains(q, "top")) &&
			(strings.Contains(q, "sebutkan") || strings.Contains(q, "ambil") || strings.Contains(q, "cari")):
			tool = "get_po_top_amount"

		// nomor PO spesifik → status 1 PO (legacy)
		case strings.Contains(q, "po ") && rePONumber.MatchString(q):
			tool = "get_po_status"

		// vendor paling banyak / top vendor
		case (strings.Contains(q, "vendor") &&
			(strings.Contains(q, "paling banyak") || strings.Contains(q, "terbanyak") || strings.Contains(q, "top"))) ||
			strings.Contains(q, "top vendor"):
			tool = "get_po_vendor_summary"

		case strings.Contains(q, "produksi") || strings.Contains(q, "production"):
			tool = "get_production"

		case strings.Contains(q, "work order") || strings.Contains(q, "wo "):
			tool = "search_work_orders"
		}

		if tool != "" && decision == "" {
			decision = "keyword"
		}
	}

	// 4) Default final
	if tool == "" {
		tool = "answer_with_docs"
		if decision == "" {
			decision = "default"
		}
	}

	// 4.5) Enrich params untuk tool tertentu (default & auto-extract)
	{
		// Ambil params sebagai map
		var pm map[string]any
		switch p := req.Params.(type) {
		case map[string]any:
			pm = p
		case json.RawMessage:
			if len(p) > 0 {
				_ = json.Unmarshal(p, &pm)
			}
		default:
			// noop
		}
		if pm == nil {
			pm = map[string]any{}
		}

		// Deteksi status dari pertanyaan (kalau ada)
		detectedStatus := detectStatusFromText(question)

		switch tool {
		case "get_po_vendor_summary":
			if _, ok := pm["status"]; !ok && detectedStatus != "" {
				pm["status"] = detectedStatus
			}
			if _, ok := pm["limit"]; !ok {
				pm["limit"] = 5
			}
		case "get_po_top_amount":
			if _, ok := pm["limit"]; !ok {
				pm["limit"] = 3
			}
		}

		// update kembali ke req.Params
		req.Params = pm
	}

	// 5) Execute (single tool, kompatibel lama)
	h, ok := Get(tool)
	if !ok {
		resp := ToolResponse{Success: false, Error: "tool not found: " + tool}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)

		logJSON(mcpLog{
			Level:           "warn",
			Event:           "mcp.route",
			RequestID:       r.Header.Get("X-Request-ID"),
			Question:        extractQuestion(req.Params),
			RequestTool:     req.Tool,
			ChosenTool:      tool,
			DecisionBy:      decision,
			CatalogCount:    len(defs),
			RegisteredCount: len(regNames),
			HasAPIKey:       hasAPIKey,
			DurationMS:      time.Since(start).Milliseconds(),
			Error:           "tool not found",
		})
		return
	}

	// Forward: handler menerima hanya Params JSON (tanpa envelope)
	forward := raw
	if req.Params != nil {
		if buf, err := json.Marshal(req.Params); err == nil {
			forward = buf
		}
	}
	r2 := r.Clone(r.Context())
	r2.Body = io.NopCloser(bytes.NewReader(forward))
	r2.Header.Set("Content-Type", "application/json") // ensure JSON

	// Eksekusi handler
	h.ServeHTTP(w, r2)

	// success log (ukur durasi SETELAH handler selesai)
	logJSON(mcpLog{
		Event:           "mcp.route",
		RequestID:       r.Header.Get("X-Request-ID"),
		Question:        extractQuestion(req.Params),
		RequestTool:     req.Tool,
		ChosenTool:      tool,
		DecisionBy:      decision,
		CatalogCount:    len(defs),
		RegisteredCount: len(regNames),
		HasAPIKey:       hasAPIKey,
		DurationMS:      time.Since(start).Milliseconds(),
	})
}

// ====== Status detector (simple) ======
func detectStatusFromText(s string) string {
	qq := strings.ToLower(s)
	cands := []string{"in_transit", "delivered", "approved", "created", "shipped", "closed", "cancelled"}
	for _, c := range cands {
		if strings.Contains(qq, c) {
			return c
		}
	}
	// varian umum lain
	alts := map[string]string{
		"in transit": "in_transit",
		"on transit": "in_transit",
		"in-transit": "in_transit",
	}
	for k, v := range alts {
		if strings.Contains(qq, k) {
			return v
		}
	}
	return ""
}

// ====== Chooser helpers ======

func extractQuestion(params interface{}) string {
	if params == nil {
		return ""
	}
	// Jika ToolRequest.Params bertipe map[string]any
	if m, ok := params.(map[string]interface{}); ok {
		if q, ok := m["question"].(string); ok {
			return q
		}
	}
	// Jika suatu saat diubah ke json.RawMessage
	if raw, ok := params.(json.RawMessage); ok && len(raw) > 0 {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err == nil {
			if q, ok := m["question"].(string); ok {
				return q
			}
		}
	}
	return ""
}

func chooseToolWithLLM(ctx context.Context, question string) string {
	defs, err := LoadToolDefs()
	if err != nil || len(defs) == 0 {
		return ""
	}

	// Filter hanya tool yang terdaftar di registry runtime
	regNames := map[string]struct{}{}
	for _, name := range List() {
		regNames[strings.ToLower(name)] = struct{}{}
	}
	var filtered []ToolDef
	for _, d := range defs {
		if _, ok := regNames[strings.ToLower(d.Name)]; ok {
			filtered = append(filtered, d)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		return ""
	}

	client, err := llm.NewFromEnv()
	if err != nil {
		return ""
	}

	system := llmSystemPromptID()
	user := buildChooserUserPrompt(question, filtered)

	// Timeout singkat agar responsif
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
	}

	out, err := client.AnswerWithRAG(ctx, system, user)
	if err != nil {
		return ""
	}

	out = sanitizeToolToken(strings.TrimSpace(out))
	for _, d := range filtered {
		if strings.EqualFold(out, d.Name) {
			return d.Name
		}
	}
	return ""
}

func llmSystemPromptID() string {
	return `Anda adalah agen router.
- Pilih tepat SATU nama tool dari daftar.
- Balas hanya dengan nama tool (misal: get_timeseries).
- Jika ragu, pilih "answer_with_docs".`
}

func buildChooserUserPrompt(question string, defs []ToolDef) string {
	var b strings.Builder
	b.WriteString("Pertanyaan user:\n")
	b.WriteString(question)
	b.WriteString("\n\nDaftar tool tersedia:\n")
	for i, d := range defs {
		desc := strings.TrimSpace(d.Description)
		if len(desc) > 300 {
			desc = desc[:300] + "…"
		}
		b.WriteString(fmt.Sprintf("%d) %s — %s\n", i+1, d.Name, desc))
	}
	b.WriteString("\nBalas hanya dengan nama tool.")
	return b.String()
}

var nonWord = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

func sanitizeToolToken(s string) string {
	s = strings.TrimSpace(s)
	s = nonWord.ReplaceAllString(s, "")
	return strings.ToLower(s)
}
