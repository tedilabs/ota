package testfx

// Convenience port fakes for tests that only need to seed a list.
// Issue #A5 v0.2.4 — replaces the dozen-plus inline `stub<X>Port` /
// `recording<X>Port` / `seeded<X>Port` types scattered across the
// internal/app and internal/tui/<screen> test packages. Tests with
// behaviors not covered here (error injection, call-recording,
// custom Get / Members / etc.) should still define a local fake;
// these helpers exist for the common "give me N rows" case.

import (
	"context"

	"github.com/tedilabs/ota/internal/domain"
)

// --- Users -------------------------------------------------------------

// SeededUsersPort returns a UsersPort whose List() iterator hands
// back exactly the supplied slice. Get() looks up by ID or login;
// every other method is a no-op or returns ErrNotFound. Suitable for
// chrome / palette / badge tests that don't exercise the lifecycle
// ops.
func SeededUsersPort(users []domain.User) domain.UsersPort {
	return &seededUsersPort{users: users}
}

type seededUsersPort struct{ users []domain.User }

func (p *seededUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &sliceIter[domain.User]{remaining: p.users}, nil
}
func (p *seededUsersPort) Get(_ context.Context, id string) (domain.User, error) {
	for _, u := range p.users {
		if u.ID == id || u.Profile.Login == id {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}
func (p *seededUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *seededUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *seededUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *seededUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *seededUsersPort) Unlock(_ context.Context, _ string) error           { return nil }
func (p *seededUsersPort) ResetFactors(_ context.Context, _ string) error     { return nil }
func (p *seededUsersPort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *seededUsersPort) Deactivate(_ context.Context, _ string, _ bool) error {
	return nil
}
func (p *seededUsersPort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *seededUsersPort) Delete(_ context.Context, _ string) error         { return nil }

// UpdateProfile is a no-op stub for chrome / palette / badge tests
// that don't exercise REQ-W01. Tests asserting mutation behaviour
// should use UsersPortFake with UpdateProfileFunc instead.
func (p *seededUsersPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

// --- Groups ------------------------------------------------------------

// SeededGroupsPort returns a GroupsPort whose List() iterator hands
// back the supplied slice. Members / Apps / Get all return empty.
func SeededGroupsPort(groups []domain.Group) domain.GroupsPort {
	return &seededGroupsPort{groups: groups}
}

type seededGroupsPort struct{ groups []domain.Group }

func (p *seededGroupsPort) List(_ context.Context, _ domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
	return &sliceIter[domain.Group]{remaining: p.groups}, nil
}
func (p *seededGroupsPort) Get(_ context.Context, id string) (domain.Group, error) {
	for _, g := range p.groups {
		if g.ID == id {
			return g, nil
		}
	}
	return domain.Group{}, domain.ErrNotFound
}
func (p *seededGroupsPort) Members(_ context.Context, _ domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
	return &sliceIter[domain.User]{}, nil
}
func (p *seededGroupsPort) AppCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (p *seededGroupsPort) ListApps(_ context.Context, _ string) ([]domain.App, error) {
	return nil, nil
}

// --- Apps --------------------------------------------------------------

// SeededAppsPort returns an AppsPort whose List() iterator hands back
// the supplied slice. Get() looks up by ID.
func SeededAppsPort(apps []domain.App) domain.AppsPort {
	return &seededAppsPort{apps: apps}
}

type seededAppsPort struct{ apps []domain.App }

func (p *seededAppsPort) List(_ context.Context, _ domain.AppsQuery) (domain.Iterator[domain.App], error) {
	return &sliceIter[domain.App]{remaining: p.apps}, nil
}
func (p *seededAppsPort) Get(_ context.Context, id string) (domain.App, error) {
	for _, a := range p.apps {
		if a.ID == id {
			return a, nil
		}
	}
	return domain.App{}, domain.ErrNotFound
}

// --- Generic slice iterator -------------------------------------------

// sliceIter is the canonical domain.Iterator[T] over a pre-fetched
// slice. Used by every Seeded*Port above. Tests with custom paging /
// error-on-Nth behavior should define their own iterator.
type sliceIter[T any] struct{ remaining []T }

func (it *sliceIter[T]) Next(_ context.Context) (T, bool, error) {
	var zero T
	if len(it.remaining) == 0 {
		return zero, false, nil
	}
	v := it.remaining[0]
	it.remaining = it.remaining[1:]
	return v, true, nil
}
func (it *sliceIter[T]) Close() error { return nil }
