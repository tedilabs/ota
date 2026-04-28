package app_test

// Issue #171: Enter on a Groups / Apps row inside User Detail must
// route through the App Shell to the matching list with the target
// resource's detail open. The shell hops screens AND forwards an
// internal OpenDetailByIDMsg to the destination so the operator
// lands on the resource they pointed at — not on a fresh list.

import (
	"context"
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

// drilldownUsersPort returns a single user that has one assigned
// group and one assigned app, so opening the detail surface fills
// the focusable Groups / Apps boxes deterministically.
type drilldownUsersPort struct {
	user  domain.User
	group domain.Group
	link  domain.AppLink
}

func (p *drilldownUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &oneShotUserIter{u: p.user, more: true}, nil
}
func (p *drilldownUsersPort) Get(_ context.Context, _ string) (domain.User, error) {
	return p.user, nil
}
func (p *drilldownUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return []domain.Group{p.group}, nil
}
func (p *drilldownUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *drilldownUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return []domain.AppLink{p.link}, nil
}
func (p *drilldownUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *drilldownUsersPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *drilldownUsersPort) ResetFactors(_ context.Context, _ string) error { return nil }

type oneShotUserIter struct {
	u    domain.User
	more bool
}

func (it *oneShotUserIter) Next(_ context.Context) (domain.User, bool, error) {
	if !it.more {
		return domain.User{}, false, nil
	}
	it.more = false
	return it.u, true, nil
}
func (it *oneShotUserIter) Close() error { return nil }

// drilldownGroupsPort answers Get with a fixed group so the App
// Shell's OpenGroupDetailMsg path can land on detail mode.
type drilldownGroupsPort struct {
	group domain.Group
	hits  int
}

func (p *drilldownGroupsPort) List(_ context.Context, _ domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
	return &emptyGroupsIter{}, nil
}
func (p *drilldownGroupsPort) Get(_ context.Context, id string) (domain.Group, error) {
	p.hits++
	if id != p.group.ID {
		return domain.Group{}, domain.ErrNotFound
	}
	return p.group, nil
}
func (p *drilldownGroupsPort) Members(_ context.Context, _ domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
	return &drilldownEmptyUsersIter{}, nil
}
func (p *drilldownGroupsPort) AppCount(_ context.Context, _ string) (int, error) { return 0, nil }

type emptyGroupsIter struct{}

func (it *emptyGroupsIter) Next(_ context.Context) (domain.Group, bool, error) {
	return domain.Group{}, false, nil
}
func (it *emptyGroupsIter) Close() error { return nil }

type drilldownEmptyUsersIter struct{}

func (it *drilldownEmptyUsersIter) Next(_ context.Context) (domain.User, bool, error) {
	return domain.User{}, false, nil
}
func (it *drilldownEmptyUsersIter) Close() error { return nil }

// drilldownAppsPort answers Get for the App detail drill-down.
type drilldownAppsPort struct {
	app  domain.App
	hits int
}

func (p *drilldownAppsPort) List(_ context.Context, _ domain.AppsQuery) (domain.Iterator[domain.App], error) {
	return &emptyAppsIter{}, nil
}
func (p *drilldownAppsPort) Get(_ context.Context, id string) (domain.App, error) {
	p.hits++
	if id != p.app.ID {
		return domain.App{}, domain.ErrNotFound
	}
	return p.app, nil
}

type emptyAppsIter struct{}

func (it *emptyAppsIter) Next(_ context.Context) (domain.App, bool, error) {
	return domain.App{}, false, nil
}
func (it *emptyAppsIter) Close() error { return nil }

func runCmd(t *testing.T, m app.Model, cmd tea.Cmd) app.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = runCmd(t, m, c)
		}
		return m
	}
	updated, next := m.Update(msg)
	mdl, ok := updated.(app.Model)
	require.True(t, ok)
	return runCmd(t, mdl, next)
}

func sendKey(t *testing.T, m app.Model, msg tea.KeyMsg) app.Model {
	t.Helper()
	updated, cmd := m.Update(msg)
	mdl, ok := updated.(app.Model)
	require.True(t, ok)
	return runCmd(t, mdl, cmd)
}

