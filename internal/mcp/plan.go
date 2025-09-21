// internal/mcp/plan.go
package mcp

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

type RouteKind string

const (
	RouteMCP RouteKind = "mcp"
	RouteRAG RouteKind = "rag"
)

type Route struct {
	Kind     RouteKind       `json:"kind"`               // "mcp" | "rag"
	Tool     string          `json:"tool,omitempty"`     // utk MCP atau nama tool RAG
	Endpoint string          `json:"endpoint,omitempty"` // opsional: hint endpoint HTTP
	Params   json.RawMessage `json:"params,omitempty"`   // payload JSON utk handler tool (RAW)
	Query    string          `json:"query,omitempty"`    // utk RAG
	TopK     int             `json:"top_k,omitempty"`    // utk RAG
}

type Plan struct {
	Mode     string  `json:"mode"`               // "mcp" | "rag" | "hybrid"
	Routes   []Route `json:"routes"`             // bisa banyak endpoint
	Reason   string  `json:"reason,omitempty"`   // penjelasan singkat
	Fallback bool    `json:"fallback,omitempty"` // true jika fallback
}

type Planner interface {
	Plan(ctx context.Context, question string) (Plan, error)
}

// ------------------------------
// Plan Normalizer & Helpers
// ------------------------------

// NormalizePlan memastikan rute yang dihasilkan LLM/planner benar.
// a) Normalisasi RAG: mode "rag" atau route RAG lama → rewrite ke rag_search_v2
//    dan fallback dari detect_anomalies jika payload tidak valid tapi pertanyaannya dokumen.
// b) Perbaiki kasus "top N PO by amount": tambahkan/benarkan get_po_top_amount.
// c) Tambahkan RAG pendukung bila relevan.
func NormalizePlan(ctx context.Context, question string, p Plan) Plan {
	qLower := strings.ToLower(strings.TrimSpace(question))

	// ---------- (a) Normalisasi RAG ----------
	// - Semua route Kind=RAG (tool lama "rag") → gunakan "rag_search_v2" + body {query, top_k, alpha}.
	// - Jika ada tool detect_anomalies tapi payload tidak valid dan query terlihat kueri dokumen → fallback ke RAG.
	for i := range p.Routes {
		r := &p.Routes[i]

		// 1) Rewrite semua RouteRAG -> rag_search_v2
		if r.Kind == RouteRAG {
			q := strings.TrimSpace(r.Query)
			if q == "" {
				var tmp struct{ Query string `json:"query"` }
				_ = json.Unmarshal(r.Params, &tmp)
				q = strings.TrimSpace(tmp.Query)
				if q == "" {
					q = question // fallback
				}
			}
			body := map[string]any{
				"query": q,
				"top_k": pickTopK(r.TopK, 10),
				"alpha": 0.6,
			}
			b, _ := json.Marshal(body)
			r.Tool = "rag_search_v2"
			r.Params = b
			r.Endpoint = "/rag/search_v2" // hint, executor HTTP boleh pakai ini
			// r.Kind tetap RouteRAG
			continue
		}

		// 2) Fallback dari detect_anomalies → RAG jika payload tidak valid dan pertanyaannya seperti dokumen
		if strings.EqualFold(strings.TrimSpace(r.Tool), "detect_anomalies") && !looksLikeDetectAnomaliesPayload(r.Params) {
			// cari query kandidat
			q := strings.TrimSpace(r.Query)
			if q == "" {
				var tmp struct{ Query string `json:"query"` }
				_ = json.Unmarshal(r.Params, &tmp)
				q = strings.TrimSpace(tmp.Query)
				if q == "" {
					q = question
				}
			}
			if looksLikeDocQuery(q) {
				body := map[string]any{
					"query": q,
					"top_k": pickTopK(r.TopK, 10),
					"alpha": 0.6,
				}
				b, _ := json.Marshal(body)
				r.Kind = RouteRAG
				r.Tool = "rag_search_v2"
				r.Params = b
				r.Endpoint = "/rag/search_v2"
				continue
			}
		}
	}

	// ---------- (b) Kasus "top N PO by amount" ----------
	wantTopAmount := isAskingTopPOAmount(qLower)
	limit := extractFirstInt(qLower)
	if limit <= 0 {
		limit = 3
	}

	// Apakah sudah ada get_po_top_amount?
	hasTopAmount := false
	for _, r := range p.Routes {
		if r.Kind == RouteMCP && r.Tool == "get_po_top_amount" {
			hasTopAmount = true
			break
		}
	}

	var newRoutes []Route
	newRoutes = append(newRoutes, p.Routes...)

	// Perbaiki rute yang salah: get_po_status dipakai untuk top amount → tambahkan get_po_top_amount yang benar
	addedFromFix := false
	for _, r := range p.Routes {
		if r.Kind == RouteMCP && r.Tool == "get_po_status" && wantTopAmount {
			params := pickTopAmountParams(r.Params, limit)
			newRoutes = append(newRoutes, Route{
				Kind:   RouteMCP,
				Tool:   "get_po_top_amount",
				Params: params,
			})
			hasTopAmount = true
			addedFromFix = true
			break
		}
	}

	// Jika belum ada rute top_amount padahal pertanyaan minta itu, tambahkan default
	if wantTopAmount && !hasTopAmount {
		params := buildTopAmountParams(nil, limit)
		newRoutes = append(newRoutes, Route{
			Kind:   RouteMCP,
			Tool:   "get_po_top_amount",
			Params: params,
		})
		addedFromFix = true
	}

	// (opsional) Tambahkan RAG pendukung jika tidak ada RAG sama sekali dan konteksnya top amount
	if wantTopAmount && !hasRAG(newRoutes) {
		newRoutes = append(newRoutes, Route{
			Kind:     RouteRAG,
			Tool:     "rag_search_v2",
			Endpoint: "/rag/search_v2",
			Query:    "Sebutkan vendor, status, dan ETA dari PO dengan nilai (amount) tertinggi.",
			TopK:     10,
			Params:   mustJSON(map[string]any{"query": "Sebutkan vendor, status, dan ETA dari PO dengan nilai (amount) tertinggi.", "top_k": 10, "alpha": 0.6}),
		})
	}

	if addedFromFix && strings.ToLower(p.Mode) != "hybrid" {
		p.Mode = "hybrid"
	}

	p.Routes = newRoutes
	if p.Reason == "" && wantTopAmount {
		p.Reason = "Menormalkan rute: gunakan get_po_top_amount untuk top-N PO berdasarkan amount."
	}
	return p
}

