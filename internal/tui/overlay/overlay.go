// Package overlay hosts the non-resource overlays: command palette, search
// prompt, help, confirm, errors log, and about/ratelimit/healthcheck
// (SCR-900..905, 910). Overlays are composable tea.Models rendered above the
// active resource screen; the App Shell owns activation state.
package overlay

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/tui/shared"
)

// --- Command palette (SCR-900) -----------------------------------------------

// paletteHints lists the commands rendered as hints below the input buffer
// (REQ-U02 AC-1).
var paletteHints = []string{
	":users", ":groups", ":grouprules", ":policies", ":logs",
	":profile", ":search", ":filter", ":unmask", ":mask", ":raw",
	":refresh", ":about", ":ratelimit", ":errors", ":healthcheck",
	":debug", ":help", ":quit",
}

// CmdPaletteModel renders the `:` prompt with in-progress input and command
// hints. Real command parsing happens in internal/app when Enter is pressed.
type CmdPaletteModel struct {
	buffer string
}

// NewCmdPaletteModel constructs an empty palette.
func NewCmdPaletteModel() CmdPaletteModel { return CmdPaletteModel{} }

// Init implements tea.Model.
func (m CmdPaletteModel) Init() tea.Cmd { return nil }

// Update handles typing. Ctrl-c closes the program (used by teatest flows).
func (m CmdPaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyCtrlC:
			return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
		case tea.KeyBackspace:
			if n := len(m.buffer); n > 0 {
				m.buffer = m.buffer[:n-1]
			}
		case tea.KeyRunes:
			m.buffer += string(km.Runes)
		}
	}
	return m, nil
}

// View renders the palette modal — a RoundedBorder box hosting the prompt
// line and a list of matching commands (TUI_DESIGN §15.8).
func (m CmdPaletteModel) View() string {
	var body strings.Builder
	body.WriteString(":" + m.buffer)
	body.WriteByte('\n')
	body.WriteString("Commands:")
	body.WriteByte('\n')
	needle := strings.TrimPrefix(m.buffer, ":")
	prefix := ":" + needle
	for _, h := range paletteHints {
		if needle == "" || strings.HasPrefix(h, prefix) {
			body.WriteString("  " + h + "\n")
		}
	}
	body.WriteString("\n<Tab> complete · <Enter> run · <Esc> cancel")
	return shared.Modal("Command Palette", body.String(), 60)
}

// Buffer returns the current command text (App Shell reads on Enter).
func (m CmdPaletteModel) Buffer() string { return m.buffer }

// --- Help (SCR-902) ----------------------------------------------------------

// HelpModel renders a three-column key reference (TUI_DESIGN §3 +
// issue #147 widescreen layout) with an optional inline `/` filter
// (REQ-U06 AC-2). Width is the available content width; the App
// Shell pipes the terminal's contentWidth in via WithWidth so the
// modal fills the screen instead of clinging to the top-left.
type HelpModel struct {
	screen    string
	entries   []helpEntry
	filter    string
	filtering bool
	width     int
}

type helpEntry struct {
	key, desc string
}

// NewHelpModel constructs the default (Users-screen) help overlay with
// TUI_DESIGN §3 entries.
func NewHelpModel() HelpModel {
	return NewHelpModelFor("users")
}

// NewHelpModelFor constructs a help overlay scoped to the named screen, so
// `?` shows only the keys that actually do something on the current view —
// e.g. `s` for the Logs tail toggle is hidden on the Users screen.
//
// Recognised screen names (matching internal/app Screen.String() output):
// "users", "groups", "rules", "policies", "logs", "user-detail",
// "group-detail", "rule-detail", "policy-detail", "log-detail". Unknown
// names fall back to the global entries only.
func NewHelpModelFor(screen string) HelpModel {
	return HelpModel{
		screen:  screen,
		entries: helpEntriesForScreen(screen),
	}
}

