package okta

import (
	"context"
	"net/url"
)

// Activate posts to /api/v1/policies/{id}/lifecycle/activate.
// System policies refuse with 403; the errormap layer translates.
func (a *PoliciesAdapter) Activate(ctx context.Context, policyID string) error {
	u := a.client.buildURL("/api/v1/policies/" + url.PathEscape(policyID) + "/lifecycle/activate")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}

// Deactivate posts to /api/v1/policies/{id}/lifecycle/deactivate.
func (a *PoliciesAdapter) Deactivate(ctx context.Context, policyID string) error {
	u := a.client.buildURL("/api/v1/policies/" + url.PathEscape(policyID) + "/lifecycle/deactivate")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}