func hasRAG(routes []Route) bool {
	for _, r := range routes {
		if r.Kind == RouteRAG {
			return true
		}
	}
	return false
}

// pickTopAmountParams: baca params lama (RawMessage) dan ambil field yang relevan
// untuk get_po_top_amount: limit (override), statuses, vendor, days_back, currency.
func pickTopAmountParams(raw json.RawMessage, limit int) json.RawMessage {
	m := map[string]any{}
	if len(strings.TrimSpace(string(raw))) > 0 && string(raw) != "null" {
		_ = json.Unmarshal(raw, &m)
	}
	out := map[string]any{
		"limit":    limit,
		"currency": firstOr(m["currency"], "USD"),
	}
	if v, ok := m["statuses"]; ok {
		switch vv := v.(type) {
		case []any:
			var ss []string
			for _, x := range vv {
				if s, ok2 := x.(string); ok2 && strings.TrimSpace(s) != "" {
					ss = append(ss, s)
				}
			}
			if len(ss) > 0 {
				out["statuses"] = ss
			}
		case string:
			s := strings.TrimSpace(vv)
			if s != "" {
				out["statuses"] = []string{s}
			}
		}
	}
	if v, ok := m["vendor"].(string); ok && strings.TrimSpace(v) != "" {
		out["vendor"] = v
	}
	if v, ok := asInt(m["days_back"]); ok && v > 0 {
		out["days_back"] = v
	}
	b, _ := json.Marshal(out)
	return b
}

func buildTopAmountParams(seed map[string]any, limit int) json.RawMessage {
	out := map[string]any{
		"limit":    limit,
		"currency": "USD",
	}
	for k, v := range seed {
		out[k] = v
	}
	b, _ := json.Marshal(out)
	return b
}

// Heuristik "top N PO by amount"
func isAskingTopPOAmount(q string) bool {
	if strings.Contains(q, "amount tertinggi") ||
		strings.Contains(q, "nilai tertinggi") ||
		strings.Contains(q, "top po") ||
		(strings.Contains(q, "po") && strings.Contains(q, "tertinggi")) {
		return true
	}
	if strings.Contains(q, "nilai (amount)") && strings.Contains(q, "po") {
		return true
	}
	return false
}

func extractFirstInt(q string) int {
	re := regexp.MustCompile(`\b(\d{1,3})\b`)
	m := re.FindStringSubmatch(q)
	if len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return 0
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n, true
		}
	}
	return 0, false
}

func firstOr(v any, def string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return def
}

// ---- Helpers tambahan untuk normalisasi RAG ----

func pickTopK(values ...int) int {
	for _, v := range values {
		if v > 0 && v <= 100 {
			return v
		}
	}
	return 10
}

func looksLikeDocQuery(q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if len(q) == 0 {
		return false
	}
	keywords := []string{"report", "manual", "register", "policy", "procedure", "sop", "guideline", "standard", "minutes", "note", "rev", "revision", "forecast", "plan"}
	for _, k := range keywords {
		if strings.Contains(q, k) {
			return true
		}
	}
	return len(q) > 18 && strings.Count(q, " ") >= 1
}

func looksLikeDetectAnomaliesPayload(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var tmp struct {
		Series any `json:"series"`
	}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return false
	}
	_, ok := tmp.Series.([]any)
	return ok
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
