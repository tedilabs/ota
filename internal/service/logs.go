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
	port         domain.LogsPort
	log          *slog.Logger
	clock        clock.Clock
	pollInterval time.Duration
	adaptive     bool
}

// NewLogsService constructs a LogsService.
func NewLogsService(port domain.LogsPort, opts ...ServiceOption) *LogsService {
	o := applyOptions(opts)
	return &LogsService{port: port, log: o.Logger, clock: o.Clock, pollInterval: 7 * time.Second}
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
// published within the last `window`. ASCENDING sort so the result set
// can be rendered top-to-bottom oldest-first / newest-last (issue #116).
func (s *LogsService) HistoryQueryWindow(window time.Duration) domain.LogsQuery {
	q := domain.LogsQuery{
		SortOrder: domain.SortAscending,
		Limit:     1000,
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
