package shared_test

// Tests for the §15.0a column layout algorithm. Two-shape coverage:
// (1) the §15.0a.5 Users scenarios (W = 80/100/120/180/240) — visibility +
// flex distribution + last-flex absorption; (2) edge cases (zero width,
// no FLEX columns, gutter overhead).

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/tui/shared"
)

// usersSpecsFixture mirrors the live Users column spec (§15.0a.2). Tests
// keep an in-line copy so they don't depend on package internals.
func usersSpecsFixture() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "STATUS", Kind: shared.ColumnFixed, Min: 14, DropPriority: 0},
		{Title: "LOGIN", Kind: shared.ColumnFixed, Min: 22, DropPriority: 0},
		{Title: "DISPLAY NAME", Kind: shared.ColumnFlex, Min: 14, Weight: 2, DropPriority: 3},
		{Title: "LAST LOGIN", Kind: shared.ColumnFixed, Min: 10, DropPriority: 1, AlignRight: true},
		{Title: "CHANGED", Kind: shared.ColumnFlex, Min: 8, Weight: 1, DropPriority: 2, AlignRight: true},
	}
}

// Test_LayoutColumns_Width80_DropsLastLogin — at the §15.0a.5 minimum width
// (80 cells body), the smallest DropPriority column (LAST LOGIN, priority 1)
// drops first, then CHANGED (priority 2). STATUS+LOGIN+DISPLAY NAME survive.
func Test_LayoutColumns_NarrowDropsLowPriorityFirst(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	// Pick a body width too small for the full set: STATUS(14) + LOGIN(22) +
	// LAST LOGIN(10) + CHANGED(8) + DISPLAY NAME min(14) + 8 gutters = 76;
	// at width 60 we must drop columns until we fit.
	widths := shared.LayoutColumns(specs, 60, 2)

	assert.Greater(t, widths[0], 0, "STATUS must remain visible (DropPriority=0)")
	assert.Greater(t, widths[1], 0, "LOGIN must remain visible (DropPriority=0)")
	assert.Equal(t, 0, widths[3], "LAST LOGIN should drop first (priority 1)")
}

// Test_LayoutColumns_Width120_AllVisibleProportional — at body width 120,
// the §15.0a.5 standard scenario, all 5 columns must render and the FLEX
// budget is split between DISPLAY NAME (weight 2) and CHANGED (weight 1).
func Test_LayoutColumns_StandardSplitsFlexBudgetByWeight(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	widths := shared.LayoutColumns(specs, 120, 2)

	for i, w := range widths {
		assert.Greater(t, w, 0, "column %d (%s) must be visible at width 120", i, specs[i].Title)
	}

	// DISPLAY NAME (weight 2) should be wider than CHANGED (weight 1).
	assert.Greater(t, widths[2], widths[4],
		"DISPLAY NAME (weight 2) must be wider than CHANGED (weight 1) at standard width")
}

// Test_LayoutColumns_FillsTotalWidth — visible widths plus gutters must
// match the requested total width exactly so the body never leaves trailing
// blank cells. Last-FLEX absorption is the mechanism.
func Test_LayoutColumns_FillsTotalWidth(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	for _, total := range []int{120, 140, 180, 240} {
		widths := shared.LayoutColumns(specs, total, 2)
		visibleN := 0
		sum := 0
		for _, w := range widths {
			if w > 0 {
				visibleN++
				sum += w
			}
		}
		gutters := 0
		if visibleN > 1 {
			gutters = 2 * (visibleN - 1)
		}
		assert.Equal(t, total, sum+gutters,
			"layout must fill exactly %d cells (got %d cols summing %d + %d gutters)",
			total, visibleN, sum, gutters)
	}
}

// Test_LayoutColumns_ZeroWidth — 0 width means no body to render; every
// column gets width 0 (caller decides what to do).
func Test_LayoutColumns_ZeroWidth(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	widths := shared.LayoutColumns(specs, 0, 2)
	for i, w := range widths {
		assert.Equal(t, 0, w, "column %d must be 0 when total width is 0", i)
	}
}

