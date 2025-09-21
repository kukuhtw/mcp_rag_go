// repositories/mysql/drilling_repo.go
// Repo untuk data drilling events

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	
)

type DrillingRepo struct {
	DB *sql.DB
}

func NewDrillingRepo(db *sql.DB) *DrillingRepo { return &DrillingRepo{DB: db} }

// Filter used by the HTTP layer
type DrillFilter struct {
	WellID    string
	EventType string
	Start     *time.Time
	End       *time.Time
	Limit     int
	Offset    int
}

// Row shape expected by handler (nullable fields use sql.Null*)
type DrillingEvent struct {
	WellID    string
	EventType string
	SubCause  sql.NullString
	StartTime time.Time
	EndTime   sql.NullTime
	CostUSD   sql.NullInt64
}

// List builds a dynamic WHERE and supports limit/offset and time range
func (r *DrillingRepo) List(ctx context.Context, f DrillFilter) ([]DrillingEvent, error) {
	sb := strings.Builder{}
	sb.WriteString(`
		SELECT
			well_id,
			event_type,
			COALESCE(sub_cause, '') AS sub_cause,   -- still scan as NullString
			start_time,
			end_time,
			cost_usd
		FROM drilling_events
	`)

	var (
		where []string
		args  []any
	)

	if f.WellID != "" {
		where = append(where, "well_id = ?")
		args = append(args, f.WellID)
	}
	if f.EventType != "" {
		where = append(where, "event_type = ?")
		args = append(args, f.EventType)
	}
	if f.Start != nil {
		where = append(where, "start_time >= ?")
		args = append(args, f.Start.UTC())
	}
	if f.End != nil {
		where = append(where, "start_time < ?")
		args = append(args, f.End.UTC())
	}

	if len(where) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(where, " AND "))
	}

	sb.WriteString(" ORDER BY start_time DESC ")

	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	sb.WriteString(" LIMIT ? OFFSET ? ")
	args = append(args, limit, offset)

	query := sb.String()

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query drilling_events: %w", err)
	}
	defer rows.Close()

	var out []DrillingEvent
	for rows.Next() {
		var e DrillingEvent
		if err := rows.Scan(
			&e.WellID,
			&e.EventType,
			&e.SubCause,
			&e.StartTime,
			&e.EndTime,
			&e.CostUSD,
		); err != nil {
			return nil, fmt.Errorf("scan drilling_event: %w", err)
		}
		out = append(out, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return out, nil
}
