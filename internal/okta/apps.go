package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// AppsAdapter implements domain.AppsPort against /api/v1/apps.
type AppsAdapter struct{ client *Client }

// List iterates /api/v1/apps. The Okta `filter=` parameter on this
// endpoint only supports `user.id` / `group.id` / `name` predicates —
// NOT `signOnMode`, even though that's the field most operators want
// to scope by. Earlier ota builds tried `signOnMode eq "SAML_2_0"`
// and Okta rejected the request with E0000001 (issue: user reported
// "Failed to load Apps · Request rejected by Okta"). The fix is
// client-side: fetch the full app list once and filter by Type
// inside the iterator. Slightly slower for tenants with many apps
// but reliable.
func (a *AppsAdapter) List(ctx context.Context, q domain.AppsQuery) (domain.Iterator[domain.App], error) {
	initial := a.client.buildURL("/api/v1/apps" + buildAppsQuery(q))
	decode := func(raw json.RawMessage) (domain.App, error) {
		var wa wireApp
		if err := json.Unmarshal(raw, &wa); err != nil {
			return domain.App{}, err
		}
		// Preserve the wire bytes so the detail view's Raw tab has
		// something to render even when the adapter projects a
		// curated subset onto domain.App.
		out := mapApp(&wa)
		out.Raw = append(json.RawMessage(nil), raw...)
		return out, nil
	}
	base := newPagedIterator(a.client, initial, decode)
	if q.Type == "" {
		return base, nil
	}
	return &appTypeFilterIter{inner: base, want: q.Type}, nil
}

// appTypeFilterIter wraps a domain.Iterator[App] and surfaces only
// the rows whose Type matches `want`. Drops the rest so the upstream
// list / View renders a clean per-type slice.
type appTypeFilterIter struct {
	inner domain.Iterator[domain.App]
	want  domain.AppType
}

func (it *appTypeFilterIter) Next(ctx context.Context) (domain.App, bool, error) {
	for {
		app, hasMore, err := it.inner.Next(ctx)
		if err != nil || !hasMore {
			return app, hasMore, err
		}
		if app.Type == it.want {
			return app, true, nil
		}
	}
}
func (it *appTypeFilterIter) Close() error { return it.inner.Close() }

// Get fetches a single app instance by id.
func (a *AppsAdapter) Get(ctx context.Context, id string) (domain.App, error) {
	u := a.client.buildURL("/api/v1/apps/" + url.PathEscape(id))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.App{}, err
	}
	defer drainAndClose(resp)
	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		return domain.App{}, fmt.Errorf("okta: read body: %w", err)
	}
	var wa wireApp
	if err := json.Unmarshal(body.Bytes(), &wa); err != nil {
		return domain.App{}, fmt.Errorf("okta: decode app: %w", err)
	}
	out := mapApp(&wa)
	out.Raw = append(json.RawMessage(nil), body.Bytes()...)
	return out, nil
}

// wireApp mirrors the Okta /api/v1/apps response shape — minimum
// fields needed to populate the list view; Raw covers the rest.
type wireApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	SignOnMode  string `json:"signOnMode"`
	Created     string `json:"created"`
	LastUpdated string `json:"lastUpdated"`
}

func mapApp(wa *wireApp) domain.App {
	return domain.App{
		ID:          wa.ID,
		Name:        wa.Name,
		Label:       wa.Label,
		Status:      domain.AppStatus(wa.Status),
		SignOnMode:  wa.SignOnMode,
		Type:        appTypeFromSignOnMode(wa.SignOnMode),
		Created:     parseAppTime(wa.Created),
		LastUpdated: parseAppTime(wa.LastUpdated),
	}
}

// appTypeFromSignOnMode buckets each Okta signOnMode into the
// AppType enum the type-select picker uses. Unknown modes fall
// into AppTypeOther so the picker still surfaces them.
func appTypeFromSignOnMode(mode string) domain.AppType {
	switch mode {
	case "SAML_2_0":
		return domain.AppTypeSAML
	case "OPENID_CONNECT":
		return domain.AppTypeOIDC
	case "BOOKMARK":
		return domain.AppTypeBookmark
	case "AUTO_LOGIN", "BROWSER_PLUGIN", "SECURE_PASSWORD_STORE":
		return domain.AppTypeSWA
	case "SCIM_2_0", "SCIM_1_1":
		return domain.AppTypeSCIM
	}
	return domain.AppTypeOther
}

func parseAppTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func buildAppsQuery(q domain.AppsQuery) string {
	v := url.Values{}
	// Only Okta-supported predicates land on the wire; Type is
	// applied client-side via appTypeFilterIter (see List).
	if q.Filter != "" {
		v.Set("filter", q.Filter)
	}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	limit := q.Limit
	if limit == 0 {
		limit = 200
	}
	v.Set("limit", strconv.Itoa(limit))
	if q.After != "" {
		v.Set("after", q.After)
	}
	return "?" + v.Encode()
}

var _ domain.AppsPort = (*AppsAdapter)(nil)
