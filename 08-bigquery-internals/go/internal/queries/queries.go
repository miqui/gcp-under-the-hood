// Package queries builds the SQL used by the lab (pure functions).
package queries

import (
	"fmt"
	"regexp"
)

// PublicSource is the free public table used to seed the lab tables.
const PublicSource = "bigquery-public-data.austin_bikeshare.bikeshare_trips"

var ident = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Valid reports whether s is safe to splice into a backquoted identifier.
func Valid(s string) bool { return ident.MatchString(s) }

// Table returns a fully qualified, backquoted table reference.
func Table(project, dataset, table string) (string, error) {
	for _, p := range []string{project, dataset, table} {
		if !Valid(p) {
			return "", fmt.Errorf("invalid identifier %q", p)
		}
	}
	return fmt.Sprintf("`%s.%s.%s`", project, dataset, table), nil
}

// LoadSQL builds an INSERT ... SELECT that copies a sample from the public
// table into a lab table. limit bounds rows to keep the scan small.
func LoadSQL(target string, limit int) string {
	return fmt.Sprintf(`INSERT INTO %s (trip_id, subscriber_type, start_time, start_station_id, start_station_name, end_station_id, duration_minutes)
SELECT trip_id, subscriber_type, start_time, start_station_id, start_station_name, end_station_id, duration_minutes
FROM `+"`"+PublicSource+"`"+`
WHERE start_time IS NOT NULL
LIMIT %d`, target, limit)
}

// Scenario is a named query used in the bytes-billed comparison.
type Scenario struct {
	Label string
	SQL   string
}

// Scenarios returns the comparison set against the three layouts.
func Scenarios(flat, part, clust string) []Scenario {
	return []Scenario{
		{"select-star/flat", fmt.Sprintf("SELECT * FROM %s", flat)},
		{"one-column/flat", fmt.Sprintf("SELECT duration_minutes FROM %s", flat)},
		{"filter-date/flat", fmt.Sprintf("SELECT duration_minutes FROM %s WHERE start_time >= TIMESTAMP '2017-01-01' AND start_time < TIMESTAMP '2017-02-01'", flat)},
		{"filter-date/partitioned", fmt.Sprintf("SELECT duration_minutes FROM %s WHERE start_time >= TIMESTAMP '2017-01-01' AND start_time < TIMESTAMP '2017-02-01'", part)},
		{"filter-date+cluster/part+clust", fmt.Sprintf("SELECT duration_minutes FROM %s WHERE start_time >= TIMESTAMP '2017-01-01' AND start_time < TIMESTAMP '2017-02-01' AND subscriber_type = 'Annual Membership'", clust)},
		{"limit-10/flat (LIMIT does not cut scan)", fmt.Sprintf("SELECT * FROM %s LIMIT 10", flat)},
	}
}
