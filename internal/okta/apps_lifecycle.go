package okta

import (
	"context"
	"net/url"
)

// Activate posts to /api/v1/apps/{id}/lifecycle/activate. Idempotent
// — Okta returns 200 when the app is already ACTIVE.
func (a *AppsAdapter) Activate(ctx context.Context, appID string) error {
	u := a.client.buildURL("/api/v1/apps/" + url.PathEscape(appID) + "/lifecycle/activate")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}

// Deactivate posts to /api/v1/apps/{id}/lifecycle/deactivate.
func (a *AppsAdapter) Deactivate(ctx context.Context, appID string) error {
	u := a.client.buildURL("/api/v1/apps/" + url.PathEscape(appID) + "/lifecycle/deactivate")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}
