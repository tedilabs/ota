package service

import (
	"context"
	"log/slog"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
)

// GroupsQuery is the service-level Groups query alias.
type GroupsQuery = domain.GroupsQuery

// GroupsService orchestrates Groups use cases (REQ-R02). Takes both the
// GroupsPort and GroupRulesPort because the DynamicTargeted flag on each
// returned group (RULE badge, REQ-R02 AC-1) is derived by cross-referencing
// currently active group rules.
type GroupsService struct {
	port      domain.GroupsPort
	rulesPort domain.GroupRulesPort
	log       *slog.Logger
	clock     clock.Clock
}

// NewGroupsService constructs a GroupsService.
func NewGroupsService(port domain.GroupsPort, rulesPort domain.GroupRulesPort, opts ...ServiceOption) *GroupsService {
	o := applyOptions(opts)
	return &GroupsService{port: port, rulesPort: rulesPort, log: o.Logger, clock: o.Clock}
}

// Search returns a groups iterator matching q. Each yielded Group has
// DynamicTargeted set based on the currently active GroupRules (REQ-R02 AC-1).
// Rule-port failures are swallowed (badge is best-effort).
func (s *GroupsService) Search(ctx context.Context, q GroupsQuery) (domain.Iterator[domain.Group], error) {
	iter, err := s.port.List(ctx, q)
	if err != nil {
		return nil, err
	}
	groups, err := drainIterator(ctx, iter)
	if err != nil {
		return nil, err
	}

	// Skip the DynamicTargeted lookup when there are no groups — nothing to
	// badge, and skipping avoids a wasted rules-port call.
	if len(groups) > 0 {
		targeted := s.collectTargetedGroupIDs(ctx)
		for i := range groups {
			if targeted[groups[i].ID] {
				groups[i].DynamicTargeted = true
			}
		}
	}
	return newSliceIterator(groups), nil
}

// Get fetches a single group by id.
func (s *GroupsService) Get(ctx context.Context, id string) (domain.Group, error) {
	return s.port.Get(ctx, id)
}

// Members returns the group's member users iterator (REQ-R02 AC-3).
func (s *GroupsService) Members(ctx context.Context, q domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
	return s.port.Members(ctx, q)
}

// AppCount returns the number of apps assigned to the group (REQ-R02 AC-4).
func (s *GroupsService) AppCount(ctx context.Context, id string) (int, error) {
	return s.port.AppCount(ctx, id)
}

// Invalidate is a no-op in MVP; GroupsService does not yet cache.
func (s *GroupsService) Invalidate() {}

// collectTargetedGroupIDs returns the set of group IDs referenced by any
// ACTIVE GroupRule. Errors are swallowed; callers treat a nil/empty result as
// "no rules observed → no dynamic badges".
func (s *GroupsService) collectTargetedGroupIDs(ctx context.Context) map[string]bool {
	if s.rulesPort == nil {
		return nil
	}
	iter, err := s.rulesPort.List(ctx, domain.GroupRulesQuery{Limit: 200})
	if err != nil {
		return nil
	}
	rules, err := drainIterator(ctx, iter)
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(rules))
	for _, r := range rules {
		if r.Status != domain.GroupRuleStatusActive {
			continue
		}
		for _, gid := range r.TargetGroupIDs {
			out[gid] = true
		}
	}
	return out
}
