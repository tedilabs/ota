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

	// All four section headers must surface.
	for _, h := range []string{"Identity", "Contact", "Organization", "Custom"} {
		assert.Containsf(t, view, h, "Pretty mode must render %q section header", h)
	}

	// Sanity: standard fields land in their named section. We assert
	// ordering by index — Identity first, Custom last.
	idxIdentity := strings.Index(view, "Identity")
	idxContact := strings.Index(view, "Contact")
	idxOrg := strings.Index(view, "Organization")
	idxCustom := strings.Index(view, "Custom")
	require.GreaterOrEqual(t, idxIdentity, 0)
	require.Greater(t, idxContact, idxIdentity)
	require.Greater(t, idxOrg, idxContact)
	require.Greater(t, idxCustom, idxOrg)

	// Okta-standard Extras keys must land BEFORE the Custom header.
	for _, k := range []string{"city", "state", "manager"} {
		idx := strings.Index(view, k)
		require.GreaterOrEqual(t, idx, 0, "key %q must render", k)
		assert.Less(t, idx, idxCustom,
			"%q is an Okta-standard field — must land before the Custom section", k)
	}

	// Genuine custom fields must land AFTER the Custom header.
	for _, k := range []string{"githubId", "startDate"} {
		idx := strings.Index(view, k)
		require.GreaterOrEqual(t, idx, 0, "key %q must render", k)
		assert.Greater(t, idx, idxCustom,
			"%q is a tenant-specific field — must sit under the Custom section", k)
	}
}
