// Package clock provides injected time + jitter primitives so tests can drive
// polling (Logs tail), retry/backoff (Rate limit), and masking timeouts (PII)
// deterministically (CONVENTIONS §13.6).
package clock

import "time"

// Clock abstracts time.Now and sleep-style waits. Production code depends on
// this interface; tests inject FakeClock via Advance(d).
type Clock interface {
	Now() time.Time
	// NewTimer returns a Timer that fires after d. Fake implementations
	// schedule fires based on Advance calls instead of wall-clock time.
	NewTimer(d time.Duration) Timer
}

// Timer mirrors a tiny subset of time.Timer. It must be stoppable to avoid
// goroutine leaks in tests (goleak).
type Timer interface {
	// C is the channel that receives one send when the timer fires.
	C() <-chan time.Time
	// Stop attempts to prevent the Timer from firing. It returns true if it
	// successfully stopped the timer, false if the timer has already fired.
	Stop() bool
}

// Jitter applies a jitter factor to a base duration — used by the HTTP retry
// middleware to implement Retry-After ± 20% (REQ-E01 AC-2).
type Jitter interface {
	Apply(base time.Duration) time.Duration
}

// Real returns a production Clock backed by the standard library.
func Real() Clock { return realClock{} }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{t: time.NewTimer(d)}
}

type realTimer struct{ t *time.Timer }

func (r *realTimer) C() <-chan time.Time { return r.t.C }
func (r *realTimer) Stop() bool          { return r.t.Stop() }
