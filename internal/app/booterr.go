package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/okta/errormap"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/version"
)

// BootErrorModel is the App Shell rendered during a fatal startup failure
// (missing profile, unreadable config, invalid token URL, etc.). Instead of
// printing to stderr and exiting silently, ota launches tea.Program with a
// BootErrorModel so the operator gets the same chrome-styled feedback they
// see for runtime errors (TUI_DESIGN §17 / Phase 6d-6 spec).
type BootErrorModel struct {
	// Err is the original Wire error. UserMessage(Err) supplies the rendered
	// body; Err.Error() is preserved for log/export.
	Err error
	// Hint is an optional second-line hint (e.g., "set OKTA_ORG_URL or pass
	// --profile"). Surfaced below the user message.
	Hint string
	// Profile is the profile name the wire attempt used, if any.
	Profile string
}

// NewBootErrorModel constructs a BootErrorModel.
func NewBootErrorModel(err error, hint string) BootErrorModel {
	return BootErrorModel{Err: err, Hint: hint}
}

// Init implements tea.Model. No background work — the user reads the screen
// and quits with q / Ctrl-c.
func (m BootErrorModel) Init() tea.Cmd { return nil }

// Update only handles the global quit keys; everything else is no-op so the
// chrome stays still.
func (m BootErrorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyRunes:
			if string(km.Runes) == "q" || string(km.Runes) == "Q" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// View renders the boot error inside the standard chrome so the user sees a
// familiar layout even when ota cannot reach Okta.
func (m BootErrorModel) View() string {
	tk := activeTokens()
	body := shared.ErrorPanel("ota", m.Err)
	if m.Hint != "" {
		body = body + "\n\n" + tk.Muted.Render(m.Hint)
	}
	body = body + "\n\nPress <q> or <Esc> to quit."

	return shared.RenderChrome(shared.ChromeInput{
		Tokens:    tk,
		Width:     shared.ChromeWidth,
		Brand:     "ota",
		Profile:   m.Profile,
		Version:   version.Tag,
		Timezone:  "UTC",
		RateLimit: shared.RateLimitUnknown,
		Resource:  "Startup error",
		Counter:   "(boot)",
		Body:      body,
		BodyLines: 16,
		KeyHints:  " <q> quit  <Esc> quit",
	})
}

// UserMessage exposes the rendered body for tests / logs that want the same
// string the screen shows.
func (m BootErrorModel) UserMessage() string { return errormap.UserMessage(m.Err) }

var _ tea.Model = BootErrorModel{}
