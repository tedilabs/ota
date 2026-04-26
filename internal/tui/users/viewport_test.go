package users_test

// Regression tests for the chrome header disappearing when the data set
// outgrows the terminal. The user reported "데이터가 많으면 ui의 헤더부분이
// 화면에 안보이는거 같아." — the list rendered every row into the body string,
// pushing the chrome top border above the viewport. v0.1.2-3 introduced
// the shared viewport windowing; these tests pin the behaviour.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

func init() { testfx.PinTestEnvironment() }

func bigUserSlice(n int) []domain.User {
	out := make([]domain.User, n)
	for i := 0; i < n; i++ {
		out[i] = domain.User{
			ID:      "00u_" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			Status:  domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "u" + string(rune('a'+i%26)) + "@acme.com"},
		}
	}
	return out
}

// Test_UsersList_ViewportLimitsBodyToHeight asserts that on a 24-row
// terminal the rendered body never exceeds what the chrome can display.
// Without windowing the body string ran the full length of the user list.
func Test_UsersList_ViewportLimitsBodyToHeight(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: bigUserSlice(80), // 80 users, well past any reasonable terminal
		Width:        120,
		Height:       24,
	})

	view := m.View()
	rows := strings.Count(view, "\n")
	// Budget for a 24-row terminal: chrome reserves ~7, list spends ~3 on
	// context/header, leaving ~14 data rows. Add the context+header lines
	// themselves (3) and we expect well under 24 newlines total.
	assert.Less(t, rows, 24,
		"body must fit inside a 24-row terminal so the chrome header stays visible")
	// And it really did render *some* rows — not just header.
	assert.Greater(t, rows, 5, "must still surface a useful slice of the list")
}

// Test_UsersList_ViewportFollowsCursor moves the cursor past the visible
// window and asserts the cursor row is in the rendered output. Without
// the follow-the-cursor logic, scrolling past the bottom would render an
// empty selection.
func Test_UsersList_ViewportFollowsCursor(t *testing.T) {
	t.Parallel()

	users50 := bigUserSlice(50)
	m := users.NewListModel(users.Deps{
		InitialUsers: users50,
		Width:        120,
		Height:       24,
	})

	// Press `j` enough times to go past the initial visible window.
	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	for i := 0; i < 40; i++ {
		updated, _ := m.Update(jKey)
		mdl, ok := updated.(users.ListModel)
		require.True(t, ok)
		m = mdl
	}

	view := m.View()
	cursorLogin := users50[40].Profile.Login
	assert.Contains(t, view, "▸ ",
		"cursor prefix (▸) must remain visible after scrolling")
	assert.Contains(t, view, cursorLogin,
		"viewport must scroll so the cursor's row stays rendered")
	// First user must NOT still be visible — we've scrolled past it.
	assert.NotContains(t, view, users50[0].Profile.Login,
		"row 0 must scroll out of view once the cursor has advanced past the budget")
}

