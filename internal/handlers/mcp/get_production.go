// internal/handlers/mcp/get_production.go
// MCP Tool: get_production - ambil data produksi harian
// internal/handlers/mcp/get_production.go
// MCP Tool: get_production - ambil data produksi harian

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

// inject dari app
var productionRepo *mysqlrepo.ProductionRepo

func SetProductionRepo(r *mysqlrepo.ProductionRepo) {
	productionRepo = r
}

type ProductionRow struct {
	Date      string  `json:"date"`       // YYYY-MM-DD
	WellID    string  `json:"well_id"`
	GasMMSCFD float64 `json:"gas_mmscfd,omitempty"`
}

type prodReq struct {
	WellID string `json:"well_id,omitempty"`
	Start  string `json:"start,omitempty"` // "2025-09-01"
	End    string `json:"end,omitempty"`   // "2025-09-20" (exclusive)
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

func GetProductionHandler(w http.ResponseWriter, r *http.Request) {
	if productionRepo == nil {
		http.Error(w, "production repo not configured", http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()

	// Terima well_id dan well (alias)
	wellID := strings.TrimSpace(q.Get("well_id"))
	if wellID == "" {
		wellID = strings.TrimSpace(q.Get("well"))
	}

	startStr := strings.TrimSpace(q.Get("start"))
	endStr := strings.TrimSpace(q.Get("end"))

	// Default: 30 hari terakhir jika kosong
	if startStr == "" && endStr == "" {
		endStr = time.Now().Format("2006-01-02")
		startStr = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}

	in := prodReq{
		WellID: wellID,
		Start:  startStr,
		End:    endStr,
	}

	if v := q.Get("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			in.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, _ := strconv.Atoi(v); n >= 0 {
			in.Offset = n
		}
	}

	if r.Method == http.MethodPost && in.WellID == "" && in.Start == "" && in.End == "" {
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.WellID = strings.TrimSpace(in.WellID)
		in.Start = strings.TrimSpace(in.Start)
		in.End = strings.TrimSpace(in.End)
	}

	parseDate := func(s string) *time.Time {
		if s == "" {
			return nil
		}
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil
		}
		return &t
	}

	f := mysqlrepo.ProdFilter{
		WellID: in.WellID,
		Start:  parseDate(in.Start),
		End:    parseDate(in.End),
		Limit:  in.Limit,
		Offset: in.Offset,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	rows, err := productionRepo.ListDaily(ctx, f)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   "db_error",
			"message": err.Error(),
			"input":   in,
		})
		return
	}

	out := make([]ProductionRow, 0, len(rows))
	for _, rr := range rows {
		rec := ProductionRow{
			Date:   rr.ProdDate.Format("2006-01-02"),
			WellID: rr.WellID,
		}
		if rr.GasMMSCFD.Valid {
			rec.GasMMSCFD = rr.GasMMSCFD.Float64
		}
		out = append(out, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

