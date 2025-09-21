// internal/handlers/http/metrics_handler.go
// Handler untuk metrics Prometheus format sederhana

package http

import (
	"fmt"
	"net/http"
)

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP app_up 1 if the app is up\n# TYPE app_up gauge\napp_up 1\n")
}
