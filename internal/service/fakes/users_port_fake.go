package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// UsersPortFake satisfies domain.UsersPort for tests.
type UsersPortFake struct {
	t *testing.T

	ListFunc          func(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error)
	GetFunc           func(ctx context.Context, idOrLogin string) (domain.User, error)
	ListGroupsFunc    func(ctx context.Context, userID string) ([]domain.Group, error)
	ListFactorsFunc   func(ctx context.Context, userID string) ([]domain.Factor, error)
	ListAppLinksFunc  func(ctx context.Context, userID string) ([]domain.AppLink, error)
	ResetPasswordFunc func(ctx context.Context, userID string, sendEmail bool) (string, error)
	UnlockFunc        func(ctx context.Context, userID string) error
	ResetFactorsFunc  func(ctx context.Context, userID string) error
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
		// Default empty — issue #168 made detail-open call this on
		// every user, so fakes that don't care about groups
		// shouldn't have to wire a func.
		return nil, nil
	}
	return f.ListGroupsFunc(ctx, userID)
}

func (f *UsersPortFake) ListFactors(ctx context.Context, userID string) ([]domain.Factor, error) {
	f.t.Helper()
	if f.ListFactorsFunc == nil {
		// Default empty for the same reason.
		return nil, nil
	}
	return f.ListFactorsFunc(ctx, userID)
}

func (f *UsersPortFake) ListAppLinks(ctx context.Context, userID string) ([]domain.AppLink, error) {
	f.t.Helper()
	if f.ListAppLinksFunc == nil {
		// Default: empty list — most tests don't care about app
		// links and shouldn't have to wire a func.
		return nil, nil
	}
	return f.ListAppLinksFunc(ctx, userID)
}

func (f *UsersPortFake) ResetPassword(ctx context.Context, userID string, sendEmail bool) (string, error) {
	f.t.Helper()
	if f.ResetPasswordFunc == nil {
		f.t.Fatalf("UsersPortFake.ResetPassword called but ResetPasswordFunc is not set")
	}
	return f.ResetPasswordFunc(ctx, userID, sendEmail)
}

func (f *UsersPortFake) Unlock(ctx context.Context, userID string) error {
	f.t.Helper()
	if f.UnlockFunc == nil {
		f.t.Fatalf("UsersPortFake.Unlock called but UnlockFunc is not set")
	}
	return f.UnlockFunc(ctx, userID)
}

func (f *UsersPortFake) ResetFactors(ctx context.Context, userID string) error {
	f.t.Helper()
	if f.ResetFactorsFunc == nil {
		f.t.Fatalf("UsersPortFake.ResetFactors called but ResetFactorsFunc is not set")
	}
	return f.ResetFactorsFunc(ctx, userID)
}
