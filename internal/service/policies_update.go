package service

import (
	"context"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdatePolicy delegates to the PoliciesPort.
func (s *PoliciesService) UpdatePolicy(ctx context.Context, policyID string, update domain.PolicyUpdate) (domain.Policy, error) {
	return s.port.UpdatePolicy(ctx, policyID, update)
}
