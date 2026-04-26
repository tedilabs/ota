package users_test

// v0.1.1 Red — TUI_DESIGN §3.6 (v1.2.0): `d` key opens the Detail view.
//
// `d` is an Enter alternative. Operators familiar with k9s expect it to
// surface the full set of attributes including raw JSON. Today the Users
// ListModel only handles Enter (KeyEnter); pressing `d` is a no-op.
//
// Two assertions:
//
//   1. Pressing `d` with a row selected returns a non-nil Cmd. We don't
//      assert on the Msg shape (it's an internal userOpenedMsg) but its
//      presence proves the keystroke was consumed.
//   2. After the resulting userOpenedMsg processes, the model enters detail
//      mode (View output diverges from the list View).

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

// Test_UsersList_DKey_EmitsOpenCmd — pressing `d` produces a non-nil Cmd
// (the open-detail Cmd). Today's handleKey returns (m, nil) for `d` because
// the keymap doesn't bind it to IDNavSelect.
func Test_UsersList_DKey_EmitsOpenCmd(t *testing.T) {
	t.Parallel()

	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, id string) (domain.User, error) {
		return domain.User{ID: id, Profile: domain.UserProfile{Login: "alice@acme.com"}}, nil
	}

	m := users.NewListModel(users.Deps{
		Port:         port,
		Clock:        clock.NewFake(fixtureNow()),
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	require.NotNil(t, cmd,
		"pressing `d` with a selected row must emit a Cmd to fetch detail (TUI_DESIGN §3.6)")
	// Sanity: model is still ListModel (detail enter happens via the
	// userOpenedMsg from the Cmd — the developer's wiring may differ).
	_, ok := updated.(users.ListModel)
	require.True(t, ok, "Update return must remain ListModel until userOpenedMsg arrives")
}

// Test_UsersList_DKey_NoSelection_NoCmd — when there is no selectable row
// (empty users + no cursor), `d` must be a no-op (no Cmd) so we don't trigger
// a Get call against an empty selection.
func Test_UsersList_DKey_NoSelection_NoCmd(t *testing.T) {
	t.Parallel()

	m := users.NewListModel(users.Deps{
		Width:  120,
		Height: 30,
		Clock:  clock.NewFake(fixtureNow()),
	})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	assert.Nil(t, cmd,
		"`d` with empty list must not emit a Cmd (TUI_DESIGN §3.6)")
}

// Test_UsersList_DKey_OpensDetailView — after `d`, the Cmd's Msg is fed back
// to Update; the resulting View must render the detail surface (login of
// the selected user appears in detail header).
func Test_UsersList_DKey_OpensDetailView(t *testing.T) {
	t.Parallel()

	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, id string) (domain.User, error) {
		return domain.User{
			ID:      id,
			Profile: domain.UserProfile{Login: "alice@acme.com", DisplayName: "Alice Smith"},
			Status:  domain.UserStatusActive,
		}, nil
	}

	m := users.NewListModel(users.Deps{
		Port:         port,
		Clock:        clock.NewFake(fixtureNow()),
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(users.ListModel)
	require.NotNil(t, cmd, "`d` must emit a Cmd")
	msg := cmd()
	require.NotNil(t, msg, "the Cmd's Msg must be deliverable")
	updated, _ = m.Update(msg)
	m = updated.(users.ListModel)

	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "User Detail",
		"after `d` + userOpenedMsg, View must enter detail mode (TUI_DESIGN §3.6)")
	assert.Contains(t, got, "alice@acme.com",
		"detail view must surface the selected user's login")
}
