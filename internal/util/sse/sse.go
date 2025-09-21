// [FILE] internal/util/sse/sse.go
// Helper util untuk menulis SSE secara aman.

package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
)

type Flusher interface {
	Flush()
}

// Set header SSE + no-cache
func PrepareSSE(w http.ResponseWriter) Flusher {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // Nginx: disable buffering
	flusher, _ := w.(http.Flusher)
	return flusher
}

func WriteEvent(w http.ResponseWriter, flusher Flusher, event string, v any) error {
	var payload string
	switch data := v.(type) {
	case string:
		payload = data
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		payload = string(b)
	}
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}
	fmt.Fprintf(w, "data: %s\n\n", payload)

	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

// Jika ingin memastikan buffer betul2 terkirim sebelum return.
func FlushWriter(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// Pastikan writer bukan buffered-only (proxy tertentu).
func WrapBuffered(w http.ResponseWriter) *bufio.ReadWriter {
	if rw, ok := w.(interface {
		Reader() *bufio.Reader
		Writer() *bufio.Writer
	}); ok {
		return bufio.NewReadWriter(rw.Reader(), rw.Writer())
	}
	return nil
}
