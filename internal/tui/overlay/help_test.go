package overlay_test

// Phase 6b Red — REQ-U06 도움말 모달.

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/tui/overlay"
)

// REQ-U06 AC-1 — 도움말은 현재 Vim 키 몇 개를 가시한다.
func Test_Help_Render_ShowsPrimaryKeys(t *testing.T) {
	t.Parallel()

	model := overlay.NewHelpModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	require.NoError(t, err)

	s := string(out)
	// 대표 키 몇 개는 반드시 렌더되어야 한다.
	for _, expect := range []string{"j", "k", "/", ":", "?", "q"} {
		require.Contains(t, s, expect,
			"Help 모달에 주요 키 %q이 노출되어야 한다 (REQ-U06 AC-1)", expect)
	}
}

// REQ-U06 AC-2 — Help 내부 `/` 검색이 가능. 입력 후 매치된 행 가시.
func Test_Help_InternalSearch_FiltersEntries(t *testing.T) {
	t.Parallel()

	model := overlay.NewHelpModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return len(b) > 0 },
		teatest.WithDuration(1*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("quit")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	require.NoError(t, err)
	require.Contains(t, string(out), "quit",
		"검색 'quit' 입력 후 quit 관련 엔트리가 가시 (REQ-U06 AC-2)")
}
