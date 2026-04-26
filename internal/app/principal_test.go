package app_test

// Pins the chrome's "as <login>" principal slot (issue #124).
//
// 1. App Shell fires GET /api/v1/users/me on the first WindowSizeMsg.
// 2. The result lands on the chrome ContextBar so operators can see whose
//    Okta token ota is using before they take any action.
// 3. When the lookup fails or hasn't completed yet, the chrome falls back
//    to "profile=…" alone (no panicky empty / "as " glitch).

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
	"github.com/tedilabs/ota/internal/testfx"
)

// principalUsersPort is a UsersPort double whose Get("me") returns a
// fixed Okta-shaped principal. List remains empty so the active screen
// stays free of seeded rows that could obscure the chrome assertion.
type principalUsersPort struct {
	me domain.User
}

func (p *principalUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &emptyUsersIter{}, nil
}
func (p *principalUsersPort) Get(_ context.Context, id string) (domain.User, error) {
	if id == "me" {
		return p.me, nil
	}
	return domain.User{}, domain.ErrNotFound
}
func (p *principalUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *principalUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *principalUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *principalUsersPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *principalUsersPort) ResetFactors(_ context.Context, _ string) error { return nil }

type emptyUsersIter struct{}

func (it *emptyUsersIter) Next(_ context.Context) (domain.User, bool, error) {
	return domain.User{}, false, nil
}
func (it *emptyUsersIter) Close() error { return nil }

// Test_AppShell_PrincipalProbe_RendersInChrome — happy path. The first
// WindowSizeMsg triggers /me, the response feeds back via Update, and the
// next View() carries "as <login>" on the ContextBar.
func Test_AppShell_PrincipalProbe_RendersInChrome(t *testing.T) {
	t.Parallel()

	port := &principalUsersPort{me: domain.User{
		ID: "00u_admin",
		Profile: domain.UserProfile{
			Login:     "admin@acme.com",
			FirstName: "Admin",
			LastName:  "User",
		},
	}}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "prod",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})

	// Drive the runtime: WindowSizeMsg fires the /me probe, its Cmd
	// produces principalLoadedMsg, which Update folds into the model.
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updated.(app.Model)
	require.NotNil(t, cmd, "WindowSizeMsg must kick off the /me probe Cmd")
	msg := cmd()
	require.NotNil(t, msg, "/me probe must produce a follow-up Msg")
	updated, _ = m.Update(msg)
	m = updated.(app.Model)

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "as admin@acme.com",
		"chrome ContextBar must surface the authenticated principal")
	assert.Contains(t, view, "profile=prod",
		"chrome ContextBar must keep the profile label after principal lands")
}

// Test_AppShell_PrincipalProbe_OnlyFiresOnce — the latch must guard the
// /me call so a stream of WindowSizeMsg events (resize, multiplexers
// re-emitting on focus changes, etc.) doesn't burn rate-limit budget.
func Test_AppShell_PrincipalProbe_OnlyFiresOnce(t *testing.T) {
	t.Parallel()

	port := &principalUsersPort{me: domain.User{
		ID:      "00u_admin",
		Profile: domain.UserProfile{Login: "admin@acme.com"},
	}}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "prod",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})

	// 1st WindowSizeMsg → Cmd present.
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updated.(app.Model)
	require.NotNil(t, cmd, "first WindowSizeMsg must fire /me")

	// 2nd WindowSizeMsg → Cmd nil (latch held).
	_, cmd2 := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	assert.Nil(t, cmd2, "subsequent WindowSizeMsg must NOT re-fire /me")
}

// Test_AppShell_PrincipalProbe_NoUsersPort — chrome stays clean when the
// caller wires no UsersPort (e.g. golden tests). The slot collapses back
// to "profile=…" with no half-rendered "as " prefix.
func Test_AppShell_PrincipalProbe_NoUsersPort(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{
		InitialScreen: app.ScreenUsers,
		Profile:       "prod",
		OrgURL:        "https://acme.okta.com",
	})
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updated.(app.Model)
	assert.Nil(t, cmd, "no UsersPort wired → no /me Cmd")

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "profile=prod",
		"chrome must still show profile=…")
	assert.NotContains(t, view, "as ",
		"chrome must not emit a half-rendered 'as ' segment")
}
