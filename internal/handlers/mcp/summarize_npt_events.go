// internal/handlers/mcp/summarize_npt_events.go
// MCP Tool: summarize_npt_events - ringkasan Non Productive Time (NPT

// internal/handlers/mcp/summarize_npt_events.go
package mcp

import (
    "context"
    "encoding/json"
    "math"
    "net/http"
    "sort" // ADD THIS IMPORT
    "strings"
    "time"

    mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)
// Memakai drillingRepo yang sudah di-inject lewat SetDrillingRepo(...) pada startup.
// (Lihat handler get_drilling_events yang sudah kita buat sebelumnya.)

type NPTSummary struct {
	SubCause string  `json:"sub_cause"`
	Hours    float64 `json:"hours"`
	CostUSD  int64   `json:"cost_usd"`
}

type NPTOutput struct {
	Breakdown  []NPTSummary `json:"breakdown"`
	Top3Levers []string     `json:"top_3_levers"`
}

type nptReq struct {
	WellID string `json:"well_id,omitempty"`
	Start  string `json:"start,omitempty"` // RFC3339, contoh: "2025-09-01T00:00:00Z"
	End    string `json:"end,omitempty"`   // RFC3339 (exclusive)
	TopK   int    `json:"top_k,omitempty"` // default 3
}

func SummarizeNPTEventsHandler(w http.ResponseWriter, r *http.Request) {
	if drillingRepo == nil {
		http.Error(w, "drilling repo not configured", http.StatusServiceUnavailable)
		return
	}

	// --- Ambil input dari query atau body ---
	in := nptReq{
		WellID: strings.TrimSpace(r.URL.Query().Get("well_id")),
		Start:  strings.TrimSpace(r.URL.Query().Get("start")),
		End:    strings.TrimSpace(r.URL.Query().Get("end")),
	}
	if r.Method == http.MethodPost && in.WellID == "" && in.Start == "" && in.End == "" {
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.WellID = strings.TrimSpace(in.WellID)
		in.Start = strings.TrimSpace(in.Start)
		in.End = strings.TrimSpace(in.End)
	}
	topK := 3
	if in.TopK > 0 && in.TopK <= 10 {
		topK = in.TopK
	}

	// --- Parse waktu; default 30 hari terakhir bila kosong ---
	now := time.Now().UTC()
	var startPtr, endPtr *time.Time
	if in.End != "" {
		if t, err := time.Parse(time.RFC3339, in.End); err == nil {
			endPtr = &t
		}
	}
	if endPtr == nil {
		te := now
		endPtr = &te
	}
	if in.Start != "" {
		if t, err := time.Parse(time.RFC3339, in.Start); err == nil {
			startPtr = &t
		}
	}
	if startPtr == nil {
		ts := endPtr.Add(-30 * 24 * time.Hour)
		startPtr = &ts
	}

	// --- Ambil data event NPT dari repo ---
	f := struct {
		WellID    string
		EventType string
		Start     *time.Time
		End       *time.Time
		Limit     int
		Offset    int
	}{
		WellID:    in.WellID,
		EventType: "NPT",
		Start:     startPtr,
		End:       endPtr,
		Limit:     10000, // cukup besar untuk agregasi; sesuaikan jika perlu
		Offset:    0,
	}

	// Karena struct filter aslinya ada di mysqlrepo.DrillFilter, kita mapping manual:
	df := mysqlrepo.DrillFilter{
		WellID:    f.WellID,
		EventType: f.EventType,
		Start:     f.Start,
		End:       f.End,
		Limit:     f.Limit,
		Offset:    f.Offset,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	rows, err := drillingRepo.List(ctx, df)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// --- Agregasi overlap durasi & biaya per sub_cause ---
	type agg struct {
		sec  float64
		cost float64
	}
	sum := map[string]*agg{}

	for _, ev := range rows {
		// Penentuan batas overlap
		evStart := ev.StartTime.UTC()
		evEnd := now
		if ev.EndTime.Valid {
			evEnd = ev.EndTime.Time.UTC()
		}
		// Event tidak valid?
		if evEnd.Before(evStart) {
			continue
		}

		// Hitung batas overlap: [max(start), min(end)]
		ovStart := maxTime(evStart, *startPtr)
		ovEnd := minTime(evEnd, *endPtr)
		if !ovEnd.After(ovStart) {
			continue
		}
		ovSec := ovEnd.Sub(ovStart).Seconds()

		// Durasi total event (untuk proporsi biaya)
		totalSec := evEnd.Sub(evStart).Seconds()
		if totalSec <= 0 {
			continue
		}
		prop := ovSec / totalSec

		// Ambil sub_cause
		sub := "unknown"
		if ev.SubCause.Valid && strings.TrimSpace(ev.SubCause.String) != "" {
			sub = strings.ToLower(strings.TrimSpace(ev.SubCause.String))
		}

		a := sum[sub]
		if a == nil {
			a = &agg{}
			sum[sub] = a
		}
		a.sec += ovSec
		if ev.CostUSD.Valid {
			a.cost += float64(ev.CostUSD.Int64) * prop
		}
	}

	// Susun output & urutkan jam terbanyak
	out := make([]NPTSummary, 0, len(sum))
	for k, v := range sum {
		out = append(out, NPTSummary{
			SubCause: k,
			Hours:    round2(v.sec / 3600.0),
			CostUSD:  int64(v.cost + 0.5), // pembulatan ke int
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hours == out[j].Hours {
			return out[i].CostUSD > out[j].CostUSD
		}
		return out[i].Hours > out[j].Hours
	})

	// Top levers = nama sub_cause teratas
	levers := make([]string, 0, min(topK, len(out)))
	for i := 0; i < len(out) && i < topK; i++ {
		levers = append(levers, out[i].SubCause)
	}

	resp := NPTOutput{
		Breakdown:  out,
		Top3Levers: levers,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// --- helpers ---

func maxTime(a, b time.Time) time.Time {
	if a.After(b) { return a }
	return b
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) { return a }
	return b
}
func min(a, b int) int {
	if a < b { return a }
	return b
}
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
