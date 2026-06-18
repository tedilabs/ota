package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdatePolicy issues PUT /api/v1/policies/{policyID} with the
// strict-replace body Okta requires. Editable in this v0.2 scope:
// name, description, priority, status. System policies refuse
// status / priority changes — Okta returns 400; the screen lets the
// 400 ride through the standard inline error mapping.
func (a *PoliciesAdapter) UpdatePolicy(ctx context.Context, policyID string, update domain.PolicyUpdate) (domain.Policy, error) {
	body, err := json.Marshal(wirePolicyUpdateBody{
		Name:        update.Name,
		Description: update.Description,
		Priority:    update.Priority,
		Status:      string(update.Status),
	})
	if err != nil {
		return domain.Policy{}, fmt.Errorf("okta: marshal policy update body: %w", err)
	}

	u := a.client.buildURL("/api/v1/policies/" + url.PathEscape(policyID))
	resp, err := a.client.doPut(ctx, u, body)
	if err != nil {
		return domain.Policy{}, err
	}
	defer drainAndClose(resp)

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return domain.Policy{}, fmt.Errorf("okta: read policy update response: %w", err)
	}
	raw := json.RawMessage(buf.Bytes())
	var wp wirePolicy
	if err := json.Unmarshal(buf.Bytes(), &wp); err != nil {
		return domain.Policy{}, fmt.Errorf("okta: decode policy update response: %w", err)
	}
	return mapPolicy(&wp, raw), nil
}

// wirePolicyUpdateBody is the request envelope for
// PUT /api/v1/policies/{id}. Fields aligned with Okta's wire schema —
// strict-replace, no omitempty (the screen always supplies current
// values for unchanged fields).
type wirePolicyUpdateBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
}
