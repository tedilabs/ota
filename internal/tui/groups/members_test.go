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

// Test_GroupDetail_M_OpensMembersTabAndFetches verifies pressing `m`
// from any detail tab jumps to the Members tab AND fires a fetch
// against GroupsPort.Members.
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

	// Open detail by pressing Enter on the seeded group.
	m, _ = feedKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	// Press `m` to switch to Members tab and trigger the fetch.
	m, cmd := feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	require.NotNil(t, cmd, "`m` must produce a Cmd that fetches members")

	// Drive the Cmd → loaded msg → Update so the model captures rows.
	msg := cmd()
	require.NotNil(t, msg)
	updated, _ := m.Update(msg)
	m = updated.(groups.ListModel)

	view := testfx.StripANSI(m.View())
	assert.Equal(t, 1, called, "Members fetch must run exactly once")
	assert.Contains(t, view, "alice@acme.com",
		"Members tab must surface the fetched users")
	assert.Contains(t, view, "bob@acme.com")
	assert.Contains(t, view, "Members  2",
		"Members tab must surface the count")
}

// Test_GroupDetail_M_RepeatPress_NoRefetch verifies a second `m` (or
// Tab back to Members) re-uses the cached member list — no extra
// rate-limit budget burned.
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

	m, _ = feedKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	require.NotNil(t, cmd)
	updated, _ := m.Update(cmd())
	m = updated.(groups.ListModel)
	require.Equal(t, 1, called)

	// Tab back to Pretty, then Tab forward to Members again.
	m, _ = feedKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	_, cmd = feedKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	assert.Nil(t, cmd, "second `m` on the same group must NOT refire fetch")
	assert.Equal(t, 1, called, "MembersFunc must stay at 1 invocation")
}
