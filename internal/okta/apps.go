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

// List iterates /api/v1/apps with optional type-filter via the Okta
// `filter=` parameter (e.g., `filter=signOnMode eq "SAML_2_0"`).
// Powers the per-type list views (issue #166).
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
	return newPagedIterator(a.client, initial, decode), nil
}

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
	// Caller filter wins; type-narrow only when no explicit filter.
	switch {
	case q.Filter != "":
		v.Set("filter", q.Filter)
	case q.Type != "":
		v.Set("filter", `signOnMode eq "`+oktaSignOnModeFor(q.Type)+`"`)
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

// oktaSignOnModeFor returns the canonical signOnMode string for an
// AppType — the inverse of appTypeFromSignOnMode for the modes Okta
// accepts in `signOnMode eq …` filters.
func oktaSignOnModeFor(t domain.AppType) string {
	switch t {
	case domain.AppTypeSAML:
		return "SAML_2_0"
	case domain.AppTypeOIDC:
		return "OPENID_CONNECT"
	case domain.AppTypeBookmark:
		return "BOOKMARK"
	case domain.AppTypeSWA:
		return "AUTO_LOGIN"
	case domain.AppTypeSCIM:
		return "SCIM_2_0"
	}
	return ""
}

var _ domain.AppsPort = (*AppsAdapter)(nil)
