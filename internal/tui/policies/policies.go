// Package policies implements the Policies type-select, list, and detail
// Screen Models (SCR-040, SCR-041, SCR-042). Rich rendering is available for
// the four MVP types (OKTA_SIGN_ON / ACCESS_POLICY / PASSWORD / MFA_ENROLL)
// per REQ-R04 AC-5; the remaining three types render as raw JSON only
// (REQ-R04 AC-6).
package policies

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// Deps bundles dependencies shared by Policies screens.
type Deps struct {
	Port   domain.PoliciesPort
	Clock  clock.Clock
	Logger *slog.Logger
	Keys   keys.ResolvedMap
	Width  int
	Height int
	// InitialPolicies lets tests seed without invoking the port.
	InitialPolicies []domain.Policy
}

// --- Type Select (SCR-040) ---------------------------------------------------

// TypeSelectModel lets the user pick a Policy type before a list fetch is
// issued (Okta's /api/v1/policies requires `type=`, REQ-R04 AC-2).
type TypeSelectModel struct {
	deps   Deps
	types  []domain.PolicyType
	rich   map[domain.PolicyType]bool
	cursor int
	picked domain.PolicyType
	done   bool
}

// NewTypeSelectModel constructs a TypeSelectModel with the 7 supported types.
func NewTypeSelectModel(deps Deps) TypeSelectModel {
	rich := map[domain.PolicyType]bool{}
	for _, t := range domain.RichRenderedPolicyTypes() {
		rich[t] = true
	}
	return TypeSelectModel{
		deps: deps,
		types: []domain.PolicyType{
			domain.PolicyTypeOktaSignOn,
			domain.PolicyTypeAccessPolicy,
			domain.PolicyTypePassword,
			domain.PolicyTypeMFAEnroll,
			domain.PolicyTypeProfileEnrollment,
			domain.PolicyTypePostAuthSession,
			domain.PolicyTypeIDPDiscovery,
		},
		rich: rich,
	}
}

// Init implements tea.Model.
func (m TypeSelectModel) Init() tea.Cmd { return nil }

