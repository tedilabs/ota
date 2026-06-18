package users_test

// Pins the User Detail extras — assigned Groups + Apps sections —
// added in issue #168. When detail opens, two lazy fetches fire
// (ListGroups + ListAppLinks); the loaded results render as
// dedicated sections beneath the 2-col Pretty layout.

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

type extrasUsersPort struct {
	user   domain.User
	groups []domain.Group
	apps   []domain.AppLink
}

func (p *extrasUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &extrasIter{rem: []domain.User{p.user}}, nil
}
func (p *extrasUsersPort) Get(_ context.Context, id string) (domain.User, error) {
	if id == p.user.ID || id == p.user.Profile.Login {
		return p.user, nil
	}
	return domain.User{}, domain.ErrNotFound
}
func (p *extrasUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return p.groups, nil
}
func (p *extrasUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *extrasUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return p.apps, nil
}
func (p *extrasUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *extrasUsersPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *extrasUsersPort) ResetFactors(_ context.Context, _ string) error { return nil }
func (p *extrasUsersPort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *extrasUsersPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *extrasUsersPort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *extrasUsersPort) Suspend(_ context.Context, _ string) error   { return nil }
func (p *extrasUsersPort) Unsuspend(_ context.Context, _ string) error { return nil }
func (p *extrasUsersPort) Delete(_ context.Context, _ string) error { return nil }
func (p *extrasUsersPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

type extrasIter struct{ rem []domain.User }

func (it *extrasIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.rem) == 0 {
		return domain.User{}, false, nil
	}
	u := it.rem[0]
	it.rem = it.rem[1:]
	return u, true, nil
}
func (it *extrasIter) Close() error { return nil }

func extrasFixture() *extrasUsersPort {
	return &extrasUsersPort{
		user: domain.User{
			ID:     "00u_alice",
			Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login:     "alice@acme.com",
				FirstName: "Alice",
				LastName:  "Anderson",
			},
		},
		groups: []domain.Group{
			{ID: "00g_eng", Type: domain.GroupTypeOkta,
				Profile: domain.GroupProfile{Name: "Engineering"}},
			{ID: "00g_admin", Type: domain.GroupTypeBuiltIn,
				Profile: domain.GroupProfile{Name: "Admins"}},
		},
		apps: []domain.AppLink{
			{ID: "0oa1", AppName: "salesforce", Label: "Salesforce"},
			{ID: "0oa2", AppName: "okta_org2org", Label: "Org2Org"},
		},
	}
}

func feedKey(t *testing.T, m users.ListModel, key tea.KeyMsg) (users.ListModel, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key)
	out, ok := updated.(users.ListModel)
	require.True(t, ok)
	return out, cmd
}

// Test_UsersDetail_Extras_Render verifies that opening detail
// fires the two extras fetches and the Pretty body grows the
// Groups + Apps sections once the results land.
func Test_UsersDetail_Extras_Render(t *testing.T) {
	t.Parallel()

	port := extrasFixture()
	m := users.NewListModel(users.Deps{
		Port:         port,
		InitialUsers: []domain.User{port.user},
		Width:        140,
		Height:       40,
	})

	// Press `d` (or Enter) to open the detail. openUserCmd fires
	// userOpenedMsg which, in v0.1.14 (#168), batches the
	// extras fetches.
	m, cmd := feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	require.NotNil(t, cmd, "`d` must produce a Cmd")
	if msg := cmd(); msg != nil {
		updated, c2 := m.Update(msg)
		m = updated.(users.ListModel)
		// Now drain the batched extras Cmd.
		if c2 != nil {
			if next := c2(); next != nil {
				if batch, ok := next.(tea.BatchMsg); ok {
					for _, sub := range batch {
						if subMsg := sub(); subMsg != nil {
							updated, _ := m.Update(subMsg)
							m = updated.(users.ListModel)
						}
					}
				} else {
					updated, _ := m.Update(next)
					m = updated.(users.ListModel)
				}
			}
		}
	}

	view := testfx.StripANSI(m.View())

	// Both new section headers must surface.
	assert.Contains(t, view, "Groups", "Groups section header must render")
	assert.Contains(t, view, "Apps", "Apps section header must render")

	// Group names and app labels must populate from the fetches.
	assert.Contains(t, view, "Engineering", "assigned group name must surface")
	assert.Contains(t, view, "Admins", "second group must surface")
	assert.Contains(t, view, "Salesforce", "assigned app label must surface")
	assert.Contains(t, view, "Org2Org", "second app must surface")
}
