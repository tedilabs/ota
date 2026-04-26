package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// RateLimitPortFake satisfies domain.RateLimitPort.
type RateLimitPortFake struct {
	t *testing.T

	SnapshotsFunc func() []domain.RateLimitSnapshot
}

func NewRateLimitPort(t *testing.T) *RateLimitPortFake {
	t.Helper()
	return &RateLimitPortFake{t: t}
}

func (f *RateLimitPortFake) Snapshots() []domain.RateLimitSnapshot {
	f.t.Helper()
	if f.SnapshotsFunc == nil {
		f.t.Fatalf("RateLimitPortFake.Snapshots called but SnapshotsFunc is not set")
	}
	return f.SnapshotsFunc()
}

// HealthPortFake satisfies domain.HealthPort.
type HealthPortFake struct {
	t *testing.T

	CheckFunc func(ctx context.Context) error
}

func NewHealthPort(t *testing.T) *HealthPortFake {
	t.Helper()
	return &HealthPortFake{t: t}
}

func (f *HealthPortFake) Check(ctx context.Context) error {
	f.t.Helper()
	if f.CheckFunc == nil {
		f.t.Fatalf("HealthPortFake.Check called but CheckFunc is not set")
	}
	return f.CheckFunc(ctx)
}

// SliceIterator is a handy hand-rolled domain.Iterator[T] backed by a slice.
// It drains the slice once, returning hasMore=false when exhausted. Close is a
// no-op but counted so tests can assert release semantics.
type SliceIterator[T any] struct {
	Items     []T
	pos       int
	closed    bool
	CloseHits int
}

func (it *SliceIterator[T]) Next(ctx context.Context) (T, bool, error) {
	var zero T
	if ctx.Err() != nil {
		return zero, false, ctx.Err()
	}
	if it.pos >= len(it.Items) {
		return zero, false, nil
	}
	v := it.Items[it.pos]
	it.pos++
	return v, true, nil
}

func (it *SliceIterator[T]) Close() error {
	it.closed = true
	it.CloseHits++
	return nil
}