// WithWidth returns a copy of m with width set so the View() can
// size its 3-column layout to the chrome's available content area.
// Width = 0 means "use the auto-fit width" (legacy behaviour).
func (m HelpModel) WithWidth(w int) HelpModel {
	m.width = w
	return m
}

// Init implements tea.Model.
func (m HelpModel) Init() tea.Cmd { return nil }

// Update supports `/` internal search (REQ-U06 AC-2) and Ctrl-c to close.
func (m HelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		switch km.Type {
		case tea.KeyEnter, tea.KeyEsc:
			m.filtering = false
		case tea.KeyBackspace:
			if n := len(m.filter); n > 0 {
				m.filter = m.filter[:n-1]
			}
		case tea.KeyRunes:
			m.filter += string(km.Runes)
		case tea.KeyCtrlC:
			return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
		}
		return m, nil
	}
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyRunes:
		if string(km.Runes) == "/" {
			m.filtering = true
			m.filter = ""
		}
	}
	return m, nil
}

// View renders the help modal — a RoundedBorder box laying out the
// three k9s-style sections (Resource / General / Navigation) as
// side-by-side columns (issue #132 + #147 widescreen). When the App
// Shell hands a width via WithWidth, the modal fills that width and
// divides the body evenly between the three columns, separated by
// `│` glyphs for clear visual separation. Without a width hint the
// modal falls back to the auto-fit form so callers that don't know
// the chrome dimensions still render something sensible.
func (m HelpModel) View() string {
	header := "Press Esc to close"
	if m.filtering {
		header = "filter: /" + m.filter
	} else if m.filter != "" {
		header = "[filter=" + m.filter + "]"
	}

	needle := strings.ToLower(m.filter)
	matches := func(e helpEntry) bool {
		if needle == "" {
			return true
		}
		return strings.Contains(strings.ToLower(e.key+" "+e.desc), needle)
	}
	filtered := func(entries []helpEntry) []helpEntry {
		out := make([]helpEntry, 0, len(entries))
		for _, e := range entries {
			if matches(e) {
				out = append(out, e)
			}
		}
		return out
	}

	resource := filtered(screenSpecificHelpEntries(m.screen))
	general := filtered(generalHelpEntries())
	navigation := filtered(navigationHelpEntries())
	palette := filtered(paletteHelpEntries())

	cols := []helpColumn{
		{Title: "Resource", Entries: resource},
		{Title: "General", Entries: general},
		{Title: "Navigation", Entries: navigation},
		{Title: "Palette", Entries: palette},
	}
	colWidths, totalWidth := pickHelpColumnLayout(cols, m.width)

	var body strings.Builder
	body.WriteString(header + "\n\n")
	body.WriteString(renderHelpColumns(cols, colWidths))
	body.WriteString("\n\n<Esc> close · </> filter")

	return shared.Modal(helpTitle(m.screen), body.String(), totalWidth)
}

// pickHelpColumnLayout resolves each column's rendered width and
// the overall modal width. Target > 0 (issue #147): the modal fills
// that width and divides the body evenly across the columns. Target
// == 0: auto-fit each column to its widest cell (legacy form).
func pickHelpColumnLayout(cols []helpColumn, target int) (colWidths []int, totalWidth int) {
	const padding = 4 // box border (2) + body padding (2)
	const sepW = 3    // " │ " separator between columns
	colWidths = make([]int, len(cols))

	if target <= 0 {
		for i, c := range cols {
			colWidths[i] = helpColumnWidth(c)
		}
		totalWidth = padding
		for i, w := range colWidths {
			if i > 0 {
				totalWidth += sepW
			}
			totalWidth += w
		}
		return colWidths, totalWidth
	}

	body := target - padding - (len(cols)-1)*sepW
	if body < len(cols)*8 {
		body = len(cols) * 8
	}
	per := body / len(cols)
	for i := range cols {
		colWidths[i] = per
	}
	colWidths[len(cols)-1] += body - per*len(cols)
	totalWidth = target
	return colWidths, totalWidth
}

