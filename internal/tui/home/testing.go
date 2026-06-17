package home

import (
	tea "github.com/charmbracelet/bubbletea"
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

// ActivityLoadedForTest constructs an internal activity-loaded msg
// so tests can render the Activity card without a real LogsPort.
// `window` should match the currently selected window label (e.g.,
// "1h", "6h", "24h") or the msg is ignored as stale.
func ActivityLoadedForTest(window string, m ActivityMetrics, sampled bool) tea.Msg {
	return activityLoadedMsg{window: window, metrics: m, sampled: sampled}
}
