// repositories/mysql/po_repo.go
// Repo untuk data Purchase Order
// repositories/mysql/po_repo.go
// Repo untuk data Purchase Order

package mysql

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "strings"
    "sync"
    "time"
)

// ==============================
// Model & Filter
// ==============================

type PO struct {
    PONumber  string
    Vendor    string
    Status    string
    Amount    float64
    ETA       *time.Time
    UpdatedAt time.Time
}

type POFilter struct {
    Q         string
    Statuses  []string
    Vendor    string
    DateStart *time.Time // filter berdasarkan updated_at >= DateStart
    DateEnd   *time.Time // filter berdasarkan updated_at <  DateEnd
    AmountMin *float64
    AmountMax *float64

    SortBy   string
    SortDesc bool
    Limit    int
    Offset   int
}

var allowedSort = map[string]string{
    "updated_at": "updated_at",
    "status":     "status",
    "amount":     "amount",
    "vendor":     "vendor",
}

// ==============================
// Utility Functions
// ==============================



// ==============================
// Repo
// ==============================

type PORepo struct{ DB *sql.DB }

// Optional: pembungkus buat context timeout default repo
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
    if _, ok := ctx.Deadline(); ok {
        return ctx, func() {}
    }
    return context.WithTimeout(ctx, d)
}

// Normalisasi status: spasi/dash -> underscore; lower-case
func normalizeStatus(s string) string {
    s = strings.TrimSpace(strings.ToLower(s))
    s = strings.ReplaceAll(s, "-", "_")
    s = strings.Join(strings.Fields(s), "_")
    return s
}

// ==============================
// Deteksi FULLTEXT (auto fallback)
// ==============================

var (
    hasFTPOOnce sync.Once
    hasFTPO     bool
)

func detectFTPO(db *sql.DB) bool {
    const q = `
      SELECT 1
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME   = 'purchase_orders'
        AND INDEX_NAME   = 'ft_po_number_vendor'
        AND INDEX_TYPE   = 'FULLTEXT'
      LIMIT 1`
    var x int
    _ = db.QueryRow(q).Scan(&x)
    return x == 1
}

func (r *PORepo) ensureFTFlag() {
    hasFTPOOnce.Do(func() { hasFTPO = detectFTPO(r.DB) })
}

// ==============================
// Query: List & Count
// ==============================

func (r *PORepo) List(ctx context.Context, f POFilter) ([]PO, error) {
    ctx, cancel := withTimeout(ctx, 5*time.Second)
    defer cancel()

    r.ensureFTFlag()

    sb := strings.Builder{}
    sb.WriteString(`
      SELECT po_number, vendor, status, amount, eta, updated_at
      FROM purchase_orders
      WHERE 1=1`)

    args := make([]any, 0, 16)

    // Q: pakai FULLTEXT jika ada, fallback ke LIKE
    if strings.TrimSpace(f.Q) != "" {
        if hasFTPO {
            sb.WriteString(` AND MATCH(po_number, vendor) AGAINST (? IN BOOLEAN MODE)`)
            // boleh "prefix*" agar bisa prefix search; tapi hati-hati noise
            args = append(args, f.Q)
        } else {
            sb.WriteString(` AND (po_number LIKE ? OR vendor LIKE ?)`)
            like := "%" + f.Q + "%"
            args = append(args, like, like)
        }
    }

    // Statuses
    if len(f.Statuses) > 0 {
        norms := make([]string, 0, len(f.Statuses))
        for _, s := range f.Statuses {
            norms = append(norms, normalizeStatus(s))
        }
        sb.WriteString(` AND status IN (` + placeholders(len(norms)) + `)`)
        for _, s := range norms {
            args = append(args, s)
        }
    }

    // Vendor contains
    if strings.TrimSpace(f.Vendor) != "" {
        sb.WriteString(` AND vendor LIKE ?`)
        args = append(args, "%"+f.Vendor+"%")
    }

    // Date range by updated_at
    if f.DateStart != nil {
        sb.WriteString(` AND updated_at >= ?`)
        args = append(args, f.DateStart.Format("2006-01-02"))
    }
    if f.DateEnd != nil {
        sb.WriteString(` AND updated_at < ?`)
        args = append(args, f.DateEnd.Format("2006-01-02"))
    }

    // Amount range
    if f.AmountMin != nil {
        sb.WriteString(` AND amount >= ?`)
        args = append(args, *f.AmountMin)
    }
    if f.AmountMax != nil {
        sb.WriteString(` AND amount <= ?`)
        args = append(args, *f.AmountMax)
    }

    // Sorting
    sortCol := allowedSort["updated_at"]
    if col, ok := allowedSort[strings.ToLower(strings.TrimSpace(f.SortBy))]; ok {
        sortCol = col
    }
    dir := "ASC"
    if f.SortDesc {
        dir = "DESC"
    }
    sb.WriteString(` ORDER BY ` + sortCol + ` ` + dir)

    // Paging
    limit := f.Limit
    if limit <= 0 {
        limit = 50
    }
    if limit > 1000 {
        limit = 1000
    }
    offset := f.Offset
    if offset < 0 {
        offset = 0
    }
    sb.WriteString(` LIMIT ? OFFSET ?`)
    args = append(args, limit, offset)

    rows, err := r.DB.QueryContext(ctx, sb.String(), args...)
    if err != nil {
        return nil, fmt.Errorf("query purchase_orders: %w", err)
    }
    defer rows.Close()

    out := make([]PO, 0, limit)
    for rows.Next() {
        var po PO
        var eta sql.NullTime
        if err := rows.Scan(&po.PONumber, &po.Vendor, &po.Status, &po.Amount, &eta, &po.UpdatedAt); err != nil {
            return nil, err
        }
        if eta.Valid {
            po.ETA = &eta.Time
        }
        out = append(out, po)
    }
    return out, rows.Err()
}

