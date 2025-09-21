package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// TimeseriesRepo: repo akses ts_signal & ts_value
type TimeseriesRepo struct {
	DB *sql.DB
}

// TSFilter: filter untuk query timeseries
type TSFilter struct {
	TagID string      // WAJIB: tag_id (string)
	Start *time.Time  // opsional: waktu mulai (UTC)
	End   *time.Time  // opsional: waktu akhir (UTC)
	Limit int         // opsional: batasi jumlah rows (0 = tak dibatasi)
	Order string      // ""|"asc"|"desc" (default asc)
}

// TSPoint: baris hasil timeseries
type TSPoint struct {
	TagID   string
	TSUTC   time.Time
	Value   sql.NullFloat64
	Quality sql.NullInt64
	Unit    sql.NullString // dari ts_signal.unit
}

// ResolveTagID menerima ident yang bisa berupa:
// - tag_id (persis sama seperti di ts_signal.tag_id), atau
// - tag_name (kode human readable, mis. "OIL_D01").
// Urutan lookup:
// 1) coba exact by tag_id
// 2) kalau tidak ada, cari by tag_name (case-insensitive)
func (r *TimeseriesRepo) ResolveTagID(ctx context.Context, ident string) (string, string, error) {
	if r == nil || r.DB == nil {
		return "", "", errors.New("timeseries repo: DB is nil")
	}
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return "", "", errors.New("timeseries repo: empty ident")
	}

	// 1) exact by tag_id
	{
		var tagID, tagName string
		const q = `SELECT tag_id, tag_name FROM ts_signal WHERE tag_id = ? LIMIT 1`
		if err := r.DB.QueryRowContext(ctx, q, ident).Scan(&tagID, &tagName); err == nil {
			return tagID, tagName, nil
		} else if err != sql.ErrNoRows {
			return "", "", err
		}
	}

	// 2) by tag_name (case-insensitive)
	{
		var tagID, tagName string
		const q = `
			SELECT tag_id, tag_name
			FROM ts_signal
			WHERE LOWER(tag_name) = LOWER(?)
			LIMIT 1
		`
		if err := r.DB.QueryRowContext(ctx, q, ident).Scan(&tagID, &tagName); err != nil {
			return "", "", err // termasuk sql.ErrNoRows
		}
		return tagID, tagName, nil
	}
}

// List: ambil deret waktu dari ts_value (opsional join unit dari ts_signal)
// - Jika Start dan/atau End diisi, pakai sebagai rentang waktu.
// - Order default asc; kalau "desc", urut mundur.
// - Limit > 0 untuk batasi jumlah baris.
// Catatan: kolom ts_utc diasumsikan UTC (sesuai skema).
func (r *TimeseriesRepo) List(ctx context.Context, f TSFilter) ([]TSPoint, error) {
	if r == nil || r.DB == nil {
		return nil, errors.New("timeseries repo: DB is nil")
	}
	if strings.TrimSpace(f.TagID) == "" {
		return nil, errors.New("timeseries repo: empty TagID")
	}

	// Bangun query dinamis sederhana
	sb := strings.Builder{}
	sb.WriteString(`
		SELECT v.tag_id, v.ts_utc, v.value, v.quality, s.unit
		FROM ts_value v
		LEFT JOIN ts_signal s ON s.tag_id = v.tag_id
		WHERE v.tag_id = ?
	`)

	args := []any{f.TagID}

	// rentang waktu
	if f.Start != nil && f.End != nil {
		sb.WriteString(" AND v.ts_utc BETWEEN ? AND ?")
		args = append(args, f.Start.UTC(), f.End.UTC())
	} else if f.Start != nil {
		sb.WriteString(" AND v.ts_utc >= ?")
		args = append(args, f.Start.UTC())
	} else if f.End != nil {
		sb.WriteString(" AND v.ts_utc <= ?")
		args = append(args, f.End.UTC())
	}

	// order
	order := strings.ToLower(strings.TrimSpace(f.Order))
	if order == "desc" {
		sb.WriteString(" ORDER BY v.ts_utc DESC")
	} else {
		sb.WriteString(" ORDER BY v.ts_utc ASC")
	}

	// limit
	if f.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, f.Limit)
	}

	q := sb.String()

	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]TSPoint, 0, 512)
	for rows.Next() {
		var p TSPoint
		if err := rows.Scan(&p.TagID, &p.TSUTC, &p.Value, &p.Quality, &p.Unit); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}
