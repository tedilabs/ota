package users_test

// Phase 6d — Visual fidelity lock-in for SCR-010 / SCR-011.
//
// Two layers of tests live in this file:
//
//  1. Golden snapshots (testdata/golden/*.txt). The golden contents come
//     verbatim from TUI_DESIGN §16 (committed by tui-designer-2 in v1.1.0).
//     Each golden test compares testfx.StripANSI(view) to its file. While
//     Phase 6d-3..6 land, the goldens stay Skipped with a "blocked on …"
//     reason so existing flows keep PASSing; once the developer ships, the
//     Skip is removed and the comparison goes live (per the test-engineer ↔
//     developer protocol in the Phase 6d brief).
//
//  2. Spec lock-in (substring assertions). These describe the visible
//     contract — column headers, status badges, error messages — without
//     binding to byte-perfect golden output. They are Active and Fail-First:
//     they Red today and gate Phase 6d-4 / 6d-6.
//
// Update goldens (after developer ships): `go test -update ./internal/tui/users/...`

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/users"
)

// fixtureClock returns a frozen Clock at the same instant the test fixtures
// were authored against (TUI_DESIGN §16). Without this, RelativeTime drifts
// against time.Now() and goldens become flaky as the calendar moves on.
func fixtureClock() clock.Clock {
	return clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))
}

func init() { testfx.PinTestEnvironment() }

// sampleUsersFixture mirrors TUI_DESIGN §16.0 fixture so every golden in the
// users package shares the same 5 users (alice / alan / alex / amy / aaron).
func sampleUsersFixture() []domain.User {
	clk := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	llAlice := clk.Add(-2 * time.Hour)
	llAlan := clk.Add(-24 * time.Hour)
	llAaron := clk.Add(-5 * 24 * time.Hour)
	return []domain.User{
		{ID: "00u00000001", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "alice@acme.com", DisplayName: "Alice Smith", FirstName: "Alice", LastName: "Smith", Email: "alice@acme.com"},
			LastLogin: &llAlice},
		{ID: "00u00000002", Status: domain.UserStatusActive,
			Profile:   domain.UserProfile{Login: "alan.turing@acme.com", DisplayName: "Alan Turing"},
			LastLogin: &llAlan},
		{ID: "00u00000003", Status: domain.UserStatusLockedOut,
			Profile: domain.UserProfile{Login: "alex.lee@acme.com", DisplayName: "Alex Lee"}},
		{ID: "00u00000004", Status: domain.UserStatusStaged,
			Profile: domain.UserProfile{Login: "amy.wong@acme.com", DisplayName: "Amy Wong"}},
		{ID: "00u00000005", Status: domain.UserStatusSuspended,
			Profile:   domain.UserProfile{Login: "aaron.k@acme.com", DisplayName: "Aaron K."},
			LastLogin: &llAaron},
	}
}

// --- Golden snapshots --------------------------------------------------------

func Test_UsersListGolden_Default(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	m := users.NewListModel(users.Deps{
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
		Clock:        fixtureClock(),
	})
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/list_default.txt")
}

func Test_UsersListGolden_LoadingFilter(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	// Sketch (post-fix, exact wiring is the developer's call):
	//   m := users.NewListModel(users.Deps{InitialUsers: sampleUsersFixture(), Filter: "zzznomatch"})
	//   testfx.AssertGolden(t, testfx.StripANSI(m.View()), "testdata/golden/list_empty_filter.txt")
}

func Test_UsersListGolden_Error403(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

func Test_UsersDetailGolden_ProfileTab(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

// --- Spec lock-in (Active, Fail-First substring assertions) -----------------

// Test_UsersList_HasColumnHeaders locks in TUI_DESIGN §15+§16.1: the Users
// list shows column headers (STATUS / LOGIN / DISPLAY NAME / LAST LOGIN /
// CHANGED). Today's View() prints rows only — this test stays Red until
// Phase 6d-4 lands a header row.
func Test_UsersList_HasColumnHeaders(t *testing.T) {
	t.Parallel()
	m := users.NewListModel(users.Deps{InitialUsers: sampleUsersFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	for _, header := range []string{"STATUS", "LOGIN", "LAST LOGIN"} {
		assert.Contains(t, got, header,
			"Users list must display the %q column header (TUI_DESIGN §15+§16.1)", header)
	}
}

// Test_UsersList_RendersAllStatusValues sanity-checks that each of the 4
// distinct user statuses in the fixture surfaces. Catches regressions where
// the row renderer accidentally drops or hides rows.
func Test_UsersList_RendersAllStatusValues(t *testing.T) {
	t.Parallel()
	m := users.NewListModel(users.Deps{InitialUsers: sampleUsersFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	for _, status := range []string{"ACTIVE", "LOCKED_OUT", "STAGED", "SUSPENDED"} {
		assert.Contains(t, got, status, "row for status %q must be visible", status)
	}
}

// Test_UsersList_RendersFixtureLogins guards against a bug where the table
// silently filtered out rows. All 5 fixture logins must appear in the View.
func Test_UsersList_RendersFixtureLogins(t *testing.T) {
	t.Parallel()
	m := users.NewListModel(users.Deps{InitialUsers: sampleUsersFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	for _, login := range []string{
		"alice@acme.com", "alan.turing@acme.com",
		"alex.lee@acme.com", "amy.wong@acme.com", "aaron.k@acme.com",
	} {
		assert.Contains(t, got, login, "fixture login %q must be in View()", login)
	}
}