// Test_LayoutColumnsHScroll_ZeroOffsetMatchesNaturalSlice — with
// hScroll == 0 and a width that fits all columns at Min, every column
// is visible (never drops a middle column to honor DropPriority).
func Test_LayoutColumnsHScroll_ZeroOffsetMatchesNaturalSlice(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	// 14+22+14+10+8 + 4*2 = 76 — comfortable fit at width 80.
	widths := shared.LayoutColumnsHScroll(specs, 80, 2, 0)
	for i, w := range widths {
		assert.Greater(t, w, 0, "column %d (%s) must be visible at hScroll=0", i, specs[i].Title)
	}
}

// Test_LayoutColumnsHScroll_OffsetSkipsLeadingColumns — with hScroll > 0
// the leading columns disappear (width 0) and the slice from hScroll
// onward fills the budget.
func Test_LayoutColumnsHScroll_OffsetSkipsLeadingColumns(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	widths := shared.LayoutColumnsHScroll(specs, 60, 2, 2)

	assert.Equal(t, 0, widths[0], "STATUS must be hidden when hScroll=2")
	assert.Equal(t, 0, widths[1], "LOGIN must be hidden when hScroll=2")
	assert.Greater(t, widths[2], 0, "DISPLAY NAME (first visible) must render")
}

// Test_LayoutColumnsHScroll_PacksUntilOverflow — the first column past
// the budget is dropped but the slice never breaks contiguity (no
// "skip and resume" mid-scroll).
func Test_LayoutColumnsHScroll_PacksUntilOverflow(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	// Budget that fits STATUS(14) + LOGIN(22) + 1 gutter(2) = 38 only.
	widths := shared.LayoutColumnsHScroll(specs, 38, 2, 0)

	assert.Greater(t, widths[0], 0, "STATUS must fit")
	assert.Greater(t, widths[1], 0, "LOGIN must fit")
	for i := 2; i < len(widths); i++ {
		assert.Equal(t, 0, widths[i], "column %d must be cut off after budget exhaustion", i)
	}
}

// Test_MaxHScroll_AllFit_ReturnsZero — when the natural Min layout fits
// the available width, hScroll has no upper bound work to do.
func Test_MaxHScroll_AllFit_ReturnsZero(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	got := shared.MaxHScroll(specs, 200, 2)
	assert.Equal(t, 0, got, "no overflow → max hScroll = 0")
}

// Test_MaxHScroll_ReportsFirstColumnPastOverflow — when not everything
// fits, the cap is the smallest hScroll that lets the trailing columns
// fit without re-overflowing. Acts as the guard rail for `l` navigation.
func Test_MaxHScroll_ReportsFirstColumnPastOverflow(t *testing.T) {
	t.Parallel()

	specs := usersSpecsFixture()
	// Width 38 only fits the last 2 cols (CHANGED 8 + LAST LOGIN 10 +
	// gutter 2 = 20 — actually fits more). Let's construct precisely:
	// last col CHANGED(8) alone = 8. + LAST LOGIN(10)+gutter(2) = 20.
	// + DISPLAY NAME min(14)+gutter(2) = 36. + LOGIN(22)+gutter(2) = 60
	// — overflow at adding LOGIN. So max scroll is 1 (skip STATUS only).
	// Budget = 50.
	got := shared.MaxHScroll(specs, 50, 2)
	assert.Greater(t, got, 0, "overflow at width 50 must produce a non-zero cap")
}

// Test_FormatRow_RespectsAlignRightPadding — AlignRight cells use leading
// spaces; left-aligned cells use trailing spaces.
func Test_FormatRow_RespectsAlignRightPadding(t *testing.T) {
	t.Parallel()

	specs := []shared.ColumnSpec{
		{Title: "L", Kind: shared.ColumnFixed, Min: 4, AlignRight: false},
		{Title: "R", Kind: shared.ColumnFixed, Min: 4, AlignRight: true},
	}
	widths := []int{4, 4}
	row := shared.FormatRow(specs, widths, []string{"a", "b"}, 2)
	// "a   " + "  " + "   b"  → length 4+2+4 = 10.
	assert.Equal(t, "a   "+"  "+"   b", row,
		"row must left-pad first cell and right-pad second cell")
	// Sanity: visible cells == 10.
	assert.Equal(t, 10, len(strings.TrimRight(row, "")))
}
