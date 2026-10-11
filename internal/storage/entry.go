package storage

import "time"

// Entry represents a single stored item, holding its data and expiration time.
type Entry struct {
	Value     Value
	ExpiresAt time.Time // Zero time means the entry never expires.
}

// IsExpired checks if the entry has expired relative to the given time.
// Returns false if the entry has no expiration time (is zero).
func (e Entry) IsExpired(now time.Time) bool {
	if e.ExpiresAt.IsZero() {
		return false
	}

	return !now.Before(e.ExpiresAt)
}
