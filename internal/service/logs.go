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

// HistoryQuery returns the default query for history mode (REQ-R05 AC-4):
// SortOrder=DESCENDING (latest first), no explicit Since.
func (s *LogsService) HistoryQuery() domain.LogsQuery {
	return domain.LogsQuery{
		SortOrder: domain.SortDescending,
		Limit:     1000,
	}
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
