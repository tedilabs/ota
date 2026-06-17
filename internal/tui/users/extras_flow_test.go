package users

// v0.1.15 (#174): User Detail's Groups+Apps boxes share a single
// linear cursor. j flows from the last Groups row into the first
// Apps row; from the last Apps row it wraps back to the first
// Groups row. k mirrors. Esc exits the boxes back to the info
// grid (a second Esc closes the whole detail surface).

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
)

type extrasFlowPort struct {
	user   domain.User
	groups []domain.Group
	links  []domain.AppLink
}

func (p *extrasFlowPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &extrasFlowIter{u: p.user, more: true}, nil
}
func (p *extrasFlowPort) Get(_ context.Context, _ string) (domain.User, error) {
	return p.user, nil
}
func (p *extrasFlowPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return p.groups, nil
}
func (p *extrasFlowPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *extrasFlowPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return p.links, nil
}
func (p *extrasFlowPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *extrasFlowPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *extrasFlowPort) ResetFactors(_ context.Context, _ string) error { return nil }
func (p *extrasFlowPort) Activate(_ context.Context, _ string, _ bool) error { return nil }
func (p *extrasFlowPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *extrasFlowPort) ExpirePassword(_ context.Context, _ string) error { return nil }
func (p *extrasFlowPort) Delete(_ context.Context, _ string) error { return nil }
func (p *extrasFlowPort) UpdateProfile(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, nil
}

type extrasFlowIter struct {
	u    domain.User
	more bool
}

func (it *extrasFlowIter) Next(_ context.Context) (domain.User, bool, error) {
	if !it.more {
		return domain.User{}, false, nil
	}
	it.more = false
	return it.u, true, nil
}
func (it *extrasFlowIter) Close() error { return nil }

func runListCmd(t *testing.T, m ListModel, cmd tea.Cmd) ListModel {
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
			m = runListCmd(t, m, c)
		}
		return m
	}
	updated, next := m.Update(msg)
	return runListCmd(t, updated.(ListModel), next)
}

func openExtrasModel(t *testing.T) ListModel {
	t.Helper()
	user := domain.User{
		ID:      "00u_alice",
		Status:  domain.UserStatusActive,
		Profile: domain.UserProfile{Login: "alice@acme.com"},
	}
	groups := []domain.Group{
		{ID: "00g_a", Profile: domain.GroupProfile{Name: "Group-A"}},
		{ID: "00g_b", Profile: domain.GroupProfile{Name: "Group-B"}},
		{ID: "00g_c", Profile: domain.GroupProfile{Name: "Group-C"}},
	}
	links := []domain.AppLink{
		{ID: "0oa_a", Label: "App-A"},
		{ID: "0oa_b", Label: "App-B"},
	}
	port := &extrasFlowPort{user: user, groups: groups, links: links}

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Port:   port,
		Clock:  clock.Real(),
		Keys:   keymap,
		Width:  120,
		Height: 40,
	})
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = upd.(ListModel)
	if init := m.Init(); init != nil {
		m = runListCmd(t, m, init)
	}
	// Open detail.
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = runListCmd(t, upd.(ListModel), cmd)
	return m
}

func sendRune(t *testing.T, m ListModel, r rune) ListModel {
	t.Helper()
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return runListCmd(t, upd.(ListModel), cmd)
}

// Test_DetailExtras_LinearFlow_GroupsToApps: with 3 groups + 2 apps,
// pressing `]` enters the boxes at Groups[0]; 3 j presses lands at
// Apps[0]; one more j on the last Apps row wraps back to Groups[0].
func Test_DetailExtras_LinearFlow_GroupsToApps(t *testing.T) {
	t.Parallel()

	m := openExtrasModel(t)
	// Enter the boxes.
	m = sendRune(t, m, ']')
	require.True(t, m.detailExtrasFocused, "] must focus the boxes")
	require.Equal(t, 0, m.detailExtrasCur, "initial linear cursor lands at Groups[0]")

	// j three times — should now sit at Apps[0] (cursor index 3).
	m = sendRune(t, m, 'j')
	m = sendRune(t, m, 'j')
	m = sendRune(t, m, 'j')
	assert.Equal(t, 3, m.detailExtrasCur,
		"j × 3 from Groups[0] (3 groups) lands on Apps[0]")

	// j twice more brings us to Apps[1]; one more wraps to Groups[0].
	m = sendRune(t, m, 'j')
	assert.Equal(t, 4, m.detailExtrasCur, "j after Apps[0] → Apps[1]")
	m = sendRune(t, m, 'j')
	assert.Equal(t, 0, m.detailExtrasCur,
		"j after the last Apps row must wrap back to Groups[0]")

	// k from Groups[0] wraps back to last Apps.
	m = sendRune(t, m, 'k')
	assert.Equal(t, 4, m.detailExtrasCur,
		"k from Groups[0] must wrap to Apps[last]")
}

// Test_DetailExtras_EscExitsBoxesNotDetail: Esc inside the boxes
// returns to the info grid; the second Esc closes the whole detail
// surface so the user-list stays accessible.
func Test_DetailExtras_EscExitsBoxesNotDetail(t *testing.T) {
	t.Parallel()

	m := openExtrasModel(t)
	m = sendRune(t, m, ']')
	require.True(t, m.detailExtrasFocused)

	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = runListCmd(t, upd.(ListModel), cmd)
	assert.False(t, m.detailExtrasFocused, "Esc must exit the boxes back to the info grid")
	assert.True(t, m.opened, "first Esc must NOT close the whole detail surface")

	upd, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = runListCmd(t, upd.(ListModel), cmd)
	assert.False(t, m.opened, "second Esc closes the detail surface")
}

// Test_DetailExtras_BracketJumpsToApps: `]` while already inside the
// boxes jumps the cursor to the Apps column's first row, matching
// the muscle memory of operators that want a quick column switch.
func Test_DetailExtras_BracketJumpsToApps(t *testing.T) {
	t.Parallel()

	m := openExtrasModel(t)
	m = sendRune(t, m, ']') // enter
	require.Equal(t, 0, m.detailExtrasCur)

	m = sendRune(t, m, ']') // jump to Apps[0]
	assert.Equal(t, 3, m.detailExtrasCur,
		"] inside the boxes jumps to the first Apps row (after 3 groups)")

	m = sendRune(t, m, '[') // back to Groups[0]
	assert.Equal(t, 0, m.detailExtrasCur,
		"[ inside the boxes returns the cursor to Groups[0]")
}
