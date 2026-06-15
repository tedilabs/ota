package home

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/dashboard"
)

// Test helpers — exported wrappers over the unexported msg types so
// the test package can synthesize Update inputs without needing to
// run a real LogsPort / UsersPort / etc.

// EventsLoadedForTest constructs an internal events-loaded msg the
// home Model folds into m.cards[CardEvents].
func EventsLoadedForTest(events []CriticalEvent, err error) tea.Msg {
	return eventsLoadedMsg{events: events, err: err}
}

// PostureLoadedForTest synthesises a posture-loaded msg with the
// supplied metrics.
func PostureLoadedForTest(m PostureMetrics) tea.Msg {
	return postureLoadedMsg{metrics: m}
}

// UsersLoadedForTest constructs an internal cardLoadedMsg for the
// Users card so tests can render the count surface without a Users
// port. Phase 2-onward cards (Groups / Apps) get analogous helpers
// when they're needed.
func UsersLoadedForTest(c dashboard.Counts) tea.Msg {
	return cardLoadedMsg{card: CardUsers, counts: c}
}
