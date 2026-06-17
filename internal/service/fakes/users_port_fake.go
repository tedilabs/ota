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
	// v0.2.2 #187 — extended lifecycle ops.
	ActivateFunc       func(ctx context.Context, userID string, sendEmail bool) error
	DeactivateFunc     func(ctx context.Context, userID string, sendEmail bool) error
	ExpirePasswordFunc func(ctx context.Context, userID string) error
	DeleteFunc         func(ctx context.Context, userID string) error
	// REQ-W01 — profile mutation Func. When nil, calls fail t.Fatalf.
	UpdateProfileFunc func(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error)
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

func (f *UsersPortFake) Activate(ctx context.Context, userID string, sendEmail bool) error {
	f.t.Helper()
	if f.ActivateFunc == nil {
		return nil
	}
	return f.ActivateFunc(ctx, userID, sendEmail)
}

func (f *UsersPortFake) Deactivate(ctx context.Context, userID string, sendEmail bool) error {
	f.t.Helper()
	if f.DeactivateFunc == nil {
		return nil
	}
	return f.DeactivateFunc(ctx, userID, sendEmail)
}

func (f *UsersPortFake) ExpirePassword(ctx context.Context, userID string) error {
	f.t.Helper()
	if f.ExpirePasswordFunc == nil {
		return nil
	}
	return f.ExpirePasswordFunc(ctx, userID)
}

func (f *UsersPortFake) Delete(ctx context.Context, userID string) error {
	f.t.Helper()
	if f.DeleteFunc == nil {
		return nil
	}
	return f.DeleteFunc(ctx, userID)
}

// UpdateProfile satisfies the REQ-W01 mutation surface on
// domain.UsersPort. When UpdateProfileFunc is nil the fake fails the
// test loudly — REQ-W01 tests must always wire it explicitly.
func (f *UsersPortFake) UpdateProfile(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error) {
	f.t.Helper()
	if f.UpdateProfileFunc == nil {
		f.t.Fatalf("UsersPortFake.UpdateProfile called but UpdateProfileFunc is not set")
	}
	return f.UpdateProfileFunc(ctx, userID, patch)
}

// ValidationErrorFake returns a stub UpdateProfileFunc that always
// rejects with *domain.BadRequestError carrying the supplied
// (field → reason) causes. Used to drive AC-6 server-side validation
// scenarios from the TUI layer.
func ValidationErrorFake(causes map[string]string) func(context.Context, string, domain.UserProfilePatch) (domain.User, error) {
	fields := make([]domain.FieldError, 0, len(causes))
	for k, v := range causes {
		fields = append(fields, domain.FieldError{Field: k, Summary: k + ": " + v})
	}
	return func(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
		return domain.User{}, &domain.BadRequestError{Causes: fields, Raw: "Api validation failed"}
	}
}
