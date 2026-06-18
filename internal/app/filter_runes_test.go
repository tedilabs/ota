package app_test

// v0.1.16 (#176): when the active child is in `/` filter mode the
// App Shell's command shortcuts (`:`, `?`, `q`, `a`) must NOT
// intercept printable runes — operators search for usernames
// containing every letter of the alphabet, including 'q' / 'a'.
// Reproduces the user-reported "filter에 q가 안 들어가고 종료
// 모달이 떠".

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
)

type filterRunesPort struct{ users []domain.User }

func (p *filterRunesPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &filterRunesIter{remaining: p.users}, nil
}
func (p *filterRunesPort) Get(_ context.Context, _ string) (domain.User, error) {
	return domain.User{}, nil
}
func (p *filterRunesPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *filterRunesPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *filterRunesPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *filterRunesPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *filterRunesPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *filterRunesPort) ResetFactors(_ context.Context, _ string) error { return nil }
func (p *filterRunesPort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *filterRunesPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *filterRunesPort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *filterRunesPort) Suspend(_ context.Context, _ string) error   { return nil }
func (p *filterRunesPort) Unsuspend(_ context.Context, _ string) error { return nil }
func (p *filterRunesPort) Delete(_ context.Context, _ string) error { return nil }
func (p *filterRunesPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

type filterRunesIter struct{ remaining []domain.User }

func (it *filterRunesIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.remaining) == 0 {
		return domain.User{}, false, nil
	}
	u := it.remaining[0]
	it.remaining = it.remaining[1:]
	return u, true, nil
}
func (it *filterRunesIter) Close() error { return nil }

// Test_AppShell_FilterMode_AcceptsCommandLetters drives `/qa` into a
// Users-list filter and asserts the buffer holds "qa" with no quit
// confirm or action menu showing. Also verifies `:` and `?` are
// passed through.
func Test_AppShell_FilterMode_AcceptsCommandLetters(t *testing.T) {
	t.Parallel()

	port := &filterRunesPort{users: []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive, Profile: domain.UserProfile{Login: "alice@acme.com"}},
		{ID: "00u_quinn", Status: domain.UserStatusActive, Profile: domain.UserProfile{Login: "quinn@acme.com"}},
	}}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = upd.(app.Model)
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			upd, _ = m.Update(msg)
			m = upd.(app.Model)
		}
	}

	// Open `/` filter and type each command-shortcut letter.
	for _, r := range []rune{'/', 'q', 'a', ':', '?'} {
		upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = upd.(app.Model)
	}

	view := m.View()
	assert.NotContains(t, view, "Quit ota?",
		"q during filter input must NOT open the quit confirm")
	assert.NotContains(t, view, "Actions · Users",
		"a during filter input must NOT open the action menu")
	assert.NotContains(t, view, "Help · Users List",
		"? during filter input must NOT open the help modal")
	assert.Contains(t, view, `q="qa:?"`,
		"command-shortcut letters must reach the filter buffer untouched")
}
