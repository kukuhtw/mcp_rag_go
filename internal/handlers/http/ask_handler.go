// internal/handlers/http/ask_handler.go
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	mcps "mcp-oilgas/internal/mcp"
	"mcp-oilgas/internal/mcp/llm"
	search "mcp-oilgas/internal/repositories/search"
)

type AskRequest struct {
	Question string                 `json:"question"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

type AskResponse struct {
	Status  string            `json:"status"`
	Plan    mcps.Plan         `json:"plan"`
	Sources []mcps.ExecResult `json:"sources"`
	Answer  string            `json:"answer"`
	Error   string            `json:"error,omitempty"`
}

type AskDeps struct {
	RAGRepo search.RAGRepo
}

func NewAskHandler(deps AskDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Question == "" {
			http.Error(w, "question required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, 18*time.Second)
			defer cancel()
		}

		// Fase 1: Planner (LLM) → Plan (array routes)
		defs, _ := mcps.LoadToolDefs()
		var tools []llm.ToolLite
		for _, d := range defs {
			tools = append(tools, llm.ToolLite{Name: d.Name, Description: d.Description})
		}
		planner, err := llm.NewRoutePlannerFromEnv()
		var plan mcps.Plan
		if err != nil {
			plan = mcps.Plan{Mode: "rag", Fallback: true, Routes: []mcps.Route{{Kind: mcps.RouteRAG, Query: req.Question, TopK: 10}}, Reason: "planner init failed"}
		} else {
			raw, err := planner.PlanRaw(ctx, tools, req.Question)
			if err != nil || raw == "" {
				plan = mcps.Plan{Mode: "rag", Fallback: true, Routes: []mcps.Route{{Kind: mcps.RouteRAG, Query: req.Question, TopK: 10}}, Reason: "planner error/fallback"}
			} else {
				if uerr := json.Unmarshal([]byte(raw), &plan); uerr != nil || len(plan.Routes) == 0 {
					plan = mcps.Plan{Mode: "rag", Fallback: true, Routes: []mcps.Route{{Kind: mcps.RouteRAG, Query: req.Question, TopK: 10}}, Reason: "planner unmarshal/fallback"}
				}
			}
		}

		// Eksekusi routes (MCP/RAG)
		ragFn := func(ctx context.Context, query string, topK int) ([]map[string]any, error) {
			hits, err := deps.RAGRepo.Retrieve(ctx, query, topK)
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
					"page":    h.Page,
					"score":   h.Score,
				})
			}
			return out, nil
		}
		sources, _ := mcps.ExecuteRoutes(ctx, plan.Routes, ragFn)

		// Fase 2: Synth jawaban via LLM
		oclient, err := llm.NewFromEnv()
		answer := ""
		if err == nil {
			payload := struct {
				Question string            `json:"question"`
				Sources  []mcps.ExecResult `json:"sources"`
			}{Question: req.Question, Sources: sources}
			b, _ := json.Marshal(payload)
			sys := `Anda adalah asisten teknis.
- Jawab singkat, akurat, gunakan data pada "sources".
- Jika beberapa sumber, gabungkan dan sebutkan angka utama.
- Jika data kurang, sebutkan batasannya. Balas hanya jawaban final.`
			answer, _ = oclient.AnswerWithRAG(ctx, sys, string(b))
		}
		if answer == "" {
			answer = "Maaf, terjadi kendala saat menyusun jawaban."
		}

		resp := AskResponse{Status: "ok", Plan: plan, Sources: sources, Answer: answer}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
