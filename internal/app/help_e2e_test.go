package app_test

// E2E for the `?` help shortcut: pressing it must open a modal whose body
// surfaces both global keys and the keys specific to the active screen,
// and Esc must close it. Reproduces the user's request that `?` show the
// shortcuts available on the current view.

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

// testUsersForKeyTest seeds two users for the key-delegation regression
// test below. Stable, alphabetised by login so cursor advance is
// deterministic.
func testUsersForKeyTest() []domain.User {
	return []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "alice@acme.com"}},
		{ID: "00u_bob", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "bob@acme.com"}},
	}
}

func newHelpTestModel(t *testing.T) app.Model {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:    keymap,
		Clock:   clock.Real(),
		Profile: "test",
		OrgURL:  "https://acme.okta.com",
	})
	// Issue #170 added a hard body-row cap on the chrome so detail
	// surfaces don't push the top border off-screen. Send a
	// generous WindowSizeMsg so the help modal (~20 rows) fits
	// inside the chrome's body budget; without this the closing
	// `<Esc> close` footer gets clipped before the test sees it.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	return updated.(app.Model)
}

// pressKey feeds a single rune through the App Shell's tea.Update and, when
// the resulting Cmd produces a follow-up Msg (the typical pattern for the
// open*Cmd Cmds), runs that Msg through Update too. Mirrors what tea.Program
// does at runtime so the test can observe the final post-cmd state.
func pressKey(t *testing.T, m app.Model, r rune) app.Model {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	mdl, ok := updated.(app.Model)
	require.True(t, ok, "Update must return an app.Model")
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = mdl.Update(msg)
			mdl, ok = updated.(app.Model)
			require.True(t, ok, "follow-up Update must also return an app.Model")
		}
	}
	return mdl
}

// Test_AppShell_QuestionMark_OpensFullHelpModal locks in the user-visible
// behaviour: `?` replaces the body with the screen-scoped HelpModel modal
// (chrome RoundedBorder + key reference table), not just a footer hint.
func Test_AppShell_QuestionMark_OpensFullHelpModal(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')

	view := m.View()
	// Modal title — proves the full HelpModel rendered, not the placeholder.
	assert.Contains(t, view, "Help · Users List",
		"`?` must open the Users-scoped help modal as the body")
	// Sort keys are Users-specific and must be visible.
	for _, key := range []string{"Shift+S", "Shift+N", "Shift+L", "Shift+C"} {
		assert.Contains(t, view, key,
			"Users help modal must list %q sort key", key)
	}
	// Modal hint footer.
	assert.Contains(t, view, "<Esc> close")
}

// Test_AppShell_HelpEsc_ClosesAndReturnsToList verifies the close path.
func Test_AppShell_HelpEsc_ClosesAndReturnsToList(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')
	require.Contains(t, m.View(), "Help · Users List", "precondition: help open")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(app.Model)

	view := m.View()
	assert.NotContains(t, view, "Help · Users List",
		"Esc must close the help modal")
}

// Test_AppShell_QuestionMarkToggle verifies pressing `?` again closes the
// modal — operators expect symmetric open/close.
func Test_AppShell_QuestionMarkToggle(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')
	require.Contains(t, m.View(), "Help · Users List", "precondition: help open")

	m = pressKey(t, m, '?')
	assert.NotContains(t, m.View(), "Help · Users List",
		"second `?` must close the modal")
}

// Test_AppShell_EscReachesChild_ClosesDetail covers the user-reported gap
// "Detail View에서 이전 화면으로 넘어올 방법이 없는거 같아." App Shell used
// to swallow Esc with `case tea.KeyEsc: return m, nil` before the child
// could see it, so the detail-close handler in Users/Groups/Rules never
// fired. Esc now falls through to child delegation; this test pins it.
func Test_AppShell_EscReachesChild_ClosesDetail(t *testing.T) {
	t.Parallel()

	port := &seededUsersPort{users: testUsersForKeyTest()}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})

	// Boot: load users.
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(app.Model)
		}
	}

	// Press `d` to open detail. d → openUserCmd → userOpenedMsg.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(app.Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(app.Model)
		}
	}
	require.Contains(t, m.View(), "User Detail",
		"precondition: detail view active after `d`")

	// Now press Esc — must close detail and return to the list rendering.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(app.Model)

	view := m.View()
	assert.NotContains(t, view, "User Detail",
		"Esc must reach the child Users screen and close detail mode")
	assert.Contains(t, view, "alice@acme.com",
		"after closing detail, the list rendering must surface again")
}

