package form

// v2 modal-style render helpers for SCR-012 redesign
// (`_workspace/edit-form-users/redesign/03_tui_design_v2.md`).
//
// These helpers produce the "focus lift" look: non-focused field rows
// are plain `label value`, focused rows get a `▎` left bar plus
// `┃ value ┃` border. Section headers fold the dirty-field count
// inline (`─ Identity · 2* ──`) so the operator can scan per-section
// progress without leaving the form.
//
// The helpers are intentionally pure string functions — they take
// rendered tokens + state and return a line. The owning screen
// (EditModel.renderEditingModal) composes them.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tedilabs/ota/internal/tui/shared"
)

// FieldRowOpts collects everything RenderFieldRow needs to lay out one
// field. LabelCol is the cell count reserved for the label column
// (label + padding); InputCol is the cell budget for the value column
// after that (no border). The helper handles dirty / focus / read-only
// / masked toggles uniformly so each variant is one assignment, not a
// branch in the caller.
type FieldRowOpts struct {
	Label     string
	Value     string
	Focused   bool
	Dirty     bool
	ReadOnly  bool
	Masked    bool
	InlineErr string
	LabelCol  int
	InputCol  int
	// CursorPos is the byte position the caret should stamp at within
	// Value. Used only when Focused && !ReadOnly. Negative values
	// disable cursor rendering (matches Form.CursorPos() == -1).
	// A position equal to len(Value) renders the caret as an
	// inserted-after-end block so the operator sees where the next
	// keystroke lands.
	CursorPos int
}

// RenderFieldRow lays out a single field line per D-W20.
//
//	non-focus, clean:    "  Label         value"
//	non-focus, dirty:    "* Label         value"
//	focus,    clean:     "▎ Label       ┃ value           ┃"
//	focus,    dirty:     "*▎ Label      ┃ value           ┃"
//	read-only:           "  Label         alice@…  (read-only)"
//	masked:              "  Mobile        +1-…    (masked)"
//
// When InlineErr is non-empty, a second line is appended ("    ↳ <err>").
// Returns a multi-line string; callers append a newline if continuing.
func RenderFieldRow(tk shared.Tokens, opts FieldRowOpts) string {
	if opts.LabelCol < 4 {
		opts.LabelCol = 4
	}
	if opts.InputCol < 4 {
		opts.InputCol = 4
	}

	// Prefix: left margin = 2 cells.
	//   focused + dirty → "*▎"
	//   focused only    → " ▎"
	//   dirty only      → "* "
	//   neither         → "  "
	var prefix string
	switch {
	case opts.Focused && opts.Dirty:
		prefix = styled(tk.Warning, "*") + styled(tk.Accent, "▎")
	case opts.Focused:
		prefix = " " + styled(tk.Accent, "▎")
	case opts.Dirty:
		prefix = styled(tk.Warning, "*") + " "
	default:
		prefix = "  "
	}

	// Label column — bold + accent when focused, muted otherwise.
	labelText := padRight(opts.Label, opts.LabelCol-1)
	var labelStyled string
	switch {
	case opts.Focused:
		labelStyled = styled(tk.Accent.Bold(true), labelText)
	default:
		labelStyled = styled(tk.Muted, labelText)
	}

	// Value column.
	value := opts.Value
	trail := ""
	if opts.ReadOnly {
		trail = "  " + styled(tk.Muted, "(read-only)")
	} else if opts.Masked {
		trail = "  " + styled(tk.Muted, "(masked)")
	}

	// inputCol budget governs the *cell width* visible in the value
	// box. We pad/truncate to InputCol so dirty + (read-only)/(masked)
	// markers align across rows.
	valuePadded := padRight(value, opts.InputCol)
	var valueStyled string
	switch {
	case opts.Focused && !opts.ReadOnly && opts.CursorPos >= 0:
		// Stamp a reverse-video caret onto the focused row so the
		// operator can track edit position. Works in NO_COLOR /
		// monochrome (reverse is an SGR attribute, not a color).
		valueStyled = stampCursor(tk, valuePadded, opts.CursorPos, opts.Dirty)
	case opts.Dirty:
		valueStyled = styled(tk.Header.Bold(true), valuePadded)
	case opts.ReadOnly:
		valueStyled = styled(tk.Muted, valuePadded)
	default:
		valueStyled = styled(tk.FG, valuePadded)
	}

	var body string
	if opts.Focused && !opts.ReadOnly {
		// "┃ value ┃"
		left := styled(tk.Accent, "┃")
		right := styled(tk.Accent, "┃")
		body = prefix + " " + labelStyled + " " + left + " " + valueStyled + " " + right + trail
	} else {
		body = prefix + " " + labelStyled + " " + valueStyled + trail
	}

	if opts.InlineErr != "" {
		errLine := "    " + styled(tk.Danger, "! "+opts.InlineErr)
		body = body + "\n" + errLine
	}
	return body
}

