package logs_test

// v0.2.1 #185 — Logs server-side query mode (`Q`).
//
// Mirrors the Okta web dashboard's Search box: `Q` opens a prompt,
// Enter commits + re-fetches the history window with q=<text>
// against /api/v1/logs. Distinct from the local `/` filter which
// narrows already-loaded events client-side.

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

// Test_LogsQ_OpensPromptAndCommitsServerQuery walks the canonical
// path: `Q` → type "user.session" → Enter. The fake LogsPort
// captures the query passed to Search; assertion verifies the
// `Q` parameter reached it.
func Test_LogsQ_OpensPromptAndCommitsServerQuery(t *testing.T) {
	t.Parallel()

	port := fakes.NewLogsPort(t)
	var capturedQ string
	port.SearchFunc = func(_ context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		capturedQ = q.Q
		return &fakes.SliceIterator[domain.LogEvent]{}, nil
	}
	svc := service.NewLogsService(port)
	m := logs.NewSearchModel(logs.Deps{
		Service: svc,
		Clock:   clock.NewFake(time.Now()),
	})

	// Open the Q prompt, type the query, commit.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	m = upd.(logs.SearchModel)
	require.True(t, m.QueryEditing(),
		"`Q` must enter query-edit mode")

	for _, r := range "user.session" {
		upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = upd.(logs.SearchModel)
	}
	assert.Equal(t, "user.session", m.QueryInput(),
		"typed runes must accumulate in QueryInput")

	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(logs.SearchModel)
	require.NotNil(t, cmd, "Enter must emit a fetch Cmd")
	require.False(t, m.QueryEditing(),
		"Enter must close the query prompt")

	// Drain the fetch Cmd so SearchFunc captures the query.
	if msg := cmd(); msg != nil {
		upd, _ = m.Update(msg)
		m = upd.(logs.SearchModel)
	}
	assert.Equal(t, "user.session", capturedQ,
		"server fetch must include the operator's query in LogsQuery.Q")

	// The badge slot must surface the active query for the chrome.
	found := false
	for _, b := range m.StatusBadges() {
		if b.Key == "Q" && b.Value == "user.session" {
			found = true
		}
	}
	assert.True(t, found,
		"committed query must publish a [Q: <text>] chrome status badge")
}

// Test_LogsQ_EscClearsActiveQuery: with a query already committed,
// pressing Esc on the list (no other state) must clear it and
// trigger a fresh unfiltered fetch.
func Test_LogsQ_EscClearsActiveQuery(t *testing.T) {
	t.Parallel()

	port := fakes.NewLogsPort(t)
	port.SearchFunc = func(_ context.Context, _ domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		return &fakes.SliceIterator[domain.LogEvent]{}, nil
	}
	svc := service.NewLogsService(port)
	m := logs.NewSearchModel(logs.Deps{
		Service: svc,
		Clock:   clock.NewFake(time.Now()),
	})

	// Prime an active query via the Q→type→Enter path.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	m = upd.(logs.SearchModel)
	for _, r := range "test" {
		upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = upd.(logs.SearchModel)
	}
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(logs.SearchModel)
	require.True(t, hasQueryBadge(m, "test"),
		"precondition: query badge active before Esc")

	// Esc on the list with the query set must clear it.
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = upd.(logs.SearchModel)
	require.NotNil(t, cmd, "Esc with active query must emit a fresh fetch Cmd")
	assert.False(t, hasQueryBadge(m, "test"),
		"Esc must clear the query badge")
}

func hasQueryBadge(m logs.SearchModel, value string) bool {
	for _, b := range m.StatusBadges() {
		if b.Key == "Q" && b.Value == value {
			return true
		}
	}
	return false
}
