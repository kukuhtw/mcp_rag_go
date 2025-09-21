// internal/handlers/mcp/get_timeseries.go
package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"strconv"

	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

// ===== DI =====
var timeseriesRepo *mysqlrepo.TimeseriesRepo

func SetTimeseriesRepo(r *mysqlrepo.TimeseriesRepo) {
	timeseriesRepo = r
	readyTimeseries = (r != nil) // lihat ready_flags.go
}

// ===== DTO output =====
type TimeseriesRow struct {
	TSUTC   string   `json:"ts_utc"`
	Value   *float64 `json:"value,omitempty"`
	Quality *int64   `json:"quality,omitempty"`
}

// ===== DTO input (planner kirim JSON) =====
type tsRequestBody struct {
	TagID     *string `json:"tag_id,omitempty"`   // ← string (VARCHAR)
	Tag       string  `json:"tag,omitempty"`      // e.g. "OIL_D01"
	StartDate string  `json:"start_date,omitempty"` // RFC3339
	EndDate   string  `json:"end_date,omitempty"`   // RFC3339
	Limit     *int    `json:"limit,omitempty"`
	Order     *string `json:"order,omitempty"` // "asc"|"desc"
}

// helper parse waktu
func parseRFC3339Ptr(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// resolve tag code -> tag_id (string)
func resolveTagID(ctx context.Context, repo *mysqlrepo.TimeseriesRepo, ident string) (string, string, error) {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return "", "", errors.New("empty tag ident")
	}
	tagID, tagName, err := repo.ResolveTagID(ctx, ident)
	if err != nil {
		return "", "", err // bisa sql.ErrNoRows
	}
	return tagID, tagName, nil
}

func GetTimeseriesHandler(w http.ResponseWriter, r *http.Request) {
	if timeseriesRepo == nil {
		http.Error(w, "timeseries repo not configured", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var (
		tagID string
		start *time.Time
		end   *time.Time
		limit int
		order string
	)

	// 1) JSON body (POST/PUT, planner)
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		var body tsRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		// tag or tag_id
		if body.TagID != nil && strings.TrimSpace(*body.TagID) != "" {
			tagID = strings.TrimSpace(*body.TagID)
		} else if strings.TrimSpace(body.Tag) != "" {
			id, _, err := resolveTagID(ctx, timeseriesRepo, body.Tag)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					http.Error(w, "tag not found", http.StatusNotFound)
					return
				}
				http.Error(w, "tag lookup error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			tagID = id
		}

		start = parseRFC3339Ptr(body.StartDate)
		end = parseRFC3339Ptr(body.EndDate)

		if body.Limit != nil && *body.Limit > 0 {
			limit = *body.Limit
		}
		if body.Order != nil {
			order = strings.ToLower(strings.TrimSpace(*body.Order))
		}
	}

	// 2) Fallback querystring (kompat lama)
	if tagID == "" {
		tagID = strings.TrimSpace(r.URL.Query().Get("tag_id"))
	}
	if start == nil {
		start = parseRFC3339Ptr(r.URL.Query().Get("start"))
	}
	if end == nil {
		end = parseRFC3339Ptr(r.URL.Query().Get("end"))
	}
	if limit == 0 {
		if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
	}
	if order == "" {
		order = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	}

	// 3) Validasi
	if tagID == "" {
		http.Error(w, "missing tag_id (or tag)", http.StatusBadRequest)
		return
	}
	if start != nil && end != nil && !end.After(*start) {
		http.Error(w, "invalid start/end (end must be after start)", http.StatusBadRequest)
		return
	}

	// 4) Query repo
	f := mysqlrepo.TSFilter{
		TagID: tagID, // string
		Start: start,
		End:   end,
		Limit: limit,
		Order: order,
	}
	points, err := timeseriesRepo.List(ctx, f)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5) Output
	out := make([]TimeseriesRow, 0, len(points))
	for _, p := range points {
		row := TimeseriesRow{TSUTC: p.TSUTC.UTC().Format(time.RFC3339)}
		if p.Value.Valid {
			v := p.Value.Float64
			row.Value = &v
		}
		if p.Quality.Valid {
			q := p.Quality.Int64
			row.Quality = &q
		}
		out = append(out, row)
	}

	resp := map[string]any{
		"tag_id": tagID,
		"count":  len(out),
		"points": out,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
