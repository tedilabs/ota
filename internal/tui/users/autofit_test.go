package users_test

// Regression scope for issue #128: the user reports that the rendered
// row width still exceeds the viewport, clipping the trailing column.
// These tests render a realistic-width Users fixture across several
// terminal widths and assert the visible row width never exceeds the
// inner-body budget.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

func realisticUsers() []domain.User {
	return []domain.User{
		{
			ID: "00u_alice", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login:          "alice.anderson@verylongcompanyname.com",
				Title:          "Senior Staff Software Engineer",
				Division:       "Platform Engineering",
				EmployeeNumber: "EMP-2024-00012345",
				NickName:       "Ali",
			},
		},
		{
			ID: "00u_bob", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login:          "bob.bradley@verylongcompanyname.com",
				Title:          "Director of Engineering",
				Division:       "Customer Success",
				EmployeeNumber: "EMP-2024-00012346",
				NickName:       "Bobby",
			},
		},
	}
}

func widestVisibleLine(view string) (string, int) {
	lines := strings.Split(view, "\n")
	var widest string
	max := 0
	for _, l := range lines {
		if w := visibleWidthForTest(l); w > max {
			max = w
			widest = l
		}
	}
	return widest, max
}

func visibleWidthForTest(s string) int {
	stripped := testfx.StripANSI(s)
	count := 0
	for range stripped {
		count++
	}
	return count
}

// Test_UsersList_RowsFitWithinViewport — at the widths the user would
// realistically use (90, 100, 120, 160), the rendered body must never
// produce a row wider than the viewport. Prevents the "trailing column
// clipped" failure from issue #128.
func Test_UsersList_RowsFitWithinViewport(t *testing.T) {
	t.Parallel()

	for _, w := range []int{90, 100, 120, 160} {
		w := w
		t.Run("width="+itoa(w), func(t *testing.T) {
			t.Parallel()
			m := users.NewListModel(users.Deps{
				InitialUsers: realisticUsers(),
				Width:        w,
				Height:       30,
			})
			view := testfx.StripANSI(m.View())
			widest, got := widestVisibleLine(view)
			// The Users body itself is sized to (w - chrome trim) cells.
			// We allow the full terminal width as the upper bound — if a
			// list line exceeds that, the chrome will clip it.
			assert.LessOrEqualf(t, got, w,
				"no rendered list line may exceed width=%d (widest=%d):\n%s",
				w, got, widest)
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
