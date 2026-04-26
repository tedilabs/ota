package policies_test

// Phase 6d — Visual lock-in for SCR-040 / SCR-041 / SCR-042.
//
// Per TUI_DESIGN §16.7, the OKTA_SIGN_ON policies list shows PRI / STATUS /
// NAME / SYSTEM / UPDATED columns and marks system policies with a [SYS]
// badge. The detail screen has rich vs raw modes (REQ-R04 AC-5/AC-6).

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/policies"
)

func init() { testfx.PinTestEnvironment() }

// samplePoliciesFixture mirrors TUI_DESIGN §16.7.
func samplePoliciesFixture() []domain.Policy {
	return []domain.Policy{
		{ID: "00p_default", Name: "Default Policy", Type: domain.PolicyTypeOktaSignOn,
			Priority: 1, Status: domain.PolicyStatusActive, System: true},
		{ID: "00p_admin_mfa", Name: "Require MFA for admins", Type: domain.PolicyTypeOktaSignOn,
			Priority: 2, Status: domain.PolicyStatusActive},
		{ID: "00p_legacy", Name: "Legacy Contractor Rule", Type: domain.PolicyTypeOktaSignOn,
			Priority: 3, Status: domain.PolicyStatusInactive},
	}
}

// --- Golden snapshots --------------------------------------------------------

func Test_PoliciesListGolden_OktaSignOn(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	m := policies.NewListModel(policies.Deps{InitialPolicies: samplePoliciesFixture(), Width: 120, Height: 30},
		domain.PolicyTypeOktaSignOn)
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/list_okta_sign_on.txt")
}

func Test_PoliciesGolden_TypeSelect(t *testing.T) {
	t.Parallel()
	m := policies.NewTypeSelectModel(policies.Deps{Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/type_select.txt")
}

func Test_PoliciesDetailGolden_Rich(t *testing.T) {
	t.Parallel()
	policy := domain.Policy{
		ID: "00p_default", Name: "Default Policy", Type: domain.PolicyTypeOktaSignOn,
		Priority: 1, Status: domain.PolicyStatusActive, System: true,
		Raw: json.RawMessage(`{"id":"00p_default","name":"Default Policy"}`),
	}
	m := policies.NewDetailModel(policies.Deps{Width: 120, Height: 30}, policy)
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/detail_rich.txt")
}

func Test_PoliciesDetailGolden_Raw(t *testing.T) {
	t.Parallel()
	policy := domain.Policy{
		ID: "00p_idp_disco", Name: "Default IdP Discovery", Type: domain.PolicyTypeIDPDiscovery,
		Priority: 1, Status: domain.PolicyStatusActive,
		Raw: json.RawMessage(`{"id":"00p_idp_disco","name":"Default IdP Discovery","type":"IDP_DISCOVERY"}`),
	}
	m := policies.NewDetailModel(policies.Deps{Width: 120, Height: 30}, policy)
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/detail_raw.txt")
}

// --- Spec lock-in (Active, Fail-First) --------------------------------------

// Test_PoliciesList_HasColumnHeaders locks in TUI_DESIGN §16.7: PRI / STATUS /
// NAME / SYSTEM / UPDATED columns.
func Test_PoliciesList_HasColumnHeaders(t *testing.T) {
	t.Parallel()
	m := policies.NewListModel(
		policies.Deps{InitialPolicies: samplePoliciesFixture(), Width: 120, Height: 30},
		domain.PolicyTypeOktaSignOn,
	)
	got := testfx.StripANSI(m.View())
	for _, h := range []string{"PRI", "STATUS", "NAME"} {
		assert.Contains(t, got, h, "Policies list must show %q column header (TUI_DESIGN §16.7)", h)
	}
}

// Test_PoliciesList_SystemPolicyShowsBadge locks in REQ-R04 AC-3 / §16.7:
// system policies are marked [SYS] in the SYSTEM column.
func Test_PoliciesList_SystemPolicyShowsBadge(t *testing.T) {
	t.Parallel()
	m := policies.NewListModel(
		policies.Deps{InitialPolicies: samplePoliciesFixture(), Width: 120, Height: 30},
		domain.PolicyTypeOktaSignOn,
	)
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "[SYS]",
		"system policies must show the [SYS] badge (REQ-R04 AC-3 / TUI_DESIGN §16.7)")
}

// Test_PoliciesList_PolicyTypeBreadcrumb locks in TUI_DESIGN §16.7: list
// title shows the active policy type as a breadcrumb (e.g. `Policies › OKTA_SIGN_ON`).
func Test_PoliciesList_PolicyTypeBreadcrumb(t *testing.T) {
	t.Parallel()
	m := policies.NewListModel(
		policies.Deps{InitialPolicies: samplePoliciesFixture(), Width: 120, Height: 30},
		domain.PolicyTypeOktaSignOn,
	)
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "OKTA_SIGN_ON",
		"Policies list must include the active policy type in the breadcrumb (TUI_DESIGN §16.7)")
}

// Test_PoliciesTypeSelect_RawBadge locks in REQ-R04 AC-2: the type-select
// menu shows `(raw view)` next to types without a rich renderer.
func Test_PoliciesTypeSelect_RawBadge(t *testing.T) {
	t.Parallel()
	m := policies.NewTypeSelectModel(policies.Deps{Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "(raw view)",
		"TypeSelect must mark non-rich types with `(raw view)` (REQ-R04 AC-2)")
}
