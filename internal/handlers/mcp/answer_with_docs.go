// internal/handlers/mcp/answer_with_docs.go
// MCP Tool: answer_with_docs - menjawab pertanyaan berbasis dokumen RAG
// internal/handlers/mcp/answer_with_docs.go
// MCP Tool: answer_with_docs - menjawab pertanyaan berbasis dokumen RAG
package mcp

import (
	"context"
	"encoding/json"
	
	"fmt"
	"net/http"
	
	"sort"
	"strings"
	"time"

	"mcp-oilgas/internal/mcp/llm"
)

// ======= I/O types =======

type AnswerWithDocsInput struct {
	Question        string        `json:"question"`
	RetrievedChunks []DocChunkRef `json:"retrieved_chunks"`
	TopK            int           `json:"top_k,omitempty"` // opsional, dipakai kalau mau auto-retrieve
}

type DocChunkRef struct {
	DocID   string `json:"doc_id"`
	Title   string `json:"title,omitempty"`
	URL     string `json:"url,omitempty"`
	Snippet string `json:"snippet"`
	PageNo  int    `json:"page_no,omitempty"`
}

type AnswerWithDocsOutput struct {
	Answer    string   `json:"answer"`
	Citations []string `json:"citations"`
}

// ======= (Opsional) Hook ke RAG repo =======
// Daftarkan fungsi ini dari layer wiring (app.go) bila ingin auto-retrieve saat input.RetrievedChunks kosong.
var RetrieveFn func(ctx context.Context, query string, topK int) ([]DocChunkRef, error)

// ======= LLM client (lazy init) =======
var llmClient llm.Client
var llmInitErr error
var llmOnce = struct {
	done bool
}{}

func initLLM() {
	if llmOnce.done {
		return
	}
	llmClient, llmInitErr = llm.NewFromEnv()
	llmOnce.done = true
}

// ======= Handler =======

func AnswerWithDocsHandler(w http.ResponseWriter, r *http.Request) {
	var input AnswerWithDocsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "bad request: invalid json", http.StatusBadRequest)
		return
	}
	input.Question = strings.TrimSpace(input.Question)
	if input.Question == "" {
		http.Error(w, "bad request: question is required", http.StatusBadRequest)
		return
	}

	// Ambil chunks: dari input, atau dari repo kalau kosong & hook tersedia
	chunks := input.RetrievedChunks
	if len(chunks) == 0 && RetrieveFn != nil {
		topK := input.TopK
		if topK <= 0 || topK > 50 {
			topK = 8
		}
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		rc, err := RetrieveFn(ctx, input.Question, topK)
		if err != nil {
			http.Error(w, "retrieve error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		chunks = rc
	}

	if len(chunks) == 0 {
		http.Error(w, "no retrieved_chunks provided and retrieval not available", http.StatusBadRequest)
		return
	}

	// Susun citations unik & prompt
	cits := makeCitations(chunks) // e.g. ["doc-1#p2", "doc-7#p1"]
	system := defaultSystemPrompt()
	user := buildUserPrompt(input.Question, chunks)

	// Coba LLM kalau ada API key, jika tidak ada → fallback extractive
	initLLM()
	var answer string
	if llmInitErr == nil {
		var err error
		answer, err = llmClient.AnswerWithRAG(r.Context(), system, user)
		if err != nil {
			// fallback ke extractive
			answer = extractiveFallback(input.Question, chunks)
		}
	} else {
		answer = extractiveFallback(input.Question, chunks)
	}

	resp := AnswerWithDocsOutput{
		Answer:    strings.TrimSpace(answer),
		Citations: cits,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ======= Helpers =======

func defaultSystemPrompt() string {
	// jaga jawaban ringkas, sitasi pasti dari daftar yang diberikan
	return `You are a helpful assistant for Retrieval-Augmented Generation.
You must ONLY use the provided document snippets to answer.
Cite using the provided citation keys if you use information from them.
If the answer is not in the snippets, say you don't have enough information.`
}

func buildUserPrompt(q string, chunks []DocChunkRef) string {
	var b strings.Builder
	b.WriteString("Question:\n")
	b.WriteString(q)
	b.WriteString("\n\n")
	b.WriteString("Snippets:\n")
	for i, c := range chunks {
		key := citationKey(c)
		title := c.Title
		if title == "" {
			title = c.DocID
		}
		b.WriteString(fmt.Sprintf("[%d] %s (%s)\n", i+1, title, key))
		// potong snippet yang terlalu panjang
		sn := strings.TrimSpace(c.Snippet)
		if len(sn) > 800 {
			sn = sn[:800] + "…"
		}
		b.WriteString(sn)
		b.WriteString("\n---\n")
	}
	b.WriteString("\nInstructions:\n- Answer concisely in the user's language.\n- Include citation keys where appropriate.\n")
	return b.String()
}

func citationKey(c DocChunkRef) string {
	page := c.PageNo
	if page <= 0 {
		page = 1
	}
	return fmt.Sprintf("%s#p%d", c.DocID, page)
}

func makeCitations(chunks []DocChunkRef) []string {
	uniq := map[string]struct{}{}
	for _, c := range chunks {
		uniq[citationKey(c)] = struct{}{}
	}
	keys := make([]string, 0, len(uniq))
	for k := range uniq {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Fallback sederhana: gabungkan potongan yang paling relevan secara heuristik
func extractiveFallback(question string, chunks []DocChunkRef) string {
	q := strings.ToLower(question)
	type scored struct {
		text string
		score int
	}
	var best []scored
	for _, c := range chunks {
		txt := strings.ToLower(c.Snippet)
		s := 0
		for _, w := range strings.Fields(q) {
			if len(w) >= 4 && strings.Contains(txt, w) {
				s++
			}
		}
		best = append(best, scored{text: strings.TrimSpace(c.Snippet), score: s})
	}
	sort.Slice(best, func(i, j int) bool { return best[i].score > best[j].score })
	var out strings.Builder
	max := 3
	if len(best) < max { max = len(best) }
	for i := 0; i < max; i++ {
		if i > 0 { out.WriteString("\n") }
		seg := best[i].text
		if len(seg) > 600 {
			seg = seg[:600] + "…"
		}
		out.WriteString(seg)
	}
	if out.Len() == 0 {
		return "Maaf, aku tidak menemukan jawaban yang cukup dari dokumen yang diberikan."
	}
	return out.String()
}

// ======= (Opsional) Wiring ke RAG repo =======

// Panggil fungsi ini dari layer app (mis. internal/app/app.go) setelah inisialisasi repo.
// Contoh:
/*
   mcp.RegisterRetriever(func(ctx context.Context, q string, topK int) ([]mcp.DocChunkRef, error) {
       hits, err := ragRepo.Retrieve(ctx, q, topK)
       if err != nil { return nil, err }
       refs := make([]mcp.DocChunkRef, 0, len(hits))
       for _, h := range hits {
          refs = append(refs, mcp.DocChunkRef{
              DocID: h.DocID, Title: h.Title, URL: h.URL,
             Snippet: h.Snippet, PageNo: h.Page,
           })
       }
      return refs, nil
  })
*/
func RegisterRetriever(fn func(ctx context.Context, query string, topK int) ([]DocChunkRef, error)) {
	RetrieveFn = fn
}
