// internal/services/hsse_service.go
// Layanan HSSE: filter insiden berdasarkan tag/kategori (stub)

package services

type Incident struct {
	ID       string
	Category string   // near miss / first aid / LTI
	Tags     []string // lifting, confined space, dll.
	Severity int
}

func FilterIncidentsByTag(all []Incident, tag string) []Incident {
	var out []Incident
	for _, i := range all {
		for _, t := range i.Tags {
			if t == tag {
				out = append(out, i)
				break
			}
		}
	}
	return out
}
