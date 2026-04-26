package domain_test

// REQ-R01 — User 도메인 타입 단위 검증. 상태 집합이 PRD와 일치하는지, 상태 문자열이
// Okta API 응답과 1:1인지.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
)

// REQ-R01 AC-2 — 모든 UserStatus 문자열은 Okta API 응답 값과 일치해야 한다.
//
// PRD §7.7과 도메인 §1.2에서 Okta가 반환하는 status 값은 대문자 언더스코어.
// ota 도메인 타입은 변환 없이 그대로 보존한다.
func Test_UserStatus_StringValuesMatchOktaWire(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   domain.UserStatus
		want string
	}{
		{domain.UserStatusStaged, "STAGED"},
		{domain.UserStatusProvisioned, "PROVISIONED"},
		{domain.UserStatusActive, "ACTIVE"},
		{domain.UserStatusSuspended, "SUSPENDED"},
		{domain.UserStatusLockedOut, "LOCKED_OUT"},
		{domain.UserStatusPasswordExpired, "PASSWORD_EXPIRED"},
		{domain.UserStatusDeprovisioned, "DEPROVISIONED"},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, string(tc.in),
				"UserStatus wire value drifted: %q → %q", tc.in, tc.want)
		})
	}
}

// REQ-R01 AC-2 — SUSPENDED와 DEPROVISIONED는 시각적으로 뚜렷이 구분되어야 하므로
// 서로 다른 값이어야 한다 (TUI가 색상 분기 기준).
func Test_UserStatus_SuspendedAndDeprovisionedAreDistinct(t *testing.T) {
	t.Parallel()
	assert.NotEqual(t, domain.UserStatusSuspended, domain.UserStatusDeprovisioned,
		"SUSPENDED와 DEPROVISIONED가 동일해지면 TUI 색상 분기가 붕괴한다")
}

// REQ-R01 AC-7 — DELETED 상태는 도메인 타입에 노출되지 않는다 (API 기본 제외).
// 현재 UserStatus 상수 집합에 "DELETED" 문자열이 없음을 검증.
func Test_UserStatus_DeletedIsExcludedFromDomain(t *testing.T) {
	t.Parallel()
	knownStatuses := []domain.UserStatus{
		domain.UserStatusStaged,
		domain.UserStatusProvisioned,
		domain.UserStatusActive,
		domain.UserStatusSuspended,
		domain.UserStatusLockedOut,
		domain.UserStatusPasswordExpired,
		domain.UserStatusDeprovisioned,
	}
	for _, s := range knownStatuses {
		assert.NotEqual(t, "DELETED", string(s),
			"DELETED 상태는 도메인 상수 집합에 포함되지 않아야 한다 (REQ-R01 AC-7)")
	}
}
