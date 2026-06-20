package fakes

import (
	"context"
	"testing"

	"github.com/tedilabs/ota/internal/domain"
)

// PoliciesPortFake satisfies domain.PoliciesPort.
type PoliciesPortFake struct {
	t *testing.T

	ListFunc         func(ctx context.Context, q domain.PoliciesQuery) (domain.Iterator[domain.Policy], error)
	GetFunc          func(ctx context.Context, id string) (domain.Policy, error)
	RulesFunc        func(ctx context.Context, policyID string) ([]domain.PolicyRule, error)
	UpdatePolicyFunc func(ctx context.Context, policyID string, update domain.PolicyUpdate) (domain.Policy, error)
	ActivateFunc     func(ctx context.Context, policyID string) error
	DeactivateFunc   func(ctx context.Context, policyID string) error
}

func NewPoliciesPort(t *testing.T) *PoliciesPortFake {
	t.Helper()
	return &PoliciesPortFake{t: t}
}

func (f *PoliciesPortFake) List(ctx context.Context, q domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
	f.t.Helper()
	if f.ListFunc == nil {
		f.t.Fatalf("PoliciesPortFake.List called but ListFunc is not set")
	}
	return f.ListFunc(ctx, q)
}

func (f *PoliciesPortFake) Get(ctx context.Context, id string) (domain.Policy, error) {
	f.t.Helper()
	if f.GetFunc == nil {
		f.t.Fatalf("PoliciesPortFake.Get called but GetFunc is not set")
	}
	return f.GetFunc(ctx, id)
}

func (f *PoliciesPortFake) Rules(ctx context.Context, policyID string) ([]domain.PolicyRule, error) {
	f.t.Helper()
	if f.RulesFunc == nil {
		f.t.Fatalf("PoliciesPortFake.Rules called but RulesFunc is not set")
	}
	return f.RulesFunc(ctx, policyID)
}

func (f *PoliciesPortFake) Activate(ctx context.Context, policyID string) error {
	f.t.Helper()
	if f.ActivateFunc == nil {
		return nil
	}
	return f.ActivateFunc(ctx, policyID)
}

func (f *PoliciesPortFake) Deactivate(ctx context.Context, policyID string) error {
	f.t.Helper()
	if f.DeactivateFunc == nil {
		return nil
	}
	return f.DeactivateFunc(ctx, policyID)
}

func (f *PoliciesPortFake) UpdatePolicy(ctx context.Context, policyID string, update domain.PolicyUpdate) (domain.Policy, error) {
	f.t.Helper()
	if f.UpdatePolicyFunc == nil {
		return domain.Policy{
			ID:          policyID,
			Name:        update.Name,
			Description: update.Description,
			Priority:    update.Priority,
			Status:      update.Status,
		}, nil
	}
	return f.UpdatePolicyFunc(ctx, policyID, update)
}
