package app_test

// E2E for the Users lifecycle actions (issue #125). Exercises the full
// :reset-password / :unlock / :reset-mfa flow from palette command →
// confirmation modal → port call → toast — without hitting an actual
// Okta tenant.

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
)

// recordingUsersPort records lifecycle calls so the test can assert
// the App Shell wired the right user ID through to the port.
type recordingUsersPort struct {
	users         []domain.User
	resetCalls    []resetCall
	unlockCalls   []string
	factorsCalls  []string
	resetErr      error
	unlockErr     error
	factorsErr    error
	resetURL      string
}

type resetCall struct {
	UserID    string
	SendEmail bool
}

func (p *recordingUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &recordingIter{remaining: p.users}, nil
}
func (p *recordingUsersPort) Get(_ context.Context, id string) (domain.User, error) {
	for _, u := range p.users {
		if u.ID == id || u.Profile.Login == id {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}
func (p *recordingUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *recordingUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *recordingUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *recordingUsersPort) ResetPassword(_ context.Context, id string, sendEmail bool) (string, error) {
	p.resetCalls = append(p.resetCalls, resetCall{UserID: id, SendEmail: sendEmail})
	return p.resetURL, p.resetErr
}
func (p *recordingUsersPort) Unlock(_ context.Context, id string) error {
	p.unlockCalls = append(p.unlockCalls, id)
	return p.unlockErr
}
func (p *recordingUsersPort) ResetFactors(_ context.Context, id string) error {
	p.factorsCalls = append(p.factorsCalls, id)
	return p.factorsErr
}
func (p *recordingUsersPort) Activate(_ context.Context, _ string, _ bool) error   { return nil }
func (p *recordingUsersPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *recordingUsersPort) ExpirePassword(_ context.Context, _ string) error     { return nil }
func (p *recordingUsersPort) Suspend(_ context.Context, _ string) error   { return nil }
func (p *recordingUsersPort) Unsuspend(_ context.Context, _ string) error { return nil }
func (p *recordingUsersPort) Delete(_ context.Context, _ string) error             { return nil }
func (p *recordingUsersPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

type recordingIter struct{ remaining []domain.User }

func (it *recordingIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.remaining) == 0 {
		return domain.User{}, false, nil
	}
	u := it.remaining[0]
	it.remaining = it.remaining[1:]
	return u, true, nil
}
func (it *recordingIter) Close() error { return nil }

func newAppWithUsers(t *testing.T, port domain.UsersPort) app.Model {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})
	// Run the initial fetch so the Users screen has rows + a selection.
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(app.Model)
		}
	}
	return m
}

func typePalette(t *testing.T, m app.Model, command string) app.Model {
	t.Helper()
	step := func(mdl app.Model, msg tea.Msg) app.Model {
		updated, cmd := mdl.Update(msg)
		out := updated.(app.Model)
		// `:` triggers openCmdPaletteCmd, which returns
		// openCmdPaletteMsg via tea.Cmd — replay so the overlay flips.
		if cmd != nil {
			if next := cmd(); next != nil {
				updated, _ = out.Update(next)
				out = updated.(app.Model)
			}
		}
		return out
	}
	m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	for _, r := range command {
		m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return step(m, tea.KeyMsg{Type: tea.KeyEnter})
}


// Test_AppShell_ResetPassword_HappyPath_FiresPort — confirms y on the
// confirmation modal calls UsersPort.ResetPassword with the selected
// user ID + sendEmail=true.
func Test_AppShell_ResetPassword_HappyPath_FiresPort(t *testing.T) {
	t.Parallel()

	port := &recordingUsersPort{users: []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "alice@acme.com"}},
	}}
	m := newAppWithUsers(t, port)

	m = typePalette(t, m, "reset-password")

	// Confirmation modal must be visible with the user's login.
	view := m.View()
	assert.Contains(t, view, "Reset password",
		"action confirmation must surface the action label")
	assert.Contains(t, view, "alice@acme.com",
		"confirmation modal must surface the target user")

	// Press y to confirm.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(app.Model)
	require.NotNil(t, cmd, "y must fire the action Cmd")
	_ = cmd() // drain so the port call records

	require.Len(t, port.resetCalls, 1, "ResetPassword must be called exactly once")
	assert.Equal(t, "00u_alice", port.resetCalls[0].UserID)
	assert.True(t, port.resetCalls[0].SendEmail,
		"App Shell must default to sendEmail=true so Okta sends the reset email")
}

