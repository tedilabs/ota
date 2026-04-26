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

// ShrinkSpecsToFit returns a copy of specs with each Min reduced to fit
// the largest cell observed in that column. Header width and an absolute
// floor (4 cells) are honoured so titles never clip and very narrow
// columns still leave room for "—" placeholders.
//
// observed[i] is the visible-cell width of the widest row body for that
// column. Pass nil when no data is available; in that case the original
// specs are returned unchanged.
//
// This is the auto-fit half of issue #117: columns no longer pad to
// their declared Min when data is shorter, so a list of 5-char titles
// doesn't reserve 16 cells full of trailing whitespace.
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
		if fit < out[i].Min {
			out[i].Min = fit
		}
	}
	return out
}
