package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/tedilabs/ota/internal/domain"
)

// GroupRulesAdapter implements domain.GroupRulesPort.
type GroupRulesAdapter struct{ client *Client }

// List iterates /api/v1/groups/rules. Default limit 200 (PRD §7.3).
func (a *GroupRulesAdapter) List(ctx context.Context, q domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
	v := url.Values{}
	limit := q.Limit
	if limit == 0 {
		limit = 200
	}
	v.Set("limit", strconv.Itoa(limit))
	if q.After != "" {
		v.Set("after", q.After)
	}
	initial := a.client.buildURL("/api/v1/groups/rules?" + v.Encode())
	decode := func(raw json.RawMessage) (domain.GroupRule, error) {
		var wr wireGroupRule
		if err := json.Unmarshal(raw, &wr); err != nil {
			return domain.GroupRule{}, err
		}
		return mapGroupRule(&wr), nil
	}
	return newPagedIterator(a.client, initial, decode), nil
}

// Get fetches a single rule.
func (a *GroupRulesAdapter) Get(ctx context.Context, id string) (domain.GroupRule, error) {
	u := a.client.buildURL("/api/v1/groups/rules/" + url.PathEscape(id))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.GroupRule{}, err
	}
	defer drainAndClose(resp)
	var wr wireGroupRule
	if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
		return domain.GroupRule{}, fmt.Errorf("okta: decode rule: %w", err)
	}
	return mapGroupRule(&wr), nil
}

var _ domain.GroupRulesPort = (*GroupRulesAdapter)(nil)
