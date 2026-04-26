package logger_test

// REQ-C05 / REQ-O01 — slog ReplaceAttr 기반 민감 필드 마스킹.

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/logger"
)

// REQ-C05 AC-2 — Authorization 헤더 값이 로그에 원문으로 기록되면 안 된다.
func Test_Logger_MaskAttr_ReplacesAuthorizationValueWithAsterisks(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: logger.MaskAttr})
	log := slog.New(h)

	log.Info("okta call",
		slog.String("authorization", "SSWS SECRET-TOKEN-VALUE"),
		slog.String("path", "/api/v1/users"),
	)

	out := buf.String()
	assert.NotContains(t, out, "SECRET-TOKEN-VALUE",
		"원문 토큰이 로그에 나타나면 안 된다 (REQ-C05 AC-2)")
	assert.Contains(t, out, "***", "마스킹 마커가 있어야 한다")
	// path 같은 비민감 필드는 보존.
	assert.Contains(t, out, "/api/v1/users")
}

// REQ-C05 / REQ-R01 AC-6 — 다른 민감 키들도 모두 마스킹.
func Test_Logger_MaskAttr_MasksAllSensitiveKeys(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: logger.MaskAttr})
	log := slog.New(h)

	log.Info("pii test",
		slog.String("mobile_phone", "+1-555-123-4567"),
		slog.String("second_email", "alice.personal@secret.com"),
		slog.String("phone_number", "+1-555-987-6543"),
		slog.String("api_token", "00a-RAW"),
		slog.String("token", "00a-OTHER"),
	)

	out := buf.String()
	leak := []string{
		"+1-555-123-4567",
		"alice.personal",
		"+1-555-987-6543",
		"00a-RAW",
		"00a-OTHER",
	}
	for _, s := range leak {
		assert.False(t, strings.Contains(out, s),
			"로그에 원문 %q 누출 (REQ-C05)", s)
	}
}

// REQ-O01 AC-2 — Logger.New는 io.Discard sink로 구성 가능해야 한다 (테스트 편의).
func Test_Logger_New_WithDiscardSinkSucceeds(t *testing.T) {
	t.Parallel()
	lg, err := logger.New(logger.Options{
		SessionID: "test-session",
		Sink:      io.Discard,
	})
	require.NoError(t, err)
	require.NotNil(t, lg)
	// 기본 호출이 panic하지 않아야 한다.
	lg.Info("ping")
}
