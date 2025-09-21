// internal/services/production_service.go
// Layanan produksi: perbandingan aktual vs forecast (stub)

package services

type DailyProd struct {
	Date       string
	WellID     string
	GasMMSCFD  float64
}

type Variance struct {
	Date   string
	Value  float64 // actual - forecast
	DeltaP float64 // % variance
}

// VarianceDaily menghitung perbedaan aktual vs forecast (array sejajar).
func VarianceDaily(actual, forecast []DailyProd) []Variance {
	n := min(len(actual), len(forecast))
	out := make([]Variance, 0, n)
	for i := 0; i < n; i++ {
		a := actual[i].GasMMSCFD
		f := forecast[i].GasMMSCFD
		d := a - f
		var p float64
		if f != 0 {
			p = d / f * 100.0
		}
		out = append(out, Variance{
			Date:   actual[i].Date,
			Value:  d,
			DeltaP: p,
		})
	}
	return out
}