// helpColumnWidth picks the rendered width for one help column —
// max("── Title ──", every "key + 2 spaces + desc" entry).
func helpColumnWidth(c helpColumn) int {
	const minKey = 6
	keyW := minKey
	for _, e := range c.Entries {
		if w := len(e.key); w > keyW {
			keyW = w
		}
	}
	headerW := len("── " + c.Title + " ──")
	w := headerW
	for _, e := range c.Entries {
		row := keyW + 2 + len(e.desc)
		if row > w {
			w = row
		}
	}
	return w
}

type helpColumn struct {
	Title   string
	Entries []helpEntry
}

// renderHelpColumns lays out N columns side-by-side, separated by
// ` │ ` for clear visual separation (issue #147 — operators
// reported the previous 2-space gap looked like one wide column).
// Each column's content gets padded to colWidths[i] cells; the body
// is truncated when a single key+desc cell exceeds the column
// allocation so the separator stays put.
func renderHelpColumns(cols []helpColumn, colWidths []int) string {
	maxRows := 0
	for _, c := range cols {
		if len(c.Entries) > maxRows {
			maxRows = len(c.Entries)
		}
	}
	colKeyW := make([]int, len(cols))
	for i, c := range cols {
		const minKey = 6
		w := minKey
		for _, e := range c.Entries {
			if k := len(e.key); k > w {
				w = k
			}
		}
		// Cap key width at half the column so descriptions still fit.
		if half := colWidths[i] / 2; half > 0 && w > half {
			w = half
		}
		colKeyW[i] = w
	}

	const sep = " │ "
	var lines []string
	{
		var row strings.Builder
		for i, c := range cols {
			if i > 0 {
				row.WriteString(sep)
			}
			row.WriteString(padRight("── "+c.Title+" ──", colWidths[i]))
		}
		lines = append(lines, row.String())
	}
	for r := 0; r < maxRows; r++ {
		var row strings.Builder
		for i, c := range cols {
			if i > 0 {
				row.WriteString(sep)
			}
			cell := ""
			if r < len(c.Entries) {
				e := c.Entries[r]
				cell = padRight(e.key, colKeyW[i]) + "  " + e.desc
			}
			// Truncate cells that exceed their allocation so the
			// next column's separator stays aligned.
			if visibleLen(cell) > colWidths[i] {
				cell = truncateAscii(cell, colWidths[i])
			}
			row.WriteString(padRight(cell, colWidths[i]))
		}
		lines = append(lines, row.String())
	}
	return strings.Join(lines, "\n")
}

func visibleLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func truncateAscii(s string, width int) string {
	if visibleLen(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	out := make([]rune, 0, width)
	for _, r := range s {
		if len(out)+1 >= width {
			break
		}
		out = append(out, r)
	}
	return string(out) + "…"
}

// helpTitle returns the modal heading for a screen name. Unknown names get
// the bare "Help" label so the overlay still has a recognisable header.
func helpTitle(screen string) string {
	switch screen {
	case "users":
		return "Help · Users List"
	case "user-detail":
		return "Help · User Detail"
	case "groups":
		return "Help · Groups List"
	case "group-detail":
		return "Help · Group Detail"
	case "rules":
		return "Help · Group Rules List"
	case "rule-detail":
		return "Help · Group Rule Detail"
	case "policies":
		return "Help · Policies"
	case "policy-detail":
		return "Help · Policy Detail"
	case "logs":
		return "Help · System Logs"
	case "log-detail":
		return "Help · Log Event"
	default:
		return "Help"
	}
}

// helpEntriesForScreen returns the (Resource ∪ General ∪ Navigation)
// help rows for the named screen. Order matches the View() rendering so
// substring filters on the flat list still hit the same entries.
func helpEntriesForScreen(screen string) []helpEntry {
	out := append([]helpEntry{}, screenSpecificHelpEntries(screen)...)
	out = append(out, generalHelpEntries()...)
	out = append(out, navigationHelpEntries()...)
	return out
}

// generalHelpEntries are the app-wide commands surfaced on every screen.
// k9s slots these into a "General" section so the screen-specific (Resource)
// and motion (Navigation) keys read at a glance (issue #120).
func generalHelpEntries() []helpEntry {
	return []helpEntry{
		{":", "open command palette"},
		{"/", "incremental search (lists)"},
		{"?", "this help"},
		{"a", "resource action menu (issue #175)"},
		{"l", "open Logs scoped to current resource (#F2)"},
		{"Esc", "cancel mode / close overlay"},
		{"q", "close screen / quit (with confirm)"},
		{"Ctrl-c", "soft quit (tail confirm)"},
		{"Ctrl-l", "force redraw"},
		{"R", "refresh (cache invalidate)"},
		{":quit", "quit ota"},
	}
}

// paletteHelpEntries lists the canonical `:` palette commands so
// operators can discover routes without reading the source. Issue
// #U14 v0.2.4 — surfaced in the Help overlay's 4th column. Aliases
// (e.g. `q`, `gr`) are intentionally omitted — only canonical names
// appear here so the column stays readable.
func paletteHelpEntries() []helpEntry {
	return []helpEntry{
		{":users", "Users list"},
		{":groups", "Groups list"},
		{":group-rules", "Group Rules list"},
		{":policies", "Policies (with type picker)"},
		{":apps", "Apps (with type picker)"},
		{":authenticator", "Authenticators (factor methods)"},
		{":logs", "System Log"},
		{":saml-app", "SAML apps"},
		{":oidc-app", "OIDC apps"},
		{":okta-sign-on", "Okta Sign-On policies"},
		{":password-policy", "Password policies"},
		{":reset-password", "trigger Reset Password"},
		{":unlock", "Unlock the selected user"},
		{":reset-mfa", "Reset MFA factors"},
		{":unmask <fld>", "reveal a masked detail field"},
		{":mask", "re-mask all PII fields"},
		{":help", "open this overlay"},
		{":quit", "quit ota"},
	}
}

// navigationHelpEntries lists the motion keys — Vim cursor + page nav.
// Wired in v0.1.5-2 (Ctrl-f/b/d/u page nav).
func navigationHelpEntries() []helpEntry {
	return []helpEntry{
		{"j / k", "cursor down / up"},
		{"h / l", "scroll columns left / right"},
		{"gg / G", "top / bottom"},
		{"Ctrl-d / Ctrl-u", "half-page down / up"},
		{"Ctrl-f / Ctrl-b", "page down / up"},
	}
}

// screenSpecificHelpEntries are appended after the global ones for screens
// that bind extra keys.
func screenSpecificHelpEntries(screen string) []helpEntry {
	switch screen {
	case "users":
		return []helpEntry{
			{"Enter / d", "open detail (all attributes)"},
			{"Shift+S", "sort by STATUS"},
			{"Shift+N", "sort by NAME (login)"},
			{"Shift+L", "sort by LAST LOGIN"},
			{"Shift+C", "sort by CREATED / CHANGED"},
			{":reset-password", "send reset-password email"},
			{":unlock", "clear LOCKED_OUT state"},
			{":reset-mfa", "remove enrolled MFA factors"},
		}
	case "groups":
		return []helpEntry{
			{"Enter / d", "open detail (all attributes)"},
			{"Shift+N", "sort by NAME"},
		}
	case "rules":
		return []helpEntry{
			{"Enter / d", "open detail (all attributes)"},
			{"Shift+S", "sort by STATUS (INVALID first)"},
			{"Shift+N", "sort by NAME"},
		}
	case "policies":
		return []helpEntry{
			{"Enter", "pick type / open detail"},
			{"Esc", "back to type select / list"},
			{"r", "toggle rich ↔ raw JSON in detail"},
		}
	case "apps":
		return []helpEntry{
			{"Enter / d", "pick type / open detail"},
			{"Esc", "back to type select / list"},
			{"/", "filter apps by label / name / mode"},
			{"Tab / Shift-Tab", "cycle Pretty / JSON / YAML"},
			{"r", "toggle to / from JSON tab"},
			{"j / k · g / G", "(detail) move body cursor / top / bottom"},
			{"v / V · y", "(detail) visual select · yank to clipboard"},
			{":saml-app", "jump straight to SAML 2.0 list"},
			{":oidc-app", "jump straight to OpenID Connect list"},
			{":bookmark-app", "jump straight to Bookmark list"},
			{":swa-app", "jump straight to SWA / Auto-login list"},
		}
	case "logs":
		return []helpEntry{
			{"Enter / d", "open log event detail"},
			{"/", "filter loaded events by substring (client-side)"},
			{"Q", "search Okta server (q=… history fetch)"},
			{"F", "edit server filter (filter=… expression)"},
			{"k from row 0", "land on the load-older sentinel"},
			{"Enter on sentinel", "fetch older page (after=cursor)"},
			{"s", "toggle tail mode (on/off)"},
			{"f", "toggle auto-follow (live ↔ paused)"},
			{"r", "refresh — refetch the current window"},
			{"j / k · g / G", "(detail) move body cursor / top / bottom"},
			{"v / V · y", "(detail) visual select · yank to clipboard"},
			{"0", "range: last 30m (default)"},
			{"1", "range: last 1h"},
			{"3", "range: last 3h"},
			{"c", "range: last 12h"},
			{"e", "range: last 24h"},
		}
	case "authenticators":
		return []helpEntry{
			{"Enter / d", "open authenticator detail"},
			{"Tab / Shift-Tab", "cycle Pretty / JSON / YAML"},
			{"r", "toggle to / from JSON tab"},
			{"j / k · g / G", "(detail) move body cursor / top / bottom"},
			{"v / V · y", "(detail) visual select · yank to clipboard"},
		}
	case "user-detail":
		return []helpEntry{
			{"Tab / Shift-Tab", "cycle detail tabs"},
			{"r", "jump to / from [Raw] tab"},
			{"]", "enter Groups+Apps boxes (then j/k flows across both)"},
			{"[", "(in boxes) jump to first row"},
			{"j / k · g / G", "move body cursor / top / bottom (out of boxes)"},
			{"v / V", "toggle visual line selection"},
			{"y", "yank cursor line / visual range"},
			{"Enter", "open Group / App detail (cursor in boxes)"},
			{"Esc", "cancel visual · exit boxes · close detail"},
		}
	case "rule-detail":
		return []helpEntry{
			{"Tab / Shift-Tab", "cycle detail tabs"},
			{"r", "jump to / from [Raw] tab"},
			{"j / k · g / G", "move body cursor / top / bottom"},
			{"Ctrl-d / Ctrl-u", "half-page down / up (body)"},
			{"Ctrl-f / Ctrl-b", "page down / up (body)"},
			{"v / V", "toggle visual line selection"},
			{"y", "yank cursor line / visual range"},
			{"]", "focus TARGETS for drill-down"},
			{"Esc", "cancel visual · then back to list"},
		}
	case "group-detail":
		return []helpEntry{
			{"Tab / Shift-Tab", "cycle detail tabs"},
			{"r", "jump to / from [Raw] tab"},
			{"m", "Members tab (lazy load)"},
			{"j / k · g / G", "move body cursor / top / bottom"},
			{"Ctrl-d / Ctrl-u", "half-page down / up (body)"},
			{"Ctrl-f / Ctrl-b", "page down / up (body)"},
			{"v / V", "toggle visual line selection"},
			{"y", "yank cursor line / visual range"},
			{"]", "focus Members+Apps boxes"},
			{"Esc", "cancel visual · then back to list"},
		}
	case "policy-detail":
		return []helpEntry{
			{"r", "toggle rich ↔ raw JSON"},
			{"j / k · g / G", "move body cursor / top / bottom"},
			{"Ctrl-d / Ctrl-u", "half-page down / up (body)"},
			{"Ctrl-f / Ctrl-b", "page down / up (body)"},
			{"v / V", "toggle visual line selection"},
			{"y", "yank cursor line / visual range"},
			{"Esc", "cancel visual · then back to list"},
		}
	case "log-detail":
		return []helpEntry{
			{"Tab / Shift-Tab", "cycle detail tabs"},
			{"r", "jump to / from JSON tab"},
			{"j / k · g / G", "move body cursor / top / bottom"},
			{"Ctrl-d / Ctrl-u", "half-page down / up (body)"},
			{"Ctrl-f / Ctrl-b", "page down / up (body)"},
			{"v / V", "toggle visual line selection"},
			{"y", "yank cursor line / visual range"},
			{"Esc", "cancel visual · then back to list"},
		}
	default:
		return nil
	}
}

// padRight pads s with trailing spaces to a visible width of n.
// Uses rune count rather than byte count so multibyte glyphs (e.g.
// the "──" header rules) don't under-pad and skew the column
// separators in the help modal.
func padRight(s string, n int) string {
	w := visibleLen(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// --- Action Menu (issue #175) ------------------------------------------------

// ActionMenuItem aliases shared.ActionItem so callers that already
// depend on internal/tui/overlay don't need to also import shared.
// The concrete type lives in shared so the users / groups / apps
// packages can publish actions without an overlay import cycle.
type ActionMenuItem = shared.ActionItem

// ActionMenuModel renders the resource-specific action picker bound
// to `a` from any list / detail screen. The screen supplies the
// items via app.Actioner; the modal just owns cursor + filter.
type ActionMenuModel struct {
	title  string
	items  []ActionMenuItem
	cursor int
}

// NewActionMenuModel constructs a menu around `title` and `items`.
// Falls back to a single "(no actions)" entry when items is empty so
// the modal never renders blank.
func NewActionMenuModel(title string, items []ActionMenuItem) ActionMenuModel {
	if len(items) == 0 {
		items = []ActionMenuItem{{ID: "", Label: "(no actions for this screen)"}}
	}
	return ActionMenuModel{title: title, items: items}
}

// Init implements tea.Model.
func (m ActionMenuModel) Init() tea.Cmd { return nil }

// Update advances the cursor on j/k/up/down and quits Visual/etc on
// Ctrl-c (used by teatest harnesses). Enter / Esc are handled by the
// App Shell so the picked item routes to the right action; this
// model just tracks selection state.
func (m ActionMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyDown:
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

// View renders the picker — a centered RoundedBorder box with one
// line per item. Hint text trails each label in the muted token so
// destructive actions read at a glance. v0.2.0: routed through
// shared.MountModal so the title / body / footer slots match every
// other overlay (palette / help / confirm) without each owning its
// own rounded-border layout.
func (m ActionMenuModel) View() string {
	tk := shared.Dark()
	var b strings.Builder
	for i, it := range m.items {
		prefix := "  "
		line := it.Label
		if i == m.cursor {
			prefix = "▸ "
			line = tk.Accent.Render(line)
		}
		b.WriteString(prefix + line)
		if it.Hint != "" {
			b.WriteString("  " + tk.Muted.Render(it.Hint))
		}
		if i < len(m.items)-1 {
			b.WriteByte('\n')
		}
	}
	return shared.MountModal(shared.ModalIn{
		Title:  "Actions · " + m.title,
		Body:   b.String(),
		Footer: "<j/k> nav · <Enter> run · <Esc> cancel",
		Tone:   shared.ModalToneAccent,
		Width:  60,
		Tokens: tk,
	})
}

// Cursor exposes the current cursor index so the App Shell can
// resolve the selected item on Enter.
func (m ActionMenuModel) Cursor() int { return m.cursor }

// Items exposes the menu items so the App Shell can read the
// selected payload when Enter fires.
func (m ActionMenuModel) Items() []ActionMenuItem { return m.items }

// Selected returns the currently-highlighted item, or zero value if
// the menu is empty.
func (m ActionMenuModel) Selected() (ActionMenuItem, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return ActionMenuItem{}, false
	}
	return m.items[m.cursor], true
}

// --- Confirm (SCR-903) -------------------------------------------------------

// ConfirmModel is a simple y/N dialog.
type ConfirmModel struct{ Prompt string }

// NewConfirmModel constructs a ConfirmModel with the given prompt.
func NewConfirmModel(prompt string) ConfirmModel { return ConfirmModel{Prompt: prompt} }

// Init implements tea.Model.
func (m ConfirmModel) Init() tea.Cmd { return nil }

// Update implements tea.Model. Ctrl-c exits the overlay (teatest uses this to
// finalize output); y/n are handled by the App Shell.
func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	return m, nil
}

// View renders the confirm modal — a RoundedBorder box centered on the
// supplied prompt with a y/N cue (TUI_DESIGN §15.10 / §16.12). Includes both
// "[y/N]" and "y/n" fragments so operators never miss either form.
func (m ConfirmModel) View() string {
	body := m.Prompt + "\n\nThis action cannot be undone.\n\n[y/N] (y/n)\n<Esc> cancel"
	return shared.Modal("Confirm", body, 60)
}

// --- Search prompt (SCR-901) -------------------------------------------------

// SearchModel is the overlay counterpart of the inline `/` filter for lists
// that need a standalone prompt (e.g., logs history search).
type SearchModel struct {
	buffer string
}

// NewSearchModel constructs an empty SearchModel.
func NewSearchModel() SearchModel { return SearchModel{} }

// Init implements tea.Model.
func (m SearchModel) Init() tea.Cmd { return nil }

// Update captures typed input. Ctrl-c closes the program.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyCtrlC:
			return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
		case tea.KeyBackspace:
			if n := len(m.buffer); n > 0 {
				m.buffer = m.buffer[:n-1]
			}
		case tea.KeyRunes:
			m.buffer += string(km.Runes)
		}
	}
	return m, nil
}

