package shared

import "strings"

// RenderScrollBox draws a single rounded-border box with a title
// bar, a scrollable content window, and a vertical scrollbar on the
// right edge. Origin: User Detail Groups+Apps boxes (issue #170);
// promoted to shared in v0.2.2 (#189) so Group Detail's Members +
// Apps boxes — and any future screen with a side-by-side scroll
// layout — share the same widget.
//
// width / height are the OUTER dimensions of the box. Border +
// scrollbar reserve 5 cells from the inner content row width.
//
// focused == true lights up the border + title accent so the
// operator sees which box owns j/k. cursor is the row index inside
// items; pass -1 to suppress the highlight (e.g., when focus lives
// in a sibling box).
func RenderScrollBox(
	title string,
	items []string,
	focused bool,
	cursor, scrollTop, height, width int,
	tk Tokens,
) string {
	if width < 12 {
		width = 12
	}
	if height < 1 {
		height = 1
	}
	contentW := width - 5
	if contentW < 4 {
		contentW = 4
	}

	borderStyle := tk.Muted
	if focused {
		borderStyle = tk.Header
	}
	titleStr := title
	if focused {
		titleStr = tk.Accent.Render(title)
	}

	top := borderStyle.Render("╭─ ") + titleStr + " " +
		borderStyle.Render(strings.Repeat("─", maxIntShared(0, width-5-VisibleWidth(title)))+"╮")
	bottom := borderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")

	var lines []string
	lines = append(lines, top)
	for r := 0; r < height; r++ {
		idx := scrollTop + r
		row := ""
		if idx < len(items) {
			row = items[idx]
		}
		row = PadOrTruncateVisible(row, contentW)
		if focused && idx == cursor && idx < len(items) {
			row = tk.RowCursor.Render(StripCSI(row))
		}
		bar := " "
		if len(items) > height {
			thumbStart, thumbEnd := scrollboxThumb(scrollTop, height, len(items))
			if r >= thumbStart && r <= thumbEnd {
				bar = tk.Accent.Render("▌")
			} else {
				bar = tk.Muted.Render("│")
			}
		}
		lines = append(lines, borderStyle.Render("│ ")+row+" "+bar+borderStyle.Render("│"))
	}
	lines = append(lines, bottom)
	return strings.Join(lines, "\n")
}

// ClampScrollTop slides the scroll window so the cursor stays
// visible inside [scrollTop, scrollTop+height). Pure helper, no
// model state.
func ClampScrollTop(cursor, scrollTop, height, total int) int {
	if total <= height {
		return 0
	}
	if cursor < scrollTop {
		return cursor
	}
	if cursor >= scrollTop+height {
		return cursor - height + 1
	}
	if scrollTop+height > total {
		return total - height
	}
	if scrollTop < 0 {
		return 0
	}
	return scrollTop
}

// ComposeColumns lays two multi-line columns side-by-side, padding
// the left column to colWidth+2 cells before the right column
// starts. Either column may be longer; the shorter side ends
// earlier with empty cells in its slot.
func ComposeColumns(left, right string, colWidth int) string {
	const gutter = 2
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	rows := len(leftLines)
	if len(rightLines) > rows {
		rows = len(rightLines)
	}
	gut := strings.Repeat(" ", gutter)
	var b strings.Builder
	for i := 0; i < rows; i++ {
		var l, r string
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		w := VisibleWidth(l)
		if w < colWidth {
			l = l + strings.Repeat(" ", colWidth-w)
		}
		b.WriteString(l)
		if r != "" {
			b.WriteString(gut)
			b.WriteString(r)
		}
		if i < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// scrollboxThumb is the box-internal scrollbar thumb calculator —
// distinct from ScrollbarMark which is for list rows. Always
// returns at least a single-row thumb.
func scrollboxThumb(scrollTop, height, total int) (start, end int) {
	if total <= height {
		return 0, height - 1
	}
	scale := float64(height) / float64(total)
	thumbStart := int(float64(scrollTop) * scale)
	thumbLen := int(float64(height) * scale)
	if thumbLen < 1 {
		thumbLen = 1
	}
	thumbEnd := thumbStart + thumbLen - 1
	if thumbEnd >= height {
		thumbEnd = height - 1
		thumbStart = thumbEnd - thumbLen + 1
		if thumbStart < 0 {
			thumbStart = 0
		}
	}
	return thumbStart, thumbEnd
}

func maxIntShared(a, b int) int {
	if a > b {
		return a
	}
	return b
}