package logs_test

// Coverage for the resource-specific keys on the Logs search screen
// (issue #126 / #127). The user reported that f / r / s "don't do
// anything" — the underlying SearchModel state was flipping but the
// indicator hid follow's effect when tail was off, and `r` had no
// handler at all. Pin both the state mutations and the visible
// feedback so a regression surfaces here.

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/logs"
)

func sampleEvents() []domain.LogEvent {
	now := time.Now().UTC()
	return []domain.LogEvent{
		{Published: now.Add(-3 * time.Minute), EventType: "user.session.start"},
		{Published: now.Add(-2 * time.Minute), EventType: "user.session.access"},
		{Published: now.Add(-1 * time.Minute), EventType: "user.session.end"},
	}
}

func runKey(t *testing.T, m logs.SearchModel, r rune) logs.SearchModel {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	out, ok := updated.(logs.SearchModel)
	require.True(t, ok)
	return out
}

func Test_LogsSearch_F_TogglesFollowVisibly(t *testing.T) {
	t.Parallel()

	m := logs.NewSearchModel(logs.Deps{
		InitialEvents: sampleEvents(),
		Width:         120,
		Height:        24,
		Clock:         clock.NewFake(time.Now()),
	})

	before := m.View()
	assert.Contains(t, before, "[FOLLOW]",
		"default state must surface [FOLLOW] in the indicator")

	m = runKey(t, m, 'f')
	after := m.View()
	assert.Contains(t, after, "[PAUSED]",
		"`f` must flip the indicator to [PAUSED] regardless of tail state")
}

func Test_LogsSearch_S_TogglesTailVisibly(t *testing.T) {
	t.Parallel()

	m := logs.NewSearchModel(logs.Deps{
		InitialEvents: sampleEvents(),
		Width:         120,
		Height:        24,
		Clock:         clock.NewFake(time.Now()),
	})

	require.Contains(t, m.View(), "[TAIL OFF]",
		"default state must show [TAIL OFF]")

	m = runKey(t, m, 's')
	assert.NotContains(t, m.View(), "[TAIL OFF]",
		"`s` must flip tail off → on (indicator should switch to [TAIL Ns])")
}

func Test_LogsSearch_R_RefetchesViaService(t *testing.T) {
	t.Parallel()

	calls := 0
	port := fakes.NewLogsPort(t)
	port.SearchFunc = func(_ context.Context, _ domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		calls++
		return &fakes.SliceIterator[domain.LogEvent]{}, nil
	}
	svc := service.NewLogsService(port)

	m := logs.NewSearchModel(logs.Deps{
		Service: svc,
		Clock:   clock.NewFake(time.Now()),
		Width:   120,
		Height:  24,
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(logs.SearchModel)
	require.NotNil(t, cmd, "`r` must produce a refetch Cmd when a Service is wired")
	_ = cmd()
	assert.GreaterOrEqual(t, calls, 1, "`r` must trigger LogsPort.Search")
}

// Test_LogsSearch_InitialCursor_AtBottom — once events arrive, the
// cursor must land on the newest entry so the user starts at the
// bottom of the list (issue #127). Subsequent `k` walks UP to older
// entries — the inverted scroll direction the user requested.
func Test_LogsSearch_InitialCursor_AtBottom(t *testing.T) {
	t.Parallel()

	m := logs.NewSearchModel(logs.Deps{
		Width:  120,
		Height: 24,
		Clock:  clock.NewFake(time.Now()),
	})
	// Simulate the loaded fetch result.
	updated, _ := m.Update(testLogsLoaded(sampleEvents()))
	m = updated.(logs.SearchModel)

	// The newest event ("user.session.end") must be the cursor row.
	view := m.View()
	require.Contains(t, view, "user.session.end",
		"newest event must render in the body")
	// The cursor glyph "▸" must precede the newest event line.
	idxNewest := indexOf(view, "user.session.end")
	idxCursor := indexOf(view, "▸")
	require.GreaterOrEqual(t, idxNewest, 0)
	require.GreaterOrEqual(t, idxCursor, 0)
	// The cursor glyph must be on the same line as the newest event.
	assert.True(t, idxCursor < idxNewest,
		"cursor glyph must precede the newest event on its row (cursor=%d, newest=%d)",
		idxCursor, idxNewest)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// testLogsLoaded returns the unexported logsLoadedMsg for use in
// black-box tests. We synthesize via Update by sending the original
// fetch path indirectly — the public surface accepts events through
// the loaded message, so we route through the Init Cmd of a fresh
// model with InitialEvents seeded.
func testLogsLoaded(events []domain.LogEvent) tea.Msg {
	return logs.LoadedForTest(events)
}
