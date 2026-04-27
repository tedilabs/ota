package shared

import (
	"fmt"
	"strings"
)

// ChromeWidth is the total terminal column count the chrome renders against
// when no width is supplied. 85 matches TUI_DESIGN §16 golden snapshots
// (RoundedBorder corner + 83 inner cells + RoundedBorder corner).
const ChromeWidth = 85

// RateLimitState classifies the chrome's [RL: ...] badge state.
type RateLimitState int

const (
	RateLimitOK RateLimitState = iota
	RateLimitWarn
	RateLimitLimited
	RateLimitUnknown
)

// String returns the badge text body.
func (s RateLimitState) String() string {
	switch s {
	case RateLimitWarn:
		return "warn"
	case RateLimitLimited:
		return "limited"
	case RateLimitUnknown:
		return "?"
	default:
		return "ok"
	}
}

// ChromeInput collects the data the App Shell composes around any child
// Screen body. See TUI_DESIGN §15.1.
type ChromeInput struct {
	// Tokens drives all colors. Use Dark / HighContrast / Monochrome.
	Tokens Tokens

	// Width is the total terminal width. Defaults to ChromeWidth when 0.
	Width int

	// Brand label — typically "ota".
	Brand string

	// Tenant FQDN — e.g., "acme.okta.com".
	Tenant string

	// Profile / environment name shown next to the tenant — "prod" / "dev" /
	// "staging".  Doubles as the env classifier.
	Profile string

	// Principal is the authenticated Okta user (issue #124). Pulled from
	// /api/v1/users/me on boot; rendered on the second header line so
	// operators see whose token ota is using before they take any action.
	Principal string

	// Version string — e.g., "v0.1.0".
	Version string

	// Timezone label — typically "UTC".
	Timezone string

	// RateLimit classifies the [RL: ...] badge.
	RateLimit RateLimitState

	// Resource is the active screen label (e.g., "Users", "Groups",
	// "Policies › OKTA_SIGN_ON").
	Resource string

	// Counter shows the count line (e.g., "5 of 5", "loading…", "(error)").
	Counter string

	// Filter, if non-empty, is appended to the counter line as " · q=\"...\"".
	Filter string

	// Body is the active child Screen body. Caller is responsible for sizing
	// the body to (Width-2) columns; chrome only adds the surrounding border.
	Body string

	// BodyLines is the requested number of body rows. When > len(Body lines)
	// padding rows are appended so the bordered box stays a stable height.
	// 0 disables vertical padding.
	BodyLines int

	// KeyHints is the bottom row contents (without surrounding `<` `>`
	// brackets). Already-formatted by caller.
	KeyHints string

	// Offline, when true, appends an `[offline]` badge to the key hints row.
	Offline bool
}

