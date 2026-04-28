package users

// v0.1.16 (#178): the Users list surfaces GROUPS and APPS counts as
// new columns after STATUS. Counts populate lazily — the View
// renders "…" until the per-user fetch resolves, then the integer
// (or "?" on error). Reset / refresh ticks reuse cached counts so
// rate-limit budget stays tight.

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

type countsPort struct {
	users    []domain.User
	groups   map[string][]domain.Group
	links    map[string][]domain.AppLink
	groupErr map[string]bool
}

func (p *countsPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &countsIter{remaining: p.users}, nil
}
func (p *countsPort) Get(_ context.Context, _ string) (domain.User, error) {
	return domain.User{}, nil
}
func (p *countsPort) ListGroups(_ context.Context, userID string) ([]domain.Group, error) {
	if p.groupErr[userID] {
		return nil, domain.ErrNotFound
	}
	return p.groups[userID], nil
}
func (p *countsPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *countsPort) ListAppLinks(_ context.Context, userID string) ([]domain.AppLink, error) {
	return p.links[userID], nil
}
func (p *countsPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *countsPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *countsPort) ResetFactors(_ context.Context, _ string) error { return nil }

type countsIter struct{ remaining []domain.User }

func (it *countsIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.remaining) == 0 {
		return domain.User{}, false, nil
	}
	u := it.remaining[0]
	it.remaining = it.remaining[1:]
	return u, true, nil
}
func (it *countsIter) Close() error { return nil }

// drainCmds runs every Cmd in the chain (including tea.Batch / nested
// batches) and feeds the resulting messages back into the model so
// tests can observe the post-fetch state synchronously.
func drainCmds(t *testing.T, m ListModel, cmd tea.Cmd) ListModel {
	t.Helper()
	if cmd == nil {
		return m
	}
	out := cmd()
	if out == nil {
		return m
	}
	if batch, ok := out.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = drainCmds(t, m, c)
		}
		return m
	}
	upd, next := m.Update(out)
	mdl, ok := upd.(ListModel)
	require.True(t, ok)
	return drainCmds(t, mdl, next)
}

// Test_Users_GroupsAppsCounts_PopulateLazily: after the initial
// usersLoadedMsg, the model fires a per-user count fetch for each
// row. Drained, the View renders the integer counts in the GROUPS
// and APPS columns.
func Test_Users_GroupsAppsCounts_PopulateLazily(t *testing.T) {
	t.Parallel()

	users := []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive, Profile: domain.UserProfile{Login: "alice@acme.com"}},
		{ID: "00u_bob", Status: domain.UserStatusActive, Profile: domain.UserProfile{Login: "bob@acme.com"}},
	}
	port := &countsPort{
		users: users,
		groups: map[string][]domain.Group{
			"00u_alice": {{ID: "00g_a"}, {ID: "00g_b"}, {ID: "00g_c"}},
			"00u_bob":   {{ID: "00g_a"}},
		},
		links: map[string][]domain.AppLink{
			"00u_alice": {{ID: "0oa_x"}, {ID: "0oa_y"}},
			"00u_bob":   nil,
		},
	}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Port:  port,
		Clock: clock.Real(),
		Keys:  keymap,
		Width: 200,
	})

	// Process the initial usersLoadedMsg — this also emits the
	// per-user count fetch batch, which we drain.
	upd, cmd := m.Update(usersLoadedMsg{users: users})
	m = drainCmds(t, upd.(ListModel), cmd)

	// Counts must now be cached — alice has 3 groups / 2 apps; bob 1 / 0.
	assert.Equal(t, 3, m.groupCounts["00u_alice"])
	assert.Equal(t, 2, m.appCounts["00u_alice"])
	assert.Equal(t, 1, m.groupCounts["00u_bob"])
	assert.Equal(t, 0, m.appCounts["00u_bob"])

	view := m.View()
	// Header columns must include the new labels.
	assert.Contains(t, view, "GROUPS",
		"GROUPS column header must render after STATUS")
	assert.Contains(t, view, "APPS",
		"APPS column header must render after STATUS")
}

// Test_Users_GroupsAppsCounts_ShowsLoadingPlaceholderUntilFetched:
// before any userCountLoadedMsg arrives, both cells render "…" so
// the operator sees the gap rather than a misleading "0".
func Test_Users_GroupsAppsCounts_ShowsLoadingPlaceholderUntilFetched(t *testing.T) {
	t.Parallel()

	port := &countsPort{users: []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive, Profile: domain.UserProfile{Login: "alice@acme.com"}},
	}}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Port:  port,
		Clock: clock.Real(),
		Keys:  keymap,
		Width: 200,
	})

	// usersLoadedMsg WITHOUT draining the count fetch Cmds.
	upd, _ := m.Update(usersLoadedMsg{users: port.users})
	m = upd.(ListModel)

	view := m.View()
	assert.Contains(t, view, "…",
		"GROUPS / APPS cells must render '…' while the lazy fetch is in flight")
}

// Test_Users_GroupsAppsCounts_RendersErrorAsQuestionMark: when the
// per-user fetch returns an error, the matching cell shows '?' so
// the operator notices the gap without breaking the row layout.
func Test_Users_GroupsAppsCounts_RendersErrorAsQuestionMark(t *testing.T) {
	t.Parallel()

	users := []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive, Profile: domain.UserProfile{Login: "alice@acme.com"}},
	}
	port := &countsPort{
		users:    users,
		groupErr: map[string]bool{"00u_alice": true},
		links:    map[string][]domain.AppLink{"00u_alice": {{ID: "0oa_x"}}},
	}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Port:  port,
		Clock: clock.Real(),
		Keys:  keymap,
		Width: 200,
	})

	upd, cmd := m.Update(usersLoadedMsg{users: users})
	m = drainCmds(t, upd.(ListModel), cmd)

	assert.Equal(t, -1, m.groupCounts["00u_alice"],
		"groups fetch error must cache as -1 sentinel")
	assert.Equal(t, 1, m.appCounts["00u_alice"],
		"apps fetch must succeed independently")
	gCell, aCell := m.formatCountCells("00u_alice")
	assert.Equal(t, "?", gCell, "groups cell must render '?' on fetch error")
	assert.Equal(t, "1", aCell, "apps cell renders the integer count")
}
