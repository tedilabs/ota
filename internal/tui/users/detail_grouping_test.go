package users_test

// Pins the Pretty-mode field grouping introduced for issue #130. The
// user reported that the previous renderer dumped every Extras key
// under a single "Custom fields" header, even when the keys were
// well-known Okta-standard fields like address blocks or manager
// metadata. This test exercises a fixture carrying both genuine
// custom fields (githubId, startDate) and Okta-standard Extras
// (city, manager) and asserts they land in the right sections.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

func detailFixture() domain.User {
	return domain.User{
		ID:     "00u_alice",
		Status: domain.UserStatusActive,
		Profile: domain.UserProfile{
			Login:          "alice@acme.com",
			Email:          "alice@acme.com",
			FirstName:      "Alice",
			LastName:       "Anderson",
			DisplayName:    "Alice A.",
			Title:          "Staff Engineer",
			Division:       "Platform",
			Department:     "Identity",
			EmployeeNumber: "EMP-12345",
			MobilePhone:    "+1-555-0101",
			Extras: map[string]any{
				// Okta-standard keys that should auto-route into
				// Contact / Organization sections.
				"city":      "Seattle",
				"state":     "WA",
				"manager":   "Bob B.",
				"managerId": "00u_bob",
				// Genuine custom fields — must land in Custom.
				"githubId":  "alice-gh",
				"startDate": "2024-01-15",
			},
		},
	}
}

// renderDetailFor builds a DetailModel for the given user and returns
// the Pretty-tab body (after stripping ANSI styling).
func renderDetailFor(t *testing.T, u domain.User) string {
	t.Helper()
	m := users.NewDetailModel(users.Deps{}, u)
	return testfx.StripANSI(m.View())
}

func Test_UsersDetail_PrettyMode_GroupsByOktaSection(t *testing.T) {
	t.Parallel()

	view := renderDetailFor(t, detailFixture())

	// All six section headers must surface (issue #140 added Status
	// at the top and split Address from Contact).
	for _, h := range []string{"Status", "Identity", "Contact", "Address", "Organization", "Custom"} {
		assert.Containsf(t, view, h, "Pretty mode must render %q section header", h)
	}

	// Section order: Status first, Custom last; Address sits between
	// Contact and Organization.
	idxStatus := strings.Index(view, "Status")
	idxIdentity := strings.Index(view, "Identity")
	idxContact := strings.Index(view, "Contact")
	idxAddress := strings.Index(view, "Address")
	idxOrg := strings.Index(view, "Organization")
	idxCustom := strings.Index(view, "Custom")
	require.GreaterOrEqual(t, idxStatus, 0)
	require.Greater(t, idxIdentity, idxStatus, "Status must come BEFORE Identity (issue #140)")
	require.Greater(t, idxContact, idxIdentity)
	require.Greater(t, idxAddress, idxContact, "Address must come AFTER Contact")
	require.Greater(t, idxOrg, idxAddress, "Organization must come AFTER Address")
	require.Greater(t, idxCustom, idxOrg)

	// Address-class Extras land in the Address section, between
	// Contact and Organization.
	for _, k := range []string{"city", "state"} {
		idx := strings.Index(view, k)
		require.GreaterOrEqual(t, idx, 0, "key %q must render", k)
		assert.Greater(t, idx, idxAddress, "%q must land under Address", k)
		assert.Less(t, idx, idxOrg, "%q must land BEFORE Organization", k)
	}

	// Organization-class Extras (manager) land in Organization.
	idxManager := strings.Index(view, "manager")
	require.GreaterOrEqual(t, idxManager, 0)
	assert.Greater(t, idxManager, idxOrg, "manager must land under Organization")
	assert.Less(t, idxManager, idxCustom)

	// Genuine custom fields must land AFTER the Custom header.
	for _, k := range []string{"githubId", "startDate"} {
		idx := strings.Index(view, k)
		require.GreaterOrEqual(t, idx, 0, "key %q must render", k)
		assert.Greater(t, idx, idxCustom,
			"%q is a tenant-specific field — must sit under the Custom section", k)
	}
}

// Test_UsersDetail_PrettyMode_OrganizationFixedOrder pins the
// organization > division > department > title > manager >
// employeeNumber sequence requested in issue #140 — even when the
// underlying domain populates only a subset, the order of present
// fields must reflect the spec.
func Test_UsersDetail_PrettyMode_OrganizationFixedOrder(t *testing.T) {
	t.Parallel()

	view := renderDetailFor(t, detailFixture())
	// Find positions inside the Organization slice.
	orgStart := strings.Index(view, "Organization")
	require.GreaterOrEqual(t, orgStart, 0)
	custStart := strings.Index(view, "Custom")
	if custStart < 0 {
		custStart = len(view)
	}
	orgSlice := view[orgStart:custStart]

	// Every field present in the fixture must appear in the canonical
	// order: division, department, title, manager, employeeNumber.
	// (The fixture omits "organization" so we skip it here.)
	canonical := []string{"division", "department", "title", "manager", "employeeNumber"}
	prev := -1
	for _, k := range canonical {
		idx := strings.Index(orgSlice, k)
		require.GreaterOrEqual(t, idx, 0, "expected %q inside Organization slice", k)
		assert.Greaterf(t, idx, prev,
			"%q must appear AFTER the previous canonical field (issue #140)", k)
		prev = idx
	}
}
