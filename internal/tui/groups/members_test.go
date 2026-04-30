package groups_test

// Pins the Group Detail Members shortcut (issue #142). The user
// reported "Group Detail에 그룹 멤버 정보 보는 단축키가 동작을 안해" —
// the previous detail view said "Members tab: press l or Tab (not
// implemented in MVP stub)" and pressing the keys didn't fetch
// anything. v0.1.7 wires a real Members tab + `m` shortcut.

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/groups"
)

func init() { testfx.PinTestEnvironment() }

func sampleGroupMembers() []domain.User {
	return []domain.User{
		{
			ID: "00u_alice", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "alice@acme.com"},
		},
		{
			ID: "00u_bob", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "bob@acme.com"},
		},
	}
}

func sampleGroupForMembers() domain.Group {
	return domain.Group{
		ID:   "00g_engineering",
		Type: domain.GroupTypeOkta,
		Profile: domain.GroupProfile{
			Name:        "Engineering",
			Description: "Engineers across all squads",
		},
	}
}

func feedKey(t *testing.T, m groups.ListModel, key tea.KeyMsg) (groups.ListModel, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key)
	out, ok := updated.(groups.ListModel)
	require.True(t, ok)
	return out, cmd
}

// drainGroupsBatch walks a Cmd's output, including tea.BatchMsg
// children, and feeds each resulting Msg back into the model so the
// Members + Apps lazy fetches load synchronously in tests.
func drainGroupsBatch(t *testing.T, m groups.ListModel, cmd tea.Cmd) groups.ListModel {
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
			if c == nil {
				continue
			}
			if next := c(); next != nil {
				upd, _ := m.Update(next)
				if mdl, ok := upd.(groups.ListModel); ok {
					m = mdl
				}
			}
		}
		return m
	}
	upd, _ := m.Update(msg)
	if mdl, ok := upd.(groups.ListModel); ok {
		m = mdl
	}
	return m
}

// Test_GroupDetail_OpenFetchesMembersInline verifies opening Group
// Detail (Enter on the list) fires the Members + Apps box fetches
// (v0.2.2 #189 promoted Members from a tab to a side-by-side box,
// so the fetches now fire on detail-open instead of on `m`).
func Test_GroupDetail_M_OpensMembersTabAndFetches(t *testing.T) {
	t.Parallel()

	called := 0
	port := fakes.NewGroupsPort(t)
	port.MembersFunc = func(_ context.Context, q domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
		called++
		assert.Equal(t, "00g_engineering", q.GroupID,
			"members fetch must carry the open group's ID")
		return &fakes.SliceIterator[domain.User]{Items: sampleGroupMembers()}, nil
	}

	m := groups.NewListModel(groups.Deps{
		Port:          port,
		InitialGroups: []domain.Group{sampleGroupForMembers()},
		Width:         120, Height: 30,
	})

	// Enter opens detail + fires the lazy fetch batch.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(groups.ListModel)
	require.NotNil(t, cmd, "Enter must emit a Cmd batch (Members + Apps fetches)")
	m = drainGroupsBatch(t, m, cmd)

	view := testfx.StripANSI(m.View())
	assert.Equal(t, 1, called, "Members fetch must run exactly once on detail-open")
	assert.Contains(t, view, "alice@acme.com",
		"Members box must surface the fetched users")
	assert.Contains(t, view, "bob@acme.com")
	assert.Contains(t, view, "Members  (2)",
		"Members box title must surface the count")
}

// Test_GroupDetail_M_FocusesMembersBox verifies the `m` shortcut
// jumps cursor focus to the Members box without re-firing the
// fetch (v0.2.2 #189: fetch happens on detail-open, `m` is just a
// focus-jump now).
func Test_GroupDetail_M_RepeatPress_NoRefetch(t *testing.T) {
	t.Parallel()

	called := 0
	port := fakes.NewGroupsPort(t)
	port.MembersFunc = func(_ context.Context, _ domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
		called++
		return &fakes.SliceIterator[domain.User]{Items: sampleGroupMembers()}, nil
	}

	m := groups.NewListModel(groups.Deps{
		Port:          port,
		InitialGroups: []domain.Group{sampleGroupForMembers()},
		Width:         120, Height: 30,
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(groups.ListModel)
	m = drainGroupsBatch(t, m, cmd)
	require.Equal(t, 1, called, "Members fetch fires once on detail-open")

	// `m` now just focuses the box — no fetch.
	_, cmd = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	assert.Nil(t, cmd, "`m` after detail-open must NOT re-fire MembersFunc")
	assert.Equal(t, 1, called, "MembersFunc must stay at 1 invocation")
}
