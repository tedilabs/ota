package home_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
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
	require.Equal(t, home.CardActivity, m.FocusedCard(),
		"precondition: default focus is Activity (headline card after the Option A pivot)")

	for i := 0; i < m.CardCount(); i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(home.Model)
	}
	assert.Equal(t, home.CardActivity, m.FocusedCard(),
		"Tab through every card must wrap back to Activity")
}

func Test_Home_EnterOnActivity_OpensLogsScreen(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60})
	require.Equal(t, home.CardActivity, m.FocusedCard(),
		"precondition: Activity card is the default focus")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter on Activity must return a Cmd")
	msg := cmd()
	open, ok := msg.(shared.OpenScreenMsg)
	require.True(t, ok, "Enter on Activity must emit shared.OpenScreenMsg")
	assert.Equal(t, "logs", open.Target,
		"Activity drills into the Logs screen so the operator can investigate the spike")
}

func Test_Home_ActivityWindowToggle_CyclesThrough_1h_6h_24h(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60})
	require.Equal(t, home.CardActivity, m.FocusedCard())

	// Default window is 1h (cheap-by-default — wider windows
	// burn more logs rate-limit budget so they're opt-in).
	require.Contains(t, m.View(), "Activity (1h)",
		"precondition: Activity card opens on the cheap 1h window")

	m = runKey(t, m, 't')
	assert.Contains(t, m.View(), "Activity (6h)",
		"first 't' must widen the window to 6h")

	m = runKey(t, m, 't')
	assert.Contains(t, m.View(), "Activity (24h)",
		"second 't' must widen the window to 24h")

	m = runKey(t, m, 't')
	assert.Contains(t, m.View(), "Activity (1h)",
		"third 't' must wrap back to 1h")
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

	require.Contains(t, m.View(), "Tab to fetch",
		"Posture card pre-msg renders the lazy-fetch hint (no auto-fetch on boot to preserve rate-limit budget)")

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	m = runMsg(t, m, home.PostureLoadedForTest(home.PostureMetrics{
		WindowSince:           now.AddDate(0, 0, -7),
		ObservedAt:            now,
		SignIns7d:             1000,
		FailedSignIns7d:       42,
		AccountLocks7d:        3,
		SensitiveWrites7d:     12,
		DistinctAdminActors7d: 4,
		UserDeletes7d:         2,
	}))

	view := m.View()
	assert.Contains(t, view, "42 failed sign-ins (7d)",
		"posture card must surface 7d failed sign-ins")
	assert.Contains(t, view, "3 account lockouts (7d)",
		"posture card must surface 7d lockout count")
	assert.Contains(t, view, "12 sensitive admin writes (7d)",
		"posture card must surface 7d sensitive write count")
	assert.Contains(t, view, "4 distinct admin actors",
		"posture card must surface distinct-actor governance signal")
	assert.Contains(t, view, "2 user deletes (7d)",
		"posture card must surface 7d user-delete count")
}

func Test_Home_ActivityCard_RendersAfterActivityLoadedMsg(t *testing.T) {
	t.Parallel()
	m := home.New(home.Deps{Width: 200, Height: 60, OrgURL: "https://acme.okta.com"})

	require.Contains(t, m.View(), "Tab to fetch",
		"Activity card pre-msg renders the lazy-fetch hint")

	m = runMsg(t, m, home.ActivityLoadedForTest("1h", home.ActivityMetrics{
		WindowLabel:     "1h",
		WindowSince:     time.Now().Add(-time.Hour),
		SignIns:         4321,
		FailedSignIns:   17,
		AccountLocks:    2,
		APITokenWrites:  1,
		RoleChanges:     3,
		PolicyMutations: 5,
		UserCreates:     6,
		AppAssignAdds:   42,
	}, false))

	view := m.View()
	assert.Contains(t, view, "4,321",
		"Activity card must format Sign-ins with thousands separator")
	assert.Contains(t, view, "Failed sign-ins",
		"Activity card must surface the Failed sign-ins row")
	assert.Contains(t, view, "API token writes",
		"Activity card must surface the admin-surface API token row")
	assert.Contains(t, view, "App assign",
		"Activity card must surface the app-assignment lifecycle rows")
}

func Test_Home_ActivityCard_24hWindow_RendersDeltaFooterWhenCacheHasYesterday(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := dashboard.New(cacheDir, "https://acme.okta.com")
	require.NoError(t, err)

	// Seed yesterday's roll directly so the delta render path has
	// something to compare against. The cache key must match the
	// constant the home package writes today under (the test reads
	// it back via the Δ footer when today's activityLoadedMsg lands).
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	yesterday := now.AddDate(0, 0, -1)
	require.NoError(t, cache.Put("activity-signins-24h", dashboard.Counts{
		Total:      1000,
		ObservedAt: yesterday,
	}))

	m := home.New(home.Deps{Width: 200, Height: 60, OrgURL: "https://acme.okta.com",
		Cache: cache, Clock: clock.NewFake(now)})

	// Cycle to 24h window — default is 1h.
	m = runKey(t, m, 't')
	m = runKey(t, m, 't')

	// Fire today's activity msg — value above yesterday's roll.
	m = runMsg(t, m, home.ActivityLoadedForTest("24h", home.ActivityMetrics{
		WindowLabel: "24h",
		WindowSince: now.Add(-24 * time.Hour),
		SignIns:     1042,
	}, false))

	view := m.View()
	assert.Contains(t, view, "+42",
		"24h Activity card must render a +42 Δ vs 1d footer when cache has yesterday's roll")
	assert.Contains(t, view, "(1d)",
		"delta footer must label the 1d window cell")
}


