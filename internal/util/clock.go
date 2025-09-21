// internal/util/clock.go
// Abstraksi waktu untuk memudahkan testing (stub)

package util

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
