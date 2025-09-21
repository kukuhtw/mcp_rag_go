// internal/handlers/mcp/get_po_vendor_compare.go
package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	
)

// NOTE: JANGAN redeclare POCompareRepo di sini.
// Kita pakai variable & interface yang sudah didefinisikan di get_po_status.go:
//   type POCompareRepo interface {
//       SumAmountByVendorTotal(ctx context.Context, vendors []string, start, end time.Time, statusNorm string) ([]mysqlrepo.POVendorTotal, error)
//   }
//   var poCompareRepo POCompareRepo
//   func SetPOCompareRepo(r POCompareRepo) { poCompareRepo = r }

func normStatus(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.Join(strings.Fields(s), "_")
	return s
}

type poVendorCompareReq struct {
	Vendors   []string `json:"vendors"`
	StartDate string   `json:"start_date"` // YYYY-MM-DD (inclusive)
	EndDate   string   `json:"end_date"`   // YYYY-MM-DD (exclusive atau end-of-day; kita buat exclusive)
	Status    string   `json:"status,omitempty"`
	Currency  string   `json:"currency,omitempty"`
}

func GetPOVendorCompareHandler(w http.ResponseWriter, r *http.Request) {
	if poCompareRepo == nil {
		http.Error(w, "repo not set", http.StatusInternalServerError)
		return
	}

	var req poVendorCompareReq
	if r.Method == http.MethodGet {
		req.Vendors = r.URL.Query()["vendors"]
		req.StartDate = r.URL.Query().Get("start_date")
		req.EndDate = r.URL.Query().Get("end_date")
		req.Status = r.URL.Query().Get("status")
		req.Currency = r.URL.Query().Get("currency")
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
	}

	if len(req.Vendors) < 2 || req.StartDate == "" || req.EndDate == "" {
		http.Error(w, "vendors (>=2), start_date, end_date required", http.StatusBadRequest)
		return
	}

	start, err1 := time.Parse("2006-01-02", req.StartDate)
	end, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil || !end.After(start) {
		http.Error(w, "invalid date range", http.StatusBadRequest)
		return
	}
	// end exclusive: tidak diubah di sini, repo SumAmountByVendorTotal sudah pakai "< end"
	status := normStatus(req.Status)

	rows, err := poCompareRepo.SumAmountByVendorTotal(r.Context(), req.Vendors, start, end, status)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	topVendor := ""
	var topTotal float64
	for _, row := range rows {
		if row.Total > topTotal {
			topTotal = row.Total
			topVendor = row.Vendor
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tool":     "get_po_vendor_compare",
		"period":   map[string]string{"start": req.StartDate, "end": req.EndDate},
		"status":   status,
		"currency": req.Currency,
		"results":  rows, // []mysqlrepo.POVendorTotal
		"winner":   map[string]any{"vendor": topVendor, "total": topTotal},
	})
}
