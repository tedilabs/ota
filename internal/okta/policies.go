package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/tedilabs/ota/internal/domain"
)

// PoliciesAdapter implements domain.PoliciesPort.
type PoliciesAdapter struct{ client *Client }

// List fetches policies of a given type. /api/v1/policies requires `type=`
// (REQ-R04 AC-2). MVP default limit is 20 (PRD §7.3).
func (a *PoliciesAdapter) List(ctx context.Context, q domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
	if q.Type == "" {
		return nil, errors.New("okta: policies list requires Type")
	}
	v := url.Values{}
	v.Set("type", string(q.Type))
	limit := q.Limit
	if limit == 0 {
		limit = 20
	}
	v.Set("limit", strconv.Itoa(limit))
	if q.After != "" {
		v.Set("after", q.After)
	}
	initial := a.client.buildURL("/api/v1/policies?" + v.Encode())
	decode := func(raw json.RawMessage) (domain.Policy, error) {
		var wp wirePolicy
		if err := json.Unmarshal(raw, &wp); err != nil {
			return domain.Policy{}, err
		}
		return mapPolicy(&wp, raw), nil
	}
	return newPagedIterator(a.client, initial, decode), nil
}

// Get fetches a single policy by id.
func (a *PoliciesAdapter) Get(ctx context.Context, id string) (domain.Policy, error) {
	u := a.client.buildURL("/api/v1/policies/" + url.PathEscape(id))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.Policy{}, err
	}
	defer drainAndClose(resp)

	var raw bytes.Buffer
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		return domain.Policy{}, fmt.Errorf("okta: read policy: %w", err)
	}
	var wp wirePolicy
	if err := json.Unmarshal(raw.Bytes(), &wp); err != nil {
		return domain.Policy{}, fmt.Errorf("okta: decode policy: %w", err)
	}
	return mapPolicy(&wp, json.RawMessage(raw.Bytes())), nil
}

// Rules returns a policy's rules ordered by priority.
func (a *PoliciesAdapter) Rules(ctx context.Context, policyID string) ([]domain.PolicyRule, error) {
	u := a.client.buildURL("/api/v1/policies/" + url.PathEscape(policyID) + "/rules")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)

	var raws []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raws); err != nil {
		return nil, fmt.Errorf("okta: decode policy rules: %w", err)
	}
	out := make([]domain.PolicyRule, 0, len(raws))
	for _, r := range raws {
		var wr wirePolicyRule
		if err := json.Unmarshal(r, &wr); err != nil {
			return nil, fmt.Errorf("okta: decode policy rule: %w", err)
		}
		out = append(out, mapPolicyRule(&wr, r))
	}
	return out, nil
}

var _ domain.PoliciesPort = (*PoliciesAdapter)(nil)