// Test_AppShell_UserDetail_EnterOnGroup_OpensGroupDetail is the
// Issue #171 regression: focus on the Groups box (`]` cycles to it),
// Enter must land on Groups screen with the assigned group's detail
// open.
func Test_AppShell_UserDetail_EnterOnGroup_OpensGroupDetail(t *testing.T) {
	t.Parallel()

	user := domain.User{
		ID:      "00u_alice",
		Status:  domain.UserStatusActive,
		Profile: domain.UserProfile{Login: "alice@acme.com"},
	}
	group := domain.Group{ID: "00g_team", Profile: domain.GroupProfile{Name: "Team Alpha"}}
	link := domain.AppLink{ID: "0oa_app1", Label: "Salesforce", AppName: "salesforce"}

	usersPort := &drilldownUsersPort{user: user, group: group, link: link}
	groupsPort := &drilldownGroupsPort{group: group}
	appsPort := &drilldownAppsPort{}

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:       keymap,
		Clock:      clock.Real(),
		Profile:    "test",
		OrgURL:     "https://acme.okta.com",
		UsersPort:  usersPort,
		GroupsPort: groupsPort,
		AppsPort:   appsPort,
	})

	// Generous size so the chrome's body cap doesn't clip the boxes.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(app.Model)

	// Boot: load users.
	if init := m.Init(); init != nil {
		m = runCmd(t, m, init)
	}
	require.Contains(t, m.View(), "alice@acme.com",
		"precondition: list rendered")

	// Open detail (`d`). The detail-extras fetches kick off in the same
	// Update cycle.
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	require.Contains(t, m.View(), "User Detail")

	// Cycle focus to Groups (focus = 1) via `]`.
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})

	// Press Enter — should hop to Groups screen with detail open.
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, "groups", app.ActiveScreenName(m),
		"Enter on Groups row must switch active screen to Groups")
	assert.True(t, groupsPort.hits >= 1,
		"Groups list must call GroupsPort.Get to fetch the target group")
	view := m.View()
	assert.Contains(t, strings.ToLower(view), "team alpha",
		"Groups screen must surface the resolved group's profile after drill-down")
}

// Test_AppShell_UserDetail_EnterOnApp_OpensAppDetail is the Apps
// counterpart: `]` twice from focus=0 lands on Apps box; Enter routes
// to ScreenApps with the app detail mounted.
func Test_AppShell_UserDetail_EnterOnApp_OpensAppDetail(t *testing.T) {
	t.Parallel()

	user := domain.User{
		ID:      "00u_alice",
		Status:  domain.UserStatusActive,
		Profile: domain.UserProfile{Login: "alice@acme.com"},
	}
	group := domain.Group{ID: "00g_team", Profile: domain.GroupProfile{Name: "Team Alpha"}}
	link := domain.AppLink{ID: "0oa_app1", Label: "Salesforce", AppName: "salesforce"}
	appResource := domain.App{
		ID:         "0oa_app1",
		Label:      "Salesforce",
		Name:       "salesforce",
		Status:     "ACTIVE",
		SignOnMode: "SAML_2_0",
		Type:       domain.AppTypeSAML,
	}

	usersPort := &drilldownUsersPort{user: user, group: group, link: link}
	groupsPort := &drilldownGroupsPort{group: group}
	appsPort := &drilldownAppsPort{app: appResource}

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:       keymap,
		Clock:      clock.Real(),
		Profile:    "test",
		OrgURL:     "https://acme.okta.com",
		UsersPort:  usersPort,
		GroupsPort: groupsPort,
		AppsPort:   appsPort,
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(app.Model)

	if init := m.Init(); init != nil {
		m = runCmd(t, m, init)
	}

	// Open detail then cycle focus twice (`]` `]`) to reach the Apps box
	// (focus = 2).
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, "apps", app.ActiveScreenName(m),
		"Enter on Apps row must switch active screen to Apps")
	assert.True(t, appsPort.hits >= 1,
		"Apps Wrapper must call AppsPort.Get to fetch the target app")
	view := m.View()
	assert.Contains(t, view, "Salesforce",
		"Apps screen must surface the resolved app's detail after drill-down")
}