// Test_AppShell_KeysReachActiveChild guards the App Shell key-delegation
// path. The user reported "Shift+S 같은 단축키가 아무런 변화도 안 일으킨다."
// — the root cause was handleKey() returning (m, nil) for any rune outside
// {:, ?, q}, swallowing j/k/Shift+S/d/Enter before they reached the child
// Screen. This test pins the fix: pressing `j` on the Users screen must
// reach the seeded child and advance its cursor (visible by the "> "
// prefix moving from alice to bob).
func Test_AppShell_KeysReachActiveChild(t *testing.T) {
	t.Parallel()

	port := &seededUsersPort{users: testUsersForKeyTest()}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	m := app.New(app.Deps{
		Keys:      keymap,
		Clock:     clock.Real(),
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
		UsersPort: port,
	})

	// Run the initial fetch Cmd so the seeded users land in the child model.
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(app.Model)
		}
	}

	require.Contains(t, m.View(), "alice@acme.com",
		"precondition: seeded users rendered after Init")
	require.Contains(t, m.View(), "▸ ", "precondition: cursor (▸) visible at row 0")

	// Press `j` — should move cursor down to row 1 (bob).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(app.Model)

	view := m.View()
	// Bob's row should now carry the cursor prefix. Issue #145 column
	// order puts LOGIN first, so the cursor sits next to bob's login.
	assert.Regexp(t, `▸\s+bob@acme\.com`, view,
		"`j` must reach the child Users screen and advance the cursor")
}

// seededUsersPort is a tiny domain.UsersPort double that returns a fixed
// slice of users on List and supports nothing else. Sufficient for the
// key-delegation regression test.
type seededUsersPort struct {
	users []domain.User
}

func (p *seededUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &seededUsersIter{remaining: p.users}, nil
}

func (p *seededUsersPort) Get(_ context.Context, id string) (domain.User, error) {
	for _, u := range p.users {
		if u.ID == id || u.Profile.Login == id {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (p *seededUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}

func (p *seededUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *seededUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}

func (p *seededUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *seededUsersPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *seededUsersPort) ResetFactors(_ context.Context, _ string) error { return nil }
func (p *seededUsersPort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *seededUsersPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *seededUsersPort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *seededUsersPort) Suspend(_ context.Context, _ string) error   { return nil }
func (p *seededUsersPort) Unsuspend(_ context.Context, _ string) error { return nil }
func (p *seededUsersPort) Delete(_ context.Context, _ string) error { return nil }
func (p *seededUsersPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

type seededUsersIter struct{ remaining []domain.User }

func (it *seededUsersIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.remaining) == 0 {
		return domain.User{}, false, nil
	}
	u := it.remaining[0]
	it.remaining = it.remaining[1:]
	return u, true, nil
}
func (it *seededUsersIter) Close() error { return nil }

// Test_AppShell_HelpInternalFilter verifies `/` inside the help modal narrows
// the displayed entries — the `?` overlay's own search.
func Test_AppShell_HelpInternalFilter(t *testing.T) {
	t.Parallel()
	m := newHelpTestModel(t)
	m = pressKey(t, m, '?')

	// Open the help filter and type "sort".
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(app.Model)
	for _, r := range "sort" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(app.Model)
	}

	view := m.View()
	// After filter "sort", non-sort entries (e.g. "open command palette")
	// drop out while sort keys remain.
	assert.False(t, strings.Contains(view, "open command palette"),
		"filter \"sort\" must drop unrelated rows")
	assert.Contains(t, view, "Shift+S",
		"filter \"sort\" must keep sort entries visible")
}
