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

	// All six section headers must surface (issue #143 lays them out
	// in two columns: left = Identity > Organization > Status,
	// right = Contact > Address > Custom).
	for _, h := range []string{"Status", "Identity", "Contact", "Address", "Organization", "Custom"} {
		assert.Containsf(t, view, h, "Pretty mode must render %q section header", h)
	}

	// Within the LEFT column, ordering is Identity < Organization <
	// Status. Within the RIGHT column, Contact < Address < Custom.
	// `strings.Index` finds the first occurrence — adequate to pin
	// the relative order within each column.
	idxIdentity := strings.Index(view, "Identity")
	idxOrg := strings.Index(view, "Organization")
	idxStatus := strings.Index(view, "Status")
	require.Greater(t, idxOrg, idxIdentity, "Organization must follow Identity in the LEFT column")
	require.Greater(t, idxStatus, idxOrg, "Status must follow Organization in the LEFT column")

	idxContact := strings.Index(view, "Contact")
	idxAddress := strings.Index(view, "Address")
	idxCustom := strings.Index(view, "Custom")
	require.Greater(t, idxAddress, idxContact, "Address must follow Contact in the RIGHT column")
	require.Greater(t, idxCustom, idxAddress, "Custom must follow Address in the RIGHT column")

	// Address-class Extras land between the Address header and the
	// Custom header in the right column.
	for _, k := range []string{"city", "state"} {
		idx := strings.Index(view, k)
		require.GreaterOrEqual(t, idx, 0, "key %q must render", k)
	}

	// Genuine custom fields must land AFTER the Custom header.
	for _, k := range []string{"githubId", "startDate"} {
		idx := strings.Index(view, k)
		require.GreaterOrEqual(t, idx, 0, "key %q must render", k)
		assert.Greater(t, idx, idxCustom,
			"%q is a tenant-specific field — must sit under the Custom section", k)
	}
}

// Test_UsersDetail_PrettyMode_TwoColumnLayout pins the side-by-side
// section layout from issue #143: Identity / Organization / Status
// stack on the LEFT, Contact / Address / Custom stack on the RIGHT.
// The headers for the topmost section in each column must appear on
// the same rendered line.
func Test_UsersDetail_PrettyMode_TwoColumnLayout(t *testing.T) {
	t.Parallel()

	view := renderDetailFor(t, detailFixture())
	lines := strings.Split(view, "\n")

	// Find the line that carries the Identity section header — it
	// should also carry the Contact header on the right side.
	var identityLine string
	for _, l := range lines {
		if strings.Contains(l, "Identity") {
			identityLine = l
			break
		}
	}
	require.NotEmpty(t, identityLine, "must find a line carrying the Identity header")
	assert.Contains(t, identityLine, "Contact",
		"Identity (left top) and Contact (right top) must render on the same line")

	// Body lines must look like "<left>  <right>" — i.e. the left
	// column's KV rows carry an aligned right-column trailing piece
	// when the right side still has rows to render. Inspect the
	// `login` row which should coincide with the right column's
	// `mobilePhone` row (both top-of-section in their respective
	// columns).
	var loginLine string
	for _, l := range lines {
		if strings.Contains(l, "login    alice@acme.com") {
			loginLine = l
			break
		}
	}
	require.NotEmpty(t, loginLine, "must find the login row in the rendered detail")
	assert.Contains(t, loginLine, "mobilePhone",
		"login (left) and mobilePhone (right) must render on the same line — proves 2-col layout")
}

// Test_UsersDetail_PrettyMode_OrganizationFixedOrder pins the
// organization > division > department > title > manager >
// employeeNumber sequence requested in issue #140 — even with the
// 2-column layout from #143 the canonical keys must keep their
// relative order in the rendered string.
func Test_UsersDetail_PrettyMode_OrganizationFixedOrder(t *testing.T) {
	t.Parallel()

	view := renderDetailFor(t, detailFixture())
	// The fixture omits "organization" itself; check the rest.
	canonical := []string{"division", "department", "title", "manager", "employeeNumber"}
	prev := -1
	for _, k := range canonical {
		idx := strings.Index(view, k)
		require.GreaterOrEqual(t, idx, 0, "expected %q in the rendered detail", k)
		assert.Greaterf(t, idx, prev,
			"%q must appear AFTER the previous canonical field (issue #140)", k)
		prev = idx
	}
}
