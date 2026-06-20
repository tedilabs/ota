package okta

import (
	"context"
	"net/url"
)

// Activate flips an authenticator to ACTIVE. Org-wide change —
// every end user enrolling after this can use the factor.
func (a *AuthenticatorsAdapter) Activate(ctx context.Context, authenticatorID string) error {
	u := a.client.buildURL("/api/v1/authenticators/" + url.PathEscape(authenticatorID) + "/lifecycle/activate")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}

// Deactivate flips an authenticator to INACTIVE. Existing user
// enrollments remain on the user record; new sign-in challenges
// drop this factor.
func (a *AuthenticatorsAdapter) Deactivate(ctx context.Context, authenticatorID string) error {
	u := a.client.buildURL("/api/v1/authenticators/" + url.PathEscape(authenticatorID) + "/lifecycle/deactivate")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}
