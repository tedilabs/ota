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
// meCalls counts Get("me") calls so OnlyFiresOnce can assert the latch
// without relying on the shape of the WindowSizeMsg return value (the
// App Shell now batches child spinner-tick Cmds in alongside the
// principal probe Cmd).
type principalUsersPort struct {
	me      domain.User
	meCalls int
}

func (p *principalUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &emptyUsersIter{}, nil
}
func (p *principalUsersPort) Get(_ context.Context, id string) (domain.User, error) {
	if id == "me" {
		p.meCalls++
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
func (p *principalUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *principalUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *principalUsersPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *principalUsersPort) ResetFactors(_ context.Context, _ string) error { return nil }
func (p *principalUsersPort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *principalUsersPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *principalUsersPort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *principalUsersPort) Delete(_ context.Context, _ string) error { return nil }
func (p *principalUsersPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

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

	// Drive the runtime: WindowSizeMsg returns a tea.Batch holding
	// the /me probe alongside child spinner-tick Cmds. Calling the
	// batch and walking each sub-Cmd gives us the principalLoadedMsg
	// Update folds back into the model.
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updated.(app.Model)
	require.NotNil(t, cmd, "WindowSizeMsg must kick off the /me probe Cmd")
	for _, sub := range invokeBatch(cmd) {
		if sub == nil {
			continue
		}
		if next := sub(); next != nil {
			updated, _ = m.Update(next)
			m = updated.(app.Model)
		}
	}

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "admin@acme.com",
		"chrome TitleBar must surface the authenticated principal")
	assert.Contains(t, view, "[prod]",
		"chrome TitleBar must keep the env badge after principal lands")
}

// invokeBatch unwraps a tea.Cmd into its constituent sub-cmds. When
// the cmd is a tea.Batch, the call returns a tea.BatchMsg ([]tea.Cmd);
// otherwise the whole cmd is returned in a single-element slice so
// callers can drive it the same way regardless of wrapping.
func invokeBatch(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		return []tea.Cmd(batch)
	}
	// Non-batch — wrap the original cmd so the caller's loop still
	// runs once.
	return []tea.Cmd{func() tea.Msg { return msg }}
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

	// 1st WindowSizeMsg fires the /me probe via the App Shell's
	// kickPrincipalFetch latch. Drive each sub-Cmd in the returned
	// batch so the port observes the actual call.
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updated.(app.Model)
	require.NotNil(t, cmd, "first WindowSizeMsg must fire /me")
	for _, sub := range invokeBatch(cmd) {
		if sub != nil {
			_ = sub() // discard the message — we only care about the side effect
		}
	}
	require.Equal(t, 1, port.meCalls,
		"first WindowSizeMsg must fire /me exactly once")

	// 2nd WindowSizeMsg — latch should hold so /me does not fire
	// again. Drive any returned cmds the same way and re-check the
	// counter.
	updated, cmd2 := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(app.Model)
	for _, sub := range invokeBatch(cmd2) {
		if sub != nil {
			_ = sub()
		}
	}
	require.Equal(t, 1, port.meCalls,
		"subsequent WindowSizeMsg must NOT re-fire /me (latch held)")
	_ = m
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
	assert.Contains(t, view, "[prod]",
		"chrome must still show the env badge")
	// No principal slot expected when UsersPort is unwired — the
	// brand·tenant·[prod] segment is the entire left group.
	assert.NotContains(t, view, "@acme.com",
		"chrome must not invent a principal when no UsersPort is wired")
}
