package home_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/dashboard"
	"github.com/tedilabs/ota/internal/tui/home"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// runKey pumps one key through Update + returns the post-state.
func runKey(t *testing.T, m home.Model, r rune) home.Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	out, ok := updated.(home.Model)
	require.True(t, ok)
	return out
}

func runMsg(t *testing.T, m home.Model, msg tea.Msg) home.Model {
	t.Helper()
	updated, _ := m.Update(msg)
	out, ok := updated.(home.Model)
	require.True(t, ok)
	return out
}

func Test_Home_FocusCycle_TabWrapsAfterLastCard(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60, OrgURL: "https://acme.okta.com"})
	require.Equal(t, home.CardUsers, m.FocusedCard(),
		"precondition: default focus is Users")

	for i := 0; i < m.CardCount(); i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(home.Model)
	}
	assert.Equal(t, home.CardUsers, m.FocusedCard(),
		"Tab through every card must wrap back to Users")
}

func Test_Home_EnterOnCard_EmitsOpenScreenMsg(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60})

	// Focus the Apps card — 3rd in the order.
	for i := 0; i < 2; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(home.Model)
	}
	require.Equal(t, home.CardApps, m.FocusedCard())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter on a drillable card must return a Cmd")
	msg := cmd()
	open, ok := msg.(shared.OpenScreenMsg)
	require.True(t, ok, "Enter on Apps must emit shared.OpenScreenMsg")
	assert.Equal(t, "apps", open.Target)
}

func Test_Home_ActivityWindowToggle_CyclesThrough7dAnd30d(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60})

	// Move focus to the Activity card (4th card — Users / Groups / Apps / Activity).
	for i := 0; i < 3; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(home.Model)
	}
	require.Equal(t, home.CardActivity, m.FocusedCard())

	// Default window is 7d. `t` flips to 30d. Another `t` flips back.
	require.Contains(t, m.View(), "7d",
		"precondition: Activity card opens at 7d secondary window")

	m = runKey(t, m, 't')
	assert.Contains(t, m.View(), "30d",
		"first 't' must cycle the secondary window to 30d")

	m = runKey(t, m, 't')
	assert.Contains(t, m.View(), "7d",
		"second 't' must cycle back to 7d")
}

func Test_Home_HealthCard_RendersAfterUpdateHealthMsg(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60})

	// Before any UpdateHealthMsg, the card shows the warm-up state.
	require.Contains(t, m.View(), "warming up",
		"Health card pre-msg renders the 'warming up…' placeholder")

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	m = runMsg(t, m, home.UpdateHealthMsg{
		Snapshot: home.HealthSnapshot{
			LastFetchAt: now.Add(-2 * time.Second),
			ObservedAt:  now,
		},
	})
	assert.NotContains(t, m.View(), "warming up",
		"Health card must transition out of 'warming up' once UpdateHealthMsg lands")
}

func Test_Home_EventsCard_jk_NavigatesEventsWhenFocused(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60})

	// Seed the Events card with a few entries via the internal msg.
	// Tests don't need to hit a real LogsPort.
	events := []home.CriticalEvent{
		{When: time.Now(), EventType: "user.account.lock", ActorLogin: "alice@acme.com", ActorID: "00u_alice"},
		{When: time.Now(), EventType: "system.api_token.create", ActorLogin: "bob@acme.com", ActorID: "00u_bob"},
		{When: time.Now(), EventType: "user.role.add", ActorLogin: "carol@acme.com", ActorID: "00u_carol"},
	}
	m = runMsg(t, m, home.EventsLoadedForTest(events, nil))

	// Move focus to the Events card (last in the order).
	for i := 0; i < m.CardCount()-1; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(home.Model)
	}
	require.Equal(t, home.CardEvents, m.FocusedCard())

	// First event highlighted by default.
	require.Contains(t, m.View(), "▸ ",
		"focused Events card must render the cursor marker on row 0")

	// j → next event.
	m = runKey(t, m, 'j')
	view := m.View()
	assert.Contains(t, view, "bob@acme.com",
		"after j, bob's row should be visible (cursor sits on row 1)")

	// Enter on a focused event must emit OpenLogsMsg with actor.id filter.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	open, ok := msg.(shared.OpenLogsMsg)
	require.True(t, ok, "Enter on a focused event row must emit shared.OpenLogsMsg")
	assert.Contains(t, open.Filter, "00u_bob",
		"the drill filter must scope to the highlighted event's actor.id")
}

func Test_Home_PostureCard_RendersAfterPostureLoadedMsg(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60})

	require.Contains(t, m.View(), "loading…",
		"Posture card pre-msg renders 'loading…'")

	m = runMsg(t, m, home.PostureLoadedForTest(home.PostureMetrics{
		SuperAdmins:       7,
		TotalAdmins:       12,
		ExpiringTokens7d:  2,
		TotalTokens:       5,
		InvalidGroupRules: 1,
		TotalGroupRules:   30,
		ObservedAt:        time.Now(),
	}))

	view := m.View()
	assert.Contains(t, view, "7 SUPER_ADMINs",
		"posture card must surface the super-admin count")
	assert.Contains(t, view, "2 API tokens expire <7d",
		"posture card must surface expiring tokens row")
	assert.Contains(t, view, "1 INVALID group rules",
		"posture card must surface invalid group rules row")
}

func Test_Home_CountCard_RendersAfterCardLoadedMsg(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := dashboard.New(cacheDir, "https://acme.okta.com")
	require.NoError(t, err)
	m := home.New(home.Deps{Width: 200, Height: 60, Cache: cache, OrgURL: "https://acme.okta.com"})

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	m = runMsg(t, m, home.UsersLoadedForTest(dashboard.Counts{
		Total:      12438,
		ByStatus:   map[string]int{"ACTIVE": 11802, "SUSPENDED": 412, "LOCKED_OUT": 94},
		ObservedAt: now,
	}))

	view := m.View()
	assert.Contains(t, view, "12,438",
		"Users card must format the total with thousands separator")
	assert.Contains(t, view, "11,802",
		"Users card must list ACTIVE breakdown")
}