// View renders the prompt line.
func (m SearchModel) View() string { return "/" + m.buffer }

// Buffer returns the current search text.
func (m SearchModel) Buffer() string { return m.buffer }

// --- About (SCR-905) ---------------------------------------------------------

// AboutModel shows app metadata, rate-limit snapshots, and healthcheck info.
type AboutModel struct {
	Version      string
	Commit       string
	BuildTime    string
	Profile      string
	TokenSource  string
	RateLimitSum string
}

// NewAboutModel constructs an AboutModel with the given values.
func NewAboutModel(opts AboutModel) AboutModel { return opts }

// Init implements tea.Model.
func (m AboutModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m AboutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	return m, nil
}

// View renders app metadata.
func (m AboutModel) View() string {
	var b strings.Builder
	b.WriteString("ota ")
	b.WriteString(m.Version)
	b.WriteString("\n")
	if m.Commit != "" {
		b.WriteString("  commit: ")
		b.WriteString(m.Commit)
		b.WriteString("\n")
	}
	if m.BuildTime != "" {
		b.WriteString("  built:  ")
		b.WriteString(m.BuildTime)
		b.WriteString("\n")
	}
	if m.Profile != "" {
		b.WriteString("  profile: ")
		b.WriteString(m.Profile)
		b.WriteString("\n")
	}
	if m.TokenSource != "" {
		b.WriteString("  token:   ")
		b.WriteString(m.TokenSource)
		b.WriteString("\n")
	}
	if m.RateLimitSum != "" {
		b.WriteString("  rate:    ")
		b.WriteString(m.RateLimitSum)
		b.WriteString("\n")
	}
	return b.String()
}

var (
	_ tea.Model = CmdPaletteModel{}
	_ tea.Model = HelpModel{}
	_ tea.Model = ConfirmModel{}
	_ tea.Model = SearchModel{}
	_ tea.Model = AboutModel{}
)
