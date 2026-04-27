package users_test

// Pins the filter-Esc UX (issue #131): once the operator has applied
// a `/alice<Enter>` filter the rows are visibly narrowed; pressing
// Esc must clear the filter and restore the full row set without
// requiring them to backspace through the query.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

func filterEscFixture() []domain.User {
	return []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "alice@acme.com"}},
		{ID: "00u_bob", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "bob@acme.com"}},
	}
}

func runUsersKey(t *testing.T, m users.ListModel, key tea.KeyMsg) users.ListModel {
	t.Helper()
	updated, _ := m.Update(key)
	out, ok := updated.(users.ListModel)
	require.True(t, ok)
	return out
}

func Test_UsersList_EscClearsActiveFilter(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: filterEscFixture(),
		Width:        120,
		Height:       24,
	})

	// Apply /alice<Enter> — narrows rows to alice only.
	m = runUsersKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "alice" {
		m = runUsersKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = runUsersKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	filtered := testfx.StripANSI(m.View())
	require.Contains(t, filtered, "alice@acme.com",
		"alice must remain visible after applying /alice")
	require.NotContains(t, filtered, "bob@acme.com",
		"bob must drop out under /alice")

	// Press Esc — filter must clear, full row set must come back.
	m = runUsersKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	cleared := testfx.StripANSI(m.View())
	assert.Contains(t, cleared, "alice@acme.com")
	assert.Contains(t, cleared, "bob@acme.com",
		"Esc must clear /alice and restore bob to the visible set")
}
