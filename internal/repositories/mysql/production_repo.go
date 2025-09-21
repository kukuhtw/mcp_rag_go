// repositories/mysql/production_repo.go
// Repo untuk data produksi harian
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	
)

type ProductionRepo struct{ DB *sql.DB }

type ProdRow struct {
	ProdDate time.Time
	WellID   string
	GasMMSCFD sql.NullFloat64
}

type ProdFilter struct {
	WellID string
	Start  *time.Time // inclusive
	End    *time.Time // exclusive
	Limit  int
	Offset int
}

func (r *ProductionRepo) ListDaily(ctx context.Context, f ProdFilter) ([]ProdRow, error) {
	if f.Limit <= 0 || f.Limit > 1000 { f.Limit = 200 }
	if f.Offset < 0 { f.Offset = 0 }

	// Asumsi skema:
	//   prod_allocation_daily(prod_date DATE, well_id VARCHAR, gas_mmscfd DOUBLE, ...)
	const base = `
		SELECT date, well_id, gas_mmscfd
		FROM prod_allocation_daily
		WHERE 1=1`
	args := []any{}
	q := base

	if f.WellID != "" {
		q += ` AND well_id LIKE ?`
		args = append(args, "%"+f.WellID+"%")
	}
	if f.Start != nil {
		q += ` AND date >= ?`
		args = append(args, f.Start.Format("2006-01-02"))
	}
	if f.End != nil {
		q += ` AND date < ?`
		args = append(args, f.End.Format("2006-01-02"))
	}

	q += ` ORDER BY date DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)

	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query production daily: %w", err)
	}
	defer rows.Close()

	var out []ProdRow
	for rows.Next() {
		var rrow ProdRow
		if err := rows.Scan(&rrow.ProdDate, &rrow.WellID, &rrow.GasMMSCFD); err != nil {
			return nil, err
		}
		out = append(out, rrow)
	}
	return out, rows.Err()
}
