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
// render a line slice with RenderViewport.
//
// Top is the topmost visible line in the body viewport — it slides
// so Line stays inside [Top, Top+viewportHeight) whenever a movement
// method is called with a viewport height. The viewport-aware moves
// (PageDown / PageUp / HalfPageDown / HalfPageUp) wire Ctrl-F / B /
// D / U on detail surfaces.
//
// Screens own clipboard integration themselves — BodyCursor just
// reports the selected range so per-screen yank handlers can call
// the platform's clipboard with the chosen text.
type BodyCursor struct {
	Line   int
	Top    int
	Visual bool
	Anchor int
}

// Up moves the cursor one line up. No-op at the top.
func (c *BodyCursor) Up() {
	if c.Line > 0 {
		c.Line--
	}
	if c.Line < c.Top {
		c.Top = c.Line
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
func (c *BodyCursor) GoTop() {
	c.Line = 0
	c.Top = 0
	if !c.Visual {
		c.Anchor = 0
	}
}

// Bottom jumps the cursor to the last line.
func (c *BodyCursor) GoBottom(total int) {
	if total > 0 {
		c.Line = total - 1
	} else {
		c.Line = 0
	}
	if !c.Visual {
		c.Anchor = c.Line
	}
}

// PageDown advances the cursor by one viewport. The viewport top
// follows so the cursor stays anchored relative to it. Clamped to
// the last line.
func (c *BodyCursor) PageDown(viewport, total int) {
	if viewport <= 0 {
		viewport = 1
	}
	c.Line += viewport
	if c.Line > total-1 {
		c.Line = total - 1
	}
	if c.Line < 0 {
		c.Line = 0
	}
}

// PageUp moves the cursor up by one viewport.
func (c *BodyCursor) PageUp(viewport int) {
	if viewport <= 0 {
		viewport = 1
	}
	c.Line -= viewport
	if c.Line < 0 {
		c.Line = 0
	}
}

// HalfPageDown / HalfPageUp wire Ctrl-D / Ctrl-U.
func (c *BodyCursor) HalfPageDown(viewport, total int) {
	step := viewport / 2
	if step < 1 {
		step = 1
	}
	c.Line += step
	if c.Line > total-1 {
		c.Line = total - 1
	}
	if c.Line < 0 {
		c.Line = 0
	}
}

func (c *BodyCursor) HalfPageUp(viewport int) {
	step := viewport / 2
	if step < 1 {
		step = 1
	}
	c.Line -= step
	if c.Line < 0 {
		c.Line = 0
	}
}

// EnsureVisible slides Top so Line falls inside the viewport. Call
// before rendering when the cursor may have moved past the window.
func (c *BodyCursor) EnsureVisible(viewport, total int) {
	if viewport <= 0 {
		c.Top = 0
		return
	}
	if total <= viewport {
		c.Top = 0
		return
	}
	if c.Line < c.Top {
		c.Top = c.Line
	}
	if c.Line >= c.Top+viewport {
		c.Top = c.Line - viewport + 1
	}
	if c.Top+viewport > total {
		c.Top = total - viewport
	}
	if c.Top < 0 {
		c.Top = 0
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

// RenderViewport returns the visible window of body lines with
// cursor + visual styling applied. height is the number of rows the
// caller has reserved for the body cursor area; when <= 0 the full
// list is rendered (for tests / legacy callers without a height).
//
// Each rendered line is padded to `width` cells; the cursor row gets
// the `▸ ` prefix + RowCursor tint, lines inside the visual range
// get the same RowCursor tint without the marker (matches the Users
// detail visual-mode behavior).
func (c *BodyCursor) RenderViewport(lines []string, width, height int, tk Tokens) []string {
	if len(lines) == 0 {
		return nil
	}
	if height > 0 {
		c.EnsureVisible(height, len(lines))
	}
	start, end := c.VisualRange()
	from, to := 0, len(lines)
	if height > 0 {
		from = c.Top
		to = c.Top + height
		if to > len(lines) {
			to = len(lines)
		}
	}
	out := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		line := lines[i]
		prefix := "  "
		if i == c.Line {
			prefix = "▸ "
		}
		full := PadOrTruncateVisible(prefix+line, width)
		switch {
		case i == c.Line:
			full = tk.RowCursor.Render(StripCSI(full))
		case c.Visual && i >= start && i <= end:
			full = tk.RowCursor.Render(StripCSI(full))
		}
		out = append(out, full)
	}
	return out
}

// RenderLines is the legacy no-height shim — equivalent to
// RenderViewport with height=0.
func (c *BodyCursor) RenderLines(lines []string, width int, tk Tokens) []string {
	return c.RenderViewport(lines, width, 0, tk)
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
