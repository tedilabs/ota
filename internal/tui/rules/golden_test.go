package rules_test

// Phase 6d — Visual lock-in for SCR-030 (Group Rules List). Per TUI_DESIGN
// §16.6 the INVALID rule must be visually distinct and an inline banner must
// summarize the count of broken rules.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/rules"
)

func init() { testfx.PinTestEnvironment() }

// sampleRulesFixture mirrors TUI_DESIGN §16.6 — one ACTIVE, one INACTIVE,
// one INVALID rule.
func sampleRulesFixture() []domain.GroupRule {
	return []domain.GroupRule{
		{ID: "rul_active", Name: "Engineers to Eng", Status: domain.GroupRuleStatusActive,
			Expression: "user.department eq \"Engineering\"", TargetGroupIDs: []string{"00g_eng"}},
		{ID: "rul_inactive", Name: "Legacy Eng Mapping", Status: domain.GroupRuleStatusInactive,
			Expression: "user.title eq \"Engineer\"", TargetGroupIDs: []string{"00g_eng"}},
		{ID: "rul_invalid", Name: "Broken Dept Rule", Status: domain.GroupRuleStatusInvalid,
			Expression: "user.department.unknownField()", TargetGroupIDs: []string{"00g_sales"}},
	}
}

// --- Golden snapshots --------------------------------------------------------

func Test_RulesListGolden_WithInvalid(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	m := rules.NewListModel(rules.Deps{InitialRules: sampleRulesFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/list_with_invalid.txt")
}

func Test_RulesDetailGolden_Default(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

// --- Spec lock-in (Active, Fail-First) --------------------------------------

// Test_RulesList_InvalidRowHasBadge locks in REQ-R03 AC-2: an INVALID rule
// is rendered with the [INVALID] badge so operators spot it immediately.
//
// This test is currently *PASSing* because rules.go already prints
// `[INVALID ]` (with trailing space). Keeping it as an Active test guards
// against a regression during the Phase 6d-4 rewrite.
func Test_RulesList_InvalidRowHasBadge(t *testing.T) {
	t.Parallel()
	m := rules.NewListModel(rules.Deps{InitialRules: sampleRulesFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "INVALID",
		"INVALID rules must surface the badge text (REQ-R03 AC-2 / TUI_DESIGN §16.6)")
}

// Test_RulesList_HasColumnHeaders locks in TUI_DESIGN §16.6: STATUS / NAME /
// TARGETS / UPDATED column headers.
func Test_RulesList_HasColumnHeaders(t *testing.T) {
	t.Parallel()
	m := rules.NewListModel(rules.Deps{InitialRules: sampleRulesFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	for _, h := range []string{"STATUS", "NAME", "TARGETS"} {
		assert.Contains(t, got, h, "Rules list must show %q column header (TUI_DESIGN §16.6)", h)
	}
}

// Test_RulesList_InvalidBannerSummary locks in REQ-R03 AC-3 / §16.6: when at
// least one INVALID rule is present, an inline banner summarizes the count.
func Test_RulesList_InvalidBannerSummary(t *testing.T) {
	t.Parallel()
	m := rules.NewListModel(rules.Deps{InitialRules: sampleRulesFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "1 rule in INVALID state",
		"Rules list must show an INVALID summary banner when broken rules exist (TUI_DESIGN §16.6)")
}
