package service

import (
	"context"
	"sync"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// LogsTail encapsulates the stateful Logs tail loop (REQ-R05 AC-2/AC-3). It
// is intentionally separate from LogsService.Search (one-shot history)
// because tail is long-lived and carries its own `since` cursor, adaptive
// poll state, and pause/resume control.
//
// Callers drive the outer loop (tea.Tick); LogsTail computes the next
// LogsQuery and advances state.
type LogsTail struct {
	port domain.LogsPort

	// now returns the current time for tests. Defaults to time.Now.
	now func() time.Time

	mu       sync.Mutex
	interval time.Duration
	paused   bool
	resumeAt time.Time
}

// LogsTailOption tunes LogsTail.
type LogsTailOption func(*LogsTail)

// WithLogsTailNow injects a time provider (for tests).
func WithLogsTailNow(now func() time.Time) LogsTailOption {
	return func(t *LogsTail) { t.now = now }
}

// NewLogsTail constructs a LogsTail with default poll interval (REQ-R05 AC-2: 7s).
func NewLogsTail(port domain.LogsPort, opts ...LogsTailOption) *LogsTail {
	t := &LogsTail{
		port:     port,
		now:      time.Now,
		interval: 7 * time.Second,
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// InitialQuery returns the first-poll query: since=now-5m, sortOrder=ASCENDING,
// limit=1000 (REQ-R05 AC-2).
func (t *LogsTail) InitialQuery() domain.LogsQuery {
	since := t.now().Add(-5 * time.Minute)
	return domain.LogsQuery{
		Since:     &since,
		SortOrder: domain.SortAscending,
		Limit:     1000,
	}
}

// NextSinceAfter returns the `since` value to use on the next poll given the
// highest-observed `published` timestamp from the last batch. Adds 1ms for
// hole-free resume (REQ-R05 AC-2 / REQ-E01 AC-3).
func (t *LogsTail) NextSinceAfter(lastPublished time.Time) time.Time {
	return lastPublished.Add(time.Millisecond)
}

// ObserveRateLimit updates the polling interval based on the observed
// X-Rate-Limit-Limit value (REQ-R05 AC-2). Values < 60 upgrade to 15s.
func (t *LogsTail) ObserveRateLimit(limit int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if limit > 0 && limit < 60 {
		t.interval = 15 * time.Second
	} else {
		t.interval = 7 * time.Second
	}
}

// PollInterval returns the current interval (reflects adaptive state).
func (t *LogsTail) PollInterval() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.interval
}

// Pause sets the tail to paused for at least d; a later Resume (or the clock
// passing resumeAt) reverts. Called on 429 observation.
func (t *LogsTail) Pause(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = true
	t.resumeAt = t.now().Add(d)
}

// Resume immediately clears the paused state.
func (t *LogsTail) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = false
	t.resumeAt = time.Time{}
}

// Paused reports whether the tail is currently paused.
func (t *LogsTail) Paused() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.paused
}

// Poll runs one tick: fetches events with the given query and returns them
// plus the updated next-since.
func (t *LogsTail) Poll(ctx context.Context, q domain.LogsQuery) (events []domain.LogEvent, nextSince time.Time, err error) {
	iter, perr := t.port.Search(ctx, q)
	if perr != nil {
		return nil, time.Time{}, perr
	}
	items, derr := drainIterator(ctx, iter)
	if derr != nil {
		return nil, time.Time{}, derr
	}
	if len(items) == 0 {
		if q.Since != nil {
			return nil, *q.Since, nil
		}
		return nil, time.Time{}, nil
	}
	last := items[0].Published
	for _, e := range items[1:] {
		if e.Published.After(last) {
			last = e.Published
		}
	}
	return items, t.NextSinceAfter(last), nil
}
