package app_test

// v0.1.15: pressing `G` to jump to the bottom of a long list must keep
// the cursor visible regardless of terminal height. The previous
// chrome capped body lines at 60 while ListBodyRowBudget computed
// h - 9 with no upper bound; on terminals taller than 69 rows the
// list emitted more data rows than the chrome could show and the
// cursor row silently dropped off the bottom.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
)

type bulkUsersPort struct{ users []domain.User }

func (p *bulkUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &bulkUsersIter{remaining: p.users}, nil
}
func (p *bulkUsersPort) Get(_ context.Context, _ string) (domain.User, error) {
	return domain.User{}, nil
}
func (p *bulkUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *bulkUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *bulkUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *bulkUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *bulkUsersPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *bulkUsersPort) ResetFactors(_ context.Context, _ string) error { return nil }

type bulkUsersIter struct{ remaining []domain.User }

func (it *bulkUsersIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.remaining) == 0 {
		return domain.User{}, false, nil
	}
	u := it.remaining[0]
	it.remaining = it.remaining[1:]
	return u, true, nil
}
func (it *bulkUsersIter) Close() error { return nil }

// Test_AppShell_G_KeepsCursorVisibleOnTallTerminal pins the v0.1.15 fix.
// On a 200-user list with a 80-row terminal, `G` must leave the cursor
// row (`▸`) within the rendered body — not clipped by the chrome cap.
func Test_AppShell_G_KeepsCursorVisibleOnTallTerminal(t *testing.T) {
	t.Parallel()

	users := make([]domain.User, 200)
	for i := range users {
		users[i] = domain.User{
			ID:      fmt.Sprintf("u_%03d", i),
			Status:  domain.UserStatusActive,
			Profile: domain.UserProfile{Login: fmt.Sprintf("user%03d@acme.com", i)},
		}
	}
	port := &bulkUsersPort{users: users}

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	m = upd.(app.Model)
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			upd, _ = m.Update(msg)
			m = upd.(app.Model)
		}
	}

	// Press `G` — last row should now carry the cursor.
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = upd.(app.Model)

	view := m.View()
	assert.Contains(t, view, "▸",
		"`G` must leave the cursor (▸) visible somewhere in the rendered body")
	assert.Contains(t, view, "user199@acme.com",
		"the bottom row must be present in the rendered body after `G`")
	// The cursor must sit on the LAST row, not above it.
	cursorLine := ""
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "▸") {
			cursorLine = line
		}
	}
	assert.Contains(t, cursorLine, "user199@acme.com",
		"`G` must move the cursor onto the last user's row, not stop short")
}