// RenderChrome composes the global 3-zone chrome (Header / MainBody /
// StatusBar) around Body and returns the complete View string. Pure function;
// safe to call from tea.View().
func RenderChrome(in ChromeInput) string {
	width := in.Width
	if width <= 0 {
		width = ChromeWidth
	}
	// Inner area excludes left+right borders. Content rows are rendered as
	// "│ <text padded to (inner-1)>│" so a single-cell left padding sits
	// between border and text — matches TUI_DESIGN §16 golden snapshots.
	inner := width - 2
	if inner < 10 {
		inner = 10
	}
	contentWidth := inner - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	tk := in.Tokens

	// ---- Row 0: TitleBar -------------------------------------------------
	left := titleLeft(in.Brand, in.Tenant, in.Profile, tk)
	right := titleRight(in.RateLimit, in.Timezone, in.Version, tk)
	titleBar := joinLR(left, right, contentWidth)

	// ---- Row 1: ContextBar ----------------------------------------------
	resource := in.Resource
	if resource == "" {
		resource = "—"
	}
	resStyled := tk.Header.Render(resource)
	counterLine := in.Counter
	if in.Filter != "" {
		if counterLine != "" {
			counterLine = counterLine + " · q=\"" + in.Filter + "\""
		} else {
			counterLine = "q=\"" + in.Filter + "\""
		}
	}
	// Left-hand of the ContextBar is the resource label (+ optional
	// counter / active filter). The counter slot is empty for the
	// initial v0.1.x lineup — child screens render their own header
	// inside the body — but we still glue any non-empty counter on so
	// the slot is available without changing this signature.
	leftCtx := resStyled
	if counterLine != "" {
		leftCtx = resStyled + "  " + tk.Muted.Render(counterLine)
	}
	// Right-side of the ContextBar: profile + (when known) the
	// authenticated Okta principal so the operator can see whose token
	// is in flight (issue #124). Falls back to profile only when the
	// /me lookup hasn't completed yet.
	rightCtx := tk.Muted.Render("profile=" + in.Profile)
	if in.Principal != "" {
		rightCtx = tk.Muted.Render("as ") + tk.Accent.Render(in.Principal) + tk.Muted.Render("  profile="+in.Profile)
	}
	contextBar := joinLR(leftCtx, rightCtx, contentWidth)

	// ---- Body -----------------------------------------------------------
	bodyLines := splitLinesPadded(in.Body, contentWidth)
	for in.BodyLines > 0 && len(bodyLines) < in.BodyLines {
		bodyLines = append(bodyLines, padTo("", contentWidth))
	}

	// ---- KeyHints -------------------------------------------------------
	// Offline badge takes precedence over the right end of the hints line so
	// it never gets truncated when the terminal is narrow.
	hints := in.KeyHints
	keyHints := hints
	if in.Offline {
		offline := tk.Danger.Render("[offline]")
		offlineWidth := visibleWidth(offline)
		// Reserve room: trim hints to (contentWidth - offlineWidth - 2 spaces)
		room := contentWidth - offlineWidth - 2
		if room < 0 {
			room = 0
		}
		hintsTrimmed := truncateVisible(hints, room)
		keyHints = hintsTrimmed + strings.Repeat(" ", contentWidth-visibleWidth(hintsTrimmed)-offlineWidth) + offline
	}
	keyHints = padToVisible(keyHints, contentWidth, tk)

	// ---- Compose --------------------------------------------------------
	var b strings.Builder
	b.WriteString(roundedTop(width))
	b.WriteByte('\n')
	b.WriteString(contentRow(titleBar))
	b.WriteByte('\n')
	b.WriteString(contentRow(contextBar))
	b.WriteByte('\n')
	b.WriteString(dividerRow(width))
	b.WriteByte('\n')
	for _, line := range bodyLines {
		b.WriteString(contentRow(line))
		b.WriteByte('\n')
	}
	b.WriteString(dividerRow(width))
	b.WriteByte('\n')
	b.WriteString(contentRow(keyHints))
	b.WriteByte('\n')
	b.WriteString(roundedBottom(width))
	return b.String()
}

// contentRow wraps a (already padded to contentWidth) line with the left
// gutter space and the borders so the row hits exactly `width` columns.
func contentRow(line string) string {
	return "│ " + line + "│"
}

// titleLeft renders the brand · tenant · profile segment.
func titleLeft(brand, tenant, profile string, tk Tokens) string {
	if brand == "" {
		brand = "ota"
	}
	parts := []string{tk.Header.Render(brand)}
	if tenant != "" {
		parts = append(parts, tk.Muted.Render("· "+tenant))
	}
	if profile != "" {
		parts = append(parts, envBadge(profile, tk))
	}
	return strings.Join(parts, " ")
}

// envBadge styles the active profile token by environment classifier.
func envBadge(profile string, tk Tokens) string {
	switch strings.ToLower(profile) {
	case "prod", "production":
		return tk.Header.Render("· " + profile)
	case "staging", "stage":
		return tk.Warning.Render("· " + profile)
	default:
		return tk.Muted.Render("· " + profile)
	}
}

// titleRight renders the [RL: ok] · TZ · version segment.
func titleRight(rl RateLimitState, tz, version string, tk Tokens) string {
	if tz == "" {
		tz = "UTC"
	}
	if version == "" {
		version = "v0.0.0"
	}
	rlBadge := renderRLBadge(rl, tk)
	return rlBadge + "    " + tk.Muted.Render(tz+"  "+version)
}

func renderRLBadge(rl RateLimitState, tk Tokens) string {
	body := "[RL: " + rl.String() + "]"
	switch rl {
	case RateLimitWarn:
		return tk.Warning.Render(body)
	case RateLimitLimited:
		return tk.Danger.Render(body)
	case RateLimitUnknown:
		return tk.Muted.Render(body)
	default:
		return tk.Success.Render(body)
	}
}

