package sessions

import "time"

// RealClock returns the current wall-clock time via time.Now().
// It is the production implementation of the Clock interface.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time {
	return time.Now()
}
