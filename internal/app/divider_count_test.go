package app_test

// Pins the chrome's upper-divider count segment (issue #136). The
// user asked for "81 of 81" to live in the box border next to the
// resource label, with separators, instead of consuming a body row.
// This test seeds a Users screen with a known fixture and asserts the
// stripped View() carries `Users · 5 of 5` — and `· q="alice"` after
// applying a filter.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/testfx"
)

// #A5 v0.2.4 — was a 40-line `countingUsersPort` + `countingUsersIter`
// pair; now backed by testfx.SeededUsersPort.

func sampleUsersForCount() []domain.User {
	out := make([]domain.User, 5)
	for i := 0; i < 5; i++ {
		out[i] = domain.User{
			ID: "00u_" + string(rune('a'+i)),
			Profile: domain.UserProfile{
				Login: string(rune('a'+i)) + "@acme.com",
			},
			Status: domain.UserStatusActive,
		}
	}
	return out
}

func bootUsersAppForCount(t *testing.T) app.Model {
	t.Helper()
	port := testfx.SeededUsersPort(sampleUsersForCount())
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(app.Model)
		}
	}
	return m
}

// Test_AppShell_UpperDivider_StampsCount asserts the chrome's upper
// divider carries `Users · 5 of 5` once the seeded Users have loaded.
func Test_AppShell_UpperDivider_StampsCount(t *testing.T) {
	t.Parallel()

	m := bootUsersAppForCount(t)
	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "Users · 5 of 5",
		"chrome's upper divider must carry the resource label + count (issue #136)")
}

// Test_AppShell_UpperDivider_StampsFilter asserts the divider carries
// the active `/` filter so operators always see what's narrowing the
// row set, even after the prompt closes.
func Test_AppShell_UpperDivider_StampsFilter(t *testing.T) {
	t.Parallel()

	m := bootUsersAppForCount(t)

	// Open `/` filter and type "a" — narrows rows.
	for _, r := range "/a" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(app.Model)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(app.Model)

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, `q="a"`,
		"chrome's upper divider must surface the applied filter (issue #136)")
}
