// internal/handlers/mcp/get_drilling_events.go
// MCP Tool: get_drilling_events - fetch data event pengeboran
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

// diinject dari layer app
var drillingRepo *mysqlrepo.DrillingRepo

func SetDrillingRepo(r *mysqlrepo.DrillingRepo) {
	drillingRepo = r
	readyDrilling = (r != nil)
}

type drillingReq struct {
	WellID    string `json:"well_id,omitempty"`
	EventType string `json:"event_type,omitempty"` // mis: "NPT", "TRIP", dll
	Start     string `json:"start,omitempty"`      // "2025-09-01T00:00:00Z"
	End       string `json:"end,omitempty"`        // "2025-09-20T00:00:00Z"
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

type DrillingEventDTO struct {
	WellID    string `json:"well_id"`
	EventType string `json:"event_type"`
	SubCause  string `json:"sub_cause,omitempty"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time,omitempty"`
	CostUSD   int64  `json:"cost_usd,omitempty"`
}

func GetDrillingEventsHandler(w http.ResponseWriter, r *http.Request) {
	if drillingRepo == nil {
		http.Error(w, "drilling repo not configured", http.StatusServiceUnavailable)
		return
	}

	var in drillingReq
	in.WellID = strings.TrimSpace(r.URL.Query().Get("well_id"))
	in.EventType = strings.TrimSpace(r.URL.Query().Get("event_type"))
	in.Start = strings.TrimSpace(r.URL.Query().Get("start"))
	in.End = strings.TrimSpace(r.URL.Query().Get("end"))
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
	if r.Method == http.MethodPost && in.WellID == "" && in.EventType == "" && in.Start == "" && in.End == "" {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}

	var f mysqlrepo.DrillFilter
	f.WellID = in.WellID
	f.EventType = in.EventType
	f.Limit = in.Limit
	f.Offset = in.Offset

	parseTS := func(s string) *time.Time {
		if s == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil
		}
		return &t
	}
	f.Start = parseTS(in.Start)
	f.End = parseTS(in.End)

	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	rows, err := drillingRepo.List(ctx, f)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]DrillingEventDTO, 0, len(rows))
	for _, ev := range rows {
		dto := DrillingEventDTO{
			WellID:    ev.WellID,
			EventType: ev.EventType,
			StartTime: ev.StartTime.UTC().Format(time.RFC3339),
		}
		if ev.SubCause.Valid {
			dto.SubCause = ev.SubCause.String
		}
		if ev.EndTime.Valid {
			dto.EndTime = ev.EndTime.Time.UTC().Format(time.RFC3339)
		}
		if ev.CostUSD.Valid {
			dto.CostUSD = ev.CostUSD.Int64
		}
		out = append(out, dto)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
