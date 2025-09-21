// internal/repositories/mysql/util.go
package mysql

import "strings"

// placeholders menghasilkan "?, ?, ?, ..." sebanyak n.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
