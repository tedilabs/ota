package rules_test

// v0.1.1 Red — TUI_DESIGN §3.5 (v1.2.0): column sort on Group Rules list.
//
// Bindings (Rules):
//
//   Shift+S → STATUS
//   Shift+N → NAME
//   Shift+U → UPDATED
//
// Same cycle (off → asc → desc → off) and `↑/↓` indicator contract as
// Users / Groups.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/rules"
)

// Test_RulesList_SortByStatus_CyclesAscDescOff — Shift+S cycles asc → desc → off.
func Test_RulesList_SortByStatus_CyclesAscDescOff(t *testing.T) {
	t.Parallel()

	m := rules.NewListModel(rules.Deps{
		InitialRules: sampleRulesFixture(),
		Width:        120,
		Height:       30,
	})

	view0 := testfx.StripANSI(m.View())
	assert.NotContains(t, headerLine(view0), "STATUS↑",
		"initial state must not show asc indicator on STATUS")

	updated, _ := m.Update(shiftKey('S'))
	m = updated.(rules.ListModel)
	view1 := testfx.StripANSI(m.View())
	assert.Contains(t, headerLine(view1), "STATUS↑",
		"after 1×Shift+S, STATUS header must carry asc indicator")

	updated, _ = m.Update(shiftKey('S'))
	m = updated.(rules.ListModel)
	view2 := testfx.StripANSI(m.View())
	assert.Contains(t, headerLine(view2), "STATUS↓",
		"after 2×Shift+S, STATUS header must carry desc indicator")

	updated, _ = m.Update(shiftKey('S'))
	m = updated.(rules.ListModel)
	view3 := testfx.StripANSI(m.View())
	assert.NotContains(t, headerLine(view3), "STATUS↑",
		"after 3×Shift+S, asc indicator cleared")
	assert.NotContains(t, headerLine(view3), "STATUS↓",
		"after 3×Shift+S, desc indicator cleared")
}

// Test_RulesList_SortByStatus_AscOrder — operational rank per §3.5a:
// INVALID first (so broken rules surface at the top), then ACTIVE, then
// INACTIVE. The rationale is `§0.4 (상태는 숨기지 않는다)` — alphabetical
// would bury INVALID in the middle.
func Test_RulesList_SortByStatus_AscOrder(t *testing.T) {
	t.Parallel()

	m := rules.NewListModel(rules.Deps{
		InitialRules: sampleRulesFixture(),
		Width:        120,
		Height:       30,
	})
	updated, _ := m.Update(shiftKey('S'))
	m = updated.(rules.ListModel)

	view := testfx.StripANSI(m.View())
	// v0.1.12 dropped the [+]/[!]/[X] mono prefix (issue #156); the
	// status column now reads as a centered bare label. INACTIVE
	// is a substring of "INACTIVE" so we anchor on whitespace
	// boundaries when finding ACTIVE.
	idxInvalid := strings.Index(view, "INVALID")
	idxActive := strings.Index(view, "  ACTIVE  ")
	idxInactive := strings.Index(view, "INACTIVE")
	assert.Less(t, idxInvalid, idxActive, "asc rank: INVALID before ACTIVE (§3.5a)")
	assert.Less(t, idxActive, idxInactive, "asc rank: ACTIVE before INACTIVE (§3.5a)")
}

func shiftKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func headerLine(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "STATUS") && strings.Contains(line, "NAME") {
			return line
		}
	}
	return ""
}
