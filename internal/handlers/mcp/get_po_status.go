// internal/handlers/mcp/get_po_status.go
// MCP Tool: get_po_status - cek status Purchase Order

// internal/handlers/mcp/get_po_status.go
// MCP Tool: get_po_status - cek status Purchase Order

package mcp

import (
    "context"
    "database/sql"
    "encoding/json"
    "io"
    "net/http"
    "strconv"
    "strings"
    "time"

    mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

var allowed = map[string]bool{
    "created": true, "approved": true, "shipped": true,
    "in_transit": true, "in-transit": true, // terima kedua format
    "delivered": true, "closed": true, "cancelled": true,
}

// ==== Komparasi total PO per vendor (mode=vendor_compare) ====

// Interface minimal yang dibutuhkan handler ini.
// Return type pakai struct dari repo mysql, bukan didefinisikan ulang di sini.
type POCompareRepo interface {
    SumAmountByVendorTotal(ctx context.Context, vendors []string, start, end time.Time, statusNorm string) ([]mysqlrepo.POVendorTotal, error)
}

var poCompareRepo POCompareRepo

func SetPOCompareRepo(r POCompareRepo) { poCompareRepo = r }

// ===============================================================

func normalizeStatus(s string) string {
    s = strings.TrimSpace(strings.ToLower(s))
    s = strings.ReplaceAll(s, "-", " ")
    s = strings.Join(strings.Fields(s), "_")
    return s
}

// Ambil status dari query, form, atau JSON body (flat maupun di params)
func extractStatus(r *http.Request) string {
    // query / form
    if v := r.URL.Query().Get("status"); v != "" {
        return v
    }
    _ = r.ParseForm()
    if v := r.Form.Get("status"); v != "" {
        return v
    }

    // JSON body
    body, _ := io.ReadAll(r.Body)
    if len(body) == 0 {
        return ""
    }
    // reset supaya handler lain tetap bisa baca body
    r.Body = io.NopCloser(strings.NewReader(string(body)))

    // struct kuat
    var s1 struct {
        Status string `json:"status"`
        Params struct {
            Status string `json:"status"`
        } `json:"params"`
    }
    if json.Unmarshal(body, &s1) == nil {
        if s1.Status != "" {
            return s1.Status
        }
        if s1.Params.Status != "" {
            return s1.Params.Status
        }
    }

    // fallback map generic
    var m map[string]any
    if json.Unmarshal(body, &m) == nil {
        if v, ok := m["status"].(string); ok && v != "" {
            return v
        }
        if p, ok := m["params"].(map[string]any); ok {
            if v, ok2 := p["status"].(string); ok2 && v != "" {
                return v
            }
        }
    }
    return ""
}

type PORepo interface {
    CountByStatus(status string) (int64, error)
}

var poRepo PORepo

func SetPORepo(r PORepo) { poRepo = r }

func GetPOStatusHandler(w http.ResponseWriter, r *http.Request) {
    // ====== MODE: vendor_compare ======
    // FIX: HANYA aktif jika query param mode=vendor_compare (hapus trigger "POST JSON")
    if r.URL.Query().Get("mode") == "vendor_compare" {
        if poCompareRepo == nil {
            http.Error(w, "repo not set", http.StatusInternalServerError)
            return
        }

        // parse request
        var req struct {
            Vendors   []string `json:"vendors"`
            StartDate string   `json:"start_date"` // YYYY-MM-DD (inclusive)
            EndDate   string   `json:"end_date"`   // YYYY-MM-DD (exclusive/akan dibikin exclusive)
            Status    string   `json:"status"`     // optional
        }

        // GET query
        if r.Method == http.MethodGet {
            req.Vendors = r.URL.Query()["vendors"] // ?vendors=A&vendors=B
            req.StartDate = r.URL.Query().Get("start_date")
            req.EndDate = r.URL.Query().Get("end_date")
            req.Status = r.URL.Query().Get("status")
        }

        // Body JSON (boleh override)
        if strings.Contains(r.Header.Get("Content-Type"), "json") {
            _ = json.NewDecoder(r.Body).Decode(&req)
        }

        if len(req.Vendors) == 0 || req.StartDate == "" || req.EndDate == "" {
            http.Error(w, "vendors/start_date/end_date required", http.StatusBadRequest)
            return
        }
        start, err1 := time.Parse("2006-01-02", req.StartDate)
        end, err2 := time.Parse("2006-01-02", req.EndDate)
        if err1 != nil || err2 != nil || !end.After(start) {
            http.Error(w, "invalid date range", http.StatusBadRequest)
            return
        }

        // NOTE: repo.SumAmountByVendorTotal menganggap end exclusive;
        // kalau Anda ingin [start, end] inclusive, tambahkan +24h
        totals, err := poCompareRepo.SumAmountByVendorTotal(
            r.Context(), req.Vendors, start, end, normalizeStatus(req.Status),
        )
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // pilih pemenang
        winner := ""
        var max float64
        for _, t := range totals {
            if t.Total > max {
                max, winner = t.Total, t.Vendor
            }
        }

        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "mode":       "vendor_compare",
            "start":      req.StartDate,
            "end":        req.EndDate,
            "status":     req.Status, // kosong = semua status
            "vendors":    totals,     // []mysqlrepo.POVendorTotal
            "top_vendor": winner,
            "top_total":  max,
        })
        return
    }
    // ====== END MODE vendor_compare ======

    // ====== Legacy: hitung jumlah PO per status ======
    if poRepo == nil {
        http.Error(w, "repo not set", http.StatusInternalServerError)
        return
    }

    raw := extractStatus(r)
    status := normalizeStatus(raw)

    // Fallback: bila status kosong → kembalikan ringkasan per status (bukan 400)
    if status == "" {
        type rec struct {
            Status string `json:"status"`
            Count  int64  `json:"count"`
        }
        out := make([]rec, 0, len(allowed))
        for s := range allowed {
            n, err := poRepo.CountByStatus(s)
            if err != nil && err != sql.ErrNoRows {
                http.Error(w, "db error", http.StatusInternalServerError)
                return
            }
            out = append(out, rec{Status: s, Count: n})
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "mode":   "status_summary",
            "counts": out,
        })
        return
    }

    // Validasi nilai status spesifik
    if !allowed[status] {
        http.Error(w, "invalid status", http.StatusBadRequest)
        return
    }

    n, err := poRepo.CountByStatus(status)
    if err != nil && err != sql.ErrNoRows {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }
    if err == sql.ErrNoRows {
        n = 0
    }

    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write([]byte(`{"status":"` + status + `","count":` + strconv.FormatInt(n, 10) + `}`))
}
