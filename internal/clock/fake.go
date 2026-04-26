package clock

import (
	"sort"
	"sync"
	"time"
)

// FakeClock is a deterministic Clock for tests. Advance drives time forward
// and fires any pending timers whose deadlines fall at or before the new now.
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

// NewFake returns a FakeClock starting at `start`.
func NewFake(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

// Now returns the current fake time.
func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// NewTimer schedules a timer. The returned Timer fires once when Advance has
// carried the fake clock past the deadline.
func (f *FakeClock) NewTimer(d time.Duration) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{
		deadline: f.now.Add(d),
		ch:       make(chan time.Time, 1),
		parent:   f,
	}
	f.timers = append(f.timers, t)
	return t
}

// Advance moves the clock forward by d and fires any pending timers whose
// deadlines fall within the new interval, in deadline order.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	newNow := f.now

	type pending struct {
		t  *fakeTimer
		at time.Time
	}
	var ready []pending
	kept := f.timers[:0]
	for _, t := range f.timers {
		if t.stopped {
			continue
		}
		if !t.deadline.After(newNow) {
			ready = append(ready, pending{t, t.deadline})
			t.stopped = true
			continue
		}
		kept = append(kept, t)
	}
	f.timers = kept
	f.mu.Unlock()

	sort.SliceStable(ready, func(i, j int) bool { return ready[i].at.Before(ready[j].at) })
	for _, p := range ready {
		select {
		case p.t.ch <- p.at:
		default:
		}
	}
}

type fakeTimer struct {
	parent   *FakeClock
	deadline time.Time
	ch       chan time.Time
	stopped  bool
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

// Stop attempts to prevent the timer from firing. Returns true if it was
// stopped before firing, false if it already fired or was already stopped.
func (t *fakeTimer) Stop() bool {
	t.parent.mu.Lock()
	defer t.parent.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

// FixedJitter returns a Jitter that always returns base unchanged (for tests).
func FixedJitter() Jitter { return fixedJitter{} }

type fixedJitter struct{}

func (fixedJitter) Apply(base time.Duration) time.Duration { return base }
