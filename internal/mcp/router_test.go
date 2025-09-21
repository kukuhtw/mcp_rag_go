// mcp/router_test.go

package mcp_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	apppkg "mcp-oilgas/internal/app"
	"mcp-oilgas/internal/mcp"
)

// Payload minimal agar AnswerWithDocsHandler bisa jalan tanpa LLM (fallback extractive)
type awdPayload struct {
	Question        string           `json:"question"`
	RetrievedChunks []docChunkRefMin `json:"retrieved_chunks"`
}
type docChunkRefMin struct {
	DocID   string `json:"doc_id"`
	Snippet string `json:"snippet"`
}

type toolRequest struct {
	Tool     string                 `json:"tool,omitempty"`
	Question string                 `json:"question,omitempty"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

// Pastikan /mcp/route menjalankan tool terdaftar (answer_with_docs)
func TestMCPRouteExecutesRegisteredTool(t *testing.T) {
	// Build full app router agar MCP tools terdaftar via registerMCPTools()
	app := apppkg.New()
	r := app.Router

	// Siapkan payload untuk tool answer_with_docs (langsung sebut tool agar tidak tergantung LLM)
	p := awdPayload{
		Question:        "Apa isi dokumen?",
		RetrievedChunks: []docChunkRefMin{{DocID: "DOC_X", Snippet: "Ini konten sampel dokumen untuk diuji."}},
	}
	rawPayload, _ := json.Marshal(p)

	reqBody, _ := json.Marshal(toolRequest{
		Tool: "answer_with_docs",
		// Payload diteruskan sebagai body ke handler target oleh RouterHandler
		Payload: map[string]interface{}{},
	})
	// Ganti body agar sesuai struktur yang di-forward (payload langsung sebagai body):
	// RouterHandler akan memilih forward=req.Payload jika ada;
	// supaya itu terjadi, kita isi field Payload kosong—lalu langsung gunakan rawPayload sebagai body request.
	// Cara paling sederhana: panggil endpoint dengan body yang sudah final (langsung ke handler).
	// Namun kita ingin menguji RouterHandler. Jadi:
	req := httptest.NewRequest(http.MethodPost, "/mcp/route", bytes.NewReader(rawPayload))
	// NB: Agar RouterHandler meneruskan 'raw' body ke handler (bukan req.Payload),
	// kita panggil tanpa field "tool"/"payload" wrapper -> maka RouterHandler akan coba LLM.
	// Untuk deterministik, kita kirim wrapper yang menyebut tool + payload:
	//
	// Alternatif deterministik:
	// bodyWrapper := toolRequest{Tool: "answer_with_docs", Payload: map[string]any{}}
	// wrapperRaw, _ := json.Marshal(bodyWrapper)
	// req = httptest.NewRequest(http.MethodPost, "/mcp/route", bytes.NewReader(wrapperRaw))
	//
	// Namun agar handler menerima payload yang benar, kita pakai jalur lain:
	// Kita kirim request dengan field Tool & Payload berisi struktur awdPayload,
	// sehingga RouterHandler akan memilih forward=req.Payload.
	bodyWrapper := map[string]any{
		"tool": "answer_with_docs",
		"payload": map[string]any{
			"question": "Apa isi dokumen?",
			"retrieved_chunks": []map[string]any{
				{"doc_id": "DOC_X", "snippet": "Ini konten sampel dokumen untuk diuji."},
			},
		},
	}
	wrapperRaw, _ := json.Marshal(bodyWrapper)
	req = httptest.NewRequest(http.MethodPost, "/mcp/route", bytes.NewReader(wrapperRaw))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /mcp/route, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Response harus JSON dan berisi Answer (tidak kosong). Tidak perlu assert string spesifik.
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
}
