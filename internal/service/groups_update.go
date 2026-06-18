package service

import (
	"context"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdateProfile delegates to the GroupsPort's strict-replace update.
// Thin wrapper so the App Shell + edit screen depend on the
// service-shaped surface (mirroring UsersService.UpdateProfile).
func (s *GroupsService) UpdateProfile(ctx context.Context, groupID string, profile domain.GroupProfileUpdate) (domain.Group, error) {
	return s.port.UpdateProfile(ctx, groupID, profile)
}
