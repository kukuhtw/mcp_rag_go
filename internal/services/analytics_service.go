// internal/services/analytics_service.go
// Layanan analitik: deteksi anomali sederhana & korelasi (stub)

package services

import (
	"errors"
	"math"
)

type TSPoint struct {
	TSUTC string
	Value float64
}

type Series struct {
	Name   string
	Points []TSPoint
}

type Anomaly struct {
	Series string
	TSUTC  string
	ZScore float64
}

type CorrPair struct {
	A string
	B string
	R float64
}

// ZScoreAnomalies mendeteksi anomali berbasis z-score sederhana (mean & stddev populasi).
func ZScoreAnomalies(s Series, minZ float64) ([]Anomaly, error) {
	if len(s.Points) == 0 {
		return nil, errors.New("empty series")
	}
	// mean
	var sum float64
	for _, p := range s.Points {
		sum += p.Value
	}
	mean := sum / float64(len(s.Points))

	// stddev
	var ss float64
	for _, p := range s.Points {
		d := p.Value - mean
		ss += d * d
	}
	std := math.Sqrt(ss / float64(len(s.Points)))
	if std == 0 {
		return []Anomaly{}, nil
	}

	var out []Anomaly
	for _, p := range s.Points {
		z := (p.Value - mean) / std
		if math.Abs(z) >= minZ {
			out = append(out, Anomaly{Series: s.Name, TSUTC: p.TSUTC, ZScore: z})
		}
	}
	return out, nil
}

// PearsonCorrelation menghitung korelasi Pearson antar 2 series (berdasarkan index sejajar).
func PearsonCorrelation(a, b Series) (float64, error) {
	n := min(len(a.Points), len(b.Points))
	if n < 2 {
		return 0, errors.New("insufficient points for correlation")
	}
	var sx, sy, sxx, syy, sxy float64
	for i := 0; i < n; i++ {
		x := a.Points[i].Value
		y := b.Points[i].Value
		sx += x
		sy += y
		sxx += x * x
		syy += y * y
		sxy += x * y
	}
	num := float64(n)*sxy - sx*sy
	den := math.Sqrt((float64(n)*sxx - sx*sx) * (float64(n)*syy - sy*sy))
	if den == 0 {
		return 0, nil
	}
	return num / den, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
