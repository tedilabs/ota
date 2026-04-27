package users_test

// Regression for issue #148: a Hangul / CJK / emoji glyph counts as
// 1 rune but renders 2 cells in monospace terminals. The previous
// rune-count visibleWidth left the row's right edge drifting one
// cell per wide rune, which made the chrome's right border
// misalign when an Okta tenant carried Korean / Japanese display
// names. Now visibleWidth delegates to go-runewidth so the math
// stays honest.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/users"
)

func wideRuneFixture() []domain.User {
	return []domain.User{
		// CJK nickname — 3 runes / 6 cells.
		{ID: "00u_alice", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login:    "alice@acme.com",
				NickName: "박병진",
			}},
		// Plain ASCII row to anchor expected width.
		{ID: "00u_bob", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login:    "bob@acme.com",
				NickName: "Bob",
			}},
	}
}

func Test_UsersList_RowWidth_ConsistentAcrossWideRunes(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		InitialUsers: wideRuneFixture(),
		Width:        120,
		Height:       30,
	})

	view := testfx.StripANSI(m.View())
	lines := strings.Split(view, "\n")

	// The header row + every data row must measure to the same
	// visible width. Before the fix, the CJK row was 3 cells
	// shorter (rune-count) than the ASCII row even though each
	// Hangul char rendered as 2 cells.
	require.GreaterOrEqual(t, len(lines), 3, "must render header + 2 data rows")

	headerW := shared.VisibleWidth(lines[1])
	require.Greater(t, headerW, 0, "header line must have a positive width")
	for i, l := range lines[1:] {
		if l == "" {
			continue
		}
		w := shared.VisibleWidth(l)
		assert.Equalf(t, headerW, w,
			"line %d width drift — header=%d, line=%d, content=%q",
			i+1, headerW, w, l)
	}
}
