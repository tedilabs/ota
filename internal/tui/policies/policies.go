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

// policyRulesLoadedMsg / policyRulesErrMsg deliver the result of a
// per-policy rules fetch (issue #154 detail tab).
type policyRulesLoadedMsg struct {
	policyID string
	rules    []domain.PolicyRule
}
type policyRulesErrMsg struct {
	policyID string
	err      error
}

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
	// detailRules / detailRulesPolicy / detailRulesLoaded carry the
	// lazy-fetched rule list for the open policy (issue #154). The
	// inline detail surfaces the rules below the policy header so
	// operators can read the per-type rules without a side trip.
	detailRules        []domain.PolicyRule
	detailRulesPolicy  string
	detailRulesLoaded  bool
	detailRulesErr     error
	detailShowRaw      bool
	// detailCursor drives the body line cursor + visual mode for the
	// Policy Detail body (#F5 v0.2.5).
	detailCursor shared.BodyCursor
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

// Count returns the visible/total counts for the App Shell's upper
// divider (issue #136). Policies has no inline filter today so
// visible always equals total.
func (m ListModel) Count() (visible, total int) {
	return len(m.policies), len(m.policies)
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
	case policyRulesLoadedMsg:
		if m.opened && m.detail.ID == msg.policyID {
			m.detailRules = msg.rules
			m.detailRulesPolicy = msg.policyID
			m.detailRulesLoaded = true
			m.detailRulesErr = nil
		}
		return m, nil
	case policyRulesErrMsg:
		if m.opened && m.detail.ID == msg.policyID {
			m.detailRulesErr = msg.err
			m.detailRulesLoaded = true
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ListModel) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Arrow keys map to Vim-style runes (issue #159).
	km = shared.NormalizeArrowKey(km)
	// Esc inside detail takes precedence over other keys so operators
	// always have a way back to the list (TUI_DESIGN §3.6 / §3.6a Note).
	if m.opened && km.Type == tea.KeyEsc {
		// #F5 v0.2.5 — Esc cancels visual mode first.
		if m.detailCursor.Visual {
			m.detailCursor.CancelVisual()
			return m, nil
		}
		m.opened = false
		m.detail = domain.Policy{}
		m.detailRules = nil
		m.detailRulesPolicy = ""
		m.detailRulesLoaded = false
		m.detailRulesErr = nil
		m.detailShowRaw = false
		m.detailCursor = shared.BodyCursor{}
		return m, nil
	}
	// `r` toggles between the rich detail view (with rule list) and
	// the raw JSON view of the underlying policy.
	if m.opened && km.Type == tea.KeyRunes && string(km.Runes) == "r" {
		m.detailShowRaw = !m.detailShowRaw
		m.detailCursor = shared.BodyCursor{}
		return m, nil
	}
	// #F5 v0.2.5 — body cursor + visual mode while the detail is open.
	if m.opened {
		viewport := m.detailViewportRows()
		total := len(policyDetailLines(m.detail, !m.detailShowRaw,
			m.detailRules, m.detailRulesLoaded, m.detailRulesErr))
		switch km.Type {
		case tea.KeyCtrlF:
			m.detailCursor.PageDown(viewport, total)
			return m, nil
		case tea.KeyCtrlB:
			m.detailCursor.PageUp(viewport)
			return m, nil
		case tea.KeyCtrlD:
			m.detailCursor.HalfPageDown(viewport, total)
			return m, nil
		case tea.KeyCtrlU:
			m.detailCursor.HalfPageUp(viewport)
			return m, nil
		case tea.KeyRunes:
			switch string(km.Runes) {
			case "j":
				m.detailCursor.Down(total)
				return m, nil
			case "k":
				m.detailCursor.Up()
				return m, nil
			case "g":
				m.detailCursor.GoTop()
				return m, nil
			case "G":
				m.detailCursor.GoBottom(total)
				return m, nil
			case "v", "V":
				if m.detailCursor.Visual {
					m.detailCursor.CancelVisual()
				} else {
					m.detailCursor.StartVisual()
				}
				return m, nil
			case "y":
				return m, shared.YankCmd(m.detailCursor,
					policyDetailLines(m.detail, !m.detailShowRaw,
						m.detailRules, m.detailRulesLoaded, m.detailRulesErr),
					"Policy Detail")
			}
		}
		// Eat any remaining keys while detail is open so list-mode
		// handlers (Ctrl+C, Enter, etc.) don't silently shuffle the
		// list cursor underneath the open detail.
		return m, nil
	}
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	case tea.KeyCtrlF:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor = clampInt(m.cursor+page, 0, len(m.policies)-1)
	case tea.KeyCtrlB:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor = clampInt(m.cursor-page, 0, len(m.policies)-1)
	case tea.KeyCtrlD:
		page := max(1, shared.ListBodyRowBudget(m.height)/2)
		m.cursor = clampInt(m.cursor+page, 0, len(m.policies)-1)
	case tea.KeyCtrlU:
		page := max(1, shared.ListBodyRowBudget(m.height)/2)
		m.cursor = clampInt(m.cursor-page, 0, len(m.policies)-1)
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.policies) {
			m.detail = m.policies[m.cursor]
			m.opened = true
			return m, openPolicyRulesCmd(m.deps.Port, m.detail.ID)
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
				return m, openPolicyRulesCmd(m.deps.Port, m.detail.ID)
			}
		}
	}
	return m, nil
}

