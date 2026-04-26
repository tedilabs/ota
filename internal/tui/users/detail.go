package users

import (
	"encoding/json"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/mask"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// DetailTab indexes the visible tab in DetailModel. v0.1.2 collapsed the
// six placeholder tabs (Profile / Credentials / Timestamps / Groups /
// Factors / Recent) into the three structural views the user actually
// asked for: a curated key-value Pretty view plus full JSON / YAML
// dumps of the same domain object. Operators can switch with
// Tab / Shift-Tab; `r` toggles to JSON and back.
//
// DetailTabRaw is kept as an alias for DetailTabJSON so v0.1.1 callers
// using `WithActiveTab(DetailTabRaw)` continue to compile and behave the
// same way.
type DetailTab int

const (
	DetailTabPretty DetailTab = iota
	DetailTabJSON
	DetailTabYAML
)

// DetailTabRaw is the legacy alias for DetailTabJSON. v0.1.2+ code should
// reference DetailTabJSON directly.
const (
	DetailTabProfile = DetailTabPretty
	DetailTabRaw     = DetailTabJSON
)

// detailTabLabels lists the label rendered for each DetailTab in the tab
// bar. Index aligns with the DetailTab iota.
var detailTabLabels = []string{
	"Pretty",
	"JSON",
	"YAML",
}

// detailTabCount is the number of detail tabs (used by Tab/Shift-Tab cycling).
var detailTabCount = DetailTab(len(detailTabLabels))

// DetailModel is SCR-011 User detail with tabs (Profile/Credentials/
// Timestamps/Groups/Factors/Recent/Raw — TUI_DESIGN §15.7 v1.2.0). The
// active tab is held by the model; ListModel.opened mode owns the
// instance lifecycle and forwards Tab/Shift-Tab/r to advance it.
type DetailModel struct {
	deps      Deps
	user      domain.User
	activeTab DetailTab
	// rawReturnTab is the tab `r` jumped away from when activating Raw.
	// A second press of `r` returns to it (TUI_DESIGN §3.6 + team-lead
	// decision: r is a Raw-toggle, not a unidirectional jump).
	rawReturnTab DetailTab
	// unmasked controls per-field PII unmasking (TUI_DESIGN §7.2). Populated
	// via :unmask <field> from the App Shell.
	unmasked map[string]bool
}

// NewDetailModel constructs a DetailModel.
func NewDetailModel(deps Deps, user domain.User) DetailModel {
	return DetailModel{deps: deps, user: user, unmasked: map[string]bool{}}
}

// WithActiveTab returns a copy of the model rendered against the supplied
// tab. Used by ListModel.opened branch to keep tab state on the list side
// without making DetailModel a long-lived pointer.
func (m DetailModel) WithActiveTab(t DetailTab) DetailModel {
	m.activeTab = t
	return m
}

// ActiveTab reports the current tab.
func (m DetailModel) ActiveTab() DetailTab { return m.activeTab }

// Init implements tea.Model.
func (m DetailModel) Init() tea.Cmd { return nil }

// Update implements tea.Model. Ctrl-c finalizes output for teatest harnesses.
func (m DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	return m, nil
}

// View renders SCR-011 (TUI_DESIGN §15.7 / §16.9). The tab bar is always
// rendered; the body switches on m.activeTab. Profile is the curated v0.1.0
// view; Raw is the new v0.1.1 full-attribute JSON dump (§15.7 v1.2.0).
func (m DetailModel) View() string {
	var b strings.Builder
	b.WriteString("User Detail\n")
	b.WriteString(renderTabBar(m.activeTab))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 78))
	b.WriteByte('\n')

	switch m.activeTab {
	case DetailTabJSON:
		b.WriteString(m.renderJSONTab())
	case DetailTabYAML:
		b.WriteString(m.renderYAMLTab())
	default:
		// DetailTabPretty: curated key-value rendering, the v0.1.0 surface.
		b.WriteString(m.renderProfileTab())
	}
	return b.String()
}

// renderJSONTab is the v0.1.2 successor to renderRawTab; the implementation
// is unchanged but the name now matches the visible "JSON" tab label.
func (m DetailModel) renderJSONTab() string { return m.renderRawTab() }

// renderYAMLTab marshals the same userJSONShape as the JSON tab through
// gopkg.in/yaml.v3 so operators get a syntactically-correct YAML view.
// PII tokens carry through untouched (the value is masked at the shape
// step) and we keep the # masked annotations from annotateMaskedLines.
func (m DetailModel) renderYAMLTab() string {
	body, err := rawYAMLForUser(m.user, m.unmasked)
	if err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	return annotateMaskedLines(body) + "\n"
}

// renderTabBar lays out the §15.7 v1.2.0 tab labels with the active one
// surrounded by `[…]` and the rest by `[ … ]`.
func renderTabBar(active DetailTab) string {
	var parts []string
	for i, label := range detailTabLabels {
		if DetailTab(i) == active {
			parts = append(parts, "["+label+"]")
		} else {
			parts = append(parts, "[ "+label+" ]")
		}
	}
	return strings.Join(parts, " ")
}

