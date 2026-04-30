package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// GroupsPortFake satisfies domain.GroupsPort.
type GroupsPortFake struct {
	t *testing.T

	ListFunc     func(ctx context.Context, q domain.GroupsQuery) (domain.Iterator[domain.Group], error)
	GetFunc      func(ctx context.Context, id string) (domain.Group, error)
	MembersFunc  func(ctx context.Context, q domain.GroupMembersQuery) (domain.Iterator[domain.User], error)
	AppCountFunc func(ctx context.Context, id string) (int, error)
	// v0.2.2 #189 — ListApps powers the Group Detail Apps box.
	ListAppsFunc func(ctx context.Context, groupID string) ([]domain.App, error)
}

func NewGroupsPort(t *testing.T) *GroupsPortFake {
	t.Helper()
	return &GroupsPortFake{t: t}
}

func (f *GroupsPortFake) List(ctx context.Context, q domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
	f.t.Helper()
	if f.ListFunc == nil {
		f.t.Fatalf("GroupsPortFake.List called but ListFunc is not set")
	}
	return f.ListFunc(ctx, q)
}

func (f *GroupsPortFake) Get(ctx context.Context, id string) (domain.Group, error) {
	f.t.Helper()
	if f.GetFunc == nil {
		f.t.Fatalf("GroupsPortFake.Get called but GetFunc is not set")
	}
	return f.GetFunc(ctx, id)
}

func (f *GroupsPortFake) Members(ctx context.Context, q domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
	f.t.Helper()
	if f.MembersFunc == nil {
		f.t.Fatalf("GroupsPortFake.Members called but MembersFunc is not set")
	}
	return f.MembersFunc(ctx, q)
}

func (f *GroupsPortFake) AppCount(ctx context.Context, id string) (int, error) {
	f.t.Helper()
	if f.AppCountFunc == nil {
		f.t.Fatalf("GroupsPortFake.AppCount called but AppCountFunc is not set")
	}
	return f.AppCountFunc(ctx, id)
}

func (f *GroupsPortFake) ListApps(ctx context.Context, groupID string) ([]domain.App, error) {
	f.t.Helper()
	if f.ListAppsFunc == nil {
		return nil, nil
	}
	return f.ListAppsFunc(ctx, groupID)
}
