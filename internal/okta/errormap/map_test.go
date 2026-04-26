package errormap_test

// REQ-U04 AC-3, REQ-C04 AC-4, PRD §7.7 — Okta errorCode → domain 에러 매핑.
// 8종 errorCode 전부 테이블 드리븐. Fixture는 testdata/oktaapi/errors/.

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta/errormap"
	"github.com/tedilabs/ota/internal/okta/testfx"
)

// REQ-U04 AC-3 / REQ-C04 AC-4 — 8종 전부 센티넬로 매핑.
func Test_ErrorMap_FromResponse_MapsAllKnownCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		fixture      string
		wantSentinel error
		wantSummary  string // substring
	}{
		{"validation", "oktaapi/errors/E0000001_validation.json", domain.ErrBadRequest, "Api validation failed"},
		{"auth", "oktaapi/errors/E0000004_auth.json", domain.ErrTokenInvalid, "Authentication failed"},
		{"forbidden", "oktaapi/errors/E0000006_forbidden.json", domain.ErrForbidden, "do not have permission"},
		{"not_found", "oktaapi/errors/E0000007_not_found.json", domain.ErrNotFound, "Not found"},
		{"token_expired", "oktaapi/errors/E0000011_token_expired.json", domain.ErrTokenInvalid, "Invalid token"},
		{"delete_blocked", "oktaapi/errors/E0000022_delete_blocked.json", domain.ErrBadRequest, "does not support"},
		{"feature_disabled", "oktaapi/errors/E0000038_feature_disabled.json", domain.ErrFeatureDisabled, "not enabled"},
		{"rate_limit", "oktaapi/errors/E0000047_rate_limit.json", domain.ErrRateLimited, "exceeded rate limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := testfx.LoadHTTPResponse(t, tc.fixture)
			err := errormap.FromResponse(resp)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.wantSentinel),
				"errorCode 매핑 불일치 (%s): want errors.Is → %v; got %v",
				tc.name, tc.wantSentinel, err)
			assert.Contains(t, err.Error(), tc.wantSummary)
		})
	}
}

// REQ-U04 AC-3 — E0000001의 errorCauses는 BadRequestError에 필드별로 보존되어야 한다.
func Test_ErrorMap_BadRequest_PreservesFieldCauses(t *testing.T) {
	t.Parallel()
	resp := testfx.LoadHTTPResponse(t, "oktaapi/errors/E0000001_validation.json")
	err := errormap.FromResponse(resp)
	var bre *domain.BadRequestError
	require.ErrorAs(t, err, &bre)
	require.NotEmpty(t, bre.Causes, "E0000001은 errorCauses를 전달해야 한다 (REQ-U04 AC-3)")
	// 첫 cause의 Summary에 "login"이 포함되는지 (fixture 내용 참조)
	assert.Contains(t, bre.Causes[0].Summary, "login")
}

// REQ-E01 AC-2 — 429 응답은 RateLimitedError로 매핑되고 Retry-After를 포함해야 한다.
func Test_ErrorMap_RateLimit_ExposesRetryAfter(t *testing.T) {
	t.Parallel()
	resp := testfx.LoadHTTPResponse(t, "oktaapi/errors/E0000047_rate_limit.json")
	err := errormap.FromResponse(resp)
	var rle *domain.RateLimitedError
	require.ErrorAs(t, err, &rle)
	assert.Equal(t, 10*time.Second, rle.RetryAfter,
		"Retry-After: 10 헤더가 10s duration으로 파싱되어야 한다")
}

// PRD §7.7 — 5xx는 ErrOktaServer.
func Test_ErrorMap_ServerError_MapsToErrOktaServer(t *testing.T) {
	t.Parallel()
	// 500 없이 status만 500으로 직접 구성 (fixture는 불필요).
	// 간단히 200→500으로 살짝 비트는 경우는 fixture가 없으므로,
	// 여기서는 fixture 있는 E0000004의 상태코드를 오버라이드하지 않고
	// 5xx 전용 fixture가 없음을 Phase 6 구현자가 명시적으로 처리하게 유도.
	// 현재 Red는 FromResponse 전체가 panic이므로 이 테스트도 실패.
	// (이 테스트는 명시적 5xx 시나리오를 강제하는 설계 피드백 역할.)
	t.Skip("TODO: 5xx fixture 추가 후 복원 (Phase 6)")
}

// REQ-C04 AC-4 — 2xx는 nil 반환.
func Test_ErrorMap_Success_ReturnsNil(t *testing.T) {
	t.Parallel()
	// 200 응답 body는 errormap과 무관하지만, FromResponse는 status<400이면 nil.
	// fixture는 users list를 재사용.
	resp := testfx.LoadHTTPResponse(t, "oktaapi/users/list_page1.json")
	err := errormap.FromResponse(resp)
	assert.NoError(t, err, "2xx 응답은 nil 반환 (REQ-C04 AC-4)")
}

// ============================================================================
// Coverage 보강 엣지 케이스 — test-engineer, 2026-04-24
// 목표: internal/okta/errormap 95% coverage (경계 유닛 TESTING §9.2 기준)
// ============================================================================

// resp == nil → ErrNetwork.
func Test_ErrorMap_NilResponse_ReturnsErrNetwork(t *testing.T) {
	t.Parallel()
	err := errormap.FromResponse(nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNetwork,
		"nil response는 ErrNetwork로 분류되어야 한다")
}

