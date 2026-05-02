package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// LogsPortFake satisfies domain.LogsPort.
type LogsPortFake struct {
	t *testing.T

	SearchFunc     func(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error)
	SearchPageFunc func(ctx context.Context, q domain.LogsQuery) (domain.LogPage, error)
}

func NewLogsPort(t *testing.T) *LogsPortFake {
	t.Helper()
	return &LogsPortFake{t: t}
}

func (f *LogsPortFake) Search(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
	f.t.Helper()
	if f.SearchFunc == nil {
		f.t.Fatalf("LogsPortFake.Search called but SearchFunc is not set")
	}
	return f.SearchFunc(ctx, q)
}

func (f *LogsPortFake) SearchPage(ctx context.Context, q domain.LogsQuery) (domain.LogPage, error) {
	f.t.Helper()
	if f.SearchPageFunc != nil {
		return f.SearchPageFunc(ctx, q)
	}
	// #F3 v0.2.5 — auto-fallback to SearchFunc when only the
	// iterator-style fake is wired. Drains all pages into a single
	// LogPage with no After cursor (simulates "everything fits").
	if f.SearchFunc == nil {
		f.t.Fatalf("LogsPortFake.SearchPage called but neither SearchPageFunc nor SearchFunc is set")
	}
	iter, err := f.SearchFunc(ctx, q)
	if err != nil {
		return domain.LogPage{}, err
	}
	defer iter.Close()
	var events []domain.LogEvent
	for {
		e, hasMore, err := iter.Next(ctx)
		if err != nil {
			return domain.LogPage{}, err
		}
		if !hasMore {
			break
		}
		events = append(events, e)
	}
	return domain.LogPage{Events: events}, nil
}
