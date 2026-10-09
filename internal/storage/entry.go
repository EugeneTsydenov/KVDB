package storage

import "time"

type Entry struct {
	Value     Value
	ExpiresAt time.Time
}

func (e Entry) IsExpired(now time.Time) bool {
	if e.ExpiresAt.IsZero() {
		return false
	}

	return !now.Before(e.ExpiresAt)
}
