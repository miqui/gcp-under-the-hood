// Package logic holds pure, cloud-free helpers so they can be unit tested.
package logic

import (
	"fmt"
	"strings"
	"time"
)

// MaxV4Expiry is the hard limit GCS enforces for V4 signed URLs.
const MaxV4Expiry = 7 * 24 * time.Hour

// IAMDBUser converts a service-account email to the Postgres IAM user name:
// Cloud SQL drops the ".gserviceaccount.com" suffix.
func IAMDBUser(saEmail string) (string, error) {
	const suffix = ".gserviceaccount.com"
	if !strings.Contains(saEmail, "@") || !strings.HasSuffix(saEmail, suffix) {
		return "", fmt.Errorf("not a service account email: %q", saEmail)
	}
	return strings.TrimSuffix(saEmail, suffix), nil
}

// ValidateExpiry enforces 0 < d <= 7 days.
func ValidateExpiry(d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("expiry must be positive, got %s", d)
	}
	if d > MaxV4Expiry {
		return fmt.Errorf("expiry %s exceeds V4 maximum of %s", d, MaxV4Expiry)
	}
	return nil
}

// Env returns a required env var value or an error naming it.
func Env(get func(string) string, key string) (string, error) {
	v := get(key)
	if v == "" {
		return "", fmt.Errorf("required env var %s is not set", key)
	}
	return v, nil
}
