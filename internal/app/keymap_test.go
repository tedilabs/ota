package app_test

// REQ-U01 / REQ-U03 — 공통 키 매핑 검증.
// App Shell이 tea.KeyMsg를 receive하면 올바른 keys.ID로 분류해야 한다.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/keys"
)

// REQ-U01 AC-1 — 화살표 키와 Vim 키가 동일 ID로 분류되어야 한다.
func Test_Keymap_ClassifyKey_ArrowsEquivalentToVim(t *testing.T) {
	t.Parallel()
	resolved, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	cases := []struct {
		name string
		msg  tea.KeyMsg
		want keys.ID
	}{
		{"j_down", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, keys.IDNavDown},
		{"arrow_down", tea.KeyMsg{Type: tea.KeyDown}, keys.IDNavDown},
		{"k_up", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}, keys.IDNavUp},
		{"arrow_up", tea.KeyMsg{Type: tea.KeyUp}, keys.IDNavUp},
		{"slash_search", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}, keys.IDSearchOpen},
		{"colon_cmd", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")}, keys.IDCmdOpen},
		{"q_quit", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}, keys.IDAppQuit},
		{"question_help", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}, keys.IDAppHelp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := app.ClassifyKey(tc.msg, resolved)
			assert.Equal(t, tc.want, got,
				"key %+v → %s; got %s (REQ-U01 AC-1)", tc.msg, tc.want, got)
		})
	}
}

// REQ-U03 AC-1 — `/` 키는 리스트 화면에서만 search.open으로 해석되고, 텍스트 입력 중에는
// 일반 문자로 취급되어야 한다.
func Test_Keymap_ClassifyKey_DoesNotDispatchWhenInputCapturesKeys(t *testing.T) {
	t.Parallel()
	// resolved 맵은 정상적으로 `/` → search.open. 그러나 입력 캡처 상태에서는
	// 분류기가 빈 ID를 돌려줘야 한다. 구현은 별도 컨텍스트 매개 변수에 의존한다.
	// 이 테스트는 app.ClassifyKeyInContext(msg, resolved, ctx)가 존재하는지 유도한다.
	resolved, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	got := app.ClassifyKeyInContext(
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")},
		resolved,
		app.KeyContextInputActive,
	)
	assert.Empty(t, got, "입력 캡처 컨텍스트에서는 ID가 부여되면 안 된다 (REQ-U03 AC-1)")
}
