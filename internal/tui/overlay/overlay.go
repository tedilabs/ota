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

// HelpModel renders a two-column key reference (TUI_DESIGN §3) with an
// optional inline `/` filter (REQ-U06 AC-2).
type HelpModel struct {
	screen    string
	entries   []helpEntry
	filter    string
	filtering bool
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

// View renders the help modal — a RoundedBorder box grouped into the
// k9s-style sections Resource / General / Navigation (issue #120). The
// optional `/` filter narrows the list in place; section headers are
// dropped when their group has no matching rows.
func (m HelpModel) View() string {
	var body strings.Builder
	header := "Press Esc to close"
	if m.filtering {
		header = "filter: /" + m.filter
	} else if m.filter != "" {
		header = "[filter=" + m.filter + "]"
	}
	body.WriteString(header + "\n\n")

	needle := strings.ToLower(m.filter)
	matches := func(e helpEntry) bool {
		if needle == "" {
			return true
		}
		return strings.Contains(strings.ToLower(e.key+" "+e.desc), needle)
	}
	writeGroup := func(title string, entries []helpEntry, first *bool) {
		var rows []helpEntry
		for _, e := range entries {
			if matches(e) {
				rows = append(rows, e)
			}
		}
		if len(rows) == 0 {
			return
		}
		if !*first {
			body.WriteByte('\n')
		}
		body.WriteString("── " + title + " ──\n")
		for _, e := range rows {
			body.WriteString(padRight(e.key, 16) + e.desc + "\n")
		}
		*first = false
	}

	first := true
	writeGroup("Resource", screenSpecificHelpEntries(m.screen), &first)
	writeGroup("General", generalHelpEntries(), &first)
	writeGroup("Navigation", navigationHelpEntries(), &first)

	body.WriteString("\n<Esc> close · </> filter")
	return shared.Modal(helpTitle(m.screen), body.String(), 70)
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
		{"Esc", "cancel mode / close overlay"},
		{"q", "close screen / quit (with confirm)"},
		{"Ctrl-c", "soft quit (tail confirm)"},
		{"Ctrl-l", "force redraw"},
		{"R", "refresh (cache invalidate)"},
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
			{"Enter", "drill into policy type"},
			{"r", "toggle rich ↔ raw JSON"},
		}
	case "logs":
		return []helpEntry{
			{"Enter", "open log event detail"},
			{"s", "toggle tail mode"},
			{"f", "toggle auto-follow"},
			{"r", "toggle rich ↔ raw JSON"},
		}
	case "user-detail", "group-detail", "rule-detail":
		return []helpEntry{
			{"Tab / Shift-Tab", "cycle detail tabs"},
			{"r", "jump to / from [Raw] tab"},
			{"Esc", "back to list"},
		}
	case "policy-detail", "log-detail":
		return []helpEntry{
			{"r", "toggle rich ↔ raw JSON"},
			{"Esc", "back to list"},
		}
	default:
		return nil
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
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
