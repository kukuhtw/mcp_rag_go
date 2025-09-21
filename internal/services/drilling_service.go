// internal/services/drilling_service.go
// Layanan drilling: agregasi event NPT dan ringkasan (stub)

package services

type NPTEvent struct {
	SubCause  string
	Hours     float64
	CostUSD   float64
}

type NPTBreakdown struct {
	SubCause string
	Hours    float64
	CostUSD  float64
}

// SummarizeNPT melakukan agregasi jam & biaya per sub-cause (dummy logic).
func SummarizeNPT(events []NPTEvent) []NPTBreakdown {
	agg := map[string]NPTBreakdown{}
	for _, e := range events {
		it := agg[e.SubCause]
		it.SubCause = e.SubCause
		it.Hours += e.Hours
		it.CostUSD += e.CostUSD
		agg[e.SubCause] = it
	}
	out := make([]NPTBreakdown, 0, len(agg))
	for _, v := range agg {
		out = append(out, v)
	}
	return out
}
