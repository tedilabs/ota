package service

import (
	"context"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdateRule delegates to the GroupRulesPort. Thin wrapper mirroring
// UsersService.UpdateProfile / GroupsService.UpdateProfile.
func (s *GroupRulesService) UpdateRule(ctx context.Context, ruleID string, update domain.GroupRuleUpdate) (domain.GroupRule, error) {
	return s.port.UpdateRule(ctx, ruleID, update)
}
