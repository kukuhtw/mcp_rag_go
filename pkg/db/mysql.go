// pkg/db/mysql.go
// Helper koneksi MySQL (menggunakan database/sql)

package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func NewMySQL() (*sql.DB, error) {
	host := getenv("MYSQL_HOST", "mysql")
	port := getenv("MYSQL_PORT", "3306")
	dbname := getenv("MYSQL_DB", "mcp")
	user := getenv("MYSQL_USER", "mcpuser")
	pass := getenv("MYSQL_PASSWORD", "secret")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, dbname)
	return sql.Open("mysql", dsn)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
