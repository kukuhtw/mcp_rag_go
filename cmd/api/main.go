// cmd/api/main.go
/*
==============================================================================
Project : MCP_RAG (Oil & Gas) — Go
Version : 0.1.0
Author  : Kukuh Tripamungkas Wicaksono (Kukuh TW)
Email   : kukuhtw@gmail.com
WhatsApp: https://wa.me/628129893706
LinkedIn: https://id.linkedin.com/in/kukuhtw
License : MIT (see LICENSE)

Summary : Monorepo PoC MCP + RAG untuk studi kasus perusahaan migas.
          Fitur utama:
          - MCP Router & Tools (PO, Production, Drilling, Timeseries, NPT).
          - RAG hybrid (BM25 + cosine) via /rag/search_v2 (MySQL doc_chunks).
          - Jawaban berbasis dokumen (answer_with_docs) lengkap sitasi.
          - Chat SSE (/chat/stream): planning → normalize → execute → stream.
          - Plan normalizer (auto switch rag_search_v2, top-N PO by amount).
          - Domain REST endpoints 
          
          - Konfigurasi via ENV; dukung OpenAI untuk LLM/embeddings (opsional).
==============================================================================
*/

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mcp-oilgas/internal/app"
	"mcp-oilgas/internal/middleware"
)
// cmd/api/main.go (global var)
var BuildVersion = "dev" // diisi saat ldflags


func main() {
	a := app.New()                    // <-- inisialisasi + inject semua repos
	a.Router.Use(middleware.CORS)     // <-- tetap pasang CORS/middleware lain

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      a.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("API running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}
