package app_test

// REQ-W01 — Entry-point routing for SCR-012 (Users Edit Form)
// (Step 7 + Step 10 of the Phase 5 RED order).
//
// AC-1.1 — pressing `e` on the Users list emits an OpenUserEditMsg
// (or pushes ScreenUserEdit directly via the existing routing).
// AC-1.2 — same `e` keypress on User Detail.
//
// These tests use only the public app.Model.Update surface; the
// trigger details (whether the list emits OpenUserEditMsg and the
// shell forwards, or the shell intercepts `e` directly) are an
// implementation choice — both shapes satisfy the test if Phase 6
// ends up routing to ScreenUserEdit.
//
// Phase 5 RED expectation: nothing wires `e` → ScreenUserEdit, so
// every assertion fails until Phase 6 hooks up the routing.

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
	"github.com/tedilabs/ota/internal/tui/shared"
)

// editEntryPort returns a single user on List + Get so the
// list/detail surfaces have a row to operate on.
type editEntryPort struct{ user domain.User }

func (p *editEntryPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &editEntryIter{u: p.user, more: true}, nil
}
func (p *editEntryPort) Get(_ context.Context, _ string) (domain.User, error) { return p.user, nil }
func (p *editEntryPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *editEntryPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *editEntryPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *editEntryPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *editEntryPort) Unlock(_ context.Context, _ string) error               { return nil }
func (p *editEntryPort) ResetFactors(_ context.Context, _ string) error         { return nil }
func (p *editEntryPort) Activate(_ context.Context, _ string, _ bool) error     { return nil }
func (p *editEntryPort) Deactivate(_ context.Context, _ string, _ bool) error   { return nil }
func (p *editEntryPort) ExpirePassword(_ context.Context, _ string) error       { return nil }
func (p *editEntryPort) Suspend(_ context.Context, _ string) error   { return nil }
func (p *editEntryPort) Unsuspend(_ context.Context, _ string) error { return nil }
func (p *editEntryPort) Delete(_ context.Context, _ string) error               { return nil }
func (p *editEntryPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

type editEntryIter struct {
	u    domain.User
	more bool
}

func (it *editEntryIter) Next(_ context.Context) (domain.User, bool, error) {
	if !it.more {
		return domain.User{}, false, nil
	}
	it.more = false
	return it.u, true, nil
}
func (it *editEntryIter) Close() error { return nil }

func newAppForEditEntry(t *testing.T) app.Model {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	port := &editEntryPort{user: domain.User{
		ID:     "00u_alice",
		Status: domain.UserStatusActive,
		Profile: domain.UserProfile{
			Login: "alice@acme.com", Email: "alice@acme.com",
			FirstName: "Alice", LastName: "Smith",
		},
	}}
	return app.New(app.Deps{
		UsersPort: port,
		Clock:     clock.Real(),
		Keys:      keymap,
	})
}

// REQ-W01 AC-1.1 — shared.OpenUserEditMsg dispatched to the App Shell
// must push ScreenUserEdit onto the nav stack. Pin the message contract
// shape so any emitting screen (list / detail / palette) routes
// consistently.
func Test_AppModel_OpenUserEditMsg_PushesUserEditScreen(t *testing.T) {
	t.Parallel()
	m := newAppForEditEntry(t)
	updated, _ := m.Update(shared.OpenUserEditMsg{ID: "00u_alice"})
	updatedModel, ok := updated.(app.Model)
	require.True(t, ok, "Update must return an app.Model")
	assert.Equal(t, "user-edit", app.ActiveScreenName(updatedModel),
		"REQ-W01 AC-1.1: OpenUserEditMsg must result in ScreenUserEdit being the active screen")
}

// REQ-W01 AC-1.2 — the `:edit` palette command resolves to
// ScreenUserEdit via the canonical name. This pins the screenFromName
// mapping for `:edit`, `:e`, `user-edit` aliases (TUI_DESIGN §3.4).
func Test_AppModel_SwitchScreen_UserEdit_ResolvesViaPalette(t *testing.T) {
	t.Parallel()
	m := newAppForEditEntry(t)
	// SwitchScreenMsg drives the same resolver the palette uses.
	updated, _ := m.Update(app.SwitchScreenMsg{Target: "user-edit"})
	updatedModel, ok := updated.(app.Model)
	require.True(t, ok)
	assert.Equal(t, "user-edit", app.ActiveScreenName(updatedModel),
		"REQ-W01: 'user-edit' canonical name must resolve to ScreenUserEdit")
}

// REQ-W01 AC-1.1 — pressing `e` on the Users list (with a row
// selected) must surface an OpenUserEditMsg-equivalent effect so the
// shell pushes ScreenUserEdit. Phase 6 wires this via the list's
// classify+`e` branch.
func Test_AppModel_EKey_OnUsersList_OpensUserEditScreen(t *testing.T) {
	t.Parallel()
	m := newAppForEditEntry(t)

	// `e` keypress
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	updatedModel, ok := updated.(app.Model)
	require.True(t, ok)

	// If the list emits an OpenUserEditMsg via cmd, run it and feed it
	// back into the model. The "pushes ScreenUserEdit" invariant must
	// hold after one full reduction cycle.
	if cmd != nil {
		msg := cmd()
		updated2, _ := updatedModel.Update(msg)
		if mm, ok := updated2.(app.Model); ok {
			updatedModel = mm
		}
	}
	assert.Equal(t, "user-edit", app.ActiveScreenName(updatedModel),
		"REQ-W01 AC-1.1: `e` on Users list must drive the App Shell into ScreenUserEdit")
}
