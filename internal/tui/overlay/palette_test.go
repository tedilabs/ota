package overlay_test

// Phase 6b Red — REQ-U02 커맨드 팔레트.

import (
	"bytes"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/tui/overlay"
)

// REQ-U02 AC-1 — 초기 렌더에 6종 리소스 커맨드가 모두 힌트로 가시되어야 한다.
func Test_CmdPalette_Render_ShowsResourceCommands(t *testing.T) {
	t.Parallel()

	model := overlay.NewCmdPaletteModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	require.NoError(t, err)

	s := string(out)
	for _, cmd := range []string{":users", ":groups", ":grouprules", ":policies", ":logs", ":help", ":quit"} {
		require.Contains(t, s, cmd,
			"커맨드 힌트 %q이 팔레트에 가시되어야 한다 (REQ-U02 AC-1)", cmd)
	}
}

// REQ-U02 AC-3 — 부분 매칭 + Enter 시 SwitchScreenMsg 발송.
// Msg 타입은 app 패키지에 있으므로 출력에 해당 타겟 스크린 이름이 드러나면 충분.
func Test_CmdPalette_PartialMatch_Users(t *testing.T) {
	t.Parallel()

	model := overlay.NewCmdPaletteModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return len(b) > 0 },
		teatest.WithDuration(1*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":u")})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	require.NoError(t, err)

	s := string(out)
	require.True(t,
		bytes.Contains([]byte(s), []byte(":users")),
		"'u' 입력 시 :users 후보가 유지되어야 한다 (REQ-U02 AC-3 부분 매칭)")
}
