// internal/handlers/mcp/get_po_top_amount.go
// MCP Tool: get_po_top_amount - ambil N PO dengan nilai tertinggi

package mcp

import (
	"context" // penting: fix undefined: context
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	mysqlrepo "mcp-oilgas/internal/repositories/mysql"
)

// Interface untuk listing PO dengan filter
type POLister interface {
	List(ctx context.Context, f mysqlrepo.POFilter) ([]mysqlrepo.PO, error)
}

var poLister POLister

func SetPOLister(r POLister) { poLister = r }

func GetPOTopAmountHandler(w http.ResponseWriter, r *http.Request) {
	if poLister == nil {
		http.Error(w, "repo not set", http.StatusInternalServerError)
		return
	}

	// Parse request parameters
	var req struct {
		Limit     int      `json:"limit"`     // default: 3
		Statuses  []string `json:"statuses"`  // optional filter
		Vendor    string   `json:"vendor"`    // optional filter
		DaysBack  int      `json:"days_back"` // optional: filter by updated_at dalam N hari terakhir
		Currency  string   `json:"currency"`  // optional: label mata uang (default: USD)
		StartDate string   `json:"start_date"`// optional (YYYY-MM-DD)
		EndDate   string   `json:"end_date"`  // optional (YYYY-MM-DD)
	}

	// Parse dari query parameters (GET)
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			req.Limit = l
		}
	}
	if vendor := r.URL.Query().Get("vendor"); vendor != "" {
		req.Vendor = vendor
	}
	if daysBack := r.URL.Query().Get("days_back"); daysBack != "" {
		if d, err := strconv.Atoi(daysBack); err == nil && d > 0 {
			req.DaysBack = d
		}
	}
	if currency := r.URL.Query().Get("currency"); currency != "" {
		req.Currency = currency
	}
	if statuses := r.URL.Query()["statuses"]; len(statuses) > 0 {
		req.Statuses = statuses
	}
	if sd := r.URL.Query().Get("start_date"); sd != "" {
		req.StartDate = sd
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		req.EndDate = ed
	}

	// Parse dari JSON body (POST/PUT) — override query
	if r.Method != "GET" && strings.Contains(r.Header.Get("Content-Type"), "json") {
		decoder := json.NewDecoder(r.Body)
		_ = decoder.Decode(&req)
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 3
	}
	if req.Limit > 100 {
		req.Limit = 100 // max safety limit
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}

	// Build filter
	filter := mysqlrepo.POFilter{
		SortBy:   "amount",
		SortDesc: true, // tertinggi dulu
		Limit:    req.Limit,
		Offset:   0,
	}

	// Filter by vendor if specified
	if strings.TrimSpace(req.Vendor) != "" {
		filter.Vendor = strings.TrimSpace(req.Vendor)
	}

	// Filter by statuses if specified
	if len(req.Statuses) > 0 {
		filter.Statuses = req.Statuses
	}

	// Filter by explicit date range (takes precedence)
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			filter.DateStart = &t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			filter.DateEnd = &t
		}
	}

	// If not using start/end, fallback to days_back
	if filter.DateStart == nil && filter.DateEnd == nil && req.DaysBack > 0 {
		now := time.Now()
		startDate := now.AddDate(0, 0, -req.DaysBack)
		filter.DateStart = &startDate
		filter.DateEnd = &now
	}

	// Execute query
	pos, err := poLister.List(r.Context(), filter)
	if err != nil {
		http.Error(w, "database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Format response
	type POResponse struct {
		PONumber  string  `json:"po_number"`
		Vendor    string  `json:"vendor"`
		Status    string  `json:"status"`
		Amount    float64 `json:"amount"`
		Currency  string  `json:"currency"`
		ETA       *string `json:"eta"`        // YYYY-MM-DD format
		UpdatedAt string  `json:"updated_at"` // YYYY-MM-DD HH:MM:SS format
	}

	result := make([]POResponse, 0, len(pos))
	for _, po := range pos {
		resp := POResponse{
			PONumber:  po.PONumber,
			Vendor:    po.Vendor,
			Status:    po.Status,
			Amount:    po.Amount,
			Currency:  req.Currency,
			UpdatedAt: po.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		// Format ETA if available
		if po.ETA != nil {
			eta := po.ETA.Format("2006-01-02")
			resp.ETA = &eta
		}
		result = append(result, resp)
	}

	// Build final response
	response := map[string]any{
		"mode":            "top_amount",
		"limit":           req.Limit,
		"count":           len(result),
		"currency":        req.Currency,
		"purchase_orders": result,
	}

	// Add filter info to response for debugging
	if req.Vendor != "" {
		response["vendor_filter"] = req.Vendor
	}
	if len(req.Statuses) > 0 {
		response["status_filter"] = req.Statuses
	}
	if req.DaysBack > 0 {
		response["days_back"] = req.DaysBack
	}
	if req.StartDate != "" || req.EndDate != "" {
		response["start_date"] = req.StartDate
		response["end_date"] = req.EndDate
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
