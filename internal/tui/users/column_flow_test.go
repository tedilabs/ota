package users

// v0.1.17 (#181): User Detail Pretty tab — j/k flows the cursor
// across the LEFT column rows first, then the RIGHT column rows,
// then wraps. Cursor never enters the Groups/Apps boxes (which own
// `]`/`[` focus).

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
)

// shapedUser produces a user with both columns populated so the
// cursor has somewhere to flow. Identity / Status / Organization
// land in the left column; Contact populates the right.
func shapedUser() domain.User {
	now := time.Now()
	return domain.User{
		ID:          "00u_alice",
		Status:      domain.UserStatusActive,
		Created:     now.Add(-365 * 24 * time.Hour),
		LastUpdated: now.Add(-time.Hour),
		Profile: domain.UserProfile{
			Login:       "alice@acme.com",
			Email:       "alice@acme.com",
			FirstName:   "Alice",
			LastName:    "Anderson",
			MobilePhone: "+1-555-1234",
			SecondEmail: "alice.alt@acme.com",
			Department:  "Engineering",
			Title:       "Staff Engineer",
		},
	}
}

func bootDetailModel(t *testing.T) ListModel {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Clock:        clock.Real(),
		Keys:         keymap,
		Width:        140,
		Height:       40,
		InitialUsers: []domain.User{shapedUser()},
	})
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = upd.(ListModel)
	upd, _ = m.Update(userOpenedMsg{user: shapedUser()})
	m = upd.(ListModel)
	require.True(t, m.opened, "detail must open after userOpenedMsg")
	return m
}

func pressRune(t *testing.T, m ListModel, r rune) ListModel {
	t.Helper()
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return upd.(ListModel)
}

// Test_DetailPretty_J_FlowsLeftThenRightThenWraps walks the cursor
// across the entire (left + right) column space and verifies it
// returns to row 0 of the left column at the end.
func Test_DetailPretty_J_FlowsLeftThenRightThenWraps(t *testing.T) {
	t.Parallel()

	m := bootDetailModel(t)
	leftLines, rightLines, _ := m.newDetail().prettyColumns()
	total := len(leftLines) + len(rightLines)
	require.Greater(t, total, 0, "the shaped user must yield >0 column rows")
	require.Greater(t, len(leftLines), 0, "left column must have rows")
	require.Greater(t, len(rightLines), 0, "right column must have rows")

	// Initial cursor — left column row 0.
	require.Equal(t, 0, m.detailLine,
		"detailLine starts at 0 (left column row 0)")

	// Press j until we exit the left column. Cursor should land on
	// the FIRST right-column row.
	for i := 0; i < len(leftLines); i++ {
		m = pressRune(t, m, 'j')
	}
	assert.Equal(t, len(leftLines), m.detailLine,
		"after j × len(left) the cursor must sit on the first right-column row")

	// Continue until we exhaust the right column. The next j wraps
	// the cursor back to row 0 (left column row 0).
	for i := 0; i < len(rightLines); i++ {
		m = pressRune(t, m, 'j')
	}
	assert.Equal(t, 0, m.detailLine,
		"j past the last right-column row must wrap to left[0]")
}

// Test_DetailPretty_K_WrapsBackwards mirrors the above for `k`.
func Test_DetailPretty_K_WrapsBackwards(t *testing.T) {
	t.Parallel()

	m := bootDetailModel(t)
	leftLines, rightLines, _ := m.newDetail().prettyColumns()
	total := len(leftLines) + len(rightLines)

	// k from row 0 wraps to the last cursor position.
	m = pressRune(t, m, 'k')
	assert.Equal(t, total-1, m.detailLine,
		"k from left[0] must wrap to right[last]")

	// k again steps backwards.
	m = pressRune(t, m, 'k')
	assert.Equal(t, total-2, m.detailLine,
		"second k continues stepping backwards")
}

// Test_DetailPretty_BodyLinesExcludeExtras: detailBodyLines for the
// Pretty tab must contain only the Pretty info columns — the
// Groups/Apps boxes section is OUT of the cursor scope so j/k
// can't drift into the boxes.
func Test_DetailPretty_BodyLinesExcludeExtras(t *testing.T) {
	t.Parallel()

	m := bootDetailModel(t)
	bodyLines := m.detailBodyLines()
	assert.NotEmpty(t, bodyLines)

	for _, line := range bodyLines {
		assert.NotContains(t, line, "╭─ Groups",
			"Pretty cursor scope must not include the Groups box border")
		assert.NotContains(t, line, "╭─ Apps",
			"Pretty cursor scope must not include the Apps box border")
	}
}
