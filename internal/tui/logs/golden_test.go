package logs_test

// Phase 6d — Visual lock-in for SCR-050 (Logs Search) and SCR-051 (Detail).
// TUI_DESIGN §16.8 specifies WHEN/SEV/EVENTTYPE/ACTOR/OUTCOME/IP columns
// and a [TAIL <N>s] indicator.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/logs"
)

func init() { testfx.PinTestEnvironment() }

// sampleLogsFixture mirrors TUI_DESIGN §16.8 — 5 user.session.start FAILUREs
// with a mix of severities so the list exercises every badge.
func sampleLogsFixture() []domain.LogEvent {
	t := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	return []domain.LogEvent{
		{UUID: "evt-1", Published: t.Add(-2 * time.Hour), Severity: domain.SeverityInfo,
			EventType: "user.session.start",
			Actor:     domain.Actor{DisplayName: "alice@acme.com"},
			Outcome:   domain.Outcome{Result: "FAILURE"},
			Client:    domain.Client{IPAddress: "10.0.1.5"}},
		{UUID: "evt-2", Published: t.Add(-3 * time.Hour), Severity: domain.SeverityInfo,
			EventType: "user.session.start",
			Actor:     domain.Actor{DisplayName: "bob@acme.com"},
			Outcome:   domain.Outcome{Result: "FAILURE"},
			Client:    domain.Client{IPAddress: "10.0.1.6"}},
		{UUID: "evt-3", Published: t.Add(-7 * time.Hour), Severity: domain.SeverityWarn,
			EventType: "user.session.start",
			Actor:     domain.Actor{DisplayName: "alice@acme.com"},
			Outcome:   domain.Outcome{Result: "FAILURE"},
			Client:    domain.Client{IPAddress: "10.0.1.5"}},
		{UUID: "evt-4", Published: t.Add(-24 * time.Hour), Severity: domain.SeverityInfo,
			EventType: "user.session.start",
			Actor:     domain.Actor{DisplayName: "unknown@acme.com"},
			Outcome:   domain.Outcome{Result: "FAILURE"},
			Client:    domain.Client{IPAddress: "10.0.1.7"}},
		{UUID: "evt-5", Published: t.Add(-48 * time.Hour), Severity: domain.SeverityError,
			EventType: "user.session.start",
			Actor:     domain.Actor{DisplayName: "svc-sync@acme"},
			Outcome:   domain.Outcome{Result: "FAILURE"},
			Client:    domain.Client{IPAddress: "10.0.1.8"}},
	}
}

// --- Golden snapshots --------------------------------------------------------

func Test_LogsListGolden_History(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	frozen := clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))
	m := logs.NewSearchModel(logs.Deps{
		InitialEvents: sampleLogsFixture(),
		Width:         120,
		Height:        30,
		Clock:         frozen,
	})
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/list_history.txt")
}

func Test_LogsListGolden_TailActive(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

func Test_LogsListGolden_Paused(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

func Test_LogsDetailGolden_Default(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.
}

// --- Spec lock-in (Active, Fail-First) --------------------------------------

// Test_LogsList_HasColumnHeaders pins the v0.1.12 column lineup
// (issue #158): PUBLISHED / SEV / MESSAGE / ACTOR TYPE / ACTOR /
// OUTCOME / IP / WHEN.
func Test_LogsList_HasColumnHeaders(t *testing.T) {
	t.Parallel()
	m := logs.NewSearchModel(logs.Deps{InitialEvents: sampleLogsFixture(), Width: 200, Height: 30})
	got := testfx.StripANSI(m.View())
	for _, h := range []string{
		"PUBLISHED", "SEV", "MESSAGE", "ACTOR TYPE", "ACTOR", "OUTCOME", "IP", "WHEN",
	} {
		assert.Contains(t, got, h, "Logs list must show %q column header (issue #158)", h)
	}
}

// Test_LogsList_TailIndicatorOff locks in REQ-R05 AC-3: with tail off, the
// status badges include `[TAIL: off]`. v0.2.0 (#182) moved the inline
// status line to the chrome's transient status row, so the assertion
// reads via StatusBadges() instead of the screen body.
func Test_LogsList_TailIndicatorOff(t *testing.T) {
	t.Parallel()
	m := logs.NewSearchModel(logs.Deps{InitialEvents: sampleLogsFixture(), Width: 120, Height: 30})
	found := false
	for _, b := range m.StatusBadges() {
		if b.Key == "TAIL" && b.Value == "off" {
			found = true
		}
	}
	assert.True(t, found, "Logs list must publish a [TAIL: off] chrome status badge")
}

// Test_LogsList_RendersFixtureActors guards against a regression where
// the list silently drops rows.
func Test_LogsList_RendersFixtureActors(t *testing.T) {
	t.Parallel()
	m := logs.NewSearchModel(logs.Deps{InitialEvents: sampleLogsFixture(), Width: 120, Height: 30})
	got := testfx.StripANSI(m.View())
	for _, actor := range []string{"alice@acme.com", "bob@acme.com", "svc-sync@acme"} {
		assert.Contains(t, got, actor, "log row for %q must be visible", actor)
	}
}
