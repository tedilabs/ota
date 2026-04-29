package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// GroupRulesPortFake satisfies domain.GroupRulesPort.
type GroupRulesPortFake struct {
	t *testing.T

	ListFunc func(ctx context.Context, q domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error)
	GetFunc  func(ctx context.Context, id string) (domain.GroupRule, error)
	// v0.2.2 #188 — lifecycle hooks. Default to no-op when unset.
	ActivateFunc   func(ctx context.Context, ruleID string) error
	DeactivateFunc func(ctx context.Context, ruleID string) error
	DeleteFunc     func(ctx context.Context, ruleID string) error
}

func NewGroupRulesPort(t *testing.T) *GroupRulesPortFake {
	t.Helper()
	return &GroupRulesPortFake{t: t}
}

func (f *GroupRulesPortFake) List(ctx context.Context, q domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
	f.t.Helper()
	if f.ListFunc == nil {
		f.t.Fatalf("GroupRulesPortFake.List called but ListFunc is not set")
	}
	return f.ListFunc(ctx, q)
}

func (f *GroupRulesPortFake) Get(ctx context.Context, id string) (domain.GroupRule, error) {
	f.t.Helper()
	if f.GetFunc == nil {
		f.t.Fatalf("GroupRulesPortFake.Get called but GetFunc is not set")
	}
	return f.GetFunc(ctx, id)
}

func (f *GroupRulesPortFake) Activate(ctx context.Context, ruleID string) error {
	f.t.Helper()
	if f.ActivateFunc == nil {
		return nil
	}
	return f.ActivateFunc(ctx, ruleID)
}

func (f *GroupRulesPortFake) Deactivate(ctx context.Context, ruleID string) error {
	f.t.Helper()
	if f.DeactivateFunc == nil {
		return nil
	}
	return f.DeactivateFunc(ctx, ruleID)
}

func (f *GroupRulesPortFake) Delete(ctx context.Context, ruleID string) error {
	f.t.Helper()
	if f.DeleteFunc == nil {
		return nil
	}
	return f.DeleteFunc(ctx, ruleID)
}
