package core

import "time"

// Clock is the only source of wall-clock time in the application. Production
// wiring uses SystemClock; tests inject a fixed, mutable fake.
type Clock interface {
	Now() time.Time
}

// SystemClock reports the real current time.
type SystemClock struct{}

// Now returns time.Now.
func (SystemClock) Now() time.Time { return time.Now() }
