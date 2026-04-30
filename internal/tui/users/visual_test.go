package users_test

// Vim Visual selection + yank in the Detail pane (TUI_DESIGN §3.6 v0.1.2).
// These tests do NOT touch the system clipboard — atotto/clipboard returns
// nil/error depending on the environment. We assert the user-visible
// state: VISUAL banner, cursor row highlight, and the toast that appears
// after `y` (either "yanked N line(s)" or "yank failed: …").

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/users"
)

// detailFixturePort returns a stub UsersPort that hands back the seeded
// user from Get without panic — sufficient for opening the detail surface
// in unit tests without wiring a full okta.Client.
type detailFixturePort struct{ user domain.User }

func (p *detailFixturePort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return nil, nil
}
func (p *detailFixturePort) Get(_ context.Context, _ string) (domain.User, error) {
	return p.user, nil
}
func (p *detailFixturePort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *detailFixturePort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *detailFixturePort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *detailFixturePort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *detailFixturePort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *detailFixturePort) ResetFactors(_ context.Context, _ string) error { return nil }
func (p *detailFixturePort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *detailFixturePort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *detailFixturePort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *detailFixturePort) Delete(_ context.Context, _ string) error { return nil }

// detailHarness opens an inline Detail surface for a single seeded user
// so the keyboard-driven Visual / yank test below stays focused on the
// detail behaviour and doesn't drag in the list-fetch path.
func detailHarness(t *testing.T) users.ListModel {
	t.Helper()
	u := domain.User{
		ID:     "00u_alice",
		Status: domain.UserStatusActive,
		Profile: domain.UserProfile{
			Login:       "alice@acme.com",
			DisplayName: "Alice Smith",
		},
	}
	m := users.NewListModel(users.Deps{
		Port:         &detailFixturePort{user: u},
		InitialUsers: []domain.User{u},
		Width:        120,
		Height:       30,
	})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(users.ListModel)
	require.NotNil(t, cmd, "`d` should emit the open-detail Cmd")
	updated, _ = m.Update(cmd())
	return updated.(users.ListModel)
}

func key(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// hasVisualBadge reports whether the screen's chrome StatusBadges
// list includes a [VISUAL N lines] entry. v0.2.0 (#182) moved the
// inline `-- VISUAL --` banner into the chrome's transient status
// row, so assertions against m.View() no longer see it.
func hasVisualBadge(m users.ListModel) bool {
	for _, b := range m.StatusBadges() {
		if b.Key == "VISUAL" {
			return true
		}
	}
	return false
}

// Test_DetailVisual_VEntersAndShowsBanner asserts the `v` key flips the
// Detail pane into Visual mode. The visible badge moved to the chrome
// status row in v0.2.0 — assert via StatusBadges() instead of View().
func Test_DetailVisual_VEntersAndShowsBanner(t *testing.T) {
	t.Parallel()
	m := detailHarness(t)

	updated, _ := m.Update(key('v'))
	m = updated.(users.ListModel)

	assert.True(t, m.DetailVisualActive(), "`v` must enter Visual mode")
	assert.True(t, hasVisualBadge(m),
		"Visual mode must publish a [VISUAL N] chrome status badge")
}

// Test_DetailVisual_EscCancelsWithoutClosingDetail covers the user
// expectation that Esc inside Visual mode exits the selection but keeps
// the detail surface open (matches Vim semantics).
func Test_DetailVisual_EscCancelsWithoutClosingDetail(t *testing.T) {
	t.Parallel()
	m := detailHarness(t)
	updated, _ := m.Update(key('v'))
	m = updated.(users.ListModel)
	require.True(t, hasVisualBadge(m), "precondition: visual mode active")

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(users.ListModel)

	assert.False(t, hasVisualBadge(m),
		"first Esc must cancel Visual mode")
	assert.Contains(t, m.View(), "User Detail",
		"Esc inside Visual must NOT close the detail surface")
}

// Test_DetailVisual_YankProducesToast asserts that pressing `y` ends
// Visual mode and emits a shared.ToastMsg — either success ("yanked N
// lines") on systems where atotto/clipboard succeeded, or error
// ("yank failed:") otherwise. The App Shell consumes the msg and
// renders the floating band; this test exercises the cmd contract.
func Test_DetailVisual_YankProducesToast(t *testing.T) {
	t.Parallel()
	m := detailHarness(t)
	updated, _ := m.Update(key('v'))
	m = updated.(users.ListModel)
	updated, _ = m.Update(key('j'))
	m = updated.(users.ListModel)
	updated, cmd := m.Update(key('y'))
	m = updated.(users.ListModel)

	require.NotNil(t, cmd, "after `y`, the keypath must emit a ToastMsg cmd")
	msg := cmd()
	toast, ok := msg.(shared.ToastMsg)
	require.True(t, ok, "yank cmd must produce a shared.ToastMsg, got %T", msg)
	hasOK := strings.Contains(toast.Text, "yanked") && strings.Contains(toast.Text, "line")
	hasErr := strings.Contains(toast.Text, "yank failed:")
	assert.True(t, hasOK || hasErr,
		"toast text must read as a yank success or failure: %q", toast.Text)
	assert.False(t, m.DetailVisualActive(),
		"Visual mode must end after `y` regardless of clipboard success")
}

// Test_DetailVisual_JKMovesCursor asserts that line-cursor navigation
// works inside the detail pane (the precondition for Visual mode being
// useful). Asserts the exported DetailLine() accessor since v0.1.3-1
// dropped the visible ▸ marker — the highlight is now style-only and
// stripped under NO_COLOR.
func Test_DetailVisual_JKMovesCursor(t *testing.T) {
	t.Parallel()
	m := detailHarness(t)
	require.Equal(t, 0, m.DetailLine(), "precondition: cursor starts at line 0")

	updated, _ := m.Update(key('j'))
	m = updated.(users.ListModel)
	assert.Equal(t, 1, m.DetailLine(), "`j` must advance the line cursor")

	updated, _ = m.Update(key('j'))
	m = updated.(users.ListModel)
	assert.Equal(t, 2, m.DetailLine(), "second `j` must advance again")

	updated, _ = m.Update(key('k'))
	m = updated.(users.ListModel)
	assert.Equal(t, 1, m.DetailLine(), "`k` must move the cursor up")
}
