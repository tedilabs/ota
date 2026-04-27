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

	// Counter shows a legacy free-form count line (unused as of issue
	// #136 — superseded by CountVisible / CountTotal which the chrome
	// stamps into the upper divider next to the resource label).
	Counter string

	// CountVisible / CountTotal feed the "N of M" segment in the
	// upper divider. CountTotal == 0 disables the segment entirely
	// (detail surfaces, screens without a count).
	CountVisible int
	CountTotal   int
	HasCount     bool

	// Filter, if non-empty, gets appended to the divider label as
	// ` · q="..."` so the operator always sees what's narrowing the
	// visible row set.
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
//
// Header (k9s-style):
//
//	╭────────────────────────────────────────────────────────╮
//	│ ota v0.1.6  acme.okta.com  admin@acme.com  [prod]   [RL: ok]  UTC │
//	├─ Users · q="alice" ────────────────────────────────────┤
//	│ <body>                                                 │
//	├────────────────────────────────────────────────────────┤
//	│ <key hints>                                            │
//	╰────────────────────────────────────────────────────────╯
//
// The resource label sits inside the upper divider — k9s uses the same
// trick to keep the title visible without spending an extra row. The
// older 2-row TitleBar/ContextBar combination duplicated env info and
// burned a content row that the body needed for data.
func RenderChrome(in ChromeInput) string {
	width := in.Width
	if width <= 0 {
		width = ChromeWidth
	}
	inner := width - 2
	if inner < 10 {
		inner = 10
	}
	contentWidth := inner - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	tk := in.Tokens

	// ---- TitleBar -------------------------------------------------------
	left := titleLeftK9s(in.Brand, in.Version, in.Tenant, in.Principal, in.Profile, tk)
	right := titleRight(in.RateLimit, in.Timezone, "", tk) // version moves to left group
	titleBar := joinLR(left, right, contentWidth)

	// ---- Upper divider with embedded resource label --------------------
	resourceLabel := buildResourceLabel(in.Resource, in.Filter, in.CountVisible, in.CountTotal, in.HasCount, tk)
	upperDivider := dividerWithLabel(width, resourceLabel)

	// ---- Body -----------------------------------------------------------
	bodyLines := splitLinesPadded(in.Body, contentWidth)
	for in.BodyLines > 0 && len(bodyLines) < in.BodyLines {
		bodyLines = append(bodyLines, padTo("", contentWidth))
	}

	// ---- KeyHints -------------------------------------------------------
	hints := in.KeyHints
	keyHints := hints
	if in.Offline {
		offline := tk.Danger.Render("[offline]")
		offlineWidth := visibleWidth(offline)
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
	b.WriteString(upperDivider)
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

// titleLeftK9s renders the grouped left-hand context segment:
//
//	ota v0.1.6  ·  acme.okta.com  ·  admin@acme.com  [prod]
//
// brand+version sit together (the program identity); tenant + principal
// answer "where am I, as whom"; the env badge tags the profile.
// Principal collapses cleanly when the /me probe hasn't returned yet.
func titleLeftK9s(brand, version, tenant, principal, profile string, tk Tokens) string {
	if brand == "" {
		brand = "ota"
	}
	parts := []string{}
	if version != "" {
		parts = append(parts, tk.Header.Render(brand+" "+version))
	} else {
		parts = append(parts, tk.Header.Render(brand))
	}
	if tenant != "" {
		parts = append(parts, tk.Muted.Render("·")+" "+tk.Muted.Render(tenant))
	}
	if principal != "" {
		parts = append(parts, tk.Muted.Render("·")+" "+tk.Accent.Render(principal))
	}
	if profile != "" {
		parts = append(parts, envBadgeBracketed(profile, tk))
	}
	return strings.Join(parts, " ")
}

// envBadgeBracketed wraps the profile in brackets and color-codes by
// environment classifier. `[prod]` reads as a tag rather than a path
// segment.
func envBadgeBracketed(profile string, tk Tokens) string {
	body := "[" + profile + "]"
	switch strings.ToLower(profile) {
	case "prod", "production":
		return tk.Header.Render(body)
	case "staging", "stage":
		return tk.Warning.Render(body)
	default:
		return tk.Muted.Render(body)
	}
}

// buildResourceLabel assembles the text that gets stamped into the
// upper divider. Composes (resource, count, filter) into one styled
// string ready for dividerWithLabel:
//
//	Users
//	Users · 81 of 81
//	Users · 3 of 81 · q="alice"
//
// Each segment is added only when present so detail surfaces (no
// count, no filter) render as just the resource name.
func buildResourceLabel(resource, filter string, visible, total int, hasCount bool, tk Tokens) string {
	if resource == "" {
		resource = "—"
	}
	label := tk.Header.Render(resource)
	if hasCount && total > 0 {
		count := itoaCount(visible) + " of " + itoaCount(total)
		label = label + tk.Muted.Render(" · ") + tk.Muted.Render(count)
	}
	if filter != "" {
		label = label + tk.Muted.Render(` · q="`+filter+`"`)
	}
	return label
}

// itoaCount is a tiny strconv shim local to chrome — keeps strconv
// out of this file's import set.
func itoaCount(n int) string {
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

// dividerWithLabel returns `├─ <label> ──────────┤` of total `width`
// cells. The label sits two cells from the left border (k9s keeps the
// title left-anchored so the eye lands on it consistently). When the
// label is wider than the divider can hold, it gets truncated to fit
// rather than overflowing the box.
//
// Layout: ├ + ─ + ' ' + label + ' ' + tail*─ + ┤  →  5 fixed cells.
func dividerWithLabel(width int, label string) string {
	if width < 6 {
		return dividerRow(width)
	}
	const fixedFrame = 5
	available := width - fixedFrame
	labelW := visibleWidth(label)
	if labelW > available {
		label = truncateVisible(label, available)
		labelW = visibleWidth(label)
	}
	tail := width - fixedFrame - labelW
	if tail < 0 {
		tail = 0
	}
	return "├─ " + label + " " + strings.Repeat("─", tail) + "┤"
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

// titleRight renders the right-hand status segment: rate-limit badge
// and timezone. The version label moved into titleLeft (k9s groups
// program identity together) so the right side is now just live
// runtime state.
func titleRight(rl RateLimitState, tz, _ string, tk Tokens) string {
	if tz == "" {
		tz = "UTC"
	}
	return renderRLBadge(rl, tk) + "    " + tk.Muted.Render(tz)
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
