package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// LogsPortFake satisfies domain.LogsPort.
type LogsPortFake struct {
	t *testing.T

	SearchFunc func(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error)
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
