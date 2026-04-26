package config_test

// REQ-C01, REQ-C02, REQ-C05 — 설정 파일 로더.

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/config"
	"github.com/tedilabs/ota/internal/okta/testfx"
)

// REQ-C01 AC-2 — --config <path> 명시적 오버라이드.
func Test_ConfigLoad_ExplicitPathOverrideSucceeds(t *testing.T) {
	t.Parallel()
	p := filepath.Join(testfx.TestdataRoot(t), "config", "valid_minimal.yaml")

	gotPath, cfg, err := config.Load(config.LoadOptions{ExplicitPath: p})
	require.NoError(t, err)
	assert.Equal(t, p, gotPath, "resolved path should match explicit (REQ-C01 AC-2)")
	require.Contains(t, cfg.Profiles, "dev", "valid_minimal must register profile 'dev'")
	assert.Equal(t, "https://dev-example.okta.com", cfg.Profiles["dev"].OrgURL)
}

// REQ-C01 AC-3 — 4개 섹션(profiles, ui, keybindings, logs)이 모두 로드되어야 한다.
func Test_ConfigLoad_FullConfigAllFourSections(t *testing.T) {
	t.Parallel()
	p := filepath.Join(testfx.TestdataRoot(t), "config", "valid_full.yaml")

	_, cfg, err := config.Load(config.LoadOptions{ExplicitPath: p})
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Profiles, "profiles section loaded")
	assert.NotEmpty(t, cfg.UI.Theme, "ui section loaded")
	assert.NotEmpty(t, cfg.Keybindings, "keybindings section loaded")
	assert.Greater(t, cfg.Logs.PollIntervalSeconds, 0, "logs section loaded")
}

// REQ-C01 AC-1 — YAML syntax 오류는 친절한 에러(행/열 포함).
func Test_ConfigLoad_SyntaxErrorReportedWithLocation(t *testing.T) {
	t.Parallel()
	p := filepath.Join(testfx.TestdataRoot(t), "config", "invalid_syntax.yaml")

	_, _, err := config.Load(config.LoadOptions{ExplicitPath: p})
	require.Error(t, err, "invalid_syntax.yaml은 에러를 반환해야 한다")
	msg := err.Error()
	// line 번호 표기 여부 (엄격 format은 libray 의존. "line" 키워드 포함을 최소 보장).
	assert.Contains(t, msg, "line",
		"syntax error 메시지는 행 번호 정보를 포함해야 한다 (REQ-C01 AC-1)")
}

// REQ-C02 AC-4 + REQ-C05 AC-4 — 토큰 값은 설정 파일에 저장하지 않는다.
// Profile.APITokenEnv가 환경변수 이름이어야 함을 구조 검증.
func Test_ConfigProfile_StoresOnlyEnvVarName_NotTokenValue(t *testing.T) {
	t.Parallel()
	p := filepath.Join(testfx.TestdataRoot(t), "config", "valid_full.yaml")

	_, cfg, err := config.Load(config.LoadOptions{ExplicitPath: p})
	require.NoError(t, err)
	prof, ok := cfg.Profiles["dev"]
	require.True(t, ok)
	assert.Equal(t, "OKTA_API_TOKEN", prof.APITokenEnv,
		"설정 파일은 env var 이름만 저장 (REQ-C05 AC-4)")
}

// REQ-C01 AC-1 + CONVENTIONS §16 — https만 허용.
func Test_ConfigValidate_RejectsHTTPInProfileURL(t *testing.T) {
	t.Parallel()
	p := filepath.Join(testfx.TestdataRoot(t), "config", "invalid_profile.yaml")

	_, _, err := config.Load(config.LoadOptions{ExplicitPath: p})
	require.Error(t, err, "http URL은 거부되어야 한다 (CONVENTIONS §16)")
	assert.Contains(t, err.Error(), "https",
		"에러 메시지에 https 요구사항이 언급되어야 한다")
}
