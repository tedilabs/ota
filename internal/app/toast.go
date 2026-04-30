package app

// Toast — App Shell-level helpers around the unified shared.ToastMsg
// surface. Issue #A2 v0.2.4 — extracted from app.go.

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/tui/shared"
)

// toastInfo / toastErr build a ToastMsg with the standard auto-dismiss
// windows (3s for success, 5s for error) used by every action handler.
func toastInfo(text string) ToastMsg {
	return ToastMsg{Text: text, Level: ToastSuccess, Until: time.Now().Add(3 * time.Second)}
}
func toastErr(text string) ToastMsg {
	return ToastMsg{Text: text, Level: ToastError, Until: time.Now().Add(5 * time.Second)}
}

// toastCmdInfo wraps a synchronous info ToastMsg as a tea.Cmd.
func toastCmdInfo(text string) tea.Cmd {
	return func() tea.Msg { return toastInfo(text) }
}

// toastCmdError converts an ErrorMsg into a red error toast.
func toastCmdError(e ErrorMsg) tea.Cmd {
	return func() tea.Msg {
		return ToastMsg{
			Text:  e.Err.Error(),
			Level: ToastError,
			Until: time.Now().Add(3 * time.Second),
		}
	}
}

// toastClearMsg fires when a stored ToastMsg's expiry has elapsed and
// the floating band should disappear. gen guards against an older
// pending clear knocking out a newer toast (issue #195 v0.2.4).
type toastClearMsg struct{ gen int }

// scheduleToastClearCmd returns a tea.Tick that clears the floating
// toast after `d`. Negative / zero d collapses to a 1.5s minimum so
// the band is at least readable before it fades.
func scheduleToastClearCmd(gen int, d time.Duration) tea.Cmd {
	if d < 1500*time.Millisecond {
		d = 1500 * time.Millisecond
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return toastClearMsg{gen: gen}
	})
}

// renderToastBand returns a 1-line color-coded band — green for
// Success, red for Error, yellow for Warn, muted for Info — centered
// inside the chrome's content width (issue #195 v0.2.4). The band
// stamps on top of the body via stampOverlayOnTop so action results
// pop without stealing a chrome row.
func renderToastBand(t ToastMsg, contentWidth int, tk shared.Tokens) string {
	if t.Text == "" {
		return ""
	}
	var icon string
	var styler lipgloss.Style
	switch t.Level {
	case ToastSuccess:
		icon = "✓"
		styler = tk.Success
	case ToastError:
		icon = "✗"
		styler = tk.Danger
	case ToastWarn:
		icon = "!"
		styler = tk.Warning
	default:
		icon = "•"
		styler = tk.Accent
	}
	body := icon + "  " + t.Text
	w := shared.VisibleWidth(body)
	if contentWidth <= 0 {
		contentWidth = w + 4
	}
	if w > contentWidth {
		body = shared.Truncate(body, contentWidth)
		w = shared.VisibleWidth(body)
	}
	pad := 0
	if contentWidth > w {
		pad = (contentWidth - w) / 2
	}
	return strings.Repeat(" ", pad) + styler.Bold(true).Render(body)
}
