package keys_test

// REQ-U01 / REQ-C03 — Key bindings.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/keys"
)

// REQ-U01 AC-1 — 기본 맵은 Vim + 화살표 둘 다 지원.
func Test_KeysResolve_Defaults_IncludeVimAndArrows(t *testing.T) {
	t.Parallel()
	m, warnings, err := keys.Resolve(nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// Vim 기본 매핑 검증.
	assert.Equal(t, "j", m[keys.IDNavDown], "j → nav.down (Vim 기본, REQ-U01)")
	assert.Equal(t, "k", m[keys.IDNavUp], "k → nav.up")
	assert.Equal(t, "/", m[keys.IDSearchOpen], "/ → search.open")
	assert.Equal(t, ":", m[keys.IDCmdOpen], ": → cmd.open")
	assert.Equal(t, "q", m[keys.IDAppQuit], "q → app.quit")

	// Reverse lookup — "j" 입력 시 nav.down으로 분류되어야 한다.
	rev := m.Reverse()
	assert.Equal(t, keys.IDNavDown, rev["j"])
}

// REQ-C03 AC-2 — 사용자 override가 빌트인과 충돌 시 사용자 우선.
func Test_KeysResolve_UserOverride_WinsOnConflict(t *testing.T) {
	t.Parallel()
	m, warnings, err := keys.Resolve(map[string]string{
		"nav.down": "s", // Dvorak 사용자가 j → s로 rebind
	})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, "s", m[keys.IDNavDown],
		"사용자 override가 빌트인을 이긴다 (REQ-C03 AC-2)")
}

// REQ-C03 AC-3 — 잘못된 키 ID는 warning만, 부팅은 계속.
func Test_KeysResolve_UnknownID_ProducesWarningNotError(t *testing.T) {
	t.Parallel()
	m, warnings, err := keys.Resolve(map[string]string{
		"nav.floaty_nonsense": "f",
	})
	require.NoError(t, err, "알 수 없는 ID는 fatal이 아니어야 한다 (REQ-C03 AC-3)")
	require.NotEmpty(t, warnings, "warning은 있어야 한다")
	assert.Contains(t, warnings[0], "nav.floaty_nonsense",
		"warning 메시지는 문제 ID를 지칭해야 한다")
	// 빌트인은 유지.
	assert.Equal(t, "j", m[keys.IDNavDown])
}
