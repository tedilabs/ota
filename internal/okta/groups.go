package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/tedilabs/ota/internal/domain"
)

// GroupsAdapter implements domain.GroupsPort.
type GroupsAdapter struct{ client *Client }

// List iterates /api/v1/groups with Link-header pagination.
func (a *GroupsAdapter) List(ctx context.Context, q domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
	initial := a.client.buildURL("/api/v1/groups" + buildGroupsQuery(q))
	decode := func(raw json.RawMessage) (domain.Group, error) {
		var wg wireGroup
		if err := json.Unmarshal(raw, &wg); err != nil {
			return domain.Group{}, err
		}
		return mapGroup(&wg), nil
	}
	return newPagedIterator(a.client, initial, decode), nil
}

// Get fetches a single group.
func (a *GroupsAdapter) Get(ctx context.Context, id string) (domain.Group, error) {
	u := a.client.buildURL("/api/v1/groups/" + url.PathEscape(id))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.Group{}, err
	}
	defer drainAndClose(resp)
	var wg wireGroup
	if err := json.NewDecoder(resp.Body).Decode(&wg); err != nil {
		return domain.Group{}, fmt.Errorf("okta: decode group: %w", err)
	}
	return mapGroup(&wg), nil
}

// Members iterates /api/v1/groups/{id}/users.
func (a *GroupsAdapter) Members(ctx context.Context, q domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
	v := url.Values{}
	limit := q.Limit
	if limit == 0 {
		limit = 200
	}
	v.Set("limit", strconv.Itoa(limit))
	if q.After != "" {
		v.Set("after", q.After)
	}
	initial := a.client.buildURL("/api/v1/groups/" + url.PathEscape(q.GroupID) + "/users?" + v.Encode())
	decode := func(raw json.RawMessage) (domain.User, error) {
		var wu wireUser
		if err := json.Unmarshal(raw, &wu); err != nil {
			return domain.User{}, err
		}
		return mapUser(&wu), nil
	}
	return newPagedIterator(a.client, initial, decode), nil
}

// AppCount returns the number of apps assigned to a group. MVP returns the
// first-page count; v0.2 can extend this with full pagination (PRD REQ-R02 AC-4).
func (a *GroupsAdapter) AppCount(ctx context.Context, id string) (int, error) {
	u := a.client.buildURL("/api/v1/groups/" + url.PathEscape(id) + "/apps?limit=200")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return 0, err
	}
	defer drainAndClose(resp)
	var raws []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raws); err != nil {
		return 0, fmt.Errorf("okta: decode apps: %w", err)
	}
	return len(raws), nil
}

func buildGroupsQuery(q domain.GroupsQuery) string {
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
	// expand=stats wires the _embedded.stats.usersCount the list
	// surfaces in the MEMBERS column (issue #161).
	v.Set("expand", "stats")
	return "?" + v.Encode()
}

var _ domain.GroupsPort = (*GroupsAdapter)(nil)
