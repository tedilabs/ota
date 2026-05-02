package shared

import "strings"

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

// detailTabActiveMarker is the single-glyph indicator drawn before
// the active tab label. Survives ANSI strip so test harnesses can
// detect which tab is active without parsing escape sequences.
const detailTabActiveMarker = "▎"

// RenderDetailTabBar returns the canonical 2-line tab bar every
// resource detail surface uses: a labels row + thin divider beneath.
//
// The active tab gets a leading `▎` accent marker, bold + accent
// foreground, and a heavy `━` underline segment on the divider; all
// other tabs render in muted text with thin `─` underline. width
// caps the divider length (use the chrome's content width); 0 falls
// back to 78 for legacy callers.
//
// Example (active = JSON, width = 30):
//
//	   Pretty   ▎JSON     YAML
//	──────────━━━━━━━━────────────
func RenderDetailTabBar(active DetailTab, width int, tk Tokens) string {
	if width <= 0 {
		width = 78
	}

	hasActive := int(active) >= 0 && int(active) < len(DetailTabLabels)
	cellStart := make([]int, len(DetailTabLabels))
	cellEnd := make([]int, len(DetailTabLabels))

	var labels strings.Builder
	col := 0
	labels.WriteByte(' ')
	col++

	for i, label := range DetailTabLabels {
		if i > 0 {
			gap := "   "
			labels.WriteString(gap)
			col += len(gap)
		}
		var cell string
		if hasActive && DetailTab(i) == active {
			cell = detailTabActiveMarker + label
		} else {
			cell = " " + label
		}
		cellStart[i] = col
		cellEnd[i] = col + VisibleWidth(cell)
		col += VisibleWidth(cell)

		if hasActive && DetailTab(i) == active {
			labels.WriteString(tk.Accent.Bold(true).Render(cell))
		} else {
			labels.WriteString(tk.Muted.Render(cell))
		}
	}

	// Build the divider with a heavy `━` segment under the active
	// tab cell. When the active index is out of range (e.g. Members
	// focus), the divider stays uniformly thin.
	var divider strings.Builder
	if !hasActive {
		divider.WriteString(tk.Muted.Render(strings.Repeat("─", width)))
		return labels.String() + "\n" + divider.String()
	}
	actStart := cellStart[active]
	actEnd := cellEnd[active]
	if actEnd > width {
		actEnd = width
	}
	if actStart > width {
		actStart = width
	}
	if actStart > 0 {
		divider.WriteString(tk.Muted.Render(strings.Repeat("─", actStart)))
	}
	if actEnd > actStart {
		divider.WriteString(tk.Accent.Render(strings.Repeat("━", actEnd-actStart)))
	}
	if width > actEnd {
		divider.WriteString(tk.Muted.Render(strings.Repeat("─", width-actEnd)))
	}
	return labels.String() + "\n" + divider.String()
}
