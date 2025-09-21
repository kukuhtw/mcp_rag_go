// internal/repositories/mysql/workorders_repo.go
// Repo untuk Work Orders
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type WorkOrderRepo struct{ DB *sql.DB }

type WORow struct {
	WOID     string
	AssetID  string
	Area     sql.NullString
	Priority int
	Status   string
	DueDate  sql.NullTime
	Created  time.Time
}

type WOFilter struct {
	AssetID     string
	Area        string
	Statuses    []string   // optional: IN (...)
	MinPriority *int
	MaxPriority *int
	DueStart    *time.Time // inclusive (date-only or datetime)
	DueEnd      *time.Time // exclusive
	Limit       int
	Offset      int
	Sort        string // "due_asc"(default), "due_desc", "prio_desc", "prio_asc", "Created_desc"
}

func (r *WorkOrderRepo) Search(ctx context.Context, f WOFilter) ([]WORow, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT wo_id, asset_id, area, priority, status, due_date, created_at
		FROM work_orders
		WHERE 1=1`)

	if f.AssetID != "" {
		sb.WriteString(` AND asset_id LIKE ?`)
		args = append(args, "%"+f.AssetID+"%")
	}
	if f.Area != "" {
		sb.WriteString(` AND area LIKE ?`)
		args = append(args, "%"+f.Area+"%")
	}
	if len(f.Statuses) > 0 {
		sb.WriteString(` AND status IN (` + placeholders(len(f.Statuses)) + `)`)
		for _, s := range f.Statuses {
			args = append(args, s)
		}
	}
	if f.MinPriority != nil {
		sb.WriteString(` AND priority >= ?`)
		args = append(args, *f.MinPriority)
	}
	if f.MaxPriority != nil {
		sb.WriteString(` AND priority <= ?`)
		args = append(args, *f.MaxPriority)
	}
	if f.DueStart != nil {
		sb.WriteString(` AND due_date >= ?`)
		// kirim time apa adanya; MySQL akan menyesuaikan TYPE kolom (DATE/DATETIME)
		args = append(args, f.DueStart)
	}
	if f.DueEnd != nil {
		sb.WriteString(` AND due_date < ?`)
		args = append(args, f.DueEnd)
	}

	switch strings.ToLower(f.Sort) {
	case "due_desc":
		sb.WriteString(` ORDER BY due_date DESC, priority DESC`)
	case "prio_desc":
		sb.WriteString(` ORDER BY priority DESC, due_date ASC`)
	case "prio_asc":
		sb.WriteString(` ORDER BY priority ASC, due_date ASC`)
	case "created_desc":
		sb.WriteString(` ORDER BY created_at DESC`)
	default:
		sb.WriteString(` ORDER BY due_date ASC, priority DESC`) // default
	}

	sb.WriteString(` LIMIT ? OFFSET ?`)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.DB.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query work_orders: %w", err)
	}
	defer rows.Close()

	out := make([]WORow, 0, f.Limit)
	for rows.Next() {
		var w WORow
		if err := rows.Scan(
			&w.WOID,
			&w.AssetID,
			&w.Area,
			&w.Priority,
			&w.Status,
			&w.DueDate,
			&w.Created,
		); err != nil {
			return nil, fmt.Errorf("scan work_order: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return out, nil
}

// (Opsional) Count untuk pagination total
func (r *WorkOrderRepo) Count(ctx context.Context, f WOFilter) (int64, error) {
	var sb strings.Builder
	var args []any

	sb.WriteString(`SELECT COUNT(*) FROM work_orders WHERE 1=1`)

	if f.AssetID != "" {
		sb.WriteString(` AND asset_id LIKE ?`)
		args = append(args, "%"+f.AssetID+"%")
	}
	if f.Area != "" {
		sb.WriteString(` AND area LIKE ?`)
		args = append(args, "%"+f.Area+"%")
	}
	if len(f.Statuses) > 0 {
		sb.WriteString(` AND status IN (` + placeholders(len(f.Statuses)) + `)`)
		for _, s := range f.Statuses {
			args = append(args, s)
		}
	}
	if f.MinPriority != nil {
		sb.WriteString(` AND priority >= ?`)
		args = append(args, *f.MinPriority)
	}
	if f.MaxPriority != nil {
		sb.WriteString(` AND priority <= ?`)
		args = append(args, *f.MaxPriority)
	}
	if f.DueStart != nil {
		sb.WriteString(` AND due_date >= ?`)
		args = append(args, f.DueStart)
	}
	if f.DueEnd != nil {
		sb.WriteString(` AND due_date < ?`)
		args = append(args, f.DueEnd)
	}

	var total int64
	if err := r.DB.QueryRowContext(ctx, sb.String(), args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count work_orders: %w", err)
	}
	return total, nil
}
