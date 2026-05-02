package users_test

// Regression tests for the h / l horizontal scroll behavior (issue #122).
// User report: "행도 길어지면서 오른쪽 화면이 잘렸는데, h, l 로 좌우 움직일
// 수 있으면 좋을 듯." Auto-fit (issue #117) shrinks columns to fit data,
// so the natural row often does fit at v0.1.5 widths — these tests pick
// a deliberately narrow viewport (60-cell width) to force overflow and
// pin the scroll mechanics.

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

func wideUsersFixture() []domain.User {
	return []domain.User{
		{
			ID:     "00u_alice",
			Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login:          "alice@acme.com",
				Title:          "Senior Staff Engineer",
				Division:       "Platform Engineering",
				EmployeeNumber: "EMP-00012345",
				NickName:       "Ali",
			},
		},
		{
			ID:     "00u_bob",
			Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login:          "bob@acme.com",
				Title:          "Engineering Manager",
				Division:       "Platform Engineering",
				EmployeeNumber: "EMP-00012346",
				NickName:       "Bobby",
			},
		},
	}
}

// Test_UsersList_LStepsLeftColumnsOffViewport verifies pressing `l` on
// a width that overflows the natural row drops the leftmost column out
// of the body — STATUS disappears and LOGIN shifts left, exposing the
// trailing columns the user couldn't see before.
func Test_UsersList_LStepsLeftColumnsOffViewport(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: wideUsersFixture(),
		Width:        60, // narrow enough to force MaxHScroll > 0
		Height:       24,
	})

	before := testfx.StripANSI(m.View())
	require.Contains(t, before, "LOGIN",
		"precondition: LOGIN column (leftmost in #145 lineup) visible before scroll")
	require.Contains(t, before, "alice@acme.com")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mdl, ok := updated.(users.ListModel)
	require.True(t, ok)

	after := testfx.StripANSI(mdl.View())
	assert.NotContains(t, after, "alice@acme.com",
		"`l` must drop the leftmost column (LOGIN) once the row overflows the viewport")
}

// Test_UsersList_HReturnsToLeftEdge verifies pressing `h` after `l`
// restores the original left-aligned slice — h/l are reversible.
func Test_UsersList_HReturnsToLeftEdge(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: wideUsersFixture(),
		Width:        60,
		Height:       24,
	})

	feed := func(mdl users.ListModel, r rune) users.ListModel {
		updated, _ := mdl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		out, ok := updated.(users.ListModel)
		require.True(t, ok)
		return out
	}

	m = feed(m, 'l')
	m = feed(m, 'h')

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "alice@acme.com",
		"`h` must restore the leftmost LOGIN column once we scroll back")
}

// Test_UsersList_HClampsAtZero verifies pressing `h` at the left edge
// is a safe no-op — no underflow, no panic, no glitched header.
func Test_UsersList_HClampsAtZero(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: wideUsersFixture(),
		Width:        60,
		Height:       24,
	})

	for i := 0; i < 5; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
		mdl, ok := updated.(users.ListModel)
		require.True(t, ok)
		m = mdl
	}

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "alice@acme.com",
		"hScroll must clamp at 0 — leftmost LOGIN column stays visible after repeated `h`")
}

// Test_UsersList_LClampsAtMax verifies the scroll cursor cannot run
// past MaxHScroll — repeatedly pressing `l` eventually plateaus, never
// emptying the row.
func Test_UsersList_LClampsAtMax(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: wideUsersFixture(),
		Width:        60,
		Height:       24,
	})

	for i := 0; i < 20; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		mdl, ok := updated.(users.ListModel)
		require.True(t, ok)
		m = mdl
	}

	view := testfx.StripANSI(m.View())
	// alice@acme.com may scroll out of view; confirm her *row* still
	// renders with data from a trailing column ([+] ACTIVE status
	// badge or LAST UPDATED timestamp) so we know the body didn't
	// collapse to blank.
	assert.True(t,
		strings.Contains(view, "ACTIVE") || strings.Contains(view, "ago"),
		"trailing-column row data must remain visible after over-scrolling")
	assert.False(t, strings.Contains(view, "\n\n\n\n"),
		"body must never collapse to blank after over-scrolling")
}

// Test_UsersList_NoScrollWhenNaturalFits — at a wide viewport (180
// cells) every column fits at its declared Min, so hScroll has no
// effect. Pressing `l` is a no-op.
func Test_UsersList_NoScrollWhenNaturalFits(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: wideUsersFixture(),
		Width:        180,
		Height:       30,
	})

	before := testfx.StripANSI(m.View())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mdl, ok := updated.(users.ListModel)
	require.True(t, ok)
	after := testfx.StripANSI(mdl.View())

	assert.Equal(t, before, after,
		"`l` at a wide viewport must be a no-op — there's nothing to scroll")
}
