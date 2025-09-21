// mcp/registry.go
// Registri mapping nama tool ke handler function

package mcp

import (
	"fmt"
	"net/http"
	"sync"
)

// Registry menyimpan peta nama tool -> http.Handler secara thread-safe.
type Registry struct {
	mu   sync.RWMutex
	data map[string]http.Handler
}

var (
	reg = &Registry{
		data: make(map[string]http.Handler),
	}
)

// Register mendaftarkan handler untuk sebuah tool.
// Jika nama sudah ada, handler lama akan ditimpa.
func Register(name string, h http.Handler) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.data[name] = h
}

// RegisterFunc mendaftarkan handler function biasa (http.HandlerFunc).
func RegisterFunc(name string, fn func(http.ResponseWriter, *http.Request)) {
	Register(name, http.HandlerFunc(fn))
}

// Get mengambil handler berdasarkan nama tool.
// Mengembalikan (handler, true) jika ada, atau (nil, false) jika tidak ditemukan.
func Get(name string) (http.Handler, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	h, ok := reg.data[name]
	return h, ok
}

// MustGet seperti Get namun panic jika tidak ditemukan.
// Cocok untuk inisialisasi saat startup (fail-fast).
func MustGet(name string) http.Handler {
	if h, ok := Get(name); ok {
		return h
	}
	panic(fmt.Sprintf("mcp: tool not found: %s", name))
}

// List mengembalikan daftar semua nama tool yang terdaftar.
func List() []string {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	keys := make([]string, 0, len(reg.data))
	for k := range reg.data {
		keys = append(keys, k)
	}
	return keys
}

// Serve mengeksekusi handler untuk tool 'name'.
// Jika tidak ditemukan, otomatis membalas 404.
func Serve(w http.ResponseWriter, r *http.Request, name string) {
	if h, ok := Get(name); ok {
		h.ServeHTTP(w, r)
		return
	}
	http.Error(w, "tool not found: "+name, http.StatusNotFound)
}
