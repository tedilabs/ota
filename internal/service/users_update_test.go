package service_test

// REQ-W01 — UsersService.UpdateProfile service-layer tests
// (Step 5 of the Phase 5 RED order). The service is intentionally
// thin (D-T3): it forwards the patch verbatim to the port, propagates
// every domain error sentinel, and does NOT mutate the cache
// speculatively. EditModel is the only place that broadcasts
// UserUpdatedMsg on success.
//
// Phase 5 RED expectation: UsersService.UpdateProfile is a stub
// returning "not implemented" so every assertion below fails until
// Phase 6 wires the delegation.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
)

// D-T3 — service.UpdateProfile delegates the patch verbatim to
// UsersPort.UpdateProfile. Pin (a) the user ID, (b) the exact patch
// shape, and (c) the returned User.
func Test_UsersService_UpdateProfile_DelegatesToPort_VerbatimPatch(t *testing.T) {
	t.Parallel()

	var gotID string
	var gotPatch domain.UserProfilePatch
	port := fakes.NewUsersPort(t)
	port.UpdateProfileFunc = func(_ context.Context, id string, p domain.UserProfilePatch) (domain.User, error) {
		gotID = id
		gotPatch = p
		return domain.User{
			ID: id,
			Profile: domain.UserProfile{
				Login: "alice@acme.com", Email: "alice@acme.com",
				FirstName: "Alicia",
			},
		}, nil
	}
	svc := service.NewUsersService(port)

	newFirst := "Alicia"
	patch := domain.UserProfilePatch{FirstName: &newFirst}
	user, err := svc.UpdateProfile(context.Background(), "00u_alice", patch)
	require.NoError(t, err, "REQ-W01 D-T3: service must propagate success verbatim")

	assert.Equal(t, "00u_alice", gotID, "REQ-W01: userID must reach the port unchanged")
	require.NotNil(t, gotPatch.FirstName, "REQ-W01 D-T3: patch.FirstName must reach the port")
	assert.Equal(t, "Alicia", *gotPatch.FirstName, "REQ-W01: patch field value preserved")
	assert.Nil(t, gotPatch.LastName, "REQ-W01 D-T3: unedited fields must remain nil through the service layer")

	assert.Equal(t, "Alicia", user.Profile.FirstName,
		"REQ-W01 AC-4.5: server response (via port) must propagate to the caller")
}

// AC-6 — BadRequestError flows through the service unchanged so the
// TUI can extract the Causes for inline display.
func Test_UsersService_UpdateProfile_PropagatesBadRequestError(t *testing.T) {
	t.Parallel()

	port := fakes.NewUsersPort(t)
	port.UpdateProfileFunc = fakes.ValidationErrorFake(map[string]string{
		"email":      "Email is not valid",
		"department": "Cannot exceed 100 characters",
	})
	svc := service.NewUsersService(port)

	newEmail := "bogus"
	_, err := svc.UpdateProfile(context.Background(), "00u_alice", domain.UserProfilePatch{Email: &newEmail})
	require.Error(t, err)

	var bre *domain.BadRequestError
	require.ErrorAs(t, err, &bre,
		"REQ-W01 AC-6: service must propagate *BadRequestError unchanged so the form can map Causes inline")
	assert.Len(t, bre.Causes, 2, "REQ-W01 AC-6.1: both causes must reach the caller")
}

// AC-6 — RateLimitedError flows through the service so EditModel can
// render the Retry-After countdown footer.
func Test_UsersService_UpdateProfile_PropagatesRateLimitedError(t *testing.T) {
	t.Parallel()

	port := fakes.NewUsersPort(t)
	port.UpdateProfileFunc = func(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
		return domain.User{}, &domain.RateLimitedError{RetryAfter: 0, Category: "management"}
	}
	svc := service.NewUsersService(port)

	v := "x"
	_, err := svc.UpdateProfile(context.Background(), "00u_alice", domain.UserProfilePatch{FirstName: &v})
	require.Error(t, err)

	var rle *domain.RateLimitedError
	require.ErrorAs(t, err, &rle,
		"REQ-W01 AC-6 429: service must propagate *RateLimitedError so EditModel renders the countdown")
}

// D-T5 / D-W13 — service surfaces ErrEmptyPatch when the patch is
// empty. (Port also guards it; we test the chain doesn't intercept.)
func Test_UsersService_UpdateProfile_EmptyPatch_PropagatesErrEmptyPatch(t *testing.T) {
	t.Parallel()

	called := false
	port := fakes.NewUsersPort(t)
	port.UpdateProfileFunc = func(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
		called = true
		return domain.User{}, domain.ErrEmptyPatch
	}
	svc := service.NewUsersService(port)

	_, err := svc.UpdateProfile(context.Background(), "00u_alice", domain.UserProfilePatch{})
	assert.ErrorIs(t, err, domain.ErrEmptyPatch,
		"REQ-W01 D-T5: empty patch must surface ErrEmptyPatch through the service")
	_ = called // service may either short-circuit or delegate; both are acceptable
}
