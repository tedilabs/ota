package shared

// ListBodyRowBudget converts a terminal height (from tea.WindowSizeMsg)
// into the number of data rows a list ListModel can render without pushing
// the chrome's top border off-screen.
//
// The reservation accounts for:
//   - chrome top border, 2 header rows, divider, 2 status rows, bottom
//     border (~7 rows, see app.clampBodyLines);
//   - the list's own context line / optional filter / column header
//     (~3 rows).
//
// We keep a 1-row safety margin so a sudden +1 wrap (e.g. wide column
// header in narrow terminals) doesn't push the chrome off. Returns 0 when
// height is not yet known so callers can fall back to "render everything".
func ListBodyRowBudget(height int) int {
	if height <= 0 {
		return 0
	}
	const reserved = 11
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
