package users

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/mask"
)

// FactorsTabModel renders the Factors tab of the User detail view (REQ-R01 AC-6).
// PII fields (SMS/Voice phoneNumber, Email factor email) are masked by default
// per PRD §6.2 / TUI_DESIGN §7.2; `:unmask` toggles per-factor visibility.
type FactorsTabModel struct {
	factors []domain.Factor
	// unmasked tracks per-factor (by id) unmask state (session-local,
	// TUI_DESIGN §7.2). Writes go through ToggleUnmask / RemaskAll.
	unmasked map[string]bool
}

// NewFactorsTabModel constructs a FactorsTabModel.
func NewFactorsTabModel(factors []domain.Factor) FactorsTabModel {
	return FactorsTabModel{factors: factors, unmasked: map[string]bool{}}
}

// Init implements tea.Model.
func (m FactorsTabModel) Init() tea.Cmd { return nil }

// Update currently ignores input; the App Shell routes `:unmask <id>` into
// ToggleUnmask via a palette command. Ctrl-c ends the program (teatest flow).
func (m FactorsTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	return m, nil
}

// View renders the factors table with type labels, status, and masked PII.
// REQ-R01 AC-6 specifies human-friendly factor type labels and default-masked
// phone numbers / emails.
func (m FactorsTabModel) View() string {
	if len(m.factors) == 0 {
		return "Factors\n  (none registered)\n"
	}
	var b strings.Builder
	b.WriteString("Factors\n")
	for _, f := range m.factors {
		b.WriteString("  ")
		b.WriteString(factorTypeLabel(f.Type))
		b.WriteString("  ")
		b.WriteString(string(f.Status))
		if detail := factorDetail(f, m.unmasked[f.ID]); detail != "" {
			b.WriteString("  ")
			b.WriteString(detail)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ToggleUnmask flips the unmask flag for a single factor id. Called by the
// App Shell when `:unmask <factor-id>` is dispatched.
func (m *FactorsTabModel) ToggleUnmask(factorID string) {
	if m.unmasked == nil {
		m.unmasked = map[string]bool{}
	}
	m.unmasked[factorID] = !m.unmasked[factorID]
}

// RemaskAll clears every unmask flag (triggered on screen navigation / 60s
// inactivity / `:mask` per TUI_DESIGN §7.2).
func (m *FactorsTabModel) RemaskAll() { m.unmasked = map[string]bool{} }

// factorTypeLabel returns the human-friendly label per REQ-R01 AC-6.
func factorTypeLabel(t domain.FactorType) string {
	switch t {
	case domain.FactorTypePush:
		return "Okta Verify (Push)"
	case domain.FactorTypeTOTP:
		return "TOTP"
	case domain.FactorTypeSMS:
		return "SMS"
	case domain.FactorTypeCall:
		return "Voice Call"
	case domain.FactorTypeEmail:
		return "Email"
	case domain.FactorTypeWebAuthn:
		return "WebAuthn (Security Key)"
	case domain.FactorTypeHardwareToken:
		return "Hardware Token"
	case domain.FactorTypeQuestion:
		return "Security Question"
	}
	return string(t)
}

// factorDetail returns the per-type PII-bearing field, masked by default.
// When `unmasked` is true the raw value is shown with an `[M!]` warning marker.
func factorDetail(f domain.Factor, unmasked bool) string {
	switch f.Type {
	case domain.FactorTypeSMS, domain.FactorTypeCall:
		if f.Profile.PhoneNumber == "" {
			return ""
		}
		if unmasked {
			return f.Profile.PhoneNumber + " [M!]"
		}
		return mask.Phone(f.Profile.PhoneNumber)
	case domain.FactorTypeEmail:
		if f.Profile.Email == "" {
			return ""
		}
		if unmasked {
			return f.Profile.Email + " [M!]"
		}
		return mask.Email(f.Profile.Email)
	case domain.FactorTypeWebAuthn:
		return f.Profile.CredentialID
	case domain.FactorTypePush:
		if f.Profile.Name != "" {
			return f.Profile.DeviceType + " — " + f.Profile.Name
		}
		return f.Profile.DeviceType
	}
	return ""
}

var _ tea.Model = FactorsTabModel{}
