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

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("user.session.start"))
	}, teatest.WithCheckInterval(10*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)
	require.Contains(t, string(out), "user.session.start",
		"eventType이 리스트에 렌더 (REQ-R05 AC-1)")
}

// REQ-R05 AC-3 — `s` 키로 tail on/off 토글이 되어야 하고, on 상태에서 tail 인디케이터가 노출.
func Test_LogsSearchFlow_TailToggleKey_S_EnablesIndicator(t *testing.T) {
	t.Parallel()

	port := fakes.NewLogsPort(t)
	port.SearchFunc = func(_ context.Context, _ domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		return &fakes.SliceIterator[domain.LogEvent]{}, nil
	}
	svc := service.NewLogsService(port)
	model := logs.NewSearchModel(logs.Deps{Service: svc, Clock: clock.NewFake(time.Now())})

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 30))

	// 초기 렌더 후 "s" 키를 눌러 tail 활성.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return len(b) > 0 },
		teatest.WithDuration(1*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)

	// tail on → "tail ON" 세그먼트가 나타나야 한다 (issue #152 status
	// line은 "tail OFF" / "tail ON Ns" 형태로 표시).
	s := string(out)
	require.True(t,
		bytes.Contains([]byte(s), []byte("tail ON")) &&
			!bytes.Contains([]byte(s), []byte("tail OFF")),
		"'s' 키 입력 후 tail 상태가 'tail ON' 으로 변해야 한다 (TUI_DESIGN §3.3, REQ-R05 AC-3)")
}
