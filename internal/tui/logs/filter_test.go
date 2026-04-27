package logs_test

// Pins the `/` filter on the Logs SearchModel (issue #153). User
// asked for a filter mirroring what Users / Groups / Group Rules
// already carry — substring match against eventType / actor /
// outcome / IP, with the chrome's floating input box handling the
// prompt UX.

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/logs"
)

func filterFixture() []domain.LogEvent {
	now := time.Now().UTC()
	return []domain.LogEvent{
		{Published: now.Add(-3 * time.Minute), EventType: "user.session.start",
			Actor: domain.Actor{DisplayName: "Alice", AlternateID: "alice@acme.com"}},
		{Published: now.Add(-2 * time.Minute), EventType: "user.session.access",
			Actor: domain.Actor{DisplayName: "Bob", AlternateID: "bob@acme.com"}},
		{Published: now.Add(-1 * time.Minute), EventType: "policy.evaluate",
			Actor: domain.Actor{DisplayName: "system", AlternateID: ""}},
	}
}

func feedKey(t *testing.T, m logs.SearchModel, key tea.KeyMsg) logs.SearchModel {
	t.Helper()
	updated, _ := m.Update(key)
	out, ok := updated.(logs.SearchModel)
	require.True(t, ok)
	return out
}

func Test_LogsSearch_SlashOpensFilterPrompt(t *testing.T) {
	t.Parallel()
	m := logs.NewSearchModel(logs.Deps{
		InitialEvents: filterFixture(),
		Clock:         clock.NewFake(time.Now()),
		Width:         120, Height: 30,
	})

	require.False(t, m.Filtering(), "filter prompt must be closed by default")

	m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, m.Filtering(), "`/` must open the filter prompt")

	for _, r := range "session" {
		m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	assert.Equal(t, "session", m.Filter(),
		"runes typed while filtering append to the filter buffer")
}

func Test_LogsSearch_FilterNarrowsVisibleEvents(t *testing.T) {
	t.Parallel()
	m := logs.NewSearchModel(logs.Deps{
		InitialEvents: filterFixture(),
		Clock:         clock.NewFake(time.Now()),
		Width:         120, Height: 30,
	})

	// `/session<Enter>` — should drop the policy.evaluate row.
	m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "session" {
		m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.False(t, m.Filtering(), "Enter must close the prompt")
	assert.Equal(t, "session", m.Filter(), "filter buffer must persist after Enter")

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "user.session.start")
	assert.Contains(t, view, "user.session.access")
	assert.NotContains(t, view, "policy.evaluate",
		"filter must drop events without a 'session' substring match")
}

func Test_LogsSearch_FilterEsc_RestoresFullSet(t *testing.T) {
	t.Parallel()
	m := logs.NewSearchModel(logs.Deps{
		InitialEvents: filterFixture(),
		Clock:         clock.NewFake(time.Now()),
		Width:         120, Height: 30,
	})

	// Apply a filter.
	m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "policy" {
		m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	require.NotContains(t, testfx.StripANSI(m.View()), "user.session.start",
		"precondition: filter narrows the set")

	// Esc on the list (filter prompt already closed) clears the filter.
	m = feedKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	view := testfx.StripANSI(m.View())
	assert.Empty(t, m.Filter(), "Esc must clear the filter buffer")
	assert.Contains(t, view, "user.session.start",
		"full event set must come back after Esc")
}
