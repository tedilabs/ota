package domain_test

// REQ-R03 — Group Rule 도메인 타입.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
)

// REQ-R03 AC-2 — 3개 상태가 모두 정의되어 있어야 한다.
func Test_GroupRuleStatus_ThreeStatesDefined(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   domain.GroupRuleStatus
		want string
	}{
		{domain.GroupRuleStatusActive, "ACTIVE"},
		{domain.GroupRuleStatusInactive, "INACTIVE"},
		{domain.GroupRuleStatusInvalid, "INVALID"},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, string(tc.in))
		})
	}
}

// REQ-R03 AC-2 — INVALID는 별도 경고색 렌더링 기준이므로 ACTIVE/INACTIVE와 구분되어야 한다.
func Test_GroupRuleStatus_InvalidDistinct(t *testing.T) {
	t.Parallel()
	assert.NotEqual(t, domain.GroupRuleStatusInvalid, domain.GroupRuleStatusActive)
	assert.NotEqual(t, domain.GroupRuleStatusInvalid, domain.GroupRuleStatusInactive)
}