// Update handles cursor movement and selection.
func (m TypeSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.types) {
			m.picked = m.types[m.cursor]
			m.done = true
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			if m.cursor < len(m.types)-1 {
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

// View renders the type menu with a `(raw view)` badge on the 3 types that
// only support raw JSON (REQ-R04 AC-2).
func (m TypeSelectModel) View() string {
	var b strings.Builder
	b.WriteString("Select Policy Type:\n")
	for i, t := range m.types {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		b.WriteString(prefix)
		b.WriteString(string(t))
		if !m.rich[t] {
			b.WriteString("   (raw view)")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Picked reports the selected type after Enter (zero value until done).
func (m TypeSelectModel) Picked() (domain.PolicyType, bool) { return m.picked, m.done }

// --- List (SCR-041) ----------------------------------------------------------

type policiesLoadedMsg struct{ policies []domain.Policy }

// policiesErrMsg surfaces a fetch failure (TUI_DESIGN §17, QA-022).
type policiesErrMsg struct{ err error }

// ListModel renders policies of a single type (REQ-R04 AC-3).
type ListModel struct {
	deps        Deps
	policyType  domain.PolicyType
	policies    []domain.Policy
	cursor      int
	opened      bool
	detail      domain.Policy
	lastErr     error
	width       int
	height      int
	viewportTop int
	ggChord     shared.GChord
}

// NewListModel constructs a ListModel for the given type.
func NewListModel(deps Deps, t domain.PolicyType) ListModel {
	return ListModel{
		deps:       deps,
		policyType: t,
		policies:   deps.InitialPolicies,
		width:      deps.Width,
		height:     deps.Height,
	}
}

// Init fetches policies of the selected type.
func (m ListModel) Init() tea.Cmd {
	if len(m.policies) > 0 || m.deps.Port == nil {
		return nil
	}
	return fetchPoliciesCmd(m.deps.Port, m.policyType)
}

// Update handles list navigation and detail transitions.
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case policiesLoadedMsg:
		m.policies = msg.policies
		m.lastErr = nil
		return m, nil
	case policiesErrMsg:
		m.lastErr = msg.err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ListModel) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc inside detail takes precedence over other keys so operators
	// always have a way back to the list (TUI_DESIGN §3.6 / §3.6a Note).
	if m.opened && km.Type == tea.KeyEsc {
		m.opened = false
		m.detail = domain.Policy{}
		return m, nil
	}
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.policies) {
			m.detail = m.policies[m.cursor]
			m.opened = true
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "g":
			if m.ggChord.Press(m.now()) {
				m.cursor = 0
				m.viewportTop = 0
			}
		case "G":
			m.ggChord.Reset()
			if n := len(m.policies); n > 0 {
				m.cursor = n - 1
			}
		case "j":
			m.ggChord.Reset()
			if m.cursor < len(m.policies)-1 {
				m.cursor++
			}
		case "k":
			m.ggChord.Reset()
			if m.cursor > 0 {
				m.cursor--
			}
		case "d":
			m.ggChord.Reset()
			if m.cursor >= 0 && m.cursor < len(m.policies) {
				m.detail = m.policies[m.cursor]
				m.opened = true
			}
		}
	}
	return m, nil
}


// View renders SCR-041 (TUI_DESIGN §15.5 / §16.7) with responsive column drop
// matching the other 4 list screens. Surfaces lastErr via the shared
// ErrorPanel (TUI_DESIGN §17.1, QA-022).
func (m ListModel) View() string {
	if m.opened {
		return renderPolicyDetail(m.detail, true)
	}
	if m.lastErr != nil {
		return "Policies › " + string(m.policyType) + "  (error)\n" +
			shared.ErrorPanel("Policies", m.lastErr)
	}

	tk := activeTokens()
	now := m.now()

	var b strings.Builder
	b.WriteString("Policies › ")
	b.WriteString(string(m.policyType))
	b.WriteString("  ")
	b.WriteString(itoa(len(m.policies)))
	b.WriteString(" of ")
	b.WriteString(itoa(len(m.policies)))
	b.WriteByte('\n')
	// 2-cell cursor gutter on the header keeps it aligned with data rows.
	b.WriteString("  ")
	b.WriteString(m.formatPoliciesColumns("PRI", "STATUS", "NAME", "SYSTEM", "UPDATED"))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(m.policies), shared.ListBodyRowBudget(m.height))
	for i := top; i < end; i++ {
		row := m.renderPolicyRow(m.policies[i], now, tk)
		if i == m.cursor {
			row = tk.Accent.Render("▸ " + row)
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}
	return b.String()
}

// renderPolicyRow formats one policy row.
func (m ListModel) renderPolicyRow(p domain.Policy, now time.Time, tk shared.Tokens) string {
	pri := itoa(p.Priority)
	status := shared.PolicyStatusBadge(string(p.Status), tk).Render(tk)
	system := "-"
	if p.System {
		system = "[SYS]"
	}
	updated := shared.RelativeTime(&p.LastUpdated, now)
	if p.LastUpdated.IsZero() {
		updated = "never"
	}
	return m.formatPoliciesColumns(pri, status, p.Name, system, updated)
}

// formatPoliciesColumns lays out 5 columns (TUI_DESIGN §15.5) with responsive
// drop matching the other 4 lists:
//
//   - W ≥ 120 / 0 : PRI, STATUS, NAME, SYSTEM, UPDATED
//   - 100..119    : drop UPDATED
//   - 90..99      : drop UPDATED + SYSTEM
//   - 80..89      : PRI + STATUS + NAME (NAME shorter)
//   - <80         : PRI + STATUS + NAME (very short)
func (m ListModel) formatPoliciesColumns(pri, status, name, system, updated string) string {
	w := m.width
	const (
		wPri     = 4
		wStatus  = 14
		wName    = 30
		wSystem  = 6
		wUpdated = 10
	)
	switch {
	case w >= 120 || w == 0:
		return padLeftP(pri, wPri) + "  " + padRightP(status, wStatus) + "  " +
			padRightP(name, wName) + "  " + padRightP(system, wSystem) + "  " +
			padLeftP(updated, wUpdated)
	case w >= 100:
		return padLeftP(pri, wPri) + "  " + padRightP(status, wStatus) + "  " +
			padRightP(name, wName) + "  " + padRightP(system, wSystem)
	case w >= 90:
		return padLeftP(pri, wPri) + "  " + padRightP(status, wStatus) + "  " +
			padRightP(name, wName)
	case w >= 80:
		return padLeftP(pri, wPri) + "  " + padRightP(status, wStatus) + "  " +
			padRightP(name, max(0, w-22))
	default:
		return padLeftP(pri, wPri) + "  " + padRightP(status, wStatus) + "  " +
			padRightP(name, max(0, w-22))
	}
}

// padRightP / padLeftP / visibleLenP / max are local helpers mirroring the
// other list packages. Kept package-local to avoid widening shared's API.
func padRightP(s string, width int) string {
	w := visibleLenP(s)
	if w >= width {
		return shared.Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func padLeftP(s string, width int) string {
	w := visibleLenP(s)
	if w >= width {
		return shared.Truncate(s, width)
	}
	return strings.Repeat(" ", width-w) + s
}

func visibleLenP(s string) int {
	count := 0
	i := 0
	for i < len(s) {
		c := s[i]
		if c == 0x1b && i+1 < len(s) && s[i+1] == '[' {
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
		count++
		i++
	}
	return count
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// activeTokens picks the token set per NO_COLOR.
func activeTokens() shared.Tokens {
	if shared.MonochromeEnabled() {
		return shared.Monochrome()
	}
	return shared.Dark()
}

// now returns the injected clock or wall time.
func (m ListModel) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

// --- Detail (SCR-042) --------------------------------------------------------

// DetailModel is SCR-042. Supports the `r` toggle between rich and raw JSON.
type DetailModel struct {
	deps   Deps
	policy domain.Policy
	raw    bool
}

// NewDetailModel constructs a DetailModel.
func NewDetailModel(deps Deps, p domain.Policy) DetailModel {
	return DetailModel{deps: deps, policy: p}
}

// Init implements tea.Model.
func (m DetailModel) Init() tea.Cmd { return nil }

// Update handles the `r` raw-toggle (REQ-R04 AC-6) and Ctrl-c to exit.
func (m DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	case tea.KeyRunes:
		if string(km.Runes) == "r" {
			m.raw = !m.raw
		}
	}
	return m, nil
}

// View renders either the rich action summary (for the 4 MVP types) or the
// raw JSON (default for the remaining 3 types; togglable on the 4 rich types).
func (m DetailModel) View() string {
	usesRich := isRichType(m.policy.Type)
	showRaw := !usesRich || m.raw
	return renderPolicyDetail(m.policy, !showRaw)
}

func renderPolicyDetail(p domain.Policy, rich bool) string {
	var b strings.Builder
	b.WriteString("Policy Detail\n")
	b.WriteString("  id:       ")
	b.WriteString(p.ID)
	b.WriteString("\n")
	b.WriteString("  name:     ")
	b.WriteString(p.Name)
	b.WriteString("\n")
	b.WriteString("  type:     ")
	b.WriteString(string(p.Type))
	b.WriteString("\n")
	b.WriteString("  priority: ")
	b.WriteString(itoa(p.Priority))
	b.WriteString("\n")
	b.WriteString("  status:   ")
	b.WriteString(string(p.Status))
	b.WriteString("\n")
	if p.System {
		b.WriteString("  [SYS]     system policy — cannot deactivate or delete\n")
	}
	if rich && isRichType(p.Type) {
		b.WriteString("\nAction summary:\n")
		b.WriteString("  ")
		b.WriteString(richSummary(p.Type))
		b.WriteString("\n")
		b.WriteString("\nPress `r` to toggle raw JSON.\n")
	} else {
		b.WriteString("\nRaw:\n")
		b.WriteString(prettyJSON(p.Raw))
		b.WriteString("\n")
		if isRichType(p.Type) {
			b.WriteString("\nPress `r` for rich view.\n")
		} else {
			b.WriteString("\nRich view not yet available for this type — raw JSON only.\n")
		}
	}
	return b.String()
}

// isRichType returns true for types with MVP rich renderers (REQ-R04 AC-5).
func isRichType(t domain.PolicyType) bool {
	for _, r := range domain.RichRenderedPolicyTypes() {
		if r == t {
			return true
		}
	}
	return false
}

// richSummary returns a one-line action summary per type.
func richSummary(t domain.PolicyType) string {
	switch t {
	case domain.PolicyTypeOktaSignOn:
		return "Session: maxIdle / maxLifetime / requireFactor"
	case domain.PolicyTypeAccessPolicy:
		return "Access: Require MFA / Deny / Allow (per rule)"
	case domain.PolicyTypePassword:
		return "Password: complexity / age / history"
	case domain.PolicyTypeMFAEnroll:
		return "MFA Enroll: required authenticators"
	}
	return ""
}

// prettyJSON reindents a json.RawMessage or returns a fallback string.
func prettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "  (empty)"
	}
	var indent bytes.Buffer
	if err := json.Indent(&indent, raw, "  ", "  "); err != nil {
		return "  " + string(raw)
	}
	return "  " + indent.String()
}

// --- Cmd factories -----------------------------------------------------------

func fetchPoliciesCmd(port domain.PoliciesPort, t domain.PolicyType) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		iter, err := port.List(ctx, domain.PoliciesQuery{Type: t, Limit: 20})
		if err != nil {
			return policiesErrMsg{err: err}
		}
		defer iter.Close()
		var out []domain.Policy
		for {
			p, hasMore, err := iter.Next(ctx)
			if err != nil {
				return policiesErrMsg{err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, p)
		}
		// REQ-R04 AC-3 — priority ascending.
		sort.SliceStable(out, func(i, j int) bool { return out[i].Priority < out[j].Priority })
		return policiesLoadedMsg{policies: out}
	}
}

// --- helpers -----------------------------------------------------------------

func itoa(n int) string {
	// tiny local helper to avoid a strconv import for a single call path.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

var (
	_ tea.Model = TypeSelectModel{}
	_ tea.Model = ListModel{}
	_ tea.Model = DetailModel{}
)
