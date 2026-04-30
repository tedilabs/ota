package shared

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// SpinnerInterval is the cadence the loading spinner advances at —
// 80ms reads as a smooth animation without burning frames.
const SpinnerInterval = 80 * time.Millisecond

// spinnerFrames is the braille-dot rotation k9s / lazygit / btop all
// converged on. 10 frames = a full rotation per ~800ms.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerFrame returns the glyph at the given index, wrapping. Lists
// hold an int counter and bump it on each spinner tick.
func SpinnerFrame(frame int) string {
	if frame < 0 {
		frame = -frame
	}
	return spinnerFrames[frame%len(spinnerFrames)]
}

// ScheduleSpinnerTickCmd returns a tea.Tick at SpinnerInterval that
// fires the supplied msg. Each list passes its own zero-value tick
// msg type (e.g. usersSpinnerTickMsg{}) so the Update switch can
// route precisely. Reschedule only while still loading.
func ScheduleSpinnerTickCmd(msg tea.Msg) tea.Cmd {
	return tea.Tick(SpinnerInterval, func(time.Time) tea.Msg {
		return msg
	})
}

// LoadingPlaceholder renders a centered "<spinner>  <label>" block
// padded to roughly fill the body region, so the chrome's lower
// divider doesn't snap upward when data finally lands. Issue #194
// v0.2.4 — give first-fetch a visible "I'm working on it" cue.
func LoadingPlaceholder(frame int, label string, bodyWidth, bodyHeight int, tk Tokens) string {
	glyph := SpinnerFrame(frame)
	line := tk.Accent.Render(glyph) + "  " + tk.Muted.Render(label)
	if bodyWidth < 1 {
		bodyWidth = 1
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	w := VisibleWidth(line)
	var hpad string
	if bodyWidth > w {
		hpad = strings.Repeat(" ", (bodyWidth-w)/2)
	}
	centered := hpad + line
	var b strings.Builder
	topPad := bodyHeight / 2
	for i := 0; i < topPad; i++ {
		b.WriteByte('\n')
	}
	b.WriteString(centered)
	bottomPad := bodyHeight - topPad - 1
	for i := 0; i < bottomPad; i++ {
		b.WriteByte('\n')
	}
	return b.String()
}
