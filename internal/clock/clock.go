// Package clock provides a swappable "now" function so that time-based
// validation can be tested deterministically.
package clock

import "time"

// NowFunc returns the current time, mirroring time.Now. It is a function type
// so callers (and tests) can inject deterministic clocks.
type NowFunc func() time.Time

// Default is the production NowFunc, equivalent to time.Now().
func Default() time.Time {
	return time.Now()
}
