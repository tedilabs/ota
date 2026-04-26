package rules_test

// Phase 6b Red — REQ-R03 Group Rules list.
// INVALID 상태 배지가 반드시 시각적으로 노출되어야 한다 (경고색 요구).

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
	"github.com/tedilabs/ota/internal/tui/rules"
)

// REQ-R03 AC-1/AC-2 — 리스트에 ACTIVE·INACTIVE·INVALID 3종 상태가 모두 노출.
// INVALID 행은 "INVALID" 문자열 또는 배지로 렌더.
func Test_RulesListFlow_Render_ShowsAllStatesIncludingINVALID(t *testing.T) {
	t.Parallel()

	rulesPort := fakes.NewGroupRulesPort(t)
	rulesPort.ListFunc = func(_ context.Context, _ domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
		return &fakes.SliceIterator[domain.GroupRule]{
			Items: []domain.GroupRule{
				{ID: "0pr_active", Name: "Engineers to Eng", Status: domain.GroupRuleStatusActive},
				{ID: "0pr_inactive", Name: "Sales paused", Status: domain.GroupRuleStatusInactive},
				{ID: "0pr_invalid", Name: "Broken reference", Status: domain.GroupRuleStatusInvalid},
			},
		}, nil
	}
	groupsPort := fakes.NewGroupsPort(t)

	model := rules.NewListModel(rules.Deps{
		Port:   rulesPort,
		Groups: groupsPort,
		Clock:  clock.NewFake(time.Now()),
	})
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("INVALID"))
	}, teatest.WithCheckInterval(10*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)

	s := string(out)
	require.Contains(t, s, "ACTIVE", "ACTIVE rule 가시")
	require.Contains(t, s, "INACTIVE", "INACTIVE rule 가시")
	require.Contains(t, s, "INVALID", "INVALID rule 가시 — 경고색 대상 (REQ-R03 AC-2)")
	require.Contains(t, s, "Broken reference", "INVALID rule 이름 가시")
}