// openPolicyRulesCmd lazily fetches the rule list for the supplied
// policy. Round-trips the policyID through the result message so a
// stale fetch from a previously-opened detail can't overwrite the
// current one (mirrors the Group Detail Members pattern from #142).
func openPolicyRulesCmd(port domain.PoliciesPort, policyID string) tea.Cmd {
	if port == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		rules, err := port.Rules(ctx, policyID)
		if err != nil {
			return policyRulesErrMsg{policyID: policyID, err: err}
		}
		return policyRulesLoadedMsg{policyID: policyID, rules: rules}
	}
}


// View renders SCR-041 (TUI_DESIGN §15.5 / §16.7) with responsive column drop
// matching the other 4 list screens. Surfaces lastErr via the shared
// ErrorPanel (TUI_DESIGN §17.1, QA-022).
// detailViewportRows returns the row budget the body cursor scrolls
// inside on the Policy Detail surface (#F5 v0.2.5). Policies render
// no in-body header so 0 reserved.
func (m ListModel) detailViewportRows() int {
	return shared.DetailBodyRowBudget(m.height, 0)
}

func (m ListModel) View() string {
	if m.opened {
		// #F5 v0.2.5 — render the detail body through the body
		// cursor so j/k/g/G/v/V/y all work consistently.
		lines := policyDetailLines(m.detail, !m.detailShowRaw,
			m.detailRules, m.detailRulesLoaded, m.detailRulesErr)
		width := m.chromeContentWidth() - 2
		if width <= 0 {
			var b strings.Builder
			for i, line := range lines {
				if i > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(line)
			}
			return b.String()
		}
		tk := activeTokens()
		cur := m.detailCursor
		rendered := cur.RenderViewport(lines, width, m.detailViewportRows(), tk)
		return shared.JoinLines(rendered)
	}
	if m.lastErr != nil {
		return "Policies › " + string(m.policyType) + "  (error)\n" +
			shared.ErrorPanel("Policies", m.lastErr)
	}

	tk := activeTokens()
	now := m.now()

	var b strings.Builder
	// Resource label, count, and filter all live in the chrome's
	// upper divider (issues #133 + #136); the body opens straight
	// with the policy-type subcategory then the column header.
	b.WriteString("› ")
	b.WriteString(string(m.policyType))
	b.WriteByte('\n')
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatPoliciesColumns("PRI", "STATUS", "NAME", "SYSTEM", "UPDATED")))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(m.policies), shared.ListBodyRowBudget(m.height))
	budget := end - top
	rowTarget := m.chromeContentWidth() - 2
	for i := top; i < end; i++ {
		row := m.renderPolicyRow(m.policies[i], now, tk)
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		// v0.2.0 #182 — unified cursor pipeline.
		b.WriteString(shared.RenderRowCursor(prefix+row, rowTarget, i == m.cursor, string(m.policies[i].Status), false, tk))
		b.WriteString(shared.AppendScrollbarSuffix(i-top, top, budget, len(m.policies), tk))
		b.WriteByte('\n')
	}
	return b.String()
}

// chromeContentWidth returns the body cells the chrome reserves per
// row, used to land the scrollbar gutter flush against the right
// border (issue #173).
func (m ListModel) chromeContentWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	return w - 3
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
// Issue #157: ports policies onto the shared column-spec system so
// rows auto-fit observed data widths the same way Users does.
func (m ListModel) formatPoliciesColumns(pri, status, name, system, updated string) string {
	specs := policiesColumnSpecs()
	specs = shared.ShrinkSpecsToFit(specs, m.observedColumnWidths())
	widths := m.policiesWidths(specs)
	cells := []string{pri, status, name, system, updated}
	full := make([]string, len(specs))
	for i := range specs {
		if i < len(cells) {
			full[i] = cells[i]
		} else {
			full[i] = "—"
		}
	}
	return shared.FormatRow(specs, widths, full, 2)
}

func policiesColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "PRI", Kind: shared.ColumnFixed, Min: 4, DropPriority: 0, AlignRight: true},
		{Title: "STATUS", Kind: shared.ColumnFixed, Min: 10, DropPriority: 0, AlignCenter: true},
		{Title: "NAME", Kind: shared.ColumnFlex, Min: 22, Weight: 3, DropPriority: 0},
		{Title: "SYSTEM", Kind: shared.ColumnFixed, Min: 6, DropPriority: 1, AlignCenter: true},
		{Title: "UPDATED", Kind: shared.ColumnFixed, Min: 10, DropPriority: 2, AlignRight: true},
	}
}

// policiesWidths picks the layout — tight first, hScroll fallback —
// matching the Users approach (issue #138).
func (m ListModel) policiesWidths(specs []shared.ColumnSpec) []int {
	inner := m.policiesInnerWidth()
	if w := shared.LayoutColumnsTight(specs, inner, 2); w != nil {
		return w
	}
	return shared.LayoutColumnsHScroll(specs, inner, 2, 0)
}

func (m ListModel) policiesInnerWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	// chrome border (2) + left padding (1) + cursor gutter (2) +
	// scrollbar gutter (2: " ▌"/" │", v0.1.15 issue #173).
	inner := w - 2 - 1 - 2 - 2
	if inner < 20 {
		inner = 20
	}
	return inner
}

// observedColumnWidths returns the widest observed-cell width per
// column across the visible policies. Powers ShrinkSpecsToFit so
// columns auto-fit to actual data instead of padding to declared Min.
func (m ListModel) observedColumnWidths() []int {
	if len(m.policies) == 0 {
		return nil
	}
	now := m.now()
	tk := activeTokens()
	out := make([]int, 5)
	for _, p := range m.policies {
		pri := itoa(p.Priority)
		status := shared.PolicyStatusBadge(string(p.Status), tk).Render(tk)
		name := p.Name
		system := ""
		if p.System {
			system = "[SYS]"
		}
		updated := shared.RelativeTime(&p.LastUpdated, now)
		if p.LastUpdated.IsZero() {
			updated = "—"
		}
		cells := []string{pri, status, name, system, updated}
		for i, c := range cells {
			if w := shared.VisibleWidth(c); w > out[i] {
				out[i] = w
			}
		}
	}
	return out
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

// clampInt pins v to [lo, hi]. Empty data sets pass hi = -1, which we
// surface as 0 so the cursor never goes negative on an empty list.
func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

// policyDetailLines flattens the policy header, action summary / raw
// JSON, and per-policy rules section into a single line slice the
// BodyCursor navigates and yanks against (#F5 v0.2.5).
func policyDetailLines(p domain.Policy, rich bool, rules []domain.PolicyRule, loaded bool, err error) []string {
	body := renderPolicyDetail(p, rich) + "\n" + renderPolicyRulesSection(rules, loaded, err)
	return strings.Split(strings.TrimRight(body, "\n"), "\n")
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

// renderPolicyRulesSection produces the "Rules" block appended to
// the inline policy detail (issue #154). Three states:
//
//   - !loaded && err == nil:  "loading rules…"
//   - err != nil:             "(rules failed: <err>)"
//   - loaded:                 priority-ordered rule list with
//                             status badge + name
func renderPolicyRulesSection(rules []domain.PolicyRule, loaded bool, err error) string {
	tk := activeTokens()
	var b strings.Builder
	b.WriteString(shared.SectionHeader("Rules", 56))
	b.WriteByte('\n')
	if row := shared.PlaceholderRow(loaded, err, len(rules), "rules", tk); row != "" {
		b.WriteString("  " + row + "\n")
		return b.String()
	}
	for _, r := range rules {
		statusBadge := shared.PolicyStatusBadge(string(r.Status), tk).Render(tk)
		b.WriteString("  ")
		b.WriteString(statusBadge)
		b.WriteString("  #")
		b.WriteString(itoa(r.Priority))
		b.WriteString("  ")
		b.WriteString(r.Name)
		if r.System {
			b.WriteString("  ")
			b.WriteString(tk.Muted.Render("[SYS]"))
		}
		b.WriteByte('\n')
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
