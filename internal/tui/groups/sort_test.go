package groups_test

// v0.1.1 Red — TUI_DESIGN §3.5 (v1.2.0): column sort on Groups list.
//
// Bindings (Groups):
//
//   Shift+N → NAME
//   Shift+T → TYPE
//   Shift+U → UPDATED
//
// Same cycle (off → asc → desc → off) and same `↑/↓` indicator contract as
// the Users list (TUI_DESIGN §3.5). These tests Red until v0.1.1-4 wires the
// sort handler into groups.ListModel.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/groups"
)

// Test_GroupsList_SortByName_CyclesAscDescOff — Shift+N cycles asc → desc → off.
func Test_GroupsList_SortByName_CyclesAscDescOff(t *testing.T) {
	t.Parallel()

	m := groups.NewListModel(groups.Deps{
		InitialGroups: sampleGroupsFixture(),
		Width:         120,
		Height:        30,
	})

	// State 0: no indicator.
	view0 := testfx.StripANSI(m.View())
	assert.NotContains(t, headerLine(view0), "NAME↑",
		"initial state must not show asc indicator on NAME")

	// 1×Shift+N → asc.
	updated, _ := m.Update(shiftKey('N'))
	m = updated.(groups.ListModel)
	view1 := testfx.StripANSI(m.View())
	assert.Contains(t, headerLine(view1), "NAME↑",
		"after 1×Shift+N, NAME header must carry asc indicator")

	// 2×Shift+N → desc.
	updated, _ = m.Update(shiftKey('N'))
	m = updated.(groups.ListModel)
	view2 := testfx.StripANSI(m.View())
	assert.Contains(t, headerLine(view2), "NAME↓",
		"after 2×Shift+N, NAME header must carry desc indicator")

	// 3×Shift+N → off.
	updated, _ = m.Update(shiftKey('N'))
	m = updated.(groups.ListModel)
	view3 := testfx.StripANSI(m.View())
	assert.NotContains(t, headerLine(view3), "NAME↑",
		"after 3×Shift+N, asc indicator cleared")
	assert.NotContains(t, headerLine(view3), "NAME↓",
		"after 3×Shift+N, desc indicator cleared")
}

// Test_GroupsList_SortByName_AscOrder — alphabetical asc on NAME. Fixture has
// Engineering, Jira Users, Everyone — sorted asc: Engineering, Everyone, Jira.
func Test_GroupsList_SortByName_AscOrder(t *testing.T) {
	t.Parallel()

	m := groups.NewListModel(groups.Deps{
		InitialGroups: sampleGroupsFixture(),
		Width:         120,
		Height:        30,
	})
	updated, _ := m.Update(shiftKey('N'))
	m = updated.(groups.ListModel)

	view := testfx.StripANSI(m.View())
	idxEng := strings.Index(view, "Engineering")
	idxEve := strings.Index(view, "Everyone")
	idxJira := strings.Index(view, "Jira Users")
	assert.Less(t, idxEng, idxEve, "asc by NAME: Engineering before Everyone")
	assert.Less(t, idxEve, idxJira, "asc by NAME: Everyone before Jira Users")
}

// shiftKey crafts a tea.KeyMsg representing a Shift+letter chord (uppercase
// rune — bubbletea's wire format for shifted alphabetic keys).
func shiftKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// headerLine returns the column-headers row of the rendered View.
func headerLine(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "TYPE") && strings.Contains(line, "NAME") {
			return line
		}
	}
	return ""
}
