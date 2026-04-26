package keys_test

// Phase 6b 재개방: TUI_DESIGN §3 전체 매핑을 테이블 드리븐으로 lock-in.
// 현재 코드(`keys/defaults.go`)가 TUI_DESIGN 명세와 불일치할 경우 Red.
//
// 관련 REQ: REQ-U01, REQ-U03, REQ-U07, REQ-R05 AC-3, REQ-C03.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/keys"
)

// TUI_DESIGN §3 전체 기본 키 매핑. 이 표와 코드가 다르면 Red.
//
// 특별 주의:
//   - `IDLogsTailToggle` 은 §3.3에서 `s`. 현재 코드는 `t` → Red 예상.
//   - `IDLogsFollowToggle` 은 `f`.
//   - `IDPIIUnmask`, `IDPIIMask` 는 프롬프트 커맨드 문자열.
func Test_KeysDefaults_EntireMapMatchesTUIDesign(t *testing.T) {
	t.Parallel()

	defaults, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	cases := []struct {
		id   keys.ID
		want string
	}{
		// Navigation (TUI_DESIGN §3.2)
		{keys.IDNavDown, "j"},
		{keys.IDNavUp, "k"},
		{keys.IDNavLeft, "h"},
		{keys.IDNavRight, "l"},
		{keys.IDNavTop, "g g"},
		{keys.IDNavBottom, "G"},
		{keys.IDNavPageUp, "Ctrl-b"},
		{keys.IDNavPageDn, "Ctrl-f"},

		// App (TUI_DESIGN §3.1)
		{keys.IDAppQuit, "q"},
		{keys.IDAppHelp, "?"},
		{keys.IDAppRefresh, "R"},
		{keys.IDAppBack, "Esc"},

		// Prompts/search
		{keys.IDCmdOpen, ":"},
		{keys.IDSearchOpen, "/"},
		{keys.IDSearchNext, "n"},
		{keys.IDSearchPrev, "N"},

		// Logs (TUI_DESIGN §3.3 — KEY POINT: tail_toggle = "s", follow = "f")
		{keys.IDLogsTailToggle, "s"},   // 현재 코드 "t" 이면 Red
		{keys.IDLogsFollowToggle, "f"}, // 일치

		// PII mask prompts
		{keys.IDPIIUnmask, ":unmask"},
		{keys.IDPIIMask, ":mask"},
	}

	for _, tc := range cases {
		t.Run(string(tc.id), func(t *testing.T) {
			t.Parallel()
			got, ok := defaults[tc.id]
			require.True(t, ok, "ID %q이 기본 맵에 없다", tc.id)
			assert.Equal(t, tc.want, got,
				"TUI_DESIGN §3 불일치: ID=%q 기대=%q 실제=%q (docs/TUI_DESIGN.md §3 참조)",
				tc.id, tc.want, got)
		})
	}
}

// Reverse lookup — 사용자가 "s" 입력 시 logs.tail_toggle로 분류되어야 한다.
func Test_KeysDefaults_ReverseLookup_Tail_S(t *testing.T) {
	t.Parallel()
	defaults, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	rev := defaults.Reverse()
	assert.Equal(t, keys.IDLogsTailToggle, rev["s"],
		"\"s\" 키는 logs.tail_toggle 이어야 한다 (TUI_DESIGN §3.3)")
}
