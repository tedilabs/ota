package shared

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Tokens collects every Lip Gloss style used across screens so theme changes
// happen in one place (TUI_DESIGN §6.1).
type Tokens struct {
	BG           lipgloss.Style
	FG           lipgloss.Style
	Muted        lipgloss.Style
	Header       lipgloss.Style
	Accent       lipgloss.Style
	Primary      lipgloss.Style
	Success      lipgloss.Style
	Warning      lipgloss.Style
	Danger       lipgloss.Style
	Info         lipgloss.Style
	Magenta      lipgloss.Style
	BadgeSys     lipgloss.Style
	BadgeRule    lipgloss.Style
	BadgeLarge   lipgloss.Style
	BadgeUnmask  lipgloss.Style
	// RowCursor is the cursor-row style: a subtle background tint plus
	// the foreground accent, so the active row reads at-a-glance even when
	// the operator's focus is elsewhere on the screen. Mono falls back to
	// reverse-video for the same purpose. Renamed from RowCursor in
	// v0.2.0 (#182) so the role is unambiguous — every cursor-row tint
	// across list rows, detail body, 2-col Pretty, and overlays uses
	// this token.
	RowCursor lipgloss.Style

	// RowDanger / RowWarning / RowMuted tint the entire row's
	// background when STATUS is abnormal (issue #155). The bgs are
	// chosen to read as alarm severity at a glance — dark red for
	// LOCKED_OUT / INVALID, dark amber for SUSPENDED /
	// PASSWORD_EXPIRED, dark gray for DEPROVISIONED / INACTIVE —
	// without overwhelming the underlying text colours.
	RowDanger  lipgloss.Style
	RowWarning lipgloss.Style
	RowMuted   lipgloss.Style
}

// MonochromeEnabled reports whether ota should render without colour. Set by
// the standard NO_COLOR environment variable (PRD §6.4 / TUI_DESIGN §6.2).
// Callers typically branch on this when choosing a token set at startup.
func MonochromeEnabled() bool {
	return os.Getenv("NO_COLOR") != ""
}

// Dark returns the default dark theme (TUI_DESIGN §6.1).
func Dark() Tokens {
	return Tokens{
		BG:          lipgloss.NewStyle().Background(lipgloss.Color("#0b0f14")),
		FG:          lipgloss.NewStyle().Foreground(lipgloss.Color("#d8dee9")),
		Muted:       lipgloss.NewStyle().Foreground(lipgloss.Color("#5c6a7a")),
		Header:      lipgloss.NewStyle().Foreground(lipgloss.Color("#88c0d0")).Bold(true),
		Accent:      lipgloss.NewStyle().Foreground(lipgloss.Color("#81a1c1")),
		Primary:     lipgloss.NewStyle().Foreground(lipgloss.Color("#5e81ac")),
		Success:     lipgloss.NewStyle().Foreground(lipgloss.Color("#a3be8c")),
		Warning:     lipgloss.NewStyle().Foreground(lipgloss.Color("#ebcb8b")),
		Danger:      lipgloss.NewStyle().Foreground(lipgloss.Color("#bf616a")).Bold(true),
		Info:        lipgloss.NewStyle().Foreground(lipgloss.Color("#88c0d0")),
		Magenta:     lipgloss.NewStyle().Foreground(lipgloss.Color("#b48ead")),
		BadgeSys:    lipgloss.NewStyle().Background(lipgloss.Color("#4c566a")).Foreground(lipgloss.Color("#d8dee9")),
		BadgeRule:   lipgloss.NewStyle().Background(lipgloss.Color("#a3be8c")).Foreground(lipgloss.Color("#000000")),
		BadgeLarge:  lipgloss.NewStyle().Background(lipgloss.Color("#ebcb8b")).Foreground(lipgloss.Color("#000000")),
		BadgeUnmask: lipgloss.NewStyle().Background(lipgloss.Color("#bf616a")).Foreground(lipgloss.Color("#ffffff")).Bold(true),
		RowCursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#2e3440")).
			Foreground(lipgloss.Color("#88c0d0")).
			Bold(true),
		// Status-row backgrounds — dark enough to read as a tint
		// rather than an alarm, but distinct from the chrome's
		// neutral background so abnormal rows pop visually.
		RowDanger: lipgloss.NewStyle().
			Background(lipgloss.Color("#4c1f21")).
			Foreground(lipgloss.Color("#f0d4d6")),
		RowWarning: lipgloss.NewStyle().
			Background(lipgloss.Color("#4a3a17")).
			Foreground(lipgloss.Color("#f5e7c1")),
		RowMuted: lipgloss.NewStyle().
			Background(lipgloss.Color("#2a2f38")).
			Foreground(lipgloss.Color("#7a8290")),
	}
}

// HighContrast returns the high-contrast theme.
func HighContrast() Tokens {
	t := Dark()
	t.BG = lipgloss.NewStyle().Background(lipgloss.Color("#000000"))
	t.FG = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	return t
}

// Monochrome returns the NO_COLOR theme — colour stripped, reverse video on
// focus. Used when MonochromeEnabled() is true.
func Monochrome() Tokens {
	plain := lipgloss.NewStyle()
	return Tokens{
		BG:          plain,
		FG:          plain,
		Muted:       plain,
		Header:      plain.Bold(true),
		Accent:      plain.Reverse(true),
		Primary:     plain,
		Success:     plain,
		Warning:     plain.Bold(true),
		Danger:      plain.Bold(true),
		Info:        plain,
		Magenta:     plain,
		BadgeSys:     plain.Reverse(true),
		BadgeRule:    plain.Reverse(true),
		BadgeLarge:   plain.Reverse(true),
		BadgeUnmask:  plain.Reverse(true).Bold(true),
		RowCursor: plain.Reverse(true).Bold(true),
		// In monochrome mode the danger/warning/muted bg styles fall
		// back to bold (danger), italic-ish underline (warning), and
		// dim (muted) so abnormal rows still read distinctly without
		// colour. Lipgloss + the NO_COLOR fallback keeps the tokens
		// safe — they emit attribute codes instead of colour codes.
		RowDanger:  plain.Bold(true).Underline(true),
		RowWarning: plain.Bold(true),
		RowMuted:   plain.Faint(true),
	}
}
