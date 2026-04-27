package shared

// ListBodyRowBudget converts a terminal height (from tea.WindowSizeMsg)
// into the number of data rows a list ListModel can render without pushing
// the chrome's top border off-screen.
//
// Reservation (k9s-style chrome, issue #133):
//   - chrome top border, TitleBar, upper divider, status divider,
//     KeyHints, bottom border (~6 rows);
//   - the list's own count line / column header (~2 rows);
//   - 1-row safety margin so a sudden +1 wrap doesn't clip.
//
// Returns 0 when height is not yet known so callers can fall back to
// "render everything".
func ListBodyRowBudget(height int) int {
	if height <= 0 {
		return 0
	}
	const reserved = 9
	rows := height - reserved
	if rows < 3 {
		rows = 3
	}
	return rows
}

// WindowBounds returns the half-open [top, end) slice of rows that should
// render given the cursor position, the previous viewport top, the total
// number of rows, and the body budget.
//
// Slides the viewport so the cursor stays inside [top, end). When budget is
// zero or the dataset already fits, returns [0, total) — equivalent to "no
// windowing" — so callers that don't yet know their height keep the
// existing render unchanged.
func WindowBounds(cursor, prevTop, total, budget int) (top int, end int) {
	if total == 0 {
		return 0, 0
	}
	if budget <= 0 || total <= budget {
		return 0, total
	}
	top = prevTop
	if top < 0 {
		top = 0
	}
	if cursor < top {
		top = cursor
	}
	if cursor >= top+budget {
		top = cursor - budget + 1
	}
	if top+budget > total {
		top = total - budget
	}
	if top < 0 {
		top = 0
	}
	return top, top + budget
}

// ShrinkSpecsToFit returns a copy of specs with each Min set to
// max(header_width, observed_data_width, floor) — the EXACT width
// the column needs to render its data without truncation.
//
// Issue #145 (the user's recurring "columns are clipped" complaint):
// the previous implementation only ever SHRANK Min, never expanded
// it. When observed > original_Min, Min stayed at original_Min and
// LayoutColumnsTight ran data through padCell's truncate path —
// "alice.anderson@verylongco…" instead of the full login. With the
// fix, Min always reflects the data demands and tight layout either
// fits everything or falls through to LayoutColumnsHScroll for
// horizontal scrolling.
//
// observed[i] is the visible-cell width of the widest row body for
// that column. Pass nil when no data is available; in that case the
// original specs are returned unchanged.
func ShrinkSpecsToFit(specs []ColumnSpec, observed []int) []ColumnSpec {
	if observed == nil {
		return specs
	}
	out := make([]ColumnSpec, len(specs))
	copy(out, specs)
	const floor = 4
	for i := range out {
		header := visibleWidth(out[i].Title)
		fit := header
		if i < len(observed) && observed[i] > fit {
			fit = observed[i]
		}
		if fit < floor {
			fit = floor
		}
		out[i].Min = fit
	}
	return out
}
