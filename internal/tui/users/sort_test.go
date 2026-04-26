package users_test

// v0.1.1 Red — TUI_DESIGN §3.5 (v1.2.0): column sort.
//
// Shift-modified letter keys cycle a column's sort direction:
//
//   off → asc → desc → off
//
// The bindings (Users):
//
//   Shift+S → STATUS
//   Shift+N → LOGIN (Name proxy)
//   Shift+L → LAST LOGIN
//   Shift+C → STATUS CHANGED
//
// Visual contract: the active sort column header carries an arrow indicator
// (`↑` for asc, `↓` for desc) immediately after the header text. When sort
// is OFF for every column the table renders in fetch order.
//
// Stability: two rows with equal sort-key values keep their original fetch
// order (stable sort). This protects operators from a confusing reshuffle
// when many rows share a status.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

// shiftKey crafts a tea.KeyMsg representing a Shift+letter chord.
//
// Bubbletea encodes uppercase letters as KeyRunes with the rune already
// uppercase — there is no separate Shift modifier on KeyMsg. We use the
// uppercase rune so test signal matches what a terminal sends when the
// operator presses Shift+S.
func shiftKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// Test_UsersList_SortByStatus_CyclesAscDescOff — Shift+S three times cycles
// none → asc → desc → none, with the header indicator following along.
func Test_UsersList_SortByStatus_CyclesAscDescOff(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
		Clock:        fixtureClock(),
	})

	// Initially: no indicator on STATUS header.
	view0 := testfx.StripANSI(m.View())
	assert.NotContains(t, headerLine(view0), "STATUS↑",
		"initial state must not show asc indicator")
	assert.NotContains(t, headerLine(view0), "STATUS↓",
		"initial state must not show desc indicator")

	// 1st press: asc.
	updated, _ := m.Update(shiftKey('S'))
	m = updated.(users.ListModel)
	view1 := testfx.StripANSI(m.View())
	assert.Contains(t, headerLine(view1), "STATUS↑",
		"after 1×Shift+S, STATUS header must carry asc indicator (↑)")

	// 2nd press: desc.
	updated, _ = m.Update(shiftKey('S'))
	m = updated.(users.ListModel)
	view2 := testfx.StripANSI(m.View())
	assert.Contains(t, headerLine(view2), "STATUS↓",
		"after 2×Shift+S, STATUS header must carry desc indicator (↓)")

	// 3rd press: off.
	updated, _ = m.Update(shiftKey('S'))
	m = updated.(users.ListModel)
	view3 := testfx.StripANSI(m.View())
	assert.NotContains(t, headerLine(view3), "STATUS↑",
		"after 3×Shift+S, STATUS asc indicator must be cleared")
	assert.NotContains(t, headerLine(view3), "STATUS↓",
		"after 3×Shift+S, STATUS desc indicator must be cleared")
}

// Test_UsersList_SortByLogin_NamePressKey — Shift+N sorts by LOGIN ascending.
func Test_UsersList_SortByLogin_NamePressKey(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
		Clock:        fixtureClock(),
	})
	updated, _ := m.Update(shiftKey('N'))
	m = updated.(users.ListModel)

	view := testfx.StripANSI(m.View())
	header := headerLine(view)
	assert.Contains(t, header, "LOGIN↑",
		"Shift+N must sort by LOGIN asc and place ↑ on the header")
}

// Test_UsersList_SortByLastLogin_PressKey — Shift+L sorts by LAST LOGIN.
func Test_UsersList_SortByLastLogin_PressKey(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
		Clock:        fixtureClock(),
	})
	updated, _ := m.Update(shiftKey('L'))
	m = updated.(users.ListModel)

	view := testfx.StripANSI(m.View())
	header := headerLine(view)
	assert.Contains(t, header, "LAST LOGIN↑",
		"Shift+L must sort by LAST LOGIN asc and place ↑ on the header")
}

// Test_UsersList_SortStableAcrossEqualKeys — two ACTIVE users keep their
// original fetch order when sorted by STATUS (since they share the value).
//
// Fixture has two ACTIVE rows in this order: alice, alan. After Shift+S, the
// ACTIVE rows must still appear as [alice, alan], not [alan, alice].
func Test_UsersList_SortStableAcrossEqualKeys(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
		Clock:        fixtureClock(),
	})
	updated, _ := m.Update(shiftKey('S'))
	m = updated.(users.ListModel)

	view := testfx.StripANSI(m.View())
	idxAlice := strings.Index(view, "alice@acme.com")
	idxAlan := strings.Index(view, "alan.turing@acme.com")
	require.GreaterOrEqual(t, idxAlice, 0, "alice row must render after sort")
	require.GreaterOrEqual(t, idxAlan, 0, "alan row must render after sort")
	assert.Less(t, idxAlice, idxAlan,
		"stable sort: ACTIVE rows must keep fetch order (alice before alan)")
}

// Test_UsersList_SortByStatus_PutsLockedNearTop — alphabetical asc on the
// status string. ACTIVE < LOCKED_OUT < STAGED < SUSPENDED. After Shift+S asc
// the first non-cursor row should still be ACTIVE (alice).
//
// We assert the relative position of two distinct status rows — alex
// (LOCKED_OUT) must come before aaron (SUSPENDED) when sorted asc.
func Test_UsersList_SortByStatus_AscOrder(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
		Clock:        fixtureClock(),
	})
	updated, _ := m.Update(shiftKey('S'))
	m = updated.(users.ListModel)

	view := testfx.StripANSI(m.View())
	idxLocked := strings.Index(view, "LOCKED_OUT")
	idxSuspended := strings.Index(view, "SUSPENDED")
	require.GreaterOrEqual(t, idxLocked, 0)
	require.GreaterOrEqual(t, idxSuspended, 0)
	assert.Less(t, idxLocked, idxSuspended,
		"asc sort by STATUS: LOCKED_OUT must precede SUSPENDED alphabetically")
}

// headerLine returns the line of the rendered View that contains "STATUS",
// "LOGIN", "LAST LOGIN", and "CHANGED" — the column headers row.
func headerLine(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "STATUS") && strings.Contains(line, "LOGIN") {
			return line
		}
	}
	return ""
}

// fixtureNow returns the canonical instant the Users goldens were authored
// against (Apr 24 2026 12:00 UTC). Used by sort/d-key tests so RelativeTime
// doesn't drift against the wall clock.
func fixtureNow() time.Time { return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC) }
