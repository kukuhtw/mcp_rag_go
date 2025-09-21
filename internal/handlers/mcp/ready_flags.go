// internal/handlers/mcp/ready_flags.go
package mcp

// Flag readiness per domain; diset dari Set*Repo(..) masing-masing handler.
var (
	readyPO         bool
	readyTimeseries bool
	readyWorkOrders bool
	readyProduction bool
	readyDrilling   bool
)

// ReposStatus mengembalikan status siap/tidaknya setiap repo domain.
func ReposStatus() map[string]bool {
	return map[string]bool{
		"po":          readyPO,
		"timeseries":  readyTimeseries,
		"work_orders": readyWorkOrders,
		"production":  readyProduction,
		"drilling":    readyDrilling,
	}
}
