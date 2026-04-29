package logs_test

// Phase 6b Red — REQ-R05 Logs search/tail 화면.
// Phase 6b-8 구현 완료 후 Green.

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
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/logs"
)

// REQ-R05 AC-1 — 초기 로드 후 eventType이 가시되어야 한다.
func Test_LogsSearchFlow_Render_ShowsEventType(t *testing.T) {
	t.Parallel()

	port := fakes.NewLogsPort(t)
	port.SearchFunc = func(_ context.Context, _ domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		return &fakes.SliceIterator[domain.LogEvent]{
			Items: []domain.LogEvent{
				{
					UUID:       "u1",
					Published:  time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC),
					EventType:  "user.session.start",
					Severity:   domain.SeverityInfo,
					DisplayMsg: "User login",
					Outcome:    domain.Outcome{Result: domain.OutcomeSuccess},
				},
			},
		}, nil
	}

	svc := service.NewLogsService(port)
	model := logs.NewSearchModel(logs.Deps{Service: svc, Clock: clock.NewFake(time.Now())})
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 30))

	// v0.1.12 (#158): the row's MESSAGE column prefers DisplayMsg
	// over EventType so "User login" is what surfaces on screen.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("User login"))
	}, teatest.WithCheckInterval(10*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)
	require.Contains(t, string(out), "User login",
		"DisplayMsg이 리스트의 MESSAGE 컬럼에 렌더 (REQ-R05 AC-1)")
}

// REQ-R05 AC-3 — `s` 키로 tail on/off 토글이 되어야 하고, on 상태에서 chrome 의 status row에 [TAIL: Ns] 뱃지가 노출되어야 한다.
// v0.2.0 (#182) 리디자인에서 inline status line이 chrome의 transient status row로 이동했으므로 본 테스트는 SearchModel.StatusBadges()를 통해 검증한다.
func Test_LogsSearchFlow_TailToggleKey_S_EnablesIndicator(t *testing.T) {
	t.Parallel()

	port := fakes.NewLogsPort(t)
	port.SearchFunc = func(_ context.Context, _ domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		return &fakes.SliceIterator[domain.LogEvent]{}, nil
	}
	svc := service.NewLogsService(port)
	model := logs.NewSearchModel(logs.Deps{Service: svc, Clock: clock.NewFake(time.Now())})

	// `s` 키 입력 후 모델 상태를 직접 검증 (teatest는 screen body만 캡처해서
	// chrome status row를 보지 못한다).
	upd, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = upd.(logs.SearchModel)

	tailOn := false
	for _, b := range model.StatusBadges() {
		if b.Key == "TAIL" && b.Value != "off" {
			tailOn = true
		}
	}
	require.True(t, tailOn,
		"'s' 키 입력 후 [TAIL: Ns] 뱃지가 chrome status row에 노출되어야 한다 (TUI_DESIGN §3.3, REQ-R05 AC-3)")
}
