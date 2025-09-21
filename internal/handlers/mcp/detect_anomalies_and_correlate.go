// internal/handlers/mcp/detect_anomalies_and_correlate.go
// MCP Tool: detect_anomalies_and_correlate - deteksi anomali + korelasi sederhana
// internal/handlers/mcp/detect_anomalies_and_correlate.go
// MCP Tool: detect_anomalies_and_correlate - deteksi anomali + korelasi sederhana (real calc)
// internal/handlers/mcp/detect_anomalies_and_correlate.go

package mcp

import (
    "encoding/json"
    "math"
    "net/http"
    "sort"
    "strconv"
    "strings"
    // REMOVE unused imports: context, time, mysqlrepo
)

type TimeseriesPoint struct {
	TSUTC string  `json:"ts_utc"`
	Value float64 `json:"value"`
}

type Series struct {
	Name   string            `json:"name"`
	Points []TimeseriesPoint `json:"points"`
}

type DetectInput struct {
	Series    []Series `json:"series"`
	MinZScore float64  `json:"min_zscore"`
}

type DetectOutput struct {
	Anomalies       []string `json:"anomalies"`        // "series@ts value=... z=..."
	TopCorrelations []string `json:"top_correlations"` // "A vs B r=0.87 n=123"
}

func DetectAnomaliesHandler(w http.ResponseWriter, r *http.Request) {
	var input DetectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "bad request: invalid json", http.StatusBadRequest)
		return
	}
	if len(input.Series) == 0 {
		http.Error(w, "bad request: series is required", http.StatusBadRequest)
		return
	}
	zmin := input.MinZScore
	if zmin <= 0 {
		zmin = 2.5 // default
	}

	// ==== 1) Deteksi Anomali per series (z-score) ====
	var anomalies []string
	for _, s := range input.Series {
		if len(s.Points) < 3 {
			continue
		}
		mean, std := meanStd(s.Points)
		// kalau std==0 (flat), skip
		if std == 0 {
			continue
		}
		for _, p := range s.Points {
			z := (p.Value - mean) / std
			if math.Abs(z) >= zmin {
				anom := s.Name + "@" + p.TSUTC +
					" value=" + trimFloat(p.Value) +
					" z=" + trimFloat(z)
				anomalies = append(anomalies, anom)
			}
		}
	}
	// sort anomalies by lexicographic (series then ts) biar stabil
	sort.Strings(anomalies)

	// ==== 2) Korelasi Pearson antar series (align by timestamp) ====
	type pairCorr struct {
		a, b   string
		r      float64
		n      int
		absR   float64
	}
	var corrs []pairCorr

	// indexkan series jadi map[seriesName]map[ts]value
	indexed := make(map[string]map[string]float64, len(input.Series))
	for _, s := range input.Series {
		m := make(map[string]float64, len(s.Points))
		for _, p := range s.Points {
			m[p.TSUTC] = p.Value
		}
		indexed[s.Name] = m
	}

	names := make([]string, 0, len(indexed))
	for name := range indexed {
		names = append(names, name)
	}
	sort.Strings(names)

	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			a, b := names[i], names[j]
			xs, ys := alignXY(indexed[a], indexed[b])
			if len(xs) < 3 { // perlu minimal 3 titik
				continue
			}
			r := pearson(xs, ys)
			if math.IsNaN(r) {
				continue
			}
			corrs = append(corrs, pairCorr{
				a: a, b: b, r: r, n: len(xs), absR: math.Abs(r),
			})
		}
	}

	// urutkan dari |r| terbesar
	sort.Slice(corrs, func(i, j int) bool {
		if corrs[i].absR == corrs[j].absR {
			// tie-breaker: lebih banyak titik dulu
			return corrs[i].n > corrs[j].n
		}
		return corrs[i].absR > corrs[j].absR
	})

	var topCorr []string
	const K = 5
	for i := 0; i < len(corrs) && i < K; i++ {
		c := corrs[i]
		topCorr = append(topCorr, c.a+" vs "+c.b+" r="+trimFloat(c.r)+" n="+strconv.Itoa(c.n))
	}

	out := DetectOutput{
		Anomalies:       anomalies,
		TopCorrelations: topCorr,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ===== helpers =====

func meanStd(pts []TimeseriesPoint) (mean, std float64) {
	var sum float64
	for _, p := range pts {
		sum += p.Value
	}
	n := float64(len(pts))
	mean = sum / n
	var ss float64
	for _, p := range pts {
		d := p.Value - mean
		ss += d * d
	}
	// pakai sample std (n-1) kalau n>1; else 0
	if len(pts) > 1 {
		std = math.Sqrt(ss / (n - 1))
	} else {
		std = 0
	}
	return
}

func alignXY(ma, mb map[string]float64) (xs, ys []float64) {
	// ambil irisan timestamp
	// (map iteration order acak tidak masalah untuk korelasi)
	for ts, va := range ma {
		if vb, ok := mb[ts]; ok {
			xs = append(xs, va)
			ys = append(ys, vb)
		}
	}
	return
}

func pearson(x, y []float64) float64 {
	n := float64(len(x))
	var sx, sy, sxx, syy, sxy float64
	for i := 0; i < len(x); i++ {
		xi, yi := x[i], y[i]
		sx += xi
		sy += yi
		sxx += xi * xi
		syy += yi * yi
		sxy += xi * yi
	}
	num := n*sxy - sx*sy
	den := math.Sqrt((n*sxx - sx*sx) * (n*syy - sy*sy))
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

func trimFloat(v float64) string {
	// format pendek tanpa notasi ilmiah
	s := strconv.FormatFloat(v, 'f', 6, 64)
	// hapus trailing nol
	for strings.HasSuffix(s, "0") && strings.Contains(s, ".") {
		s = s[:len(s)-1]
	}
	if strings.HasSuffix(s, ".") {
		s = s[:len(s)-1]
	}
	return s
}
