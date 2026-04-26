package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/tedilabs/ota/internal/domain"
)

// UsersAdapter implements domain.UsersPort.
type UsersAdapter struct{ client *Client }

// List iterates the /api/v1/users endpoint using Link-header pagination
// (REQ-R01 AC-4 / PRD §7.3). 429 responses are retried automatically by
// Client.doGet per REQ-E01 AC-2.
func (a *UsersAdapter) List(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
	initial := a.client.buildURL("/api/v1/users" + buildUsersQuery(q))
	decode := func(raw json.RawMessage) (domain.User, error) {
		var wu wireUser
		if err := json.Unmarshal(raw, &wu); err != nil {
			return domain.User{}, err
		}
		return mapUser(&wu), nil
	}
	return newPagedIterator(a.client, initial, decode), nil
}

// Get fetches a single user by id or login.
func (a *UsersAdapter) Get(ctx context.Context, idOrLogin string) (domain.User, error) {
	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(idOrLogin))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.User{}, err
	}
	defer drainAndClose(resp)

	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		return domain.User{}, fmt.Errorf("okta: read body: %w", err)
	}
	var wu wireUser
	if err := json.Unmarshal(body.Bytes(), &wu); err != nil {
		return domain.User{}, fmt.Errorf("okta: decode user: %w", err)
	}
	return mapUser(&wu), nil
}

// ListGroups returns the groups a user belongs to.
func (a *UsersAdapter) ListGroups(ctx context.Context, userID string) ([]domain.Group, error) {
	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(userID) + "/groups")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)

	var raws []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raws); err != nil {
		return nil, fmt.Errorf("okta: decode groups: %w", err)
	}
	out := make([]domain.Group, 0, len(raws))
	for _, r := range raws {
		var wg wireGroup
		if err := json.Unmarshal(r, &wg); err != nil {
			return nil, fmt.Errorf("okta: decode group: %w", err)
		}
		out = append(out, mapGroup(&wg))
	}
	return out, nil
}

// ListFactors returns the user's registered MFA factors (REQ-R01 AC-6).
func (a *UsersAdapter) ListFactors(ctx context.Context, userID string) ([]domain.Factor, error) {
	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(userID) + "/factors")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)

	var raws []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raws); err != nil {
		return nil, fmt.Errorf("okta: decode factors: %w", err)
	}
	out := make([]domain.Factor, 0, len(raws))
	for _, r := range raws {
		var wf wireFactor
		if err := json.Unmarshal(r, &wf); err != nil {
			return nil, fmt.Errorf("okta: decode factor: %w", err)
		}
		out = append(out, mapFactor(&wf))
	}
	return out, nil
}

// buildUsersQuery constructs the query string for /api/v1/users given q.
// Empty fields are omitted. The default limit is 200 when unset (REQ-R01 AC-4).
func buildUsersQuery(q domain.UsersQuery) string {
	v := url.Values{}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Search != "" {
		v.Set("search", q.Search)
	}
	if q.Filter != "" {
		v.Set("filter", q.Filter)
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

// ResetPassword issues POST /api/v1/users/{id}/lifecycle/reset_password
// (Okta Lifecycle API — issue #125). When sendEmail=true the operator
// flow ends with the user receiving a reset email; when false the
// response carries a one-time `resetPasswordUrl` the operator can
// share manually. Returns "" when sendEmail=true.
func (a *UsersAdapter) ResetPassword(ctx context.Context, userID string, sendEmail bool) (string, error) {
	q := url.Values{}
	q.Set("sendEmail", strconv.FormatBool(sendEmail))
	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(userID) +
		"/lifecycle/reset_password?" + q.Encode())
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return "", err
	}
	defer drainAndClose(resp)
	if sendEmail {
		return "", nil
	}
	var body struct {
		ResetPasswordURL string `json:"resetPasswordUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("okta: decode reset_password: %w", err)
	}
	return body.ResetPasswordURL, nil
}

// Unlock issues POST /api/v1/users/{id}/lifecycle/unlock — clears the
// LOCKED_OUT state. Okta returns 200 even when the user is already
// unlocked, so callers don't need to special-case the response.
func (a *UsersAdapter) Unlock(ctx context.Context, userID string) error {
	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(userID) + "/lifecycle/unlock")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}

// ResetFactors issues POST /api/v1/users/{id}/lifecycle/reset_factors
// — removes every enrolled MFA factor on the user, forcing
// re-enrollment on next sign-in. Destructive: the App Shell wraps
// this call in a confirmation modal.
func (a *UsersAdapter) ResetFactors(ctx context.Context, userID string) error {
	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(userID) + "/lifecycle/reset_factors")
	resp, err := a.client.doPost(ctx, u, nil)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}

var _ domain.UsersPort = (*UsersAdapter)(nil)
