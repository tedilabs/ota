package overlay_test

// Phase 6b Red — REQ-U07 종료 확인 다이얼로그.

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/tui/overlay"
)

// REQ-U07 AC-1 — 확인 다이얼로그는 prompt 문자열을 렌더해야 한다.
func Test_Confirm_Render_ShowsPrompt(t *testing.T) {
	t.Parallel()

	prompt := "Tail 실행 중입니다. 정말 종료하시겠습니까?"
	model := overlay.NewConfirmModel(prompt)

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	require.NoError(t, err)

	require.Contains(t, string(out), prompt,
		"confirm prompt가 렌더되어야 한다 (REQ-U07 AC-1)")
}

// REQ-U07 AC-1 — 'y' 키로 승인 선택 가시 (출력에 y/yes 안내 포함).
func Test_Confirm_Render_ShowsYNHint(t *testing.T) {
	t.Parallel()
	model := overlay.NewConfirmModel("Quit?")
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	require.NoError(t, err)
	s := string(out)
	require.True(t,
		(len(s) > 0 && (contains(s, "y/n") || contains(s, "[y/N]") || contains(s, "y:") || contains(s, "Yes"))),
		"y/n 선택 힌트가 가시되어야 한다 (REQ-U07): got %q", s)
}

// teatest 표준 helper — strings.Contains 대체.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
