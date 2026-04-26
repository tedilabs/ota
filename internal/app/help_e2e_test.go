package app_test

// E2E for the `?` help shortcut: pressing it must open a modal whose body
// surfaces both global keys and the keys specific to the active screen,
// and Esc must close it. Reproduces the user's request that `?` show the
// shortcuts available on the current view.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/keys"
)

func newHelpTestModel(t *testing.T) app.Model {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	return app.New(app.Deps{
		Keys:    keymap,
		Clock:   clock.Real(),
		Profile: "test",
		OrgURL:  "https://acme.okta.com",
	})
}

// pressKey feeds a single rune through the App Shell's tea.Update and, when
// the resulting Cmd produces a follow-up Msg (the typical pattern for the
// open*Cmd Cmds), runs that Msg through Update too. Mirrors what tea.Program
// does at runtime so the test can observe the final post-cmd state.
func pressKey(t *testing.T, m app.Model, r rune) app.Model {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	mdl, ok := updated.(app.Model)
	require.True(t, ok, "Update must return an app.Model")
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = mdl.Update(msg)
			mdl, ok = updated.(app.Model)
			require.True(t, ok, "follow-up Update must also return an app.Model")
		}
	}
	return mdl
}

// Test_AppShell_QuestionMark_OpensFullHelpModal locks in the user-visible
// behaviour: `?` replaces the body with the screen-scoped HelpModel modal
// (chrome RoundedBorder + key reference table), not just a footer hint.
func Test_AppShell_QuestionMark_OpensFullHelpModal(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')

	view := m.View()
	// Modal title — proves the full HelpModel rendered, not the placeholder.
	assert.Contains(t, view, "Help · Users List",
		"`?` must open the Users-scoped help modal as the body")
	// Sort keys are Users-specific and must be visible.
	for _, key := range []string{"Shift+S", "Shift+N", "Shift+L", "Shift+C"} {
		assert.Contains(t, view, key,
			"Users help modal must list %q sort key", key)
	}
	// Modal hint footer.
	assert.Contains(t, view, "<Esc> close")
}

// Test_AppShell_HelpEsc_ClosesAndReturnsToList verifies the close path.
func Test_AppShell_HelpEsc_ClosesAndReturnsToList(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')
	require.Contains(t, m.View(), "Help · Users List", "precondition: help open")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(app.Model)

	view := m.View()
	assert.NotContains(t, view, "Help · Users List",
		"Esc must close the help modal")
}

// Test_AppShell_QuestionMarkToggle verifies pressing `?` again closes the
// modal — operators expect symmetric open/close.
func Test_AppShell_QuestionMarkToggle(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')
	require.Contains(t, m.View(), "Help · Users List", "precondition: help open")

	m = pressKey(t, m, '?')
	assert.NotContains(t, m.View(), "Help · Users List",
		"second `?` must close the modal")
}

// Test_AppShell_HelpInternalFilter verifies `/` inside the help modal narrows
// the displayed entries — the `?` overlay's own search.
func Test_AppShell_HelpInternalFilter(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')

	// Open the help filter and type "sort".
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(app.Model)
	for _, r := range "sort" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(app.Model)
	}

	view := m.View()
	// After filter "sort", non-sort entries (e.g. "open command palette")
	// drop out while sort keys remain.
	assert.False(t, strings.Contains(view, "open command palette"),
		"filter \"sort\" must drop unrelated rows")
	assert.Contains(t, view, "Shift+S",
		"filter \"sort\" must keep sort entries visible")
}