func (r *PORepo) Count(ctx context.Context, f POFilter) (int64, error) {
    ctx, cancel := withTimeout(ctx, 5*time.Second)
    defer cancel()

    r.ensureFTFlag()

    sb := strings.Builder{}
    sb.WriteString(`SELECT COUNT(*) FROM purchase_orders WHERE 1=1`)
    args := make([]any, 0, 16)

    if strings.TrimSpace(f.Q) != "" {
        if hasFTPO {
            sb.WriteString(` AND MATCH(po_number, vendor) AGAINST (? IN BOOLEAN MODE)`)
            args = append(args, f.Q)
        } else {
            sb.WriteString(` AND (po_number LIKE ? OR vendor LIKE ?)`)
            like := "%" + f.Q + "%"
            args = append(args, like, like)
        }
    }
    if len(f.Statuses) > 0 {
        norms := make([]string, 0, len(f.Statuses))
        for _, s := range f.Statuses {
            norms = append(norms, normalizeStatus(s))
        }
        sb.WriteString(` AND status IN (` + placeholders(len(norms)) + `)`)
        for _, s := range norms {
            args = append(args, s)
        }
    }
    if strings.TrimSpace(f.Vendor) != "" {
        sb.WriteString(` AND vendor LIKE ?`)
        args = append(args, "%"+f.Vendor+"%")
    }
    if f.DateStart != nil {
        sb.WriteString(` AND updated_at >= ?`)
        args = append(args, f.DateStart.Format("2006-01-02"))
    }
    if f.DateEnd != nil {
        sb.WriteString(` AND updated_at < ?`)
        args = append(args, f.DateEnd.Format("2006-01-02"))
    }
    if f.AmountMin != nil {
        sb.WriteString(` AND amount >= ?`)
        args = append(args, *f.AmountMin)
    }
    if f.AmountMax != nil {
        sb.WriteString(` AND amount <= ?`)
        args = append(args, *f.AmountMax)
    }

    var c int64
    if err := r.DB.QueryRowContext(ctx, sb.String(), args...).Scan(&c); err != nil {
        return 0, fmt.Errorf("count purchase_orders: %w", err)
    }
    return c, nil
}

// ==============================
// Aggregation & Stats
// ==============================

type POVendorTotal struct {
    Vendor string  `json:"vendor"`
    Total  float64 `json:"total"`
}

type VendorStat struct {
    Vendor string  `json:"vendor"`
    Count  int64   `json:"count"`
    Total  float64 `json:"total"`
}

