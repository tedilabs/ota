package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
)

// GroupRulesQuery is the service-level Group Rules query alias.
type GroupRulesQuery = domain.GroupRulesQuery

// GroupRulesService orchestrates Group Rules use cases (REQ-R03). Resolves
// target group ids to names via groupsPort.
type GroupRulesService struct {
	port       domain.GroupRulesPort
	groupsPort domain.GroupsPort // for id->name resolution (REQ-R03 AC-4)
	log        *slog.Logger
	clock      clock.Clock
}

// NewGroupRulesService constructs a GroupRulesService.
func NewGroupRulesService(port domain.GroupRulesPort, groupsPort domain.GroupsPort, opts ...ServiceOption) *GroupRulesService {
	o := applyOptions(opts)
	return &GroupRulesService{port: port, groupsPort: groupsPort, log: o.Logger, clock: o.Clock}
}

// NewRulesService is a shorter alias for NewGroupRulesService.
func NewRulesService(port domain.GroupRulesPort, groupsPort domain.GroupsPort, opts ...ServiceOption) *GroupRulesService {
	return NewGroupRulesService(port, groupsPort, opts...)
}

// List returns a rules iterator.
func (s *GroupRulesService) List(ctx context.Context, q domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
	return s.port.List(ctx, q)
}

// RuleWithTargetNames pairs a GroupRule with its resolved target group names
// (REQ-R03 AC-4). Missing/forbidden names fall back to
// "<id> (name unavailable)".
type RuleWithTargetNames struct {
	domain.GroupRule
	TargetGroupNames []string
}

// ListWithTargetNames drains List and resolves each rule's target group ids
// to display names via groupsPort. Failures resolve to the id + fallback
// string rather than error out (REQ-R03 AC-4).
func (s *GroupRulesService) ListWithTargetNames(ctx context.Context) ([]RuleWithTargetNames, error) {
	iter, err := s.port.List(ctx, domain.GroupRulesQuery{Limit: 200})
	if err != nil {
		return nil, err
	}
	rules, err := drainIterator(ctx, iter)
	if err != nil {
		return nil, err
	}

	// Resolve target group names, memoizing per id.
	cache := map[string]string{}
	lookup := func(id string) string {
		if name, ok := cache[id]; ok {
			return name
		}
		if s.groupsPort == nil {
			cache[id] = fmt.Sprintf("%s (name unavailable)", id)
			return cache[id]
		}
		g, gerr := s.groupsPort.Get(ctx, id)
		if gerr != nil || g.Profile.Name == "" {
			cache[id] = fmt.Sprintf("%s (name unavailable)", id)
			return cache[id]
		}
		cache[id] = g.Profile.Name
		return cache[id]
	}

	out := make([]RuleWithTargetNames, 0, len(rules))
	for _, r := range rules {
		names := make([]string, 0, len(r.TargetGroupIDs))
		for _, id := range r.TargetGroupIDs {
			names = append(names, lookup(id))
		}
		out = append(out, RuleWithTargetNames{GroupRule: r, TargetGroupNames: names})
	}
	return out, nil
}

// Get fetches a single rule.
func (s *GroupRulesService) Get(ctx context.Context, id string) (domain.GroupRule, error) {
	return s.port.Get(ctx, id)
}

// ResolveTargetGroupNames returns names for the rule's TargetGroupIDs. Names
// missing or forbidden are returned as ""; the caller decides the fallback
// display (REQ-R03 AC-4 "(name unavailable)").
func (s *GroupRulesService) ResolveTargetGroupNames(ctx context.Context, ids []string) (map[string]string, error) {
	if s.groupsPort == nil {
		return nil, errors.New("service: groupsPort not configured")
	}
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		g, err := s.groupsPort.Get(ctx, id)
		if err != nil {
			out[id] = ""
			continue
		}
		out[id] = g.Profile.Name
	}
	return out, nil
}

// Invalidate is a no-op in MVP.
func (s *GroupRulesService) Invalidate() {}