// RenderSectionHeader produces "─ Identity · 2* ─────" style headers.
// dirtyCount > 0 picks the Accent+bold tone (per D-W21); zero stays
// Muted. width is the target cell width — the trailing dashes pad to
// that target.
func RenderSectionHeader(tk shared.Tokens, name string, dirtyCount, width int) string {
	core := "─ " + name
	if dirtyCount > 0 {
		core = fmt.Sprintf("─ %s · %d*", name, dirtyCount)
	}
	core += " "
	// Pad with dashes up to width cells.
	visW := visibleWidth(core)
	if visW < width {
		core += strings.Repeat("─", width-visW)
	}
	if dirtyCount > 0 {
		return styled(tk.Header.Bold(true), core)
	}
	return styled(tk.Muted, core)
}

// padRight pads/truncates s to exactly width cells (best-effort —
// counts runes since SCR-012 fields are ASCII-dominant).
func padRight(s string, width int) string {
	w := visibleWidth(s)
	if w >= width {
		return shared.Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// visibleWidth strips any inline ANSI escapes before counting cells —
// mirrors the safe path the chrome uses everywhere.
func visibleWidth(s string) int {
	return len([]rune(shared.StripCSI(s)))
}

// styled applies a lipgloss.Style only when it carries a foreground
// (matching the safe-render pattern in shared/modal.go). When the
// style is the zero value (e.g., monochrome tokens with no fg), the
// raw string is returned so NO_COLOR / plain modes stay clean.
func styled(s lipgloss.Style, body string) string {
	if s.GetForeground() != nil {
		return s.Render(body)
	}
	return body
}

// stampCursor splices a reverse-video caret onto padded at byte
// position pos. The character at pos (or a synthetic space when pos
// lands at the padded value's end-of-text) is rendered with
// `lipgloss.NewStyle().Reverse(true)` so the operator can locate the
// edit caret. Surrounding cells follow the field's normal styling
// (Dirty → Header bold, otherwise FG).
func stampCursor(tk shared.Tokens, padded string, pos int, dirty bool) string {
	if pos < 0 {
		pos = 0
	}
	runes := []rune(padded)
	if pos > len(runes) {
		pos = len(runes)
	}
	// `padded` was right-padded to InputCol, so cursor always has at
	// least one cell to land on (the trailing space). Defensive:
	// extend if pos lands beyond the padded length.
	if pos >= len(runes) {
		runes = append(runes, ' ')
	}
	left := string(runes[:pos])
	caret := string(runes[pos])
	right := string(runes[pos+1:])

	base := tk.FG
	if dirty {
		base = tk.Header.Bold(true)
	}
	caretStyle := lipgloss.NewStyle().Reverse(true)
	if base.GetForeground() != nil {
		caretStyle = caretStyle.Foreground(base.GetForeground())
	}

	out := ""
	if left != "" {
		out += styled(base, left)
	}
	out += caretStyle.Render(caret)
	if right != "" {
		out += styled(base, right)
	}
	return out
}
