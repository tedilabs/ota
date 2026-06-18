package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdateRule issues PUT /api/v1/groups/rules/{ruleID} with the
// strict-replace body Okta requires for rule edits. Rules must be
// INACTIVE / INVALID before the call; callers guard upstream
// (Okta returns 400 INVALID_RULE_STATE for an ACTIVE rule).
func (a *GroupRulesAdapter) UpdateRule(ctx context.Context, ruleID string, update domain.GroupRuleUpdate) (domain.GroupRule, error) {
	body, err := json.Marshal(wireGroupRuleUpdateBody{
		Name: update.Name,
		Type: "group_rule", // Okta-required wire constant
		Conditions: wireGroupRuleConditions{
			Expression: wireGroupRuleExpression{
				Value: update.Expression,
				Type:  "urn:okta:expression:1.0",
			},
		},
		Actions: wireGroupRuleActions{
			AssignUserToGroups: wireGroupRuleAssign{
				GroupIDs: update.TargetGroupIDs,
			},
		},
	})
	if err != nil {
		return domain.GroupRule{}, fmt.Errorf("okta: marshal rule update body: %w", err)
	}

	u := a.client.buildURL("/api/v1/groups/rules/" + url.PathEscape(ruleID))
	resp, err := a.client.doPut(ctx, u, body)
	if err != nil {
		return domain.GroupRule{}, err
	}
	defer drainAndClose(resp)

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return domain.GroupRule{}, fmt.Errorf("okta: read rule update response: %w", err)
	}
	var wr wireGroupRule
	if err := json.Unmarshal(buf.Bytes(), &wr); err != nil {
		return domain.GroupRule{}, fmt.Errorf("okta: decode rule update response: %w", err)
	}
	return mapGroupRule(&wr), nil
}

// wireGroupRuleUpdateBody is the strict-replace envelope for
// PUT /api/v1/groups/rules/{id}. Type is the Okta-required discriminator
// (always "group_rule" for the rule kind ota supports).
type wireGroupRuleUpdateBody struct {
	Name       string                  `json:"name"`
	Type       string                  `json:"type"`
	Conditions wireGroupRuleConditions `json:"conditions"`
	Actions    wireGroupRuleActions    `json:"actions"`
}

type wireGroupRuleConditions struct {
	Expression wireGroupRuleExpression `json:"expression"`
}

type wireGroupRuleExpression struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type wireGroupRuleActions struct {
	AssignUserToGroups wireGroupRuleAssign `json:"assignUserToGroups"`
}

type wireGroupRuleAssign struct {
	GroupIDs []string `json:"groupIds"`
}
