// internal/handlers/mcp/search_work_orders.go
// MCP Tool: search_work_orders - cari WO kritikal
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

// ===== Dependency Injection =====
// Diinject dari layer app.
var workOrderRepo *mysqlrepo.WorkOrderRepo

func SetWorkOrderRepo(r *mysqlrepo.WorkOrderRepo) {
	workOrderRepo = r
	readyWorkOrders = (r != nil) // set readiness flag (lihat ready_flags.go)
}

// ===== DTOs & Request =====
type WorkOrder struct {
	WOID     string `json:"wo_id"`
	AssetID  string `json:"asset_id"`
	Area     string `json:"area,omitempty"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
	DueDate  string `json:"due_date,omitempty"` // YYYY-MM-DD
}

type woReq struct {
	AssetID     string `json:"asset_id,omitempty"`
	Area        string `json:"area,omitempty"`
	Status      string `json:"status,omitempty"`       // single or comma-separated
	MinPriority *int   `json:"min_priority,omitempty"`
	MaxPriority *int   `json:"max_priority,omitempty"`
	DueStart    string `json:"due_start,omitempty"`    // YYYY-MM-DD
	DueEnd      string `json:"due_end,omitempty"`      // YYYY-MM-DD (exclusive)
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	Sort        string `json:"sort,omitempty"`         // due_asc|due_desc|prio_desc|prio_asc|updated_desc
}

// ===== Handler =====
func SearchWorkOrdersHandler(w http.ResponseWriter, r *http.Request) {
	if workOrderRepo == nil {
		http.Error(w, "work order repo not configured", http.StatusServiceUnavailable)
		return
	}

	// Support GET & POST
	var in woReq
	in.AssetID = strings.TrimSpace(r.URL.Query().Get("asset_id"))
	in.Area = strings.TrimSpace(r.URL.Query().Get("area"))
	in.Status = strings.TrimSpace(r.URL.Query().Get("status"))
	in.Sort = strings.TrimSpace(r.URL.Query().Get("sort"))

	if v := r.URL.Query().Get("min_priority"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			in.MinPriority = &n
		}
	}
	if v := r.URL.Query().Get("max_priority"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			in.MaxPriority = &n
		}
	}

	in.DueStart = strings.TrimSpace(r.URL.Query().Get("due_start"))
	in.DueEnd = strings.TrimSpace(r.URL.Query().Get("due_end"))

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			in.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, _ := strconv.Atoi(v); n >= 0 {
			in.Offset = n
		}
	}

	if r.Method == http.MethodPost &&
		in.AssetID == "" && in.Area == "" && in.Status == "" &&
		in.DueStart == "" && in.DueEnd == "" &&
		in.MinPriority == nil && in.MaxPriority == nil {
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.AssetID = strings.TrimSpace(in.AssetID)
		in.Area = strings.TrimSpace(in.Area)
		in.Status = strings.TrimSpace(in.Status)
		in.Sort = strings.TrimSpace(in.Sort)
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

	var statuses []string
	if in.Status != "" {
		for _, s := range strings.Split(in.Status, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}

	f := mysqlrepo.WOFilter{
		AssetID:     in.AssetID,
		Area:        in.Area,
		Statuses:    statuses,
		MinPriority: in.MinPriority,
		MaxPriority: in.MaxPriority,
		DueStart:    parseDate(in.DueStart),
		DueEnd:      parseDate(in.DueEnd),
		Limit:       in.Limit,
		Offset:      in.Offset,
		Sort:        in.Sort,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	rows, err := workOrderRepo.Search(ctx, f)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]WorkOrder, 0, len(rows))
	for _, rr := range rows {
		item := WorkOrder{
			WOID:     rr.WOID,
			AssetID:  rr.AssetID,
			Priority: rr.Priority,
			Status:   rr.Status,
		}
		if rr.Area.Valid {
			item.Area = rr.Area.String
		}
		if rr.DueDate.Valid {
			item.DueDate = rr.DueDate.Time.Format("2006-01-02")
		}
		out = append(out, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
