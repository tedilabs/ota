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
	entries   []helpEntry
	filter    string
	filtering bool
}

type helpEntry struct {
	key, desc string
}

// NewHelpModel constructs the default help overlay with TUI_DESIGN §3 entries.
func NewHelpModel() HelpModel {
	return HelpModel{entries: defaultHelpEntries()}
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

// View renders the help modal — a RoundedBorder box with a 2-column key
// reference (TUI_DESIGN §15.9 / §16.11). The optional `/` filter narrows the
// list in place.
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
	for _, e := range m.entries {
		if needle != "" {
			blob := strings.ToLower(e.key + " " + e.desc)
			if !strings.Contains(blob, needle) {
				continue
			}
		}
		body.WriteString(padRight(e.key, 14) + e.desc + "\n")
	}
	body.WriteString("\n<Tab> tab · </> filter · <?> close")
	return shared.Modal("Help · Users List", body.String(), 70)
}

func defaultHelpEntries() []helpEntry {
	// Order mirrors TUI_DESIGN §3 tables.
	return []helpEntry{
		{":", "open command palette"},
		{"/", "incremental search"},
		{"?", "this help"},
		{"Esc", "cancel current mode / close overlay"},
		{"q", "close screen (List → quit confirm)"},
		{"Ctrl-c", "soft quit (tail confirm)"},
		{"Ctrl-l", "force redraw"},
		{"j", "cursor down"},
		{"k", "cursor up"},
		{"gg / G", "top / bottom"},
		{"Ctrl-d/u", "half-page down / up"},
		{"Ctrl-f/b", "page down / up"},
		{"Enter", "select / open detail"},
		{"Tab", "next tab"},
		{"Shift-Tab", "previous tab"},
		{"R", "refresh (cache invalidate)"},
		{"r", "toggle raw JSON (Policies / Logs)"},
		{"y", "yank selected (copy)"},
		{"yy", "yank whole row"},
		{"yf", "yank field under cursor"},
		{"o", "open in Admin Console (web)"},
		{"e", "expand detail field"},
		{"s", "toggle tail (Logs)"},
		{"f", "toggle auto-scroll follow (Logs)"},
		{"n / N", "search next / previous"},
		{":quit", "quit ota"},
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
