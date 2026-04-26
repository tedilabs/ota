package domain_test

// REQ-U04 AC-3, REQ-C04 AC-4, PRD §7.7 — 에러 모델 단위 검증.
//
// 이 테스트는 도메인 에러 타입이 다음 규약을 지키는지 검증한다:
//   1) 모든 센티넬은 errors.Is로 식별 가능해야 한다.
//   2) RateLimitedError는 errors.As로 Retry-After를 노출해야 한다.
//   3) BadRequestError는 errors.As로 FieldError 목록을 노출해야 한다.
//   4) 각 wrapper는 Unwrap으로 센티넬을 돌려줘야 한다.

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
)

// REQ-C04 AC-4 — 센티넬 기본 식별.
func Test_DomainErrors_SentinelsIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
	}{
		{"not_found", domain.ErrNotFound},
		{"forbidden", domain.ErrForbidden},
		{"rate_limited", domain.ErrRateLimited},
		{"token_invalid", domain.ErrTokenInvalid},
		{"bad_request", domain.ErrBadRequest},
		{"okta_server", domain.ErrOktaServer},
		{"feature_disabled", domain.ErrFeatureDisabled},
		{"network", domain.ErrNetwork},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.NotNil(t, tc.err, "sentinel must be non-nil")
			// wrapped with %w and still identified
			wrapped := errors.Join(errors.New("outer layer"), tc.err)
			assert.ErrorIs(t, wrapped, tc.err, "wrapped sentinel must still satisfy errors.Is")
		})
	}
}

// REQ-E01 AC-2 — RateLimitedError가 Retry-After 동반.
func Test_DomainErrors_RateLimitedError_ExposesRetryAfter(t *testing.T) {
	t.Parallel()
	underlying := &domain.RateLimitedError{
		RetryAfter: 12 * time.Second,
		Category:   "logs",
	}
	// wrap one level deeper to prove errors.As unwraps properly.
	wrapped := errors.Join(errors.New("okta adapter: list"), underlying)

	var got *domain.RateLimitedError
	require.ErrorAs(t, wrapped, &got, "must unwrap to *RateLimitedError")
	assert.Equal(t, 12*time.Second, got.RetryAfter)
	assert.Equal(t, "logs", got.Category)
	// Sentinel preserved via Unwrap.
	assert.ErrorIs(t, wrapped, domain.ErrRateLimited)
}

// REQ-U04 AC-3 — BadRequestError가 필드별 원인(errorCauses) 보존.
func Test_DomainErrors_BadRequestError_PreservesCauses(t *testing.T) {
	t.Parallel()
	src := &domain.BadRequestError{
		Causes: []domain.FieldError{
			{Field: "login", Summary: "already exists"},
			{Field: "email", Summary: "invalid format"},
		},
		Raw: "E0000001",
	}

	var got *domain.BadRequestError
	require.ErrorAs(t, error(src), &got)
	require.Len(t, got.Causes, 2, "all errorCauses must be preserved")
	assert.Equal(t, "login", got.Causes[0].Field)
	assert.Equal(t, "already exists", got.Causes[0].Summary)
	assert.Equal(t, "email", got.Causes[1].Field)

	// Sentinel preserved via Unwrap.
	assert.ErrorIs(t, error(src), domain.ErrBadRequest)
}

// REQ-U04 AC-3 — 원인이 0개인 경우에도 타입은 유지.
func Test_DomainErrors_BadRequestError_ZeroCausesStillTyped(t *testing.T) {
	t.Parallel()
	e := &domain.BadRequestError{Raw: "E0000022"}

	var got *domain.BadRequestError
	require.ErrorAs(t, error(e), &got)
	assert.Empty(t, got.Causes)
	assert.ErrorIs(t, error(e), domain.ErrBadRequest)
}

// Coverage 보강: RateLimitedError.Error() 메시지 포맷 (test-engineer 2026-04-24).
func Test_DomainErrors_RateLimitedError_ErrorMessageFormat(t *testing.T) {
	t.Parallel()
	e := &domain.RateLimitedError{
		RetryAfter: 12 * time.Second,
		Category:   "logs",
	}
	msg := e.Error()
	assert.Contains(t, msg, "rate limited")
	assert.Contains(t, msg, "12s")
	assert.Contains(t, msg, "logs")
}

// Coverage 보강: BadRequestError.Error() 메시지 포맷 — 0/1/N cause 분기.
func Test_DomainErrors_BadRequestError_ErrorMessageFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   *domain.BadRequestError
		want string // substring
	}{
		{
			"no_causes",
			&domain.BadRequestError{},
			"bad request",
		},
		{
			"one_cause",
			&domain.BadRequestError{Causes: []domain.FieldError{{Field: "login", Summary: "invalid"}}},
			"1 cause",
		},
		{
			"multiple_causes",
			&domain.BadRequestError{
				Causes: []domain.FieldError{
					{Field: "login", Summary: "x"},
					{Field: "email", Summary: "y"},
				},
			},
			"2 cause",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := tc.in.Error()
			assert.Contains(t, msg, tc.want)
		})
	}
}
