package service

import (
	"context"
	"errors"
	"log/slog"
	"sort"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
)

// PoliciesQuery is the service-level Policies query alias.
type PoliciesQuery = domain.PoliciesQuery

// ErrPolicyTypeRequired signals that PoliciesService.List/ListAll was called
// without a Type (REQ-R04 AC-2; Okta's /api/v1/policies requires `type=`).
var ErrPolicyTypeRequired = errors.New("policy type is required")

// PoliciesService orchestrates Policies use cases (REQ-R04).
type PoliciesService struct {
	port  domain.PoliciesPort
	log   *slog.Logger
	clock clock.Clock
}

// NewPoliciesService constructs a PoliciesService.
func NewPoliciesService(port domain.PoliciesPort, opts ...ServiceOption) *PoliciesService {
	o := applyOptions(opts)
	return &PoliciesService{port: port, log: o.Logger, clock: o.Clock}
}

// List returns a policies iterator for the given type (REQ-R04 AC-2).
// Returns ErrPolicyTypeRequired when q.Type is empty.
func (s *PoliciesService) List(ctx context.Context, q domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
	if q.Type == "" {
		return nil, ErrPolicyTypeRequired
	}
	return s.port.List(ctx, q)
}

// ListAll drains List into a slice, ordered by priority ascending
// (REQ-R04 AC-3). Returns ErrPolicyTypeRequired on empty Type.
func (s *PoliciesService) ListAll(ctx context.Context, q domain.PoliciesQuery) ([]domain.Policy, error) {
	iter, err := s.List(ctx, q)
	if err != nil {
		return nil, err
	}
	items, err := drainIterator(ctx, iter)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Priority < items[j].Priority })
	return items, nil
}

// Get fetches a single policy.
func (s *PoliciesService) Get(ctx context.Context, id string) (domain.Policy, error) {
	return s.port.Get(ctx, id)
}

// Rules returns policy rules ordered by priority.
func (s *PoliciesService) Rules(ctx context.Context, policyID string) ([]domain.PolicyRule, error) {
	rules, err := s.port.Rules(ctx, policyID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })
	return rules, nil
}

// Invalidate is a no-op in MVP.
func (s *PoliciesService) Invalidate() {}
