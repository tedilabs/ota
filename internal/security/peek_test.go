// Lock-in test (not Fail-First derived): 본 테스트는 testdata/**/*.golden 및
// testdata/**/*.json에 스크럽되지 않은 raw PII 패턴이 섞여 들어가는 것을 막는
// 회귀 차단(peek) 게이트다. 구현이 아니라 리포지토리 상태를 검증하므로 최초
// 작성 시점에 통과하는 것이 정상이다 (TESTING §11).
//
// 보호 대상:
//   - SSWS 토큰 원문 (E0000004/E0000011 fixture에는 etc 없음)
//   - 실 도메인 이메일 (fixture는 @redacted.example.com만 허용)
//   - US 전화번호 `+1-XXX-XXX-XXXX` 중 `+1-555-0XX-XXXX`가 아닌 패턴
//
// 실패 시 조치: 해당 파일에서 PII를 제거하거나 fixtures_manifest.yaml의 scrub
// 규칙을 개선한 뒤 record-fixture.go를 재실행하라.

package security_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	rxAuthSSWS = regexp.MustCompile(`SSWS\s+[A-Za-z0-9_\-]{20,}`)
	// 일반 전화 패턴. 잡힌 결과를 allowedPhonePrefixes로 한 번 더 필터링.
	rxPhoneUS = regexp.MustCompile(`\+\d{1,2}-\d{1,3}-\d{2,4}-\d{4}`)
	// 스크럽된 값을 제외한 이메일. @redacted.example.com, @acme.com(docs 예시만) 허용.
	rxRealEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
)

// 허구 번호 프리픽스. `+1-555-000-XXXX`와 `+82-10-XXXX-XXXX`(범용 E.164 예시) 허용.
var allowedPhonePrefixes = []string{
	"+1-555-000-",
	"+82-10-1234-",
	"+1-555-123-", // mask 유닛 테스트가 로그에 쓸 수 있는 예시. fixture에는 없음.
	"+1-555-987-",
}

var allowedEmailDomains = []string{
	"@redacted.example.com",
	"@acme.com",       // PRD 예시. 실 tenant 아님.
	"@example.com",    // 범용 예시.
	"@secret.com",     // mask_attr_test의 테스트-전용 payload (코드에만 존재, fixture에는 없음).
}

// Test_Peek_Testdata_HasNoRawPII 는 testdata 디렉토리의 모든 파일을 스캔해
// 허용 패턴 외의 PII가 섞여 있지 않음을 검증한다.
func Test_Peek_Testdata_HasNoRawPII(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// <repo>/internal/security/peek_test.go → repo root = ../..
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	testdata := filepath.Join(repoRoot, "testdata")
	if _, err := os.Stat(testdata); err != nil {
		t.Skipf("testdata not present: %v", err)
	}

	err := filepath.WalkDir(testdata, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		// 관련 확장자만 스캔.
		switch strings.ToLower(filepath.Ext(path)) {
		case ".json", ".yaml", ".yml", ".golden", ".txt":
		default:
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(b)
		rel, _ := filepath.Rel(repoRoot, path)

		// SSWS 원문 토큰 스캔.
		if m := rxAuthSSWS.FindString(content); m != "" {
			t.Errorf("PII leak: SSWS token in %s: %q", rel, m)
		}

		// 실 도메인 이메일 스캔.
		for _, m := range rxRealEmail.FindAllString(content, -1) {
			if isAllowedEmail(m) {
				continue
			}
			t.Errorf("PII leak: email %q in %s (허용 도메인 아님)", m, rel)
		}

		// 전화번호 스캔. 허용 프리픽스 외에는 경고.
		for _, m := range rxPhoneUS.FindAllString(content, -1) {
			if isAllowedPhone(m) {
				continue
			}
			t.Errorf("PII leak: real-looking phone %q in %s (허용 프리픽스 외)", m, rel)
		}
		return nil
	})
	require.NoError(t, err)
}

func isAllowedEmail(addr string) bool {
	lower := strings.ToLower(addr)
	for _, d := range allowedEmailDomains {
		if strings.HasSuffix(lower, d) {
			return true
		}
	}
	return false
}

func isAllowedPhone(v string) bool {
	for _, p := range allowedPhonePrefixes {
		if strings.HasPrefix(v, p) {
			return true
		}
	}
	return false
}