// renderProfileTab is the v0.1.0 Profile body, kept verbatim.
func (m DetailModel) renderProfileTab() string {
	u := m.user
	const keyWidth = 16
	var b strings.Builder

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

// renderRawTab returns the §15.7 v1.2.0 Raw JSON tab content. The user
// struct is masked through a JSON-aware copy (rawJSONForUser) so PII
// fields render as their masked tokens; lines whose value contains a
// mask token get a trailing `# masked` muted comment.
func (m DetailModel) renderRawTab() string {
	body, err := rawJSONForUser(m.user, m.unmasked)
	if err != nil {
		return "(raw render error: " + err.Error() + ")\n"
	}
	return annotateMaskedLines(body) + "\n"
}

// rawJSONForUser builds a sanitised copy of u with PII fields swapped for
// their masked tokens (or left plain when the operator has unmasked them
// for this session) and returns the json.MarshalIndent output.
func rawJSONForUser(u domain.User, unmasked map[string]bool) (string, error) {
	buf, err := json.MarshalIndent(userJSONShapeFor(u, unmasked), "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// rawYAMLForUser is the YAML counterpart to rawJSONForUser; it shares the
// projection so the two views stay in lockstep.
func rawYAMLForUser(u domain.User, unmasked map[string]bool) (string, error) {
	buf, err := yaml.Marshal(userJSONShapeFor(u, unmasked))
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(buf), "\n"), nil
}

// userJSONShapeFor centralises the PII-aware projection used by both the
// JSON and YAML tabs.
func userJSONShapeFor(u domain.User, unmasked map[string]bool) userJSONShape {
	return userJSONShape{
		ID:              u.ID,
		Status:          string(u.Status),
		Profile:         userProfileJSON(u.Profile, unmasked),
		Credentials:     userCredentialsJSON(u.Credentials),
		Created:         formatJSONTime(u.Created),
		Activated:       formatJSONTimePtr(u.Activated),
		StatusChanged:   formatJSONTimePtr(u.StatusChanged),
		LastLogin:       formatJSONTimePtr(u.LastLogin),
		LastUpdated:     formatJSONTime(u.LastUpdated),
		PasswordChanged: formatJSONTimePtr(u.PasswordChanged),
	}
}

// userJSONShape is the deterministic projection used by Raw tab serialisation.
// We keep field order explicit (rather than reflecting domain.User directly)
// so the golden file is stable across Go map-iteration changes.
type userJSONShape struct {
	ID              string           `json:"id" yaml:"id"`
	Status          string           `json:"status" yaml:"status"`
	Profile         userProfileShape `json:"profile" yaml:"profile"`
	Credentials     userCredsShape   `json:"credentials" yaml:"credentials"`
	Created         string           `json:"created,omitempty" yaml:"created,omitempty"`
	Activated       string           `json:"activated,omitempty" yaml:"activated,omitempty"`
	StatusChanged   string           `json:"statusChanged,omitempty" yaml:"statusChanged,omitempty"`
	LastLogin       string           `json:"lastLogin,omitempty" yaml:"lastLogin,omitempty"`
	LastUpdated     string           `json:"lastUpdated,omitempty" yaml:"lastUpdated,omitempty"`
	PasswordChanged string           `json:"passwordChanged,omitempty" yaml:"passwordChanged,omitempty"`
}

type userProfileShape struct {
	Login       string         `json:"login" yaml:"login"`
	Email       string         `json:"email,omitempty" yaml:"email,omitempty"`
	FirstName   string         `json:"firstName,omitempty" yaml:"firstName,omitempty"`
	LastName    string         `json:"lastName,omitempty" yaml:"lastName,omitempty"`
	DisplayName string         `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	MobilePhone string         `json:"mobilePhone,omitempty" yaml:"mobilePhone,omitempty"`
	SecondEmail string         `json:"secondEmail,omitempty" yaml:"secondEmail,omitempty"`
	Department  string         `json:"department,omitempty" yaml:"department,omitempty"`
	Extras      map[string]any `json:"extras,omitempty" yaml:"extras,omitempty"`
}

type userCredsShape struct {
	Provider     string `json:"provider,omitempty" yaml:"provider,omitempty"`
	ProviderType string `json:"providerType,omitempty" yaml:"providerType,omitempty"`
}

func userProfileJSON(p domain.UserProfile, unmasked map[string]bool) userProfileShape {
	out := userProfileShape{
		Login:       p.Login,
		Email:       p.Email,
		FirstName:   p.FirstName,
		LastName:    p.LastName,
		DisplayName: p.DisplayName,
		Department:  p.Department,
		Extras:      p.Extras,
	}
	if p.MobilePhone != "" {
		if unmasked["mobilePhone"] {
			out.MobilePhone = p.MobilePhone
		} else {
			out.MobilePhone = mask.Phone(p.MobilePhone)
		}
	}
	if p.SecondEmail != "" {
		if unmasked["secondEmail"] {
			out.SecondEmail = p.SecondEmail
		} else {
			out.SecondEmail = mask.Email(p.SecondEmail)
		}
	}
	return out
}

func userCredentialsJSON(c domain.UserCredentials) userCredsShape {
	return userCredsShape{Provider: c.Provider, ProviderType: c.ProviderType}
}

// formatJSONTime returns t in RFC 3339 (Okta's wire format) or "" so the
// `omitempty` tag drops zero-valued fields from the marshal output.
func formatJSONTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatJSONTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// annotateMaskedLines walks JSON output and appends ` # masked` after any
// line whose value contains the mask token "***" (mask.Phone / mask.Email
// guarantee that token is present). Comment is intentionally plain text —
// JSON parsers will reject it, but the Raw tab is for human reading only.
func annotateMaskedLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "***") {
			lines[i] = ln + " # masked"
		}
	}
	return strings.Join(lines, "\n")
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
