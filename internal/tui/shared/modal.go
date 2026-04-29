package shared

import "strings"

// ModalTone classifies the title row's emphasis. The chrome
// renders all overlays with a shared rounded border + 1-cell
// padding + Muted body; the title row picks tone per the
// overlay's role.
type ModalTone int

const (
	ModalToneNormal ModalTone = iota // default — Header bold
	ModalToneAccent                  // Accent (palette / picker)
	ModalToneDanger                  // Danger (quit / destructive ops)
)

// ModalIn collects the pieces every overlay supplies so MountModal
// can produce a consistent surface (v0.2.0): rounded border, 1-cell
// gutter, optional title and footer, picked tone for the title row.
// Body is the already-rendered content (multi-line string); Footer
// is rendered Muted directly above the closing border.
type ModalIn struct {
	Title  string
	Body   string
	Footer string
	Tone   ModalTone
	Width  int
	Tokens Tokens
}

// MountModal renders the v0.2.0 modal surface — a rounded-border
// box with consistent title / body / footer slots. Replaces the
// hand-rolled Modal call sites that mixed footer concatenation
// styles across overlays. Title tone selects emphasis (Accent for
// pickers, Danger for destructive). Tokens drives all colours;
// pass Dark / HighContrast / Monochrome.
//
// Width is the box's outer cell count; min 6. When Tokens is the
// zero value (e.g., legacy callers), the box renders without
// styling — the title and footer are still rendered, just plain.
func MountModal(in ModalIn) string {
	width := in.Width
	if width < 6 {
		width = 6
	}
	contentWidth := width - 3 // 1 left border + 1 padding + 1 right border
	if contentWidth < 1 {
		contentWidth = 1
	}

	var b strings.Builder
	b.WriteString(modalTop(width))
	b.WriteByte('\n')
	if in.Title != "" {
		title := in.Title
		switch in.Tone {
		case ModalToneAccent:
			if in.Tokens.Accent.GetForeground() != nil {
				title = in.Tokens.Accent.Render(in.Title)
			}
		case ModalToneDanger:
			if in.Tokens.Danger.GetForeground() != nil {
				title = in.Tokens.Danger.Render(in.Title)
			}
		default:
			if in.Tokens.Header.GetForeground() != nil {
				title = in.Tokens.Header.Render(in.Title)
			}
		}
		b.WriteString("│ " + padToVisible(title, contentWidth, in.Tokens) + "│")
		b.WriteByte('\n')
		b.WriteString(modalDivider(width))
		b.WriteByte('\n')
	}
	for _, line := range strings.Split(strings.TrimRight(in.Body, "\n"), "\n") {
		b.WriteString("│ " + padToVisible(line, contentWidth, in.Tokens) + "│")
		b.WriteByte('\n')
	}
	if in.Footer != "" {
		b.WriteString(modalDivider(width))
		b.WriteByte('\n')
		footer := in.Footer
		if in.Tokens.Muted.GetForeground() != nil {
			footer = in.Tokens.Muted.Render(in.Footer)
		}
		b.WriteString("│ " + padToVisible(footer, contentWidth, in.Tokens) + "│")
		b.WriteByte('\n')
	}
	b.WriteString(modalBottom(width))
	return b.String()
}

// Modal renders a centered RoundedBorder box around the supplied body lines.
// Each body line is padded with one cell of left/right gutter and the box is
// `width` cells wide overall. Returns the box as a single multi-line string
// without trailing newline.
//
// Pass title="" to skip the title row. Border characters use the same Unicode
// glyphs as RenderChrome so visual style stays consistent.
//
// Legacy entry point — prefer MountModal for new overlays. Kept so
// the existing Modal callers compile without churn.
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
