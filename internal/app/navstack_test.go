package app_test

// Tests for the screen-level navigation stack (Android activity-history
// semantics). The contract:
//
//   - ':' palette commands replace the entire stack with the chosen
//     resource as the new root.
//   - Cross-resource drill-downs (OpenResourceMsg, OpenLogsMsg, …)
//     push frames onto the stack instead of replacing the active
//     screen.
//   - Esc with nothing locally to close pops the top frame.
//   - Popping the root frame fires the quit confirm overlay.
//
// We assert via observable behaviour (active screen + presence of the
// quit confirm modal in View()) — the navStack field itself is
// unexported by design.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// newNavTestModel constructs a minimal App Shell with a Users seed
// port + WindowSizeMsg so the nav-stack flows below have a backing
// Users screen they can switch onto. Detail-state ports for Logs /
// Groups / Apps stay nil — the nav stack itself doesn't fan out to
// those, only the screen ID changes.
func newNavTestModel(t *testing.T) app.Model {
	t.Helper()
	port := &seededUsersPort{users: testUsersForKeyTest()}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	return updated.(app.Model)
}

// dispatch routes a tea.Msg through Update, chases any returned
// Cmd one round-trip (so quitConfirmCmd / openHelpCmd / etc. land
// as the follow-up Msg the runtime would dispatch), and returns
// the updated Model.
func dispatch(t *testing.T, m app.Model, msg tea.Msg) app.Model {
	t.Helper()
	updated, cmd := m.Update(msg)
	out, ok := updated.(app.Model)
	require.True(t, ok)
	if cmd != nil {
		if next := cmd(); next != nil {
			updated2, _ := out.Update(next)
			out, ok = updated2.(app.Model)
			require.True(t, ok)
		}
	}
	return out
}

func Test_NavStack_DefaultRoot_IsUsers(t *testing.T) {
	t.Parallel()
	m := newNavTestModel(t)
	assert.Equal(t, "users", app.ActiveScreenName(m),
		"default initial screen is Users — that's the bottom of the nav stack")
}

func Test_NavStack_PaletteResetsRoot(t *testing.T) {
	t.Parallel()
	m := newNavTestModel(t)
	// ':' palette commands flow through ScreenChangeMsg.
	m = dispatch(t, m, app.ScreenChangeMsg{Target: app.ScreenLogs})
	assert.Equal(t, "logs", app.ActiveScreenName(m),
		"palette command must switch the active root")
	// Esc on the new root should fire quit confirm — nothing else
	// is on the stack since palette commands reset.
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Contains(t, m.View(), "Quit ota?",
		"Esc on the root frame must fire the quit confirm overlay")
}

func Test_NavStack_OpenLogsMsg_PushesOnTop(t *testing.T) {
	t.Parallel()
	m := newNavTestModel(t)
	require.Equal(t, "users", app.ActiveScreenName(m), "precondition: rooted on Users")

	// Cross-resource drill-down via `l` shortcut from any resource.
	m = dispatch(t, m, shared.OpenLogsMsg{Filter: `actor.id eq "alice"`})
	assert.Equal(t, "logs", app.ActiveScreenName(m),
		"OpenLogsMsg must switch the active screen to Logs")

	// Esc with nothing locally to close pops back to Users.
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, "users", app.ActiveScreenName(m),
		"Esc must pop the Logs frame and return to the Users root")
	assert.NotContains(t, m.View(), "Quit ota?",
		"the pop must NOT fire the quit confirm — there's still a frame")
}

func Test_NavStack_RootEsc_FiresQuitConfirm(t *testing.T) {
	t.Parallel()
	m := newNavTestModel(t)
	// Sanity: stack is at the root.
	require.Equal(t, "users", app.ActiveScreenName(m))

	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Contains(t, m.View(), "Quit ota?",
		"Esc on the root frame must fire the quit confirm overlay")
}

