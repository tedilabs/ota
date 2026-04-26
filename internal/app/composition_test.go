package app_test

// QA-019/QA-020 regression — App Shell composes the active child Screen.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
)

// REQ-U05 — App.View() must include the body of the active child Screen.
// With InitialScreen=ScreenUsers and an empty UsersPort, the placeholder text
// produced by the Users ListModel ("Users") must appear in the composed View.
func Test_App_ChildScreenViewIsComposed(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers})
	out := m.View()
	assert.Contains(t, out, "Users",
		"Users list child View should appear in the composed App Shell View")
}

// REQ-U02 AC-1 — SwitchScreenMsg must materialize the target Screen so its
// View contributes to subsequent renders.
func Test_App_LazyInit_OnScreenSwitch(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers})
	require.True(t, m.HasScreen(app.ScreenUsers),
		"initial screen must be materialized eagerly")
	assert.False(t, m.HasScreen(app.ScreenGroups),
		"non-active screens are not yet materialized")

	updated, _ := m.Update(app.SwitchScreenMsg{Target: "groups"})
	got, ok := updated.(app.Model)
	require.True(t, ok)
	assert.True(t, got.HasScreen(app.ScreenGroups),
		"after SwitchScreenMsg, target screen must be cached for next View")

	out := got.View()
	assert.True(t, strings.Contains(out, "Groups"),
		"after switching to groups, groups child body must render in View")
}

// REQ-U05 AC-1 — OpenResourceMsg switches to the matching detail Screen.
func Test_App_OpenResourceMsg_ActivatesDetailScreen(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers})
	updated, _ := m.Update(app.OpenResourceMsg{Kind: "user", ID: "00u1"})
	got, ok := updated.(app.Model)
	require.True(t, ok)
	assert.Equal(t, "user-detail", app.ActiveScreenName(got))
}

// Sanity: a key press routed through the App Shell when no overlay is open
// reaches the child screen — verified indirectly by the model surviving the
// Update without panic and the active screen remaining unchanged.
func Test_App_KeyDelegationWithoutPanic(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got, ok := updated.(app.Model)
	require.True(t, ok)
	assert.Equal(t, app.ScreenUsers, got.Active())
}
