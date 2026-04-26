package domain_test

// REQ-R04 — Policy 도메인 타입 단위 검증.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
)

// REQ-R04 AC-1 — MVP는 7개 Policy 타입 전부 지원. 이 중 4개는 rich 렌더, 3개는 raw-only.
func Test_PolicyType_AllSevenTypesAreDefined(t *testing.T) {
	t.Parallel()
	required := map[domain.PolicyType]bool{
		domain.PolicyTypeOktaSignOn:        false,
		domain.PolicyTypeAccessPolicy:      false,
		domain.PolicyTypePassword:          false,
		domain.PolicyTypeMFAEnroll:         false,
		domain.PolicyTypeProfileEnrollment: false,
		domain.PolicyTypePostAuthSession:   false,
		domain.PolicyTypeIDPDiscovery:      false,
	}
	// flip booleans to ensure every type string is non-empty and unique.
	seen := map[string]domain.PolicyType{}
	for pt := range required {
		assert.NotEmpty(t, string(pt), "policy type must have wire value")
		if prior, dup := seen[string(pt)]; dup {
			t.Errorf("duplicate wire value %q: %v vs %v", pt, prior, pt)
		}
		seen[string(pt)] = pt
	}
	assert.Len(t, seen, 7, "MVP는 7개 Policy 타입 (REQ-R04 AC-1)")
}

// REQ-R04 AC-5 — Rich 렌더러는 정확히 4종에 대해서만 존재해야 한다.
func Test_PolicyType_RichRenderedTypesAreExactlyFour(t *testing.T) {
	t.Parallel()
	rich := domain.RichRenderedPolicyTypes()
	assert.Len(t, rich, 4, "rich 렌더러는 4종 (REQ-R04 AC-5)")

	want := map[domain.PolicyType]bool{
		domain.PolicyTypeOktaSignOn:   true,
		domain.PolicyTypeAccessPolicy: true,
		domain.PolicyTypePassword:     true,
		domain.PolicyTypeMFAEnroll:    true,
	}
	for _, pt := range rich {
		assert.True(t, want[pt], "rich set에 허용 외 타입이 포함됨: %s", pt)
	}
}

// REQ-R04 AC-1 — Raw-only 3종은 rich set에 포함되지 않아야 한다.
func Test_PolicyType_RawOnlyTypesExcludedFromRich(t *testing.T) {
	t.Parallel()
	rawOnly := []domain.PolicyType{
		domain.PolicyTypeProfileEnrollment,
		domain.PolicyTypePostAuthSession,
		domain.PolicyTypeIDPDiscovery,
	}
	richSet := map[domain.PolicyType]bool{}
	for _, p := range domain.RichRenderedPolicyTypes() {
		richSet[p] = true
	}
	for _, pt := range rawOnly {
		assert.False(t, richSet[pt],
			"raw-only 타입이 rich set에 포함됨: %s (REQ-R04 AC-1)", pt)
	}
}
