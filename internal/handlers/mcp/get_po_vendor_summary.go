// internal/handlers/mcp/get_po_vendor_summary.go
package mcp

import (
	"encoding/json"
	"net/http"
	// 🔧 tambahkan import repo mysql
	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

type POStatRepo interface {
	// 🔧 gunakan tipe dari paket repo agar cocok
	TopVendorsByStatus(status string, limit int) ([]mysqlrepo.VendorStat, error)
}

var poStatRepo POStatRepo

func SetPOStatRepo(r POStatRepo) { poStatRepo = r }

func GetPOVendorSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if poStatRepo == nil {
		http.Error(w, "repo not set", http.StatusInternalServerError)
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "in_transit"
	}
	status = normalizeStatus(status)

	vendors, err := poStatRepo.TopVendorsByStatus(status, 5)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  status,
		"vendors": vendors,
	})
}
