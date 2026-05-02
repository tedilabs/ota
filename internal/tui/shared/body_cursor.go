package shared

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

// BodyCursor carries the line-cursor + visual-mode state every
// detail surface uses (#F5 v0.2.5). Embed in a detail model and
// drive via Up / Down / Top / Bottom / StartVisual / CancelVisual;
// render a line slice with RenderLines.
//
// Screens own clipboard integration themselves — BodyCursor just
// reports the selected range so per-screen yank handlers can call
// the platform's clipboard with the chosen text.
type BodyCursor struct {
	Line   int
	Visual bool
	Anchor int
}

// Up moves the cursor one line up. No-op at the top.
func (c *BodyCursor) Up() {
	if c.Line > 0 {
		c.Line--
	}
}

// Down moves one line down, clamped to the last line. Pass the
// rendered body's line count.
func (c *BodyCursor) Down(total int) {
	if c.Line < total-1 {
		c.Line++
	}
}

// Top jumps the cursor to line 0.
func (c *BodyCursor) Top() {
	c.Line = 0
	if !c.Visual {
		c.Anchor = 0
	}
}

// Bottom jumps the cursor to the last line.
func (c *BodyCursor) Bottom(total int) {
	if total > 0 {
		c.Line = total - 1
	} else {
		c.Line = 0
	}
	if !c.Visual {
		c.Anchor = c.Line
	}
}

// StartVisual flips into visual mode anchored at the current cursor
// row. Subsequent Up/Down extend the selection range.
func (c *BodyCursor) StartVisual() {
	c.Visual = true
	c.Anchor = c.Line
}

// CancelVisual exits visual mode, leaving the cursor where it is.
func (c *BodyCursor) CancelVisual() {
	c.Visual = false
	c.Anchor = c.Line
}

// VisualRange returns [start, end] inclusive line indices for the
// visual selection. When Visual is false, the range is just the
// cursor row (so callers that always read VisualRange get the
// "operate on cursor line" semantics for free).
func (c BodyCursor) VisualRange() (start, end int) {
	if !c.Visual {
		return c.Line, c.Line
	}
	start, end = c.Anchor, c.Line
	if start > end {
		start, end = end, start
	}
	return start, end
}

// SelectedLines returns the slice of body lines covered by the
// visual selection (or just the cursor line when Visual is false).
// Used by the per-screen yank handler.
func (c BodyCursor) SelectedLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	start, end := c.VisualRange()
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	if start > end {
		return nil
	}
	return lines[start : end+1]
}

// RenderLines returns the body lines with cursor + visual styling
// applied. Each line is padded to `width` cells; the cursor row
// gets the `▸ ` prefix + RowCursor tint, lines inside the visual
// range get an accent tint.
func (c BodyCursor) RenderLines(lines []string, width int, tk Tokens) []string {
	if len(lines) == 0 {
		return nil
	}
	start, end := c.VisualRange()
	out := make([]string, len(lines))
	for i, line := range lines {
		prefix := "  "
		if i == c.Line {
			prefix = "▸ "
		}
		full := PadOrTruncateVisible(prefix+line, width)
		switch {
		case i == c.Line:
			full = tk.RowCursor.Render(StripCSI(full))
		case c.Visual && i >= start && i <= end:
			full = tk.Accent.Render(StripCSI(full))
		}
		out[i] = full
	}
	return out
}

// JoinLines glues a slice with newlines — convenience for callers
// pushing the rendered lines back into a strings.Builder.
func JoinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// YankCmd returns a tea.Cmd that copies the cursor's selected lines
// to the platform clipboard, emitting a ToastMsg with the result.
// Strips ANSI before copying so notes / pastes carry plain text
// only. Empty selection emits an info toast and skips the clipboard
// call. (#F5 v0.2.5)
func YankCmd(c BodyCursor, lines []string, label string) tea.Cmd {
	sel := c.SelectedLines(lines)
	if len(sel) == 0 {
		return toastInfoCmd("nothing to yank")
	}
	plain := make([]string, len(sel))
	for i, l := range sel {
		plain[i] = StripCSI(l)
	}
	text := strings.Join(plain, "\n")
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil {
			return ToastMsg{
				Text:  "yank failed: " + err.Error(),
				Level: ToastError,
				Until: time.Now().Add(5 * time.Second),
			}
		}
		unit := "line"
		if len(sel) != 1 {
			unit = "lines"
		}
		text := "yanked " + itoa(len(sel)) + " " + unit
		if label != "" {
			text += " from " + label
		}
		return ToastMsg{
			Text:  text,
			Level: ToastSuccess,
			Until: time.Now().Add(3 * time.Second),
		}
	}
}

func toastInfoCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return ToastMsg{Text: text, Level: ToastInfo, Until: time.Now().Add(2 * time.Second)}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
