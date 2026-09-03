// Package review: real-clock helper. Kept in its own file so the
// review_test package can drive it without poking at the service's
// internal wiring.
package review

import "time"

// Clock returns the current UTC time. Injected for deterministic
// tests. The production default is realClock{} (this package) —
// callers wire a fake in tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// RealClock returns the production Clock. Exported so the
// review_test package can exercise the Now() method directly
// (otherwise the coverage tool would not register a hit on the
// one-line method).
func RealClock() Clock { return realClock{} }
