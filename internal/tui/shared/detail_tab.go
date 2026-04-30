package shared

// DetailTab indexes the Pretty / JSON / YAML tab bar that every
// resource detail surface uses (Users, Groups, Group Rules, Apps,
// Logs, Policies). Issue #A4 v0.2.4 — collapses the 5 duplicate
// per-screen enums into one canonical type so the tab order, label
// list, and count are defined exactly once.
type DetailTab int

const (
	DetailTabPretty DetailTab = iota
	DetailTabJSON
	DetailTabYAML
)

// DetailTabLabels lists the label rendered for each tab in the tab
// bar. Index aligns with the DetailTab iota above.
var DetailTabLabels = []string{"Pretty", "JSON", "YAML"}

// DetailTabCount is the number of detail tabs (used by Tab/Shift-Tab
// cycling so screens don't have to compute it locally).
var DetailTabCount = DetailTab(len(DetailTabLabels))

// NextTab cycles forward (Tab key): Pretty → JSON → YAML → Pretty.
func NextTab(t DetailTab) DetailTab {
	return (t + 1) % DetailTabCount
}

// PrevTab cycles backward (Shift-Tab key).
func PrevTab(t DetailTab) DetailTab {
	return (t + DetailTabCount - 1) % DetailTabCount
}

// ToggleRawTab implements the `r` shortcut every detail screen uses:
// from any non-JSON tab, jump to JSON and remember where we came
// from; from JSON, jump back. Returns the new active tab + the new
// rawReturn value so the caller can persist both.
func ToggleRawTab(active, rawReturn DetailTab) (newActive, newRawReturn DetailTab) {
	if active == DetailTabJSON {
		return rawReturn, rawReturn
	}
	return DetailTabJSON, active
}
