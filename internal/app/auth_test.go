package app_test

// REQ-C04 — 토큰 결정 우선순위. CLI flag > env > interactive prompt.
//
// Phase 5 Red: internal/app/auth.go (또는 동등 API)가 아직 없어 컴파일 실패.
// Phase 6에서 app.ResolveToken (또는 실제 이름) 구현 후 green.

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/config"
)

// REQ-C04 AC-1 — CLI --token-env가 최우선. 환경변수 표기만 받아 값을 읽는다.
func Test_Auth_Resolve_CLIFlagWinsOverEnv(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() — Go 1.17+ rule.
	t.Setenv("OKTA_API_TOKEN", "default-token")
	t.Setenv("OKTA_CUSTOM_TOKEN", "cli-token")

	got, src, err := app.ResolveToken(app.ResolveTokenInput{
		CLITokenEnv: "OKTA_CUSTOM_TOKEN",
		Profile:     config.Profile{APITokenEnv: "OKTA_API_TOKEN"},
	})
	require.NoError(t, err)
	assert.Equal(t, "cli-token", got)
	assert.Equal(t, "env OKTA_CUSTOM_TOKEN", src,
		":about에 노출될 소스 설명 (REQ-C04 AC-1)")
}

// REQ-C04 AC-1 — 프로필의 api_token_env가 두 번째 우선순위.
func Test_Auth_Resolve_ProfileEnvWinsWhenCLIMissing(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() — Go 1.17+ rule.
	t.Setenv("OKTA_API_TOKEN", "profile-token")
	os.Unsetenv("OKTA_CUSTOM_TOKEN")

	got, src, err := app.ResolveToken(app.ResolveTokenInput{
		Profile: config.Profile{APITokenEnv: "OKTA_API_TOKEN"},
	})
	require.NoError(t, err)
	assert.Equal(t, "profile-token", got)
	assert.Equal(t, "env OKTA_API_TOKEN", src)
}

// REQ-C04 AC-3 — 토큰 없이 TUI가 뜨지 않음. 명시 에러 + exit code 1 메시지.
func Test_Auth_Resolve_NoTokenReturnsError(t *testing.T) {
	os.Unsetenv("OKTA_API_TOKEN")
	os.Unsetenv("OKTA_CUSTOM_TOKEN")

	_, _, err := app.ResolveToken(app.ResolveTokenInput{
		Profile: config.Profile{APITokenEnv: "OKTA_API_TOKEN"},
	})
	require.Error(t, err, "토큰 없으면 명시 에러 (REQ-C04 AC-3)")
	assert.Contains(t, err.Error(), "OKTA_API_TOKEN",
		"에러 메시지는 환경변수 이름을 포함해야 한다")
}

// REQ-C05 AC-2 — 반환된 에러 메시지에 토큰 원문이 포함되면 안 된다.
func Test_Auth_Resolve_ErrorDoesNotLeakToken(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() — Go 1.17+ rule.
	t.Setenv("OKTA_API_TOKEN", "   ") // whitespace only → invalid
	_, _, err := app.ResolveToken(app.ResolveTokenInput{
		Profile: config.Profile{APITokenEnv: "OKTA_API_TOKEN"},
	})
	if err == nil {
		t.Skip("공백 토큰을 거부하지 않는 구현은 별도 테스트로 유도")
	}
	// 공백 문자열 자체가 있을 수는 있지만 메모리에 보관되는 원문이 에러에 나오면 안 된다.
	// 이 테스트는 구현자가 trim하고 원문 대신 env 이름만 돌려주는지 검증한다.
	assert.NotContains(t, err.Error(), "   ")
}
