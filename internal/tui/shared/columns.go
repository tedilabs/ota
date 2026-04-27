package shared

import "strings"

// ColumnKind discriminates a column's sizing behaviour.
type ColumnKind int

const (
	// ColumnFixed columns render at a constant Min width regardless of
	// available space. Used for badge/status/timestamp slots whose visual
	// alignment matters more than fluid expansion.
	ColumnFixed ColumnKind = iota
	// ColumnFlex columns expand to absorb the remaining width budget,
	// distributed by Weight. Their actual rendered width is
	// max(Min, flexBudget * Weight / sum(Weight)).
	ColumnFlex
)

// ColumnSpec describes one cell of a list table per TUI_DESIGN §15.0a.
// DropPriority controls the order in which columns disappear when the
// terminal is too narrow to fit them; lower values disappear first. A spec
// with DropPriority == 0 is treated as "essential — never drop". Specs are
// processed in declaration order so a stable left-to-right column layout is
// preserved across W transitions.
type ColumnSpec struct {
	Title        string
	Kind         ColumnKind
	Min          int
	Weight       int
	DropPriority int
	AlignRight   bool
}

// LayoutColumns picks which specs are visible at the given inner-body width
// (W' from §15.0a.1) and computes the rendered width for each.  The returned
// slice has the same length as `specs` — a non-visible column has Width == 0.
//
// The algorithm follows §15.0a.1:
//
//	1. Drop columns by ascending DropPriority (skipping essentials with 0)
//	   until the minimum width budget fits in W'.
//	2. Distribute the leftover budget to FLEX columns proportional to Weight.
//	3. Round-up jitter (≤ visibleN-1 cells) is absorbed by the last FLEX.
//
// gutter is the number of spaces inserted between adjacent columns (k9s
// convention: 2). Width must be >= 0.
func LayoutColumns(specs []ColumnSpec, width, gutter int) []int {
	n := len(specs)
	out := make([]int, n)
	if n == 0 || width <= 0 {
		return out
	}
	if gutter < 0 {
		gutter = 0
	}

	// Step 1 — visibility.  Start with all columns visible; if minSum +
	// gutters exceeds W', drop the lowest-essential column with the
	// smallest DropPriority > 0 and retry.
	visible := make([]bool, n)
	for i := range specs {
		visible[i] = true
	}
	for !fits(specs, visible, width, gutter) {
		idx := pickDropCandidate(specs, visible)
		if idx < 0 {
			break // no further drops possible — render what we have
		}
		visible[idx] = false
	}

	// Step 2 — width distribution.
	visibleN := 0
	fixedSum := 0
	flexSum := 0
	for i, c := range specs {
		if !visible[i] {
			continue
		}
		visibleN++
		switch c.Kind {
		case ColumnFixed:
			fixedSum += c.Min
		case ColumnFlex:
			flexSum += c.Weight
		}
	}
	if visibleN == 0 {
		return out
	}
	gutterTotal := gutter * (visibleN - 1)
	flexBudget := width - fixedSum - gutterTotal
	if flexBudget < 0 {
		flexBudget = 0
	}

	// Each FLEX column gets max(Min, floor(flexBudget * weight / flexSum)).
	// Track cumulative consumption so the last FLEX absorbs rounding.
	lastFlex := -1
	consumed := 0
	for i, c := range specs {
		if !visible[i] {
			continue
		}
		switch c.Kind {
		case ColumnFixed:
			out[i] = c.Min
			consumed += c.Min
		case ColumnFlex:
			share := c.Min
			if flexSum > 0 {
				calc := flexBudget * c.Weight / flexSum
				if calc > share {
					share = calc
				}
			}
			out[i] = share
			consumed += share
			lastFlex = i
		}
	}

	// Step 3 — sweep up rounding jitter into the last FLEX column.
	if lastFlex >= 0 {
		want := width - gutterTotal
		if consumed < want {
			out[lastFlex] += want - consumed
		}
	}
	return out
}

// fits reports whether the visible column set's minimum widths plus gutters
// fit into width.
func fits(specs []ColumnSpec, visible []bool, width, gutter int) bool {
	visibleN := 0
	minSum := 0
	for i, c := range specs {
		if !visible[i] {
			continue
		}
		visibleN++
		minSum += c.Min
	}
	if visibleN == 0 {
		return true
	}
	return minSum+gutter*(visibleN-1) <= width
}

// pickDropCandidate returns the index of the best column to drop: the one
// with the smallest non-zero DropPriority among currently-visible columns.
// Ties are broken by declaration order so transitions are stable. Returns
// -1 when no further drop is possible.
func pickDropCandidate(specs []ColumnSpec, visible []bool) int {
	idx := -1
	best := 0
	for i, c := range specs {
		if !visible[i] || c.DropPriority == 0 {
			continue
		}
		if idx == -1 || c.DropPriority < best {
			idx = i
			best = c.DropPriority
		}
	}
	return idx
}

// LayoutColumnsTight lays out columns at exactly their declared Min
// widths, with no flex puffing — the row's total width is the sum of
// Mins plus gutters; any leftover space inside the inner body is
// returned as empty cells at the end of the row (the chrome handles
// trailing padding). Returns nil when the tight layout doesn't fit
// the supplied width, so callers can fall back to the dropping
// LayoutColumns or the hScroll path.
//
// Use this in tandem with ShrinkSpecsToFit so the Min reflects each
// column's observed-data width — the row then renders as tight as
// the data demands, never wider. Issue #138 (the user's repeated
// "LOGIN 컬럼이 불필요하게 많은 크기를 차지하고 있어" complaint).
func LayoutColumnsTight(specs []ColumnSpec, width, gutter int) []int {
	n := len(specs)
	if n == 0 || width <= 0 {
		return nil
	}
	if gutter < 0 {
		gutter = 0
	}
	consumed := 0
	for i, c := range specs {
		need := c.Min
		if i > 0 {
			need += gutter
		}
		if consumed+need > width {
			return nil
		}
		consumed += need
	}
	out := make([]int, n)
	for i, c := range specs {
		out[i] = c.Min
	}
	return out
}

// LayoutColumnsHScroll lays out columns starting at the hScroll offset
// (a column index, not a character offset), packing as many columns as
// fit at their declared Min width into the inner-body width. Columns
// before hScroll get width 0 so FormatRow skips them, and columns past
// the budget similarly stay at 0 — they re-enter the viewport when the
// caller decrements hScroll.
//
// Unlike LayoutColumns this never drops "middle" columns by
// DropPriority. The caller has explicitly opted into horizontal-scroll
// mode (h / l keys), so the contract is "show a contiguous slice of
// columns, in declaration order". Combine with ShrinkSpecsToFit to make
// the slice cover exactly what the data demands.
//
// Leftover budget is distributed to ColumnFlex columns proportionally
// to Weight; the last visible flex column absorbs rounding so the row
// renders flush to width.
func LayoutColumnsHScroll(specs []ColumnSpec, width, gutter, hScroll int) []int {
	n := len(specs)
	out := make([]int, n)
	if n == 0 || width <= 0 {
		return out
	}
	if gutter < 0 {
		gutter = 0
	}
	if hScroll < 0 {
		hScroll = 0
	}
	if hScroll >= n {
		hScroll = n - 1
	}

	consumed := 0
	visibleN := 0
	for i := hScroll; i < n; i++ {
		need := specs[i].Min
		if visibleN > 0 {
			need += gutter
		}
		if consumed+need > width {
			break
		}
		out[i] = specs[i].Min
		consumed += need
		visibleN++
	}

	// Distribute leftover to flex columns inside the visible slice.
	flexSum := 0
	lastFlex := -1
	for i := hScroll; i < n; i++ {
		if out[i] == 0 {
			break
		}
		if specs[i].Kind == ColumnFlex {
			flexSum += specs[i].Weight
			lastFlex = i
		}
	}
	if flexSum > 0 && consumed < width {
		leftover := width - consumed
		added := 0
		for i := hScroll; i < n; i++ {
			if out[i] == 0 {
				break
			}
			if specs[i].Kind != ColumnFlex {
				continue
			}
			share := leftover * specs[i].Weight / flexSum
			out[i] += share
			added += share
		}
		if lastFlex >= 0 && added < leftover {
			out[lastFlex] += leftover - added
		}
	}
	return out
}

// MaxHScroll returns the largest valid hScroll the caller can apply
// before all remaining columns fit at Min within width. Used to clamp
// h / l navigation so the user can't scroll into an empty row.
func MaxHScroll(specs []ColumnSpec, width, gutter int) int {
	n := len(specs)
	if n == 0 || width <= 0 {
		return 0
	}
	if gutter < 0 {
		gutter = 0
	}
	consumed := 0
	visibleN := 0
	// Walk right-to-left: as long as the rightmost (n-1)..k columns fit,
	// increment k. The first k that overflows is one past the maximum
	// scroll, so return k+1.
	for i := n - 1; i >= 0; i-- {
		need := specs[i].Min
		if visibleN > 0 {
			need += gutter
		}
		if consumed+need > width {
			return i + 1
		}
		consumed += need
		visibleN++
	}
	return 0
}

// FormatRow renders cells with the given widths and a fixed gutter.  Each
// cell is padded (or truncated with "…") to its column's width.  Cells with
// width 0 (i.e. dropped columns) are skipped entirely. AlignRight specs are
// honoured via leading-space padding.
func FormatRow(specs []ColumnSpec, widths []int, cells []string, gutter int) string {
	if gutter < 0 {
		gutter = 0
	}
	gut := strings.Repeat(" ", gutter)

	var b strings.Builder
	first := true
	for i := range specs {
		w := widths[i]
		if w <= 0 {
			continue
		}
		if !first {
			b.WriteString(gut)
		}
		first = false
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		b.WriteString(padCell(cell, w, specs[i].AlignRight))
	}
	return b.String()
}

// padCell pads or truncates s to exactly width visible cells.
func padCell(s string, width int, alignRight bool) string {
	if width <= 0 {
		return ""
	}
	w := visibleWidth(s)
	if w == width {
		return s
	}
	if w > width {
		return Truncate(s, width)
	}
	pad := strings.Repeat(" ", width-w)
	if alignRight {
		return pad + s
	}
	return s + pad
}