func Test_NavStack_PushIsIdempotent(t *testing.T) {
	t.Parallel()
	m := newNavTestModel(t)

	// Push Logs once.
	m = dispatch(t, m, shared.OpenLogsMsg{Filter: `actor.id eq "alice"`})
	require.Equal(t, "logs", app.ActiveScreenName(m))

	// Push Logs again from the same screen — should be a no-op
	// (pushNav idempotency: pushing the screen the operator is
	// already on shouldn't pile up duplicate frames).
	m = dispatch(t, m, shared.OpenLogsMsg{Filter: `actor.id eq "bob"`})
	require.Equal(t, "logs", app.ActiveScreenName(m))

	// First Esc pops to Users (one Logs frame on the stack).
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, "users", app.ActiveScreenName(m),
		"single pop must reach the Users root — duplicate Logs pushes shouldn't pile up")

	// Second Esc fires quit confirm.
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Contains(t, m.View(), "Quit ota?")
}

func Test_NavStack_DeepDrill_PopsOneFrameAtATime(t *testing.T) {
	t.Parallel()
	m := newNavTestModel(t)

	// Build a 3-frame stack: Users → Groups (push) → Logs (push).
	m = dispatch(t, m, app.OpenGroupDetailMsg{ID: "00g_engineers"})
	require.Equal(t, "groups", app.ActiveScreenName(m),
		"OpenGroupDetailMsg must push the Groups frame")
	m = dispatch(t, m, shared.OpenLogsMsg{Filter: `target.id eq "00g_engineers"`})
	require.Equal(t, "logs", app.ActiveScreenName(m),
		"OpenLogsMsg from the Groups frame must push the Logs frame on top")

	// Pop back one at a time — Esc with nothing locally to close
	// walks the trail in reverse.
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, "groups", app.ActiveScreenName(m), "first Esc pops Logs → Groups")
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, "users", app.ActiveScreenName(m), "second Esc pops Groups → Users")
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Contains(t, m.View(), "Quit ota?",
		"third Esc on the root frame fires the quit confirm")
}

func Test_NavStack_PaletteAfterDrill_ResetsStack(t *testing.T) {
	t.Parallel()
	m := newNavTestModel(t)
	// Drill to a deep-ish stack.
	m = dispatch(t, m, shared.OpenLogsMsg{Filter: `actor.id eq "alice"`})
	require.Equal(t, "logs", app.ActiveScreenName(m))

	// Operator types `:groups` — palette resets the stack.
	m = dispatch(t, m, app.ScreenChangeMsg{Target: app.ScreenGroups})
	require.Equal(t, "groups", app.ActiveScreenName(m))

	// Esc fires quit confirm — Logs frame should be gone since
	// `:groups` reset the stack to [Groups].
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Contains(t, m.View(), "Quit ota?",
		"`:` palette commands reset the stack — Esc on the new root must quit")
}

// quitConfirmVisible is a small helper for readers — keeps the
// "Quit ota?" string check semantic in one place even though we
// inline it above. Defined here so future tests can reuse it.
func quitConfirmVisible(view string) bool {
	return strings.Contains(view, "Quit ota?")
}

var _ = quitConfirmVisible

// Test_NavStack_RootEsc_PrefersScreenOverQuit pins the precedence at
// the root frame: when the active screen has local Esc work to do
// (detail open, filter applied), the Shell forwards Esc to the
// screen FIRST so the operator can clear that local state without
// the quit confirm jumping in. Only after the screen reports
// "nothing left to close" does the next Esc fire the quit confirm.
//
// This preserves every screen's existing detail-close + filter-
// clear semantics for standalone use ('ota' boots straight on
// Users) while the cross-resource pop-back is the new behavior
// that kicks in once the operator drills into other resources.
func Test_NavStack_RootEsc_PrefersScreenOverQuit(t *testing.T) {
	t.Parallel()
	port := &seededUsersPort{users: testUsersForKeyTest()}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(app.Model)

	// Pump Init so the seeded users land.
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(app.Model)
		}
	}

	// Open alice's detail.
	m = pressKey(t, m, 'd')
	require.Contains(t, m.View(), "User Detail",
		"precondition: alice's User Detail open at the root frame")

	// First Esc: at root, screen has work (detail open) → forward
	// to screen → detail closes.
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.NotContains(t, m.View(), "User Detail",
		"first Esc at root must close the open detail (screen-local)")
	assert.NotContains(t, m.View(), "Quit ota?",
		"first Esc must NOT fire the quit confirm — the screen still had work")

	// Second Esc: nothing else to close → quit confirm.
	m = dispatch(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Contains(t, m.View(), "Quit ota?",
		"second Esc on the cleared root must fire the quit confirm")
}
