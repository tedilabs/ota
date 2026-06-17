package service

import (
	"context"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdateProfile delegates to UsersPort.UpdateProfile (REQ-W01 /
// D-T3). The service layer is intentionally thin — it does NOT
// validate the patch shape (the domain.UsersPort contract guards
// IsEmpty), it does NOT mutate the cache speculatively (D-T3:
// optimistic UI disabled, server response is authoritative), and
// it does NOT retry on transient errors (REQ-E01 AC-2 handles 429
// at the transport layer). On success, the caller (EditModel) is
// responsible for emitting UserUpdatedMsg so list/detail caches
// receive the server-echoed snapshot.
func (s *UsersService) UpdateProfile(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error) {
	return s.port.UpdateProfile(ctx, userID, patch)
}
