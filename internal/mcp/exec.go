// internal/mcp/exec.go
// internal/mcp/exec.go
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ExecResult struct {
	Route Route       `json:"route"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// ExecuteRoutes menjalankan semua rute: MCP in-process dan/atau RAG.
func ExecuteRoutes(
	ctx context.Context,
	routes []Route,
	ragFn func(ctx context.Context, query string, topK int) ([]map[string]any, error),
) ([]ExecResult, error) {
	var out []ExecResult

	for _, r := range routes {
		switch r.Kind {

		case RouteMCP:
			h, ok := Get(r.Tool)
			if !ok {
				out = append(out, ExecResult{Route: r, Error: "tool not found"})
				continue
			}

			// Body: default {}. Untuk RawMessage, Marshal mengembalikan raw bytes (aman).
			body := []byte("{}")
			if len(r.Params) > 0 && !isJSONNullOrEmpty(r.Params) {
				if b, err := json.Marshal(r.Params); err == nil && len(b) > 0 {
					body = b
				}
			}

			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/mcp/internal/"+r.Tool, bytes.NewReader(body))
			// Penting: biar handler mau decode JSON body
			req.Header.Set("Content-Type", "application/json")

			rr := newMemRecorder()
			h.ServeHTTP(rr, req)

			// Sukses: coba decode JSON. Kalau gagal, kirim raw string biar gampang debug.
			if rr.status >= 200 && rr.status < 300 {
				if len(rr.buf) == 0 {
					out = append(out, ExecResult{Route: r, Data: map[string]any{}})
					continue
				}
				var anyData interface{}
				if err := json.Unmarshal(rr.buf, &anyData); err != nil {
					out = append(out, ExecResult{Route: r, Data: string(rr.buf)})
				} else {
					out = append(out, ExecResult{Route: r, Data: anyData})
				}
				continue
			}

			// Error: ambil pesan dari body kalau ada
			msg := strings.TrimSpace(string(rr.buf))
			if msg == "" {
				msg = fmt.Sprintf("status %d", rr.status)
			}
			out = append(out, ExecResult{Route: r, Error: msg})

		case RouteRAG:
			topk := r.TopK
			if topk <= 0 || topk > 50 {
				topk = 10
			}
			hits, err := ragFn(ctx, r.Query, topk)
			if err != nil {
				out = append(out, ExecResult{Route: r, Error: err.Error()})
				continue
			}
			out = append(out, ExecResult{Route: r, Data: map[string]any{"retrieved_chunks": hits}})

		default:
			out = append(out, ExecResult{Route: r, Error: "unknown route kind"})
		}
	}

	return out, nil
}

// ---- mini response recorder (in-memory) ----
type memRecorder struct {
	buf        []byte
	status     int
	statusText string
	header     http.Header
}

func newMemRecorder() *memRecorder { return &memRecorder{header: http.Header{}, status: 200} }
func (m *memRecorder) Header() http.Header { return m.header }
func (m *memRecorder) Write(b []byte) (int, error) {
	m.buf = append(m.buf, b...)
	return len(b), nil
}
func (m *memRecorder) WriteHeader(code int) { m.status = code }
func (m *memRecorder) ReadCloser() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(m.buf))
}

// Util: cek apakah Params = null / {} / whitespace
func isJSONNullOrEmpty(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null" || s == "{}"
}
