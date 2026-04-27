package logs_test

// Pins the Logs Detail Pretty / JSON / YAML tabs (issue #135). The
// user reported "Log 디테일 뷰에서도 JSON / YAML 탭 만들어줘" — the
// previous detail view only had the curated Pretty body; this test
// drives the tab cycle and asserts each tab surfaces its expected
// content.

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/logs"
)

func detailFixture() []domain.LogEvent {
	now := time.Now().UTC()
	return []domain.LogEvent{
		{
			UUID:       "evt-001",
			Published:  now.Add(-1 * time.Minute),
			Severity:   domain.SeverityInfo,
			EventType:  "user.session.start",
			DisplayMsg: "User session started",
			Actor: domain.Actor{
				ID:          "00u_alice",
				Type:        domain.ActorTypeUser,
				DisplayName: "Alice",
				AlternateID: "alice@acme.com",
			},
			Outcome: domain.Outcome{Result: domain.OutcomeSuccess},
			Client:  domain.Client{IPAddress: "10.0.0.1"},
		},
	}
}

func feedDetailKey(t *testing.T, m logs.SearchModel, key tea.KeyMsg) logs.SearchModel {
	t.Helper()
	updated, _ := m.Update(key)
	out, ok := updated.(logs.SearchModel)
	require.True(t, ok)
	return out
}

func openLogDetail(t *testing.T) logs.SearchModel {
	t.Helper()
	m := logs.NewSearchModel(logs.Deps{
		InitialEvents: detailFixture(),
		Clock:         clock.NewFake(time.Now()),
		Width:         120, Height: 30,
	})
	// Press Enter on the seeded event to open detail.
	return feedDetailKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
}

func Test_LogsDetail_PrettyTab_ShowsCuratedBody(t *testing.T) {
	t.Parallel()
	m := openLogDetail(t)

	view := testfx.StripANSI(m.View())
	// Pretty body still surfaces the curated section labels.
	assert.Contains(t, view, "Actor")
	assert.Contains(t, view, "alice@acme.com")
	// Tab bar shows three labels.
	for _, label := range []string{"Pretty", "JSON", "YAML"} {
		assert.Containsf(t, view, label, "tab bar must include %q", label)
	}
}

func Test_LogsDetail_TabCyclesToJSONAndYAML(t *testing.T) {
	t.Parallel()
	m := openLogDetail(t)

	// Tab → JSON.
	m = feedDetailKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, `"eventType"`,
		"JSON tab must surface the event JSON shape")
	assert.Contains(t, view, "user.session.start")

	// Tab → YAML.
	m = feedDetailKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	view = testfx.StripANSI(m.View())
	assert.Contains(t, view, "eventType: user.session.start",
		"YAML tab must surface key: value form")
}

func Test_LogsDetail_R_TogglesJSONFromAnyTab(t *testing.T) {
	t.Parallel()
	m := openLogDetail(t)

	// First press of `r` jumps from Pretty to JSON.
	m = feedDetailKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, `"eventType"`,
		"`r` from Pretty must land on JSON")

	// Second press of `r` returns to Pretty.
	m = feedDetailKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	view = testfx.StripANSI(m.View())
	assert.Contains(t, view, "Actor",
		"`r` from JSON must return to Pretty body")
}