// 5xx → ErrOktaServer (PRD §7.7 fallback).
func Test_ErrorMap_5xx_MapsToErrOktaServer(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"errorCode":"E9999999","errorSummary":"Oh no"}`))),
	}
	err := errormap.FromResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrOktaServer)
	assert.Contains(t, err.Error(), "Oh no")
}

// Unknown errorCode + 4xx → HTTP status 기반 fallback.
func Test_ErrorMap_UnknownErrorCode_FallsBackToHTTPStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		status    int
		wantSenti error
	}{
		{"401_unknown", http.StatusUnauthorized, domain.ErrTokenInvalid},
		{"403_unknown", http.StatusForbidden, domain.ErrForbidden},
		{"404_unknown", http.StatusNotFound, domain.ErrNotFound},
		{"400_unknown", http.StatusBadRequest, domain.ErrBadRequest},
		{"418_unknown_fallthrough", http.StatusTeapot, domain.ErrOktaServer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := []byte(`{"errorCode":"E9999999","errorSummary":"unknown"}`)
			resp := &http.Response{
				StatusCode: tc.status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(body)),
			}
			err := errormap.FromResponse(resp)
			assert.ErrorIs(t, err, tc.wantSenti,
				"status %d unknown errorCode → %v", tc.status, tc.wantSenti)
		})
	}
}

// 429 + Retry-After HTTP-date — 상대 초로 파싱.
func Test_ErrorMap_RateLimit_RetryAfterHTTPDate_Parsed(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(15 * time.Second).UTC().Format(http.TimeFormat)
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Retry-After":  []string{future},
		},
		Body: io.NopCloser(bytes.NewReader([]byte(`{"errorCode":"E0000047","errorSummary":"rate"}`))),
	}
	err := errormap.FromResponse(resp)
	var rle *domain.RateLimitedError
	require.ErrorAs(t, err, &rle)
	assert.InDelta(t, 15.0, rle.RetryAfter.Seconds(), 2.0,
		"HTTP-date Retry-After는 현재-델타 초로 파싱 (±2s 허용)")
}

// 429 + Retry-After 헤더 없음.
func Test_ErrorMap_RateLimit_NoRetryAfterHeader_ZeroDuration(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"errorCode":"E0000047","errorSummary":"rate"}`))),
	}
	err := errormap.FromResponse(resp)
	var rle *domain.RateLimitedError
	require.ErrorAs(t, err, &rle)
	assert.Equal(t, time.Duration(0), rle.RetryAfter)
}

// 429 + 과거 HTTP-date → clamp to 0.
func Test_ErrorMap_RateLimit_RetryAfterPastDate_ClampedToZero(t *testing.T) {
	t.Parallel()
	past := time.Now().Add(-10 * time.Minute).UTC().Format(http.TimeFormat)
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Retry-After":  []string{past},
		},
		Body: io.NopCloser(bytes.NewReader([]byte(`{"errorCode":"E0000047","errorSummary":"rate"}`))),
	}
	err := errormap.FromResponse(resp)
	var rle *domain.RateLimitedError
	require.ErrorAs(t, err, &rle)
	assert.Equal(t, time.Duration(0), rle.RetryAfter,
		"과거 HTTP-date는 0으로 clamp")
}

// 429 + Retry-After unparseable.
func Test_ErrorMap_RateLimit_RetryAfterUnparseable_ZeroDuration(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Retry-After":  []string{"not-a-number-or-date"},
		},
		Body: io.NopCloser(bytes.NewReader([]byte(`{"errorCode":"E0000047","errorSummary":"rate"}`))),
	}
	err := errormap.FromResponse(resp)
	var rle *domain.RateLimitedError
	require.ErrorAs(t, err, &rle)
	assert.Equal(t, time.Duration(0), rle.RetryAfter)
}

// Malformed JSON body → HTTP status로 fallback.
func Test_ErrorMap_MalformedBody_FallsBackToHTTPStatus(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"broken json`))),
	}
	err := errormap.FromResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound,
		"malformed body라도 status 404는 ErrNotFound 분류")
}

// Empty body → http.StatusText를 summary로 사용.
func Test_ErrorMap_EmptyBody_UsesStatusTextAsSummary(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}
	err := errormap.FromResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrOktaServer)
	assert.Contains(t, err.Error(), http.StatusText(http.StatusBadGateway),
		"empty body는 http.StatusText로 summary fallback")
}

// BadRequest cause가 콜론 없는 자유 문자열 (splitCause Field="").
func Test_ErrorMap_BadRequest_CauseWithoutColon_FieldEmpty(t *testing.T) {
	t.Parallel()
	body := []byte(`{"errorCode":"E0000001","errorSummary":"validation",` +
		`"errorCauses":[{"errorSummary":"free-form reason no colon"}]}`)
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	err := errormap.FromResponse(resp)
	var bre *domain.BadRequestError
	require.ErrorAs(t, err, &bre)
	require.Len(t, bre.Causes, 1)
	assert.Equal(t, "", bre.Causes[0].Field,
		"콜론 없는 cause는 Field=\"\"; Summary는 원문 유지")
	assert.Contains(t, bre.Causes[0].Summary, "free-form")
}

// Unknown status code(999) + empty body → wrap이 summary=""일 때 센티넬만 반환.
func Test_ErrorMap_UnknownStatus_EmptyBody_SentinelOnly(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: 999, // StatusText → ""
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}
	err := errormap.FromResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrOktaServer,
		"999는 fall-through로 ErrOktaServer. wrap이 summary=\"\"일 때 원본 센티넬 반환")
}

// errors.Is는 이미 §8.3 테스트가 확보했지만, 테스트 import에 `errors` 사용 명분 유지 차원에서 간단 회귀.
func Test_ErrorMap_ErrorsIsCompatibleAcrossWrappers(t *testing.T) {
	t.Parallel()
	resp := testfx.LoadHTTPResponse(t, "oktaapi/errors/E0000007_not_found.json")
	err := errormap.FromResponse(resp)
	require.True(t, errors.Is(err, domain.ErrNotFound))
}
