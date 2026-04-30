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

	// RowChanged briefly tints rows whose data just changed in the
	// most recent refresh (issue #193 v0.2.3). Cyan/teal bg so the
	// flash reads as "fresh" rather than alarm. Applied for ~1s
	// then cleared by the View when the per-row timestamp ages out.
	RowChanged lipgloss.Style
}

// MonochromeEnabled reports whether ota should render without colour. Set by
// the standard NO_COLOR environment variable (PRD §6.4 / TUI_DESIGN §6.2).
// Callers typically branch on this when choosing a token set at startup.
func MonochromeEnabled() bool {
	return os.Getenv("NO_COLOR") != ""
}

// ThemeName classifies the active token set. Issue #U12 v0.2.5 — adds
// the Light variant for operators on white-background terminals.
type ThemeName string

const (
	ThemeDark         ThemeName = "dark"
	ThemeLight        ThemeName = "light"
	ThemeHighContrast ThemeName = "high-contrast"
	ThemeMonochrome   ThemeName = "monochrome"
)

// ResolveTheme picks the active theme based on (in priority order):
//  1. NO_COLOR env var → ThemeMonochrome.
//  2. Explicit override (e.g., from cfg / --theme flag) when non-empty
//     and recognised.
//  3. COLORFGBG env var heuristic (terminals like xterm export
//     "fg;bg" with bg ∈ {0…15} — bg ≥ 8 is a light terminal).
//  4. Fallback to ThemeDark.
//
// override accepts any ThemeName-stringly-equal value ("dark" /
// "light" / "high-contrast" / "monochrome"); unknown values fall
// through to the env-based detection.
func ResolveTheme(override string) ThemeName {
	if MonochromeEnabled() {
		return ThemeMonochrome
	}
	switch ThemeName(override) {
	case ThemeDark, ThemeLight, ThemeHighContrast, ThemeMonochrome:
		return ThemeName(override)
	}
	if isLightTerminal() {
		return ThemeLight
	}
	return ThemeDark
}

// isLightTerminal heuristically detects a light-background terminal
// via the COLORFGBG env var. xterm + many descendants export
// "<fg>;<bg>" where bg is an ANSI 16-color index — bg ≥ 8 indicates
// a bright/light background. Returns false on parse failure so the
// default stays the existing Dark theme.
func isLightTerminal() bool {
	v := os.Getenv("COLORFGBG")
	if v == "" {
		return false
	}
	// Parse last component as the bg index.
	for i := len(v) - 1; i >= 0; i-- {
		if v[i] == ';' {
			tail := v[i+1:]
			n := 0
			for _, c := range tail {
				if c < '0' || c > '9' {
					return false
				}
				n = n*10 + int(c-'0')
			}
			return n >= 8 && n <= 15
		}
	}
	return false
}

// PickTheme returns the Tokens set matching the named theme. Used by
// the App Shell's activeTokens helper and by tests that want to pin
// a specific theme regardless of env.
func PickTheme(name ThemeName) Tokens {
	switch name {
	case ThemeMonochrome:
		return Monochrome()
	case ThemeHighContrast:
		return HighContrast()
	case ThemeLight:
		return Light()
	}
	return Dark()
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
		// Cyan/teal bg — distinct from the alarm tones (red/amber)
		// and the cursor token (slate) so a refresh flash reads as
		// "fresh data, not an alarm".
		RowChanged: lipgloss.NewStyle().
			Background(lipgloss.Color("#1f3d4c")).
			Foreground(lipgloss.Color("#d4ecf0")),
	}
}

// Light returns the light-terminal theme (#U12 v0.2.5). Inverts the
// Dark palette's lightness while keeping each role's hue family so
// the colour-per-role mapping stays stable: blue accents read as
// "active / focus", green as "ok", amber as "warning", red as
// "danger". Tested against the Solarized-Light + Tomorrow-Light +
// macOS Default-Light terminal palettes.
func Light() Tokens {
	return Tokens{
		BG:      lipgloss.NewStyle().Background(lipgloss.Color("#fdf6e3")),
		FG:      lipgloss.NewStyle().Foreground(lipgloss.Color("#1c2733")),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7a89")),
		Header:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1f6f8b")).Bold(true),
		Accent:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1f6f8b")),
		Primary: lipgloss.NewStyle().Foreground(lipgloss.Color("#214b73")),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("#3f7d3f")),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("#a17317")),
		Danger:  lipgloss.NewStyle().Foreground(lipgloss.Color("#a8323a")).Bold(true),
		Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("#1f6f8b")),
		Magenta: lipgloss.NewStyle().Foreground(lipgloss.Color("#7a3a76")),
		BadgeSys: lipgloss.NewStyle().
			Background(lipgloss.Color("#d3d8dc")).
			Foreground(lipgloss.Color("#1c2733")),
		BadgeRule: lipgloss.NewStyle().
			Background(lipgloss.Color("#3f7d3f")).
			Foreground(lipgloss.Color("#ffffff")),
		BadgeLarge: lipgloss.NewStyle().
			Background(lipgloss.Color("#a17317")).
			Foreground(lipgloss.Color("#ffffff")),
		BadgeUnmask: lipgloss.NewStyle().
			Background(lipgloss.Color("#a8323a")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true),
		RowCursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#cfe1f0")).
			Foreground(lipgloss.Color("#214b73")).
			Bold(true),
		// Light-bg row tints — pastel reds / ambers / grays that
		// pop without overwhelming the body text. Same hue families
		// as Dark so abnormal-row recognition transfers between
		// themes without retraining the operator.
		RowDanger: lipgloss.NewStyle().
			Background(lipgloss.Color("#f9d3d6")).
			Foreground(lipgloss.Color("#5a1d22")),
		RowWarning: lipgloss.NewStyle().
			Background(lipgloss.Color("#f5e7c1")).
			Foreground(lipgloss.Color("#5b4111")),
		RowMuted: lipgloss.NewStyle().
			Background(lipgloss.Color("#e6e9ec")).
			Foreground(lipgloss.Color("#6c7a89")),
		RowChanged: lipgloss.NewStyle().
			Background(lipgloss.Color("#cfe6e9")).
			Foreground(lipgloss.Color("#1f4f5b")),
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
		RowChanged: plain.Underline(true),
	}
}