// CountByStatus: jumlah PO untuk satu status
func (r *PORepo) CountByStatus(status string) (int64, error) {
    norm := normalizeStatus(status)
    var c int64
    err := r.DB.QueryRow(
        `SELECT COUNT(*) FROM purchase_orders WHERE status = ?`,
        norm,
    ).Scan(&c)
    if err != nil {
        return 0, fmt.Errorf("count by status: %w", err)
    }
    return c, nil
}

// SumAmountByVendorTotal: total amount per vendor pada rentang updated_at [start, end)
func (r *PORepo) SumAmountByVendorTotal(ctx context.Context, vendors []string, start, end time.Time, statusNorm string) ([]POVendorTotal, error) {
    ctx, cancel := withTimeout(ctx, 6*time.Second)
    defer cancel()

    if len(vendors) == 0 {
        return nil, errors.New("vendors empty")
    }
    endPlus := end // exclusive di bawah pakai < endPlus
    // NOTE: start inclusive, end exclusive (konsisten dengan filter lain)

    ph := placeholders(len(vendors))
    args := make([]any, 0, 4+len(vendors))
    args = append(args, start, endPlus)
    for _, v := range vendors {
        args = append(args, v)
    }

    q := `
      SELECT COALESCE(vendor, 'UNKNOWN') AS vendor,
             COALESCE(SUM(amount),0)     AS total
      FROM purchase_orders
      WHERE updated_at >= ? AND updated_at < ?
        AND vendor IN (` + ph + `)`
    if statusNorm != "" {
        q += ` AND status = ?`
        args = append(args, normalizeStatus(statusNorm))
    }
    q += ` GROUP BY COALESCE(vendor, 'UNKNOWN')`

    rows, err := r.DB.QueryContext(ctx, q, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    out := make([]POVendorTotal, 0, len(vendors))
    for rows.Next() {
        var v POVendorTotal
        if err := rows.Scan(&v.Vendor, &v.Total); err != nil {
            return nil, err
        }
        out = append(out, v)
    }
    return out, rows.Err()
}

// Varian helper: input tanggal string "YYYY-MM-DD"
func (r *PORepo) SumAmountByVendorTotalDates(ctx context.Context, vendors []string, startDate, endDate string, statusNorm string) ([]POVendorTotal, error) {
    start, err := time.Parse("2006-01-02", startDate)
    if err != nil {
        return nil, fmt.Errorf("parse startDate: %w", err)
    }
    // end exclusive → tambahkan 1 hari agar [start, end+1)
    end, err := time.Parse("2006-01-02", endDate)
    if err != nil {
        return nil, fmt.Errorf("parse endDate: %w", err)
    }
    endPlus := end.Add(24 * time.Hour)
    return r.SumAmountByVendorTotal(ctx, vendors, start, endPlus, statusNorm)
}

// Top vendor berdasarkan status (jumlah & total)
func (r *PORepo) TopVendorsByStatus(status string, limit int) ([]VendorStat, error) {
    if limit <= 0 {
        limit = 5
    }
    q := `
      SELECT vendor, COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS total
      FROM purchase_orders
      WHERE status = ?
      GROUP BY vendor
      ORDER BY cnt DESC, total DESC
      LIMIT ?`
    rows, err := r.DB.Query(q, normalizeStatus(status), limit)
    if err != nil {
        return nil, fmt.Errorf("top vendors by status: %w", err)
    }
    defer rows.Close()

    out := make([]VendorStat, 0, limit)
    for rows.Next() {
        var v VendorStat
        if err := rows.Scan(&v.Vendor, &v.Count, &v.Total); err != nil {
            return nil, err
        }
        out = append(out, v)
    }
    return out, rows.Err()
}

// ==============================
// Get by number (opsional util)
// ==============================

func (r *PORepo) GetByNumber(ctx context.Context, poNumber string) (*PO, error) {
    ctx, cancel := withTimeout(ctx, 4*time.Second)
    defer cancel()

    row := r.DB.QueryRowContext(ctx, `
      SELECT po_number, vendor, status, amount, eta, updated_at
      FROM purchase_orders WHERE po_number = ?`, poNumber)

    var po PO
    var eta sql.NullTime
    if err := row.Scan(&po.PONumber, &po.Vendor, &po.Status, &po.Amount, &eta, &po.UpdatedAt); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil
        }
        return nil, err
    }
    if eta.Valid {
        po.ETA = &eta.Time
    }
    return &po, nil
}