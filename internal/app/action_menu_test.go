package app_test

// v0.1.15 (#175): pressing `a` on the Users list opens the
// resource-specific action menu. Picking "Reset password" routes
// the App Shell into the existing y/N confirmation overlay so the
// destructive op stays gated by the same prompt the `:` palette
// uses (issue #125).

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

type actionMenuPort struct{ users []domain.User }

func (p *actionMenuPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &actionMenuIter{users: p.users}, nil
}
func (p *actionMenuPort) Get(_ context.Context, _ string) (domain.User, error) {
	return domain.User{}, nil
}
func (p *actionMenuPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *actionMenuPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *actionMenuPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *actionMenuPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *actionMenuPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *actionMenuPort) ResetFactors(_ context.Context, _ string) error { return nil }
func (p *actionMenuPort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *actionMenuPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *actionMenuPort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *actionMenuPort) Delete(_ context.Context, _ string) error { return nil }

type actionMenuIter struct{ users []domain.User }

func (it *actionMenuIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.users) == 0 {
		return domain.User{}, false, nil
	}
	u := it.users[0]
	it.users = it.users[1:]
	return u, true, nil
}
func (it *actionMenuIter) Close() error { return nil }

func bootActionMenuApp(t *testing.T) app.Model {
	t.Helper()
	users := []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive, Profile: domain.UserProfile{Login: "alice@acme.com"}},
	}
	port := &actionMenuPort{users: users}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = upd.(app.Model)
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			upd, _ = m.Update(msg)
			m = upd.(app.Model)
		}
	}
	return m
}

func sendAppKey(t *testing.T, m app.Model, msg tea.KeyMsg) app.Model {
	t.Helper()
	upd, cmd := m.Update(msg)
	mdl := upd.(app.Model)
	if cmd == nil {
		return mdl
	}
	out := cmd()
	if out == nil {
		return mdl
	}
	if batch, ok := out.(tea.BatchMsg); ok {
		for _, c := range batch {
			if next := c(); next != nil {
				upd, _ = mdl.Update(next)
				mdl = upd.(app.Model)
			}
		}
		return mdl
	}
	upd, _ = mdl.Update(out)
	return upd.(app.Model)
}

// Test_AppShell_ActionMenu_OpensOnA: pressing `a` on the Users list
// opens the menu modal whose body lists the three lifecycle
// operations.
func Test_AppShell_ActionMenu_OpensOnA(t *testing.T) {
	t.Parallel()

	m := bootActionMenuApp(t)
	m = sendAppKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	view := m.View()
	assert.Contains(t, view, "Actions · Users",
		"action menu modal title must include the active resource label")
	for _, want := range []string{"Reset password", "Unlock account", "Reset MFA factors"} {
		assert.Contains(t, view, want,
			"action menu must list %q", want)
	}
}

// Test_AppShell_ActionMenu_PicksResetPasswordOpensConfirm: with the
// menu open, j once + Enter must dispatch into the existing y/N
// confirmation overlay for the unlock action (the second item).
func Test_AppShell_ActionMenu_PicksUnlockOpensConfirm(t *testing.T) {
	t.Parallel()

	m := bootActionMenuApp(t)
	m = sendAppKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = sendAppKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = sendAppKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	assert.Contains(t, view, "Unlock account for ",
		"Enter on the Unlock row must open the y/N confirmation modal")
	assert.Contains(t, view, "alice@acme.com",
		"the confirmation must name the selected user")
}

// Test_AppShell_ActionMenu_EscClosesWithoutFiring: Esc on the menu
// returns to the list with no pending action queued.
func Test_AppShell_ActionMenu_EscClosesWithoutFiring(t *testing.T) {
	t.Parallel()

	m := bootActionMenuApp(t)
	m = sendAppKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Contains(t, m.View(), "Actions · Users")

	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = upd.(app.Model)

	view := m.View()
	assert.NotContains(t, view, "Actions · Users",
		"Esc must close the action menu")
	assert.NotContains(t, view, "Reset password for ",
		"Esc must NOT open the y/N confirmation as a side effect")
}
