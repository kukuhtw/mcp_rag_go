// repositories/mysql/hsse_repo.go
// Repo untuk data HSSE (Health, Safety, Security, Environment)

package mysql

import (
	"database/sql"
	"fmt"
	"time"
	
)

type HSSERepo struct {
	DB *sql.DB
}

type HSSEIncident struct {
	ID          int
	Category    string
	Description string
	EventTime   time.Time
	Location    string
}

func (r *HSSERepo) GetIncidents(limit int) ([]HSSEIncident, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, category, description, event_time, location
		FROM hsse_incidents
		ORDER BY event_time DESC
		LIMIT ?`

	rows, err := r.DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("query hsse_incidents: %w", err)
	}
	defer rows.Close()

	var list []HSSEIncident
	for rows.Next() {
		var e HSSEIncident
		if err := rows.Scan(&e.ID, &e.Category, &e.Description, &e.EventTime, &e.Location); err != nil {
			return nil, fmt.Errorf("scan hsse_incident: %w", err)
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return list, nil
}
