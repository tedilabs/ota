package shared

import "strings"

// Modal renders a centered RoundedBorder box around the supplied body lines.
// Each body line is padded with one cell of left/right gutter and the box is
// `width` cells wide overall. Returns the box as a single multi-line string
// without trailing newline.
//
// Pass title="" to skip the title row. Border characters use the same Unicode
// glyphs as RenderChrome so visual style stays consistent.
func Modal(title, body string, width int) string {
	if width < 6 {
		width = 6
	}
	inner := width - 2
	contentWidth := inner - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	var b strings.Builder
	b.WriteString(modalTop(width))
	b.WriteByte('\n')
	if title != "" {
		b.WriteString("│ " + padTo(title, contentWidth) + "│")
		b.WriteByte('\n')
		b.WriteString(modalDivider(width))
		b.WriteByte('\n')
	}
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		b.WriteString("│ " + padTo(line, contentWidth) + "│")
		b.WriteByte('\n')
	}
	b.WriteString(modalBottom(width))
	return b.String()
}

func modalTop(width int) string    { return "╭" + strings.Repeat("─", width-2) + "╮" }
func modalBottom(width int) string { return "╰" + strings.Repeat("─", width-2) + "╯" }
func modalDivider(width int) string {
	return "├" + strings.Repeat("─", width-2) + "┤"
}

// KVRow returns a "<key padded right>  <value>" line where the key is
// right-aligned to keyWidth cells. Used by detail-view definition lists
// (TUI_DESIGN §15.7 Profile tab).
func KVRow(key, value string, keyWidth int) string {
	w := visibleWidth(key)
	if w > keyWidth {
		key = Truncate(key, keyWidth)
		w = visibleWidth(key)
	}
	return strings.Repeat(" ", keyWidth-w) + key + "    " + value
}

// SectionHeader returns a divider line "— <label> ─────...".
func SectionHeader(label string, width int) string {
	prefix := "— " + label + " "
	w := visibleWidth(prefix)
	if w >= width {
		return prefix
	}
	return prefix + strings.Repeat("─", width-w)
}
