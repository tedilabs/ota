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
