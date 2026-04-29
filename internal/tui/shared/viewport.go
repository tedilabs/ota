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
	// v0.2.0: chrome reserved 7 rows + 1 column header + 1 safety
	// margin + 1 status row above key hints = 10. Each list emits
	// header + budget data rows; the chrome cap (height - 7) keeps
	// the cursor visible on tall terminals.
	const reserved = 10
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

// ScrollbarMark returns the per-row scrollbar marker for a single
// visible row index `r` (0-based, where 0 is the topmost rendered
// row in the window). Pure ASCII / box-drawing — caller styles the
// returned strings via the active token set.
//
//   - When the dataset already fits the window (`total <= budget`)
//     ScrollbarMark returns "" so list views can skip rendering a
//     scrollbar gutter entirely.
//   - Otherwise the function returns a thumb glyph (▌) when `r` is
//     inside the active scroll thumb and a track glyph (│) when it
//     is outside.
//
// The thumb's start row scales with `scrollTop / total` and its
// length with `budget / total`, matching the renderScrollBox helper
// the User Detail Groups/Apps boxes already use (issue #170). Lists
// reuse the same algorithm so the chrome's visual language is
// consistent.
func ScrollbarMark(r, scrollTop, budget, total int) string {
	if budget <= 0 || total <= budget {
		return ""
	}
	thumbStart, thumbEnd := scrollbarThumb(scrollTop, budget, total)
	if r >= thumbStart && r <= thumbEnd {
		return "▌"
	}
	return "│"
}

// AppendScrollbarSuffix returns the trailing " ▌" / " │" gutter for
// a single rendered list row. Empty when the dataset already fits the
// budget so callers can avoid reserving the gutter on small lists.
//
// The scrollbar is rendered as 2 cells: a 1-cell visual gap from the
// content followed by a 1-cell thumb / track glyph. Tints with the
// active token set — accent for the thumb so the operator's eye
// snaps to it; muted for the track so it recedes into the chrome.
func AppendScrollbarSuffix(rowInWindow, scrollTop, budget, total int, tk Tokens) string {
	mark := ScrollbarMark(rowInWindow, scrollTop, budget, total)
	if mark == "" {
		return ""
	}
	if mark == "▌" {
		return " " + tk.Accent.Render(mark)
	}
	return " " + tk.Muted.Render(mark)
}

// scrollbarThumb computes the thumb's [start, end] inclusive row
// range inside a window of `budget` rows, given the scroll offset
// and the total dataset size. Always returns at least a single-row
// thumb so the operator sees their position even on huge lists.
func scrollbarThumb(scrollTop, budget, total int) (start, end int) {
	if total <= budget {
		return 0, budget - 1
	}
	scale := float64(budget) / float64(total)
	thumbStart := int(float64(scrollTop) * scale)
	thumbLen := int(float64(budget) * scale)
	if thumbLen < 1 {
		thumbLen = 1
	}
	thumbEnd := thumbStart + thumbLen - 1
	if thumbEnd >= budget {
		thumbEnd = budget - 1
		thumbStart = thumbEnd - thumbLen + 1
		if thumbStart < 0 {
			thumbStart = 0
		}
	}
	return thumbStart, thumbEnd
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
