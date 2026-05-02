package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
)

// LogsQuery is the service-level Logs query alias.
type LogsQuery = domain.LogsQuery

// GroupMembersQuery is the service-level alias for group-member listings.
type GroupMembersQuery = domain.GroupMembersQuery

// TailEvent is emitted by the tail loop for each polled batch.
type TailEvent struct {
	Events    []domain.LogEvent
	NextSince time.Time
}

// LogsService orchestrates System Logs use cases (REQ-R05). Tail mode lives
// on the separate LogsTail type (see logs_tail.go); LogsService is the
// history-query entrypoint.
type LogsService struct {
	port          domain.LogsPort
	log           *slog.Logger
	clock         clock.Clock
	pollInterval  time.Duration
	adaptive      bool
	limitPerFetch int
}

// NewLogsService constructs a LogsService. limitPerFetch ≤ 0 falls
// back to the default 100 (#F3 v0.2.5) — a small enough page that
// the screen stays readable, big enough that most last-30-min orgs
// fit in one batch without triggering the load-older sentinel.
func NewLogsService(port domain.LogsPort, opts ...ServiceOption) *LogsService {
	o := applyOptions(opts)
	return &LogsService{port: port, log: o.Logger, clock: o.Clock,
		pollInterval: 7 * time.Second, limitPerFetch: 100}
}

// WithLimitPerFetch sets the per-page cap on /api/v1/logs requests.
// Values ≤ 0 are ignored; callers that haven't picked a value get
// the constructor default. (#F3 v0.2.5)
func (s *LogsService) WithLimitPerFetch(n int) *LogsService {
	if n > 0 {
		s.limitPerFetch = n
	}
	return s
}

// LimitPerFetch reports the active page cap (used by the TUI for
// status hints).
func (s *LogsService) LimitPerFetch() int { return s.limitPerFetch }

// SearchPage runs a one-page History query — newest events within the
// window, capped at LimitPerFetch — and returns the parsed page plus
// the next-page cursor for the load-older flow (#F3 v0.2.5).
func (s *LogsService) SearchPage(ctx context.Context, q domain.LogsQuery) (domain.LogPage, error) {
	if q.Limit <= 0 {
		q.Limit = s.limitPerFetch
	}
	return s.port.SearchPage(ctx, q)
}

// Search runs a one-shot query (delegates to the port).
func (s *LogsService) Search(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
	return s.port.Search(ctx, q)
}

// HistoryQuery returns the default query for history mode. Pre-v0.1.4
// behaviour: DESCENDING + no Since. v0.1.4 made the default window 30
// minutes and switched to ASCENDING so the newest event renders at the
// bottom of the list (terminal-style log tail). Use HistoryQueryWindow
// when the operator picks a different range via the `1 / 3 / c / e`
// shortcuts.
func (s *LogsService) HistoryQuery() domain.LogsQuery {
	return s.HistoryQueryWindow(30 * time.Minute)
}

// HistoryQueryWindow returns a history query filtered to events
// published within the last `window`. #F3 v0.2.5 — sortOrder is now
// DESCENDING so when the result set exceeds LimitPerFetch the API
// returns the most-recent N events; older rows arrive via the
// load-older sentinel (after=cursor). The TUI re-sorts to
// ASCENDING for display so the terminal-tail layout (oldest-top,
// newest-bottom) is preserved.
func (s *LogsService) HistoryQueryWindow(window time.Duration) domain.LogsQuery {
	q := domain.LogsQuery{
		SortOrder: domain.SortDescending,
		Limit:     s.limitPerFetch,
	}
	if window > 0 {
		t := s.now().Add(-window)
		q.Since = &t
	}
	return q
}

// now returns the injected clock or wall time.
func (s *LogsService) now() time.Time {
	if s.clock != nil {
		return s.clock.Now()
	}
	return time.Now()
}

// PollInterval reports the current polling period (reflects adaptive state).
func (s *LogsService) PollInterval() time.Duration { return s.pollInterval }

// SetAdaptive upgrades the polling interval to 15s when a low rate-limit
// tenant is observed (REQ-R05 AC-2). Idempotent.
func (s *LogsService) SetAdaptive(enabled bool) {
	s.adaptive = enabled
	if enabled {
		s.pollInterval = 15 * time.Second
	} else {
		s.pollInterval = 7 * time.Second
	}
}
