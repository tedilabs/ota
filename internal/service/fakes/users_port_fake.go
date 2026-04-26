package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// UsersPortFake satisfies domain.UsersPort for tests.
type UsersPortFake struct {
	t *testing.T

	ListFunc         func(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error)
	GetFunc          func(ctx context.Context, idOrLogin string) (domain.User, error)
	ListGroupsFunc   func(ctx context.Context, userID string) ([]domain.Group, error)
	ListFactorsFunc  func(ctx context.Context, userID string) ([]domain.Factor, error)
}

// NewUsersPort returns a fake UsersPort wired to t so unexpected calls fail
// loudly via t.Fatalf.
func NewUsersPort(t *testing.T) *UsersPortFake {
	t.Helper()
	return &UsersPortFake{t: t}
}

func (f *UsersPortFake) List(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
	f.t.Helper()
	if f.ListFunc == nil {
		f.t.Fatalf("UsersPortFake.List called but ListFunc is not set")
	}
	return f.ListFunc(ctx, q)
}

func (f *UsersPortFake) Get(ctx context.Context, idOrLogin string) (domain.User, error) {
	f.t.Helper()
	if f.GetFunc == nil {
		f.t.Fatalf("UsersPortFake.Get called but GetFunc is not set")
	}
	return f.GetFunc(ctx, idOrLogin)
}

func (f *UsersPortFake) ListGroups(ctx context.Context, userID string) ([]domain.Group, error) {
	f.t.Helper()
	if f.ListGroupsFunc == nil {
		f.t.Fatalf("UsersPortFake.ListGroups called but ListGroupsFunc is not set")
	}
	return f.ListGroupsFunc(ctx, userID)
}

func (f *UsersPortFake) ListFactors(ctx context.Context, userID string) ([]domain.Factor, error) {
	f.t.Helper()
	if f.ListFactorsFunc == nil {
		f.t.Fatalf("UsersPortFake.ListFactors called but ListFactorsFunc is not set")
	}
	return f.ListFactorsFunc(ctx, userID)
}