// joinLR builds a "<left>...<right>" line padded to total visible cells.
func joinLR(left, right string, total int) string {
	lw := visibleWidth(left)
	rw := visibleWidth(right)
	gap := total - lw - rw
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// splitLinesPadded splits body into lines and pads each to inner width with
// trailing spaces (so the right border lines up).
func splitLinesPadded(body string, inner int) []string {
	if body == "" {
		return nil
	}
	raw := strings.Split(strings.TrimRight(body, "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		out = append(out, padTo(line, inner))
	}
	return out
}

// padTo pads s with trailing spaces so its visible width hits exactly width.
// Truncates if longer (rare — caller should pre-fit but we don't want to blow
// the box layout).
func padTo(s string, width int) string {
	w := visibleWidth(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	// Truncate — naive byte slice; visible-width truncation across ANSI is a
	// later optimization. Plain text bodies (no escapes) are exact.
	return s[:width]
}

// padToVisible is padTo but accepts pre-styled strings; identical behavior in
// the ASCII profile we ship and a placeholder for future width-aware logic.
func padToVisible(s string, width int, _ Tokens) string {
	return padTo(s, width)
}

// truncateVisible trims s so its visible width is <= width, ignoring ANSI
// escape sequences. Returns s unchanged when already short enough.
func truncateVisible(s string, width int) string {
	if visibleWidth(s) <= width {
		return s
	}
	if width <= 0 {
		return ""
	}
	// We want to preserve any prefix-only ANSI escapes intact for the trimmed
	// portion. Walk runes, count visible cells, drop ANSI CSI sequences whole.
	var b strings.Builder
	visible := 0
	i := 0
	for i < len(s) && visible < width {
		c := s[i]
		if c == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				if s[j] >= 0x40 && s[j] <= 0x7e {
					break
				}
				j++
			}
			if j < len(s) {
				b.WriteString(s[i : j+1])
				i = j + 1
			} else {
				i = len(s)
			}
			continue
		}
		b.WriteByte(c)
		visible++
		i++
	}
	return b.String()
}

// VisibleWidth is the exported counterpart of visibleWidth — returns
// the rendered cell count of a (possibly ANSI-styled) string. Use this
// instead of len() or hand-rolled escape skippers when measuring
// columns; the latter routinely miscount because the CSI introducer
// `[` itself sits in the 0x40-0x7e final-byte range and naive scanners
// exit escape mode prematurely.
func VisibleWidth(s string) int { return visibleWidth(s) }

// visibleWidth returns the visible cell count of s, ignoring ANSI escapes.
// We strip CSI sequences (lipgloss uses these) so width math survives styled
// segments. A full grapheme/runewidth pass would be more correct; this is
// adequate for the ASCII profile used by goldens and the runes we render.
func visibleWidth(s string) int {
	stripped := stripCSI(s)
	// rune count works since chrome uses ASCII text + box-drawing glyphs that
	// each occupy one cell.
	count := 0
	for range stripped {
		count++
	}
	return count
}

// stripCSI is a lightweight ANSI-CSI stripper for width calculation. Mirrors
// what testfx.StripANSI does at the test boundary; we reimplement a minimal
// version inline so styles.go has no test-only dependency.
func stripCSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// skip until final byte (0x40..0x7e)
			j := i + 2
			for j < len(s) {
				if s[j] >= 0x40 && s[j] <= 0x7e {
					break
				}
				j++
			}
			i = j + 1
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// roundedTop returns the top frame row "╭─...─╮" with width cells total.
func roundedTop(width int) string {
	if width < 2 {
		return "╭╮"
	}
	return "╭" + strings.Repeat("─", width-2) + "╮"
}

// roundedBottom returns the bottom frame row "╰─...─╯".
func roundedBottom(width int) string {
	if width < 2 {
		return "╰╯"
	}
	return "╰" + strings.Repeat("─", width-2) + "╯"
}

// dividerRow returns the horizontal divider "├─...─┤".
func dividerRow(width int) string {
	if width < 2 {
		return "├┤"
	}
	return "├" + strings.Repeat("─", width-2) + "┤"
}

// FormatCount renders the standard counter ("N of M") used by ContextBar.
func FormatCount(visible, total int) string {
	if total <= 0 && visible <= 0 {
		return ""
	}
	if visible == total {
		return fmt.Sprintf("%d of %d", visible, total)
	}
	return fmt.Sprintf("%d of %d", visible, total)
}
