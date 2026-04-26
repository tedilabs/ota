package groups_test

// Phase 6b Red — REQ-R02 Groups list/detail.
//
// Stub 상태에서는 Init/Update/View가 전부 no-op이므로 Red로 실패한다.
// Phase 6b-5 구현 완료 후 Green.

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/groups"
)

// REQ-R02 AC-1 — 리스트에 OKTA_GROUP / BUILT_IN 이 렌더되고 이름이 노출되어야 한다.
func Test_GroupsListFlow_InitialRender_ShowsNames(t *testing.T) {
	t.Parallel()

	port := fakes.NewGroupsPort(t)
	port.ListFunc = func(_ context.Context, _ domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
		return &fakes.SliceIterator[domain.Group]{
			Items: []domain.Group{
				{ID: "00g_engineering", Type: domain.GroupTypeOkta, Profile: domain.GroupProfile{Name: "Engineering"}},
				{ID: "00g_everyone", Type: domain.GroupTypeBuiltIn, Profile: domain.GroupProfile{Name: "Everyone"}},
			},
		}, nil
	}

	fixed := clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))
	model := groups.NewListModel(groups.Deps{Port: port, Clock: fixed})

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Engineering")) && bytes.Contains(b, []byte("Everyone"))
	}, teatest.WithCheckInterval(10*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)
	require.Contains(t, string(out), "Engineering",
		"그룹 이름이 리스트에 렌더되어야 한다 (REQ-R02 AC-1)")
}

// REQ-U03 — `/` 키로 필터 입력 진입.
func Test_GroupsListFlow_SlashEntersFilterMode(t *testing.T) {
	t.Parallel()

	port := fakes.NewGroupsPort(t)
	port.ListFunc = func(_ context.Context, _ domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
		return &fakes.SliceIterator[domain.Group]{
			Items: []domain.Group{
				{ID: "00g_engineering", Type: domain.GroupTypeOkta, Profile: domain.GroupProfile{Name: "Engineering"}},
				{ID: "00g_sales", Type: domain.GroupTypeOkta, Profile: domain.GroupProfile{Name: "Sales"}},
			},
		}, nil
	}

	model := groups.NewListModel(groups.Deps{Port: port, Clock: clock.NewFake(time.Now())})
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Engineering"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sales")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)
	require.Contains(t, string(out), "Sales",
		"필터 'sales' 입력 후 Sales가 가시되어야 한다 (REQ-U03)")
}
