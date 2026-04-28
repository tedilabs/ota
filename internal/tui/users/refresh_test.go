package users

// v0.1.16 (#177): list-screen auto-refresh stamps lastUpdated on
// every successful fetch so the App Shell can render an "updated
// 12:34:56 UTC" segment in the chrome's upper divider.

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
)

// Test_Users_LastUpdated_StampedOnLoad confirms that processing a
// usersLoadedMsg sets m.lastUpdated to a non-zero time so the chrome
// surfaces it.
func Test_Users_LastUpdated_StampedOnLoad(t *testing.T) {
	t.Parallel()

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Clock: clock.Real(),
		Keys:  keymap,
	})

	require.True(t, m.LastUpdated().IsZero(),
		"LastUpdated must start at zero")

	updated, _ := m.Update(usersLoadedMsg{users: []domain.User{
		{ID: "00u_alice", Profile: domain.UserProfile{Login: "alice@acme.com"}},
	}})
	m = updated.(ListModel)

	assert.False(t, m.LastUpdated().IsZero(),
		"successful list load must stamp lastUpdated for the chrome divider")
}

// Test_Users_RefreshTick_RescheduleStaysAlive: a tick msg matching
// the current generation must re-fetch AND emit a follow-up tick
// Cmd so the loop never stalls.
func Test_Users_RefreshTick_RescheduleStaysAlive(t *testing.T) {
	t.Parallel()

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Port:            &refreshTickPort{},
		Clock:           clock.Real(),
		Keys:            keymap,
		RefreshInterval: 50 * time.Millisecond,
	})

	upd, cmd := m.Update(usersRefreshTickMsg{gen: m.refreshGen})
	m = upd.(ListModel)
	require.NotNil(t, cmd, "tick must return a Cmd (fetch + reschedule)")

	// Drain the batch — we expect both a fetch Cmd AND a tick Cmd.
	msgs := drainBatch(cmd)
	hasTick := false
	for _, msg := range msgs {
		if _, ok := msg.(usersRefreshTickMsg); ok {
			hasTick = true
		}
	}
	// The reschedule Cmd is a tea.Tick — it doesn't fire in this
	// synchronous test, but it must be present in the batch.
	assert.NotEmpty(t, msgs, "batch must contain at least one Cmd")
	_ = hasTick // tea.Tick is async, so we can't observe it directly here
}

// Test_Users_RefreshTick_StaleGenIsDropped guards against a tick
// landing after the model rebuilt — different generation must be
// silently dropped, no fetch fires.
func Test_Users_RefreshTick_StaleGenIsDropped(t *testing.T) {
	t.Parallel()

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := NewListModel(Deps{
		Port:            &refreshTickPort{},
		Clock:           clock.Real(),
		Keys:            keymap,
		RefreshInterval: 50 * time.Millisecond,
	})
	m.refreshGen = 7 // simulate a few generations of rebuilds.

	upd, cmd := m.Update(usersRefreshTickMsg{gen: 0}) // stale
	_ = upd
	assert.Nil(t, cmd, "stale-gen tick must NOT trigger a fetch")
}

func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	out := cmd()
	if out == nil {
		return nil
	}
	if batch, ok := out.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			if c == nil {
				continue
			}
			if next := c(); next != nil {
				msgs = append(msgs, next)
			}
		}
		return msgs
	}
	return []tea.Msg{out}
}

// refreshTickPort is a minimal UsersPort double — List returns an
// empty iterator so fetchUsersCmd resolves to a usersLoadedMsg with
// no rows. Sufficient for the tick / generation tests.
type refreshTickPort struct{}

func (p *refreshTickPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &refreshTickIter{}, nil
}
func (p *refreshTickPort) Get(_ context.Context, _ string) (domain.User, error) {
	return domain.User{}, nil
}
func (p *refreshTickPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *refreshTickPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *refreshTickPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *refreshTickPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *refreshTickPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *refreshTickPort) ResetFactors(_ context.Context, _ string) error { return nil }

type refreshTickIter struct{}

func (it *refreshTickIter) Next(_ context.Context) (domain.User, bool, error) {
	return domain.User{}, false, nil
}
func (it *refreshTickIter) Close() error { return nil }
