package users

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/mask"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// DetailModel is SCR-011 User detail with tabs (Profile/Credentials/
// Timestamps/Groups/Factors/Recent Logs). The MVP renders the Profile tab
// inline; other tabs are sketched as hints below the section divider so
// operators can navigate via the App Shell-owned tab key (Tab / Shift-Tab).
type DetailModel struct {
	deps Deps
	user domain.User
	// unmasked controls per-field PII unmasking (TUI_DESIGN §7.2). Populated
	// via :unmask <field> from the App Shell.
	unmasked map[string]bool
}

// NewDetailModel constructs a DetailModel.
func NewDetailModel(deps Deps, user domain.User) DetailModel {
	return DetailModel{deps: deps, user: user, unmasked: map[string]bool{}}
}

// Init implements tea.Model.
func (m DetailModel) Init() tea.Cmd { return nil }

// Update implements tea.Model. Ctrl-c finalizes output for teatest harnesses.
func (m DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	return m, nil
}

// View renders SCR-011 (TUI_DESIGN §15.7 / §16.9). Profile tab is the active
// MVP view; other tabs surface as inactive labels in the tab bar so operators
// see the navigation surface even before each tab's content lands.
func (m DetailModel) View() string {
	u := m.user
	const keyWidth = 16
	var b strings.Builder

	// Tab bar — Profile is bold; others are muted hints.
	b.WriteString("[Profile] [ Credentials ] [ Timestamps ] [ Groups ] [ Factors ] [ Recent ]")
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 78))
	b.WriteByte('\n')

	// Profile section.
	b.WriteString(shared.KVRow("login", u.Profile.Login, keyWidth))
	b.WriteByte('\n')
	if u.Profile.Email != "" {
		b.WriteString(shared.KVRow("email", u.Profile.Email, keyWidth))
		b.WriteByte('\n')
	}
	if u.Profile.FirstName != "" {
		b.WriteString(shared.KVRow("firstName", u.Profile.FirstName, keyWidth))
		b.WriteByte('\n')
	}
	if u.Profile.LastName != "" {
		b.WriteString(shared.KVRow("lastName", u.Profile.LastName, keyWidth))
		b.WriteByte('\n')
	}
	if u.Profile.DisplayName != "" {
		b.WriteString(shared.KVRow("displayName", u.Profile.DisplayName, keyWidth))
		b.WriteByte('\n')
	}
	tk := activeTokens()
	statusCell := shared.UserStatusBadge(string(u.Status), tk).Render(tk)
	b.WriteString(shared.KVRow("status", statusCell, keyWidth))
	b.WriteByte('\n')

	if v := u.Profile.MobilePhone; v != "" {
		val := mask.Phone(v)
		hint := "<- masked · :unmask mobilePhone"
		if m.unmasked["mobilePhone"] {
			val = v + "  [M!]"
			hint = ""
		}
		row := shared.KVRow("mobilePhone", val, keyWidth)
		if hint != "" {
			row = row + "       " + hint
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}
	if v := u.Profile.SecondEmail; v != "" {
		val := mask.Email(v)
		hint := "<- masked"
		if m.unmasked["secondEmail"] {
			val = v + "  [M!]"
			hint = ""
		}
		row := shared.KVRow("secondEmail", val, keyWidth)
		if hint != "" {
			row = row + "    " + hint
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}

	// Custom fields.
	if len(u.Profile.Extras) > 0 {
		b.WriteByte('\n')
		b.WriteString(shared.SectionHeader("Custom fields", 56))
		b.WriteByte('\n')
		for k, v := range u.Profile.Extras {
			b.WriteString(shared.KVRow(k, formatExtra(v), keyWidth))
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// ToggleUnmask flips the unmask flag for a profile field id (e.g.,
// "mobilePhone" / "secondEmail"). Called by the App Shell on :unmask <field>.
func (m *DetailModel) ToggleUnmask(field string) {
	if m.unmasked == nil {
		m.unmasked = map[string]bool{}
	}
	m.unmasked[field] = !m.unmasked[field]
}

// RemaskAll clears every unmask flag (TUI_DESIGN §7.2 inactivity rule).
func (m *DetailModel) RemaskAll() { m.unmasked = map[string]bool{} }

// formatExtra renders Extras values as plain strings.
func formatExtra(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

var _ tea.Model = DetailModel{}