// Test_AppShell_Unlock_FiresPort — symmetric coverage for :unlock.
func Test_AppShell_Unlock_FiresPort(t *testing.T) {
	t.Parallel()

	port := &recordingUsersPort{users: []domain.User{
		{ID: "00u_locked", Status: domain.UserStatusLockedOut,
			Profile: domain.UserProfile{Login: "locked@acme.com"}},
	}}
	m := newAppWithUsers(t, port)
	m = typePalette(t, m, "unlock")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(app.Model)
	require.NotNil(t, cmd)
	_ = cmd()

	require.Len(t, port.unlockCalls, 1)
	assert.Equal(t, "00u_locked", port.unlockCalls[0])
}

// Test_AppShell_ResetFactors_FiresPort — and for :reset-mfa.
func Test_AppShell_ResetFactors_FiresPort(t *testing.T) {
	t.Parallel()

	port := &recordingUsersPort{users: []domain.User{
		{ID: "00u_mfa", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "mfa@acme.com"}},
	}}
	m := newAppWithUsers(t, port)
	m = typePalette(t, m, "reset-mfa")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(app.Model)
	require.NotNil(t, cmd)
	_ = cmd()

	require.Len(t, port.factorsCalls, 1)
	assert.Equal(t, "00u_mfa", port.factorsCalls[0])
}

// Test_AppShell_ActionConfirm_NCancels_NoPortCall — the n key must
// dismiss the modal without invoking the port. This is the safety
// gate the user implicitly relies on for destructive ops.
func Test_AppShell_ActionConfirm_NCancels_NoPortCall(t *testing.T) {
	t.Parallel()

	port := &recordingUsersPort{users: []domain.User{
		{ID: "00u_alice", Profile: domain.UserProfile{Login: "alice@acme.com"}},
	}}
	m := newAppWithUsers(t, port)
	m = typePalette(t, m, "reset-password")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(app.Model)
	assert.Nil(t, cmd, "n on the confirmation modal must NOT produce a Cmd")
	assert.Empty(t, port.resetCalls,
		"n cancels — UsersPort.ResetPassword must not be called")

	view := m.View()
	assert.NotContains(t, view, "Reset password",
		"confirmation modal must close after n")
}

// Test_AppShell_ActionConfirm_PortError_SurfacesToast — when the port
// returns an error, the App Shell renders an error-level toast carrying
// the failure detail so operators see why the action didn't take.
// #U11 v0.2.4 — the error path now emits an internal actionFailedMsg
// the shell expands into a toast + an ActionFailed broadcast; this
// test drives that whole pipeline and asserts the rendered View
// contains the failure detail.
func Test_AppShell_ActionConfirm_PortError_SurfacesToast(t *testing.T) {
	t.Parallel()

	port := &recordingUsersPort{
		users: []domain.User{
			{ID: "00u_alice", Profile: domain.UserProfile{Login: "alice@acme.com"}},
		},
		unlockErr: errors.New("403 forbidden"),
	}
	m := newAppWithUsers(t, port)
	m = typePalette(t, m, "unlock")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(app.Model)
	require.NotNil(t, cmd)
	failedMsg := cmd()
	require.NotNil(t, failedMsg, "y on the action confirm must produce a Cmd that emits an internal failure msg")

	// Drive the msg through the App Shell. It dispatches a toast + a
	// shared.ActionFailedMsg broadcast via tea.Batch.
	updated2, batched := m.Update(failedMsg)
	m = updated2.(app.Model)
	require.NotNil(t, batched, "App Shell must batch toast + failure broadcast")

	// Drain the Batch — drives the toast into m.toast so View renders the band.
	// The Cmd returns a tea.BatchMsg ([]tea.Cmd). Drive each contained
	// Cmd individually so the toast lands in m.toast for View().
	if batched != nil {
		bm := batched()
		if cmds, ok := bm.(tea.BatchMsg); ok {
			for _, c := range cmds {
				if c == nil {
					continue
				}
				if inner := c(); inner != nil {
					updated3, _ := m.Update(inner)
					m = updated3.(app.Model)
				}
			}
		}
	}

	view := m.View()
	assert.Contains(t, view, "unlock failed",
		"the floating toast band must surface the action label on failure")
	assert.Contains(t, view, "403 forbidden",
		"the toast band must carry the underlying error detail")
}
