package domain_test

// REQ-W01 — Domain unit tests for UserProfilePatch (the partial-merge
// shape for POST /api/v1/users/{id}). These are Step 1 of the Phase 5
// RED order — smallest unit, fewest dependencies. The 11-field matrix
// here pins the AC-2 catalog (firstName / lastName / displayName /
// nickName / email / title / division / department / employeeNumber /
// mobilePhone / secondEmail; login intentionally absent — D-W2).
//
// Phase 5 RED expectation: UserProfilePatch.IsEmpty() is a stub
// returning false for everything, so Test_UserProfilePatch_IsEmpty_AllNil
// fails and the 11 set-field cases pass trivially. The set-field
// matrix is intentionally explicit so Phase 6 can't claim a one-line
// `return false` and skip the truth check.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
)

// AC-2 / D-T4 — when every field pointer is nil, IsEmpty returns true.
// Phase 6 must implement the nil-check on all 11 fields.
func Test_UserProfilePatch_IsEmpty_AllNil_IsTrue(t *testing.T) {
	t.Parallel()
	var p domain.UserProfilePatch
	assert.True(t, p.IsEmpty(),
		"REQ-W01 D-T4 / D-T5: a zero-value patch must report IsEmpty()=true so the adapter short-circuits with ErrEmptyPatch (no HTTP call)")
}

// AC-2 / D-T4 — single-field-set matrix: every one of the 11 fields,
// when populated, must flip IsEmpty to false. This is a defensive
// check against a "return false" / "return true" stub.
func Test_UserProfilePatch_IsEmpty_SingleFieldSet_IsFalse(t *testing.T) {
	t.Parallel()
	v := "x" // sentinel value — any non-empty string works
	cases := []struct {
		name string
		mut  func(*domain.UserProfilePatch)
	}{
		{"firstName", func(p *domain.UserProfilePatch) { p.FirstName = &v }},
		{"lastName", func(p *domain.UserProfilePatch) { p.LastName = &v }},
		{"displayName", func(p *domain.UserProfilePatch) { p.DisplayName = &v }},
		{"nickName", func(p *domain.UserProfilePatch) { p.NickName = &v }},
		{"email", func(p *domain.UserProfilePatch) { p.Email = &v }},
		{"title", func(p *domain.UserProfilePatch) { p.Title = &v }},
		{"division", func(p *domain.UserProfilePatch) { p.Division = &v }},
		{"department", func(p *domain.UserProfilePatch) { p.Department = &v }},
		{"employeeNumber", func(p *domain.UserProfilePatch) { p.EmployeeNumber = &v }},
		{"mobilePhone", func(p *domain.UserProfilePatch) { p.MobilePhone = &v }},
		{"secondEmail", func(p *domain.UserProfilePatch) { p.SecondEmail = &v }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var p domain.UserProfilePatch
			tc.mut(&p)
			assert.False(t, p.IsEmpty(),
				"setting %s alone must make IsEmpty()=false (REQ-W01 AC-2 11-field catalog)", tc.name)
		})
	}
}

// AC-2 / D-T4 — empty-string value (not nil pointer) still counts as
// "set". Pinning this prevents Phase 6 from treating *string("") as
// "unchanged" — that would conflict with explicit-clear semantics
// which we deferred per D-W13 but the pointer convention is still
// "presence of pointer = field included in patch".
func Test_UserProfilePatch_IsEmpty_EmptyStringValue_IsFalse(t *testing.T) {
	t.Parallel()
	empty := ""
	p := domain.UserProfilePatch{Email: &empty}
	assert.False(t, p.IsEmpty(),
		"a non-nil pointer to empty string still counts as a present field — pointer presence is the patch key, value is the payload")
}

// AC-2 / D-T4 — every field set together. This is the upper-bound
// case and is a sanity check.
func Test_UserProfilePatch_IsEmpty_AllSet_IsFalse(t *testing.T) {
	t.Parallel()
	v := "v"
	p := domain.UserProfilePatch{
		FirstName: &v, LastName: &v, DisplayName: &v, NickName: &v,
		Email: &v, Title: &v, Division: &v, Department: &v,
		EmployeeNumber: &v, MobilePhone: &v, SecondEmail: &v,
	}
	assert.False(t, p.IsEmpty(),
		"a fully populated patch must report IsEmpty()=false")
}

// REQ-W01 / D-T5 — ErrEmptyPatch is a stable sentinel comparable with
// errors.Is. Adapter and service tests downstream rely on this.
func Test_ErrEmptyPatch_IsSentinel(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, domain.ErrEmptyPatch, "ErrEmptyPatch must be exported and non-nil")
	assert.Contains(t, domain.ErrEmptyPatch.Error(), "empty patch",
		"ErrEmptyPatch.Error() must carry the 'empty patch' phrase so logs are searchable")
}
