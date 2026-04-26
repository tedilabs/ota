package shared_test

// Phase 6b Red — PRD §6.4 접근성: NO_COLOR 환경변수 존중.
// TUI_DESIGN §6.2 "monochrome (NO_COLOR 감지): 색 제거, 기호만 사용. 포커스는 reverse video로."

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/tui/shared"
)

// NO_COLOR 환경 변수가 설정되면 shared 스타일이 monochrome 모드로 동작해야 한다.
func Test_Styles_NOCOLOR_EnablesMonochrome(t *testing.T) {
	// Cannot t.Parallel() due to t.Setenv (Go 1.17+ rule).
	t.Setenv("NO_COLOR", "1")

	require.True(t, shared.MonochromeEnabled(),
		"NO_COLOR=1 설정 시 monochrome 모드가 활성화되어야 한다 (TUI_DESIGN §6.2, PRD §6.4)")
}

// NO_COLOR 미설정이면 normal 모드.
func Test_Styles_NoNOCOLOR_ColoredByDefault(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	assert.False(t, shared.MonochromeEnabled(),
		"NO_COLOR 미설정 시 monochrome=false")
}
