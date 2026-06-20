package service

import "context"

// Activate delegates to the PoliciesPort.
func (s *PoliciesService) Activate(ctx context.Context, policyID string) error {
	return s.port.Activate(ctx, policyID)
}

// Deactivate delegates to the PoliciesPort.
func (s *PoliciesService) Deactivate(ctx context.Context, policyID string) error {
	return s.port.Deactivate(ctx, policyID)
}
