package groups_test

// Phase 6d — Visual lock-in for SCR-020 (Groups List). Mirror of the users
// golden_test.go layering: golden snapshots Skipped while Phase 6d-3..5 land,
// substring spec lock-in tests Active and Fail-First.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/groups"
)

func init() { testfx.PinTestEnvironment() }

// sampleGroupsFixture mirrors TUI_DESIGN §16.5 — Engineering (rule-targeted),
// Jira Users (app), Everyone (built-in / large).
func sampleGroupsFixture() []domain.Group {
	return []domain.Group{
		{
			ID:              "00g_eng",
			Type:            domain.GroupTypeOkta,
			Profile:         domain.GroupProfile{Name: "Engineering", Description: "All engineers"},
			DynamicTargeted: true,
		},
		{
			ID:      "00g_jira",
			Type:    domain.GroupTypeApp,
			Profile: domain.GroupProfile{Name: "Jira Users", Description: "Synced from Atlassian"},
		},
		{
			ID:      "00g_everyone",
			Type:    domain.GroupTypeBuiltIn,
			Profile: domain.GroupProfile{Name: "Everyone", Description: "All organization members"},
		},
	}
}

// --- Golden snapshots --------------------------------------------------------

func Test_GroupsListGolden_Default(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	m := groups.NewListModel(groups.Deps{InitialGroups: sampleGroupsFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/list_default.txt")
}

func Test_GroupsDetailGolden_RuleTargeted(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

func Test_GroupsDetailGolden_LargeBuiltin(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

// --- Spec lock-in (Active, Fail-First) --------------------------------------

// Test_GroupsList_HasColumnHeaders locks in TUI_DESIGN §16.5: TYPE / NAME /
// DESCRIPTION / UPDATED / TAGS column headers.
func Test_GroupsList_HasColumnHeaders(t *testing.T) {
	t.Parallel()
	m := groups.NewListModel(groups.Deps{InitialGroups: sampleGroupsFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	for _, h := range []string{"TYPE", "NAME", "DESCRIPTION", "UPDATED"} {
		assert.Contains(t, got, h, "Groups list must show %q column header (TUI_DESIGN §16.5)", h)
	}
}

// Test_GroupsList_RuleTargetedShowsBadge locks in REQ-R02 AC-1: dynamic-targeted
// groups display a [RULE] badge in the TAGS column.
func Test_GroupsList_RuleTargetedShowsBadge(t *testing.T) {
	t.Parallel()
	m := groups.NewListModel(groups.Deps{InitialGroups: sampleGroupsFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "[RULE]",
		"groups targeted by Group Rules must show the [RULE] badge (REQ-R02 AC-1 / TUI_DESIGN §16.5)")
}

// Test_GroupsList_LargeBuiltinShowsBadges locks in TUI_DESIGN §16.5: built-in
// groups (e.g., Everyone) carry [SYS][LARGE] badges.
func Test_GroupsList_LargeBuiltinShowsBadges(t *testing.T) {
	t.Parallel()
	m := groups.NewListModel(groups.Deps{InitialGroups: sampleGroupsFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "[SYS]", "built-in group must show [SYS] badge (TUI_DESIGN §16.5)")
	assert.Contains(t, got, "[LARGE]", "built-in (Everyone) must show [LARGE] warning (TUI_DESIGN §16.5)")
}
