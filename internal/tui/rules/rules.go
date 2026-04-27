// Package rules implements the Group Rules list/detail Screen Models
// (SCR-030, SCR-031). INVALID state is rendered with an [INVALID] badge per
// REQ-R03 AC-2 so operators spot broken rules immediately.
package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// SortKey identifies the column the user has selected via Shift+letter
// (TUI_DESIGN §3.5a). Rules honours SortStatus and SortName.
type SortKey int

const (
	SortNone SortKey = iota
	SortStatus
	SortName
)

// SortDir is the on/off cycle direction (off → asc → desc → off).
type SortDir int

const (
	SortOff SortDir = iota
	SortAsc
	SortDesc
)

// Deps bundles dependencies shared by Rules screens.
type Deps struct {
	Port         domain.GroupRulesPort
	Groups       domain.GroupsPort // for id→name resolution (REQ-R03 AC-4)
	Clock        clock.Clock
	Logger       *slog.Logger
	Keys         keys.ResolvedMap
	Width        int
	Height       int
	InitialRules []domain.GroupRule
}

// --- List --------------------------------------------------------------------

type rulesLoadedMsg struct{ rules []domain.GroupRule }

// rulesErrMsg surfaces a fetch failure (TUI_DESIGN §17).
type rulesErrMsg struct{ err error }

// ListModel is SCR-030.
type ListModel struct {
	deps      Deps
	rules     []domain.GroupRule
	cursor    int
	filter    string
	filtering bool
	opened    bool
	detail    domain.GroupRule
	// detailTab tracks the active Detail tab while m.opened is true
	// (TUI_DESIGN §15.7 v1.2.0).
	detailTab RuleDetailTab
	// detailRawReturn is the tab `r` jumped from (Raw toggle target).
	detailRawReturn RuleDetailTab
	lastErr         error
	width           int
	height          int
	viewportTop     int
	// sortBy / sortDir track the active column sort cycle (TUI_DESIGN §3.5).
	sortBy  SortKey
	sortDir SortDir
	ggChord shared.GChord
	// hScroll — horizontal column offset (issue #122). h/l move the
	// column slice when the natural row exceeds the viewport.
	hScroll int
}

// RuleDetailTab indexes the Group Rule Detail tab bar. v0.1.2 collapsed
// the placeholder tabs into Pretty / JSON / YAML; the old Profile / Raw
// constants survive as aliases for backward compatibility.
type RuleDetailTab int

const (
	RuleDetailTabPretty RuleDetailTab = iota
	RuleDetailTabJSON
	RuleDetailTabYAML
)

const (
	RuleDetailTabProfile = RuleDetailTabPretty
	RuleDetailTabRaw     = RuleDetailTabJSON
)

var ruleDetailTabLabels = []string{"Pretty", "JSON", "YAML"}
var ruleDetailTabCount = RuleDetailTab(len(ruleDetailTabLabels))

// NewListModel constructs a ListModel.
func NewListModel(deps Deps) ListModel {
	return ListModel{
		deps:   deps,
		rules:  deps.InitialRules,
		width:  deps.Width,
		height: deps.Height,
	}
}

// Init fetches the rules list on entry.
func (m ListModel) Init() tea.Cmd {
	if len(m.rules) > 0 || m.deps.Port == nil {
		return nil
	}
	return fetchRulesCmd(m.deps.Port)
}

// Update handles key input and fetch results.
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case rulesLoadedMsg:
		m.rules = msg.rules
		m.lastErr = nil
		return m, nil
	case rulesErrMsg:
		m.lastErr = msg.err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ListModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	// Detail mode (TUI_DESIGN §3.6 + §15.7): Esc returns to the list; Tab /
	// Shift-Tab cycle through tabs; `r` toggles the Raw tab against the
	// last-visited non-Raw tab.
	if m.opened {
		switch msg.Type {
		case tea.KeyEsc:
			m.opened = false
			m.detail = domain.GroupRule{}
			m.detailTab = RuleDetailTabProfile
			m.detailRawReturn = RuleDetailTabProfile
			return m, nil
		case tea.KeyTab:
			m.detailTab = (m.detailTab + 1) % ruleDetailTabCount
			return m, nil
		case tea.KeyShiftTab:
			m.detailTab = (m.detailTab + ruleDetailTabCount - 1) % ruleDetailTabCount
			return m, nil
		case tea.KeyRunes:
			if string(msg.Runes) == "r" {
				if m.detailTab == RuleDetailTabRaw {
					m.detailTab = m.detailRawReturn
				} else {
					m.detailRawReturn = m.detailTab
					m.detailTab = RuleDetailTabRaw
				}
			}
			return m, nil
		}
		return m, nil
	}
	if m.filtering {
		switch msg.Type {
		case tea.KeyEnter:
			m.filtering = false
			m.cursor = 0
			return m, nil
		case tea.KeyEsc:
			m.filtering = false
			m.filter = ""
			return m, nil
		case tea.KeyBackspace:
			if n := len(m.filter); n > 0 {
				m.filter = m.filter[:n-1]
			}
			return m, nil
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
			return m, nil
		}
		return m, nil
	}

	// Esc on the list (after the `/` prompt closed) clears any active
	// filter (issue #131).
	if msg.Type == tea.KeyEsc && m.filter != "" {
		m.filter = ""
		m.cursor = 0
		m.viewportTop = 0
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlF:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		return m.cursorBy(page), nil
	case tea.KeyCtrlB:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		return m.cursorBy(-page), nil
	case tea.KeyCtrlD:
		return m.cursorBy(max(1, shared.ListBodyRowBudget(m.height)/2)), nil
	case tea.KeyCtrlU:
		return m.cursorBy(-max(1, shared.ListBodyRowBudget(m.height)/2)), nil
	}

	if msg.Type == tea.KeyRunes {
		switch string(msg.Runes) {
		case "g":
			// gg chord (TUI_DESIGN §3.2).
			if m.ggChord.Press(m.now()) {
				m.cursor = 0
				m.viewportTop = 0
			}
			return m, nil
		case "G":
			m.ggChord.Reset()
			if vis := m.visible(); len(vis) > 0 {
				m.cursor = len(vis) - 1
			}
			return m, nil
		case "/":
			m.ggChord.Reset()
			m.filtering = true
			m.filter = ""
			return m, nil
		case "j":
			m.ggChord.Reset()
			m.cursor++
			return m, nil
		case "k":
			m.ggChord.Reset()
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "h":
			m.ggChord.Reset()
			if m.hScroll > 0 {
				m.hScroll--
			}
			return m, nil
		case "l":
			m.ggChord.Reset()
			specs := rulesColumnSpecs()
			max := shared.MaxHScroll(specs, m.rulesInnerWidth(), 2)
			if m.hScroll < max {
				m.hScroll++
			}
			return m, nil
		case "S":
			// §3.5a — Rules: Shift+S sorts by STATUS.
			m.cycleSort(SortStatus)
			return m, nil
		case "N":
			// §3.5a — Rules: Shift+N sorts by NAME.
			m.cycleSort(SortName)
			return m, nil
		case "d":
			// §3.6 — `d` opens the inline detail surface. Mirrors Enter.
			sel := m.selected()
			if sel == nil {
				return m, nil
			}
			m.detail = *sel
			m.opened = true
			return m, nil
		}
	}
	if msg.Type == tea.KeyEnter {
		sel := m.selected()
		if sel == nil {
			return m, nil
		}
		m.detail = *sel
		m.opened = true
	}
	return m, nil
}

// View renders SCR-030 (TUI_DESIGN §15.4 / §16.6). Columns: STATUS / NAME /
// TARGETS / UPDATED. When at least one rule is INVALID an inline banner
// summarizes the count (REQ-R03 AC-3).
func (m ListModel) View() string {
	if m.opened {
		return renderRuleDetailTabbed(m.detail, m.detailTab)
	}
	if m.lastErr != nil {
		return "Group Rules  (error)\n" + shared.ErrorPanel("group rules", m.lastErr)
	}

	tk := activeTokens()
	rows := m.visible()

	var b strings.Builder
	// Resource label + filter both live in the chrome's upper divider
	// now (issue #133); body just surfaces the visible count.
	b.WriteString(fmt.Sprintf("%d of %d", len(rows), len(m.rules)))
	b.WriteByte('\n')
	// Inline "filter:" dropped in v0.1.5-6 — App Shell renders a floating
	// input box for `/`.
	// 2-cell cursor gutter on the header keeps it aligned with data rows.
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatRulesColumns(
		rulesSortLabel("STATUS", m.sortBy, SortStatus, m.sortDir, tk),
		rulesSortLabel("NAME", m.sortBy, SortName, m.sortDir, tk),
		"TARGETS",
		"UPDATED",
	)))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(rows), shared.ListBodyRowBudget(m.height))
	for i := top; i < end; i++ {
		row := m.renderRulesRow(rows[i], m.now(), tk)
		if i == m.cursor {
			row = tk.Accent.Render("▸ " + row)
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}

	if invalidN := countInvalid(m.rules); invalidN > 0 {
		b.WriteByte('\n')
		noun := "rule"
		if invalidN > 1 {
			noun = "rules"
		}
		b.WriteString(fmt.Sprintf("[!] %d %s in INVALID state — expression cannot be evaluated by Okta.\n", invalidN, noun))
		b.WriteString("    Open the rule to view why and what to fix.\n")
	}

	return b.String()
}

// renderRulesRow formats one rule.
func (m ListModel) renderRulesRow(r domain.GroupRule, now time.Time, tk shared.Tokens) string {
	status := shared.RuleStatusBadge(string(r.Status), tk).Render(tk)
	targets := formatTargets(r.TargetGroupIDs)
	updated := shared.RelativeTime(&r.LastUpdated, now)
	if r.LastUpdated.IsZero() {
		updated = "—"
	}
	return m.formatRulesColumns(status, r.Name, targets, updated)
}

// formatTargets joins target group IDs for the TARGETS column. The service
// layer resolves IDs→names (REQ-R03 AC-4); when only IDs are present we
// surface them so operators still know how many groups are touched.
func formatTargets(ids []string) string {
	if len(ids) == 0 {
		return "—"
	}
	return strings.Join(ids, ", ")
}

// rulesColumnSpecs returns the §15.0a.4 column definitions in declaration
// order: STATUS, NAME, TARGETS, UPDATED. Drop priorities (low first):
// TARGETS (1) → UPDATED (2); STATUS / NAME never drop.
func rulesColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "STATUS", Kind: shared.ColumnFixed, Min: 14, DropPriority: 0},
		{Title: "NAME", Kind: shared.ColumnFlex, Min: 22, Weight: 2, DropPriority: 0},
		{Title: "TARGETS", Kind: shared.ColumnFlex, Min: 16, Weight: 2, DropPriority: 1},
		{Title: "UPDATED", Kind: shared.ColumnFixed, Min: 10, DropPriority: 2, AlignRight: true},
	}
}

// formatRulesColumns lays out STATUS / NAME / TARGETS / UPDATED per the
// TUI_DESIGN §15.0a Min/Weight + DropPriority model. h/l-aware
// (issue #122) when the natural row overflows the viewport.
func (m ListModel) formatRulesColumns(cells ...string) string {
	specs := rulesColumnSpecs()
	innerWidth := m.rulesInnerWidth()
	var widths []int
	if shared.MaxHScroll(specs, innerWidth, 2) == 0 {
		widths = shared.LayoutColumns(specs, innerWidth, 2)
	} else {
		widths = shared.LayoutColumnsHScroll(specs, innerWidth, 2, m.hScroll)
	}

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

// rulesInnerWidth mirrors users.usersInnerWidth — body width after the
// chrome border (2), left padding (1), and cursor gutter (2).
func (m ListModel) rulesInnerWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	inner := w - 2 - 1 - 2
	if inner < 20 {
		inner = 20
	}
	return inner
}

// countInvalid returns the number of rules whose Status is INVALID.
func countInvalid(rules []domain.GroupRule) int {
	n := 0
	for _, r := range rules {
		if r.Status == domain.GroupRuleStatusInvalid {
			n++
		}
	}
	return n
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

func (m ListModel) visible() []domain.GroupRule {
	var out []domain.GroupRule
	if m.filter == "" {
		out = append(out, m.rules...)
	} else {
		needle := strings.ToLower(m.filter)
		out = make([]domain.GroupRule, 0, len(m.rules))
		for _, r := range m.rules {
			if strings.Contains(strings.ToLower(r.Name), needle) {
				out = append(out, r)
			}
		}
	}
	if m.sortBy != SortNone && m.sortDir != SortOff {
		sortRulesByKey(out, m.sortBy, m.sortDir)
	}
	return out
}

// cycleSort advances the sort state per TUI_DESIGN §3.5.
func (m *ListModel) cycleSort(target SortKey) {
	if m.sortBy != target {
		m.sortBy = target
		m.sortDir = SortAsc
		m.cursor = 0
		return
	}
	switch m.sortDir {
	case SortOff:
		m.sortDir = SortAsc
	case SortAsc:
		m.sortDir = SortDesc
	case SortDesc:
		m.sortBy = SortNone
		m.sortDir = SortOff
	}
	m.cursor = 0
}

// sortRulesByKey applies a stable sort honouring §3.5a Rules.
//   - SortStatus uses the operational rank (INVALID first → ACTIVE → INACTIVE)
//     so the asc cycle surfaces broken rules at the top.
//   - SortName is case-insensitive alphabetical on rule.Name.
func sortRulesByKey(xs []domain.GroupRule, key SortKey, dir SortDir) {
	less := rulesComparator(key)
	if less == nil {
		return
	}
	sort.SliceStable(xs, func(i, j int) bool {
		if dir == SortDesc {
			return less(xs[j], xs[i])
		}
		return less(xs[i], xs[j])
	})
}

func rulesComparator(key SortKey) func(a, b domain.GroupRule) bool {
	switch key {
	case SortStatus:
		return func(a, b domain.GroupRule) bool {
			return ruleStatusRank(a.Status) < ruleStatusRank(b.Status)
		}
	case SortName:
		return func(a, b domain.GroupRule) bool {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	}
	return nil
}

// ruleStatusRank returns the §3.5a operational rank for asc sorting.
// INVALID first surfaces broken rules at the top of the list.
func ruleStatusRank(s domain.GroupRuleStatus) int {
	switch s {
	case domain.GroupRuleStatusInvalid:
		return 0
	case domain.GroupRuleStatusActive:
		return 1
	case domain.GroupRuleStatusInactive:
		return 2
	}
	return 3
}

// rulesSortLabel appends "↑" / "↓" to title when the active key matches.
func rulesSortLabel(title string, active, key SortKey, dir SortDir, tk shared.Tokens) string {
	if active != key || dir == SortOff {
		return title
	}
	switch dir {
	case SortAsc:
		return title + shared.SortGlyph("asc", tk)
	case SortDesc:
		return title + shared.SortGlyph("desc", tk)
	}
	return title
}

// Filtering / Filter expose the `/` filter state so the App Shell can
// render its floating filter box (issue #123).
func (m ListModel) Filtering() bool { return m.filtering }
func (m ListModel) Filter() string  { return m.filter }

// cursorBy moves the cursor by delta rows, clamped to the visible range
// (issue #119).
func (m ListModel) cursorBy(delta int) ListModel {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if vis := m.visible(); m.cursor >= len(vis) {
		if len(vis) > 0 {
			m.cursor = len(vis) - 1
		} else {
			m.cursor = 0
		}
	}
	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m ListModel) selected() *domain.GroupRule {
	vs := m.visible()
	if m.cursor < 0 || m.cursor >= len(vs) {
		return nil
	}
	return &vs[m.cursor]
}

func ruleStatusBadge(s domain.GroupRuleStatus) string {
	switch s {
	case domain.GroupRuleStatusActive:
		return "[ACTIVE  ]"
	case domain.GroupRuleStatusInactive:
		return "[INACTIVE]"
	case domain.GroupRuleStatusInvalid:
		return "[INVALID ]"
	}
	return "[UNKNOWN ]"
}

// --- Detail ------------------------------------------------------------------

// DetailModel is SCR-031.
type DetailModel struct {
	deps Deps
	rule domain.GroupRule
}

// NewDetailModel constructs a DetailModel.
func NewDetailModel(deps Deps, r domain.GroupRule) DetailModel {
	return DetailModel{deps: deps, rule: r}
}

// Init implements tea.Model.
func (m DetailModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m DetailModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

// View renders the rule detail with the expression in a monospace block and
// a deactivation warning (REQ-R03 AC-5).
func (m DetailModel) View() string { return renderRuleDetailTabbed(m.rule, RuleDetailTabProfile) }

// renderRuleDetailTabbed adds a §15.7 v1.2.0 tab bar around the legacy
// single-surface rule detail. Profile keeps the v0.1.0 layout; Raw
// serialises domain.GroupRule as JSON; Conditions / Targets are
// placeholders until v0.2.
func renderRuleDetailTabbed(r domain.GroupRule, active RuleDetailTab) string {
	var b strings.Builder
	b.WriteString("Group Rule Detail\n")
	b.WriteString(renderRuleTabBar(active))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 78))
	b.WriteByte('\n')
	switch active {
	case RuleDetailTabJSON:
		b.WriteString(renderRuleRawTab(r))
	case RuleDetailTabYAML:
		b.WriteString(renderRuleYAMLTab(r))
	default:
		b.WriteString(renderRuleDetail(r))
	}
	return b.String()
}

// renderRuleYAMLTab marshals the same ruleJSONShape projection as the
// JSON tab through gopkg.in/yaml.v3, with a 2-space indent (issue #109)
// and shared syntax highlighting (issue #110).
func renderRuleYAMLTab(r domain.GroupRule) string {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(ruleJSONShapeFor(r)); err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	if err := enc.Close(); err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	body := strings.TrimRight(buf.String(), "\n")
	return shared.HighlightYAML(body, activeTokens()) + "\n"
}

func renderRuleTabBar(active RuleDetailTab) string {
	var parts []string
	for i, label := range ruleDetailTabLabels {
		if RuleDetailTab(i) == active {
			parts = append(parts, "["+label+"]")
		} else {
			parts = append(parts, "[ "+label+" ]")
		}
	}
	return strings.Join(parts, " ")
}

// renderRuleRawTab returns the §15.7 v1.2.0 Raw JSON tab content with
// v0.1.3 syntax highlighting.
func renderRuleRawTab(r domain.GroupRule) string {
	buf, err := json.MarshalIndent(ruleJSONShapeFor(r), "", "  ")
	if err != nil {
		return "(raw render error: " + err.Error() + ")\n"
	}
	return shared.HighlightJSON(string(buf), activeTokens()) + "\n"
}

// ruleJSONShapeFor centralises the deterministic projection so JSON and
// YAML tabs render identical data.
func ruleJSONShapeFor(r domain.GroupRule) ruleJSONShape {
	return ruleJSONShape{
		ID:             r.ID,
		Name:           r.Name,
		Status:         string(r.Status),
		Expression:     r.Expression,
		TargetGroupIDs: r.TargetGroupIDs,
		Created:        formatRuleJSONTime(r.Created),
		LastUpdated:    formatRuleJSONTime(r.LastUpdated),
	}
}

type ruleJSONShape struct {
	ID             string   `json:"id" yaml:"id"`
	Name           string   `json:"name" yaml:"name"`
	Status         string   `json:"status" yaml:"status"`
	Expression     string   `json:"expression" yaml:"expression"`
	TargetGroupIDs []string `json:"targetGroupIds,omitempty" yaml:"targetGroupIds,omitempty"`
	Created        string   `json:"created,omitempty" yaml:"created,omitempty"`
	LastUpdated    string   `json:"lastUpdated,omitempty" yaml:"lastUpdated,omitempty"`
}

func formatRuleJSONTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func renderRuleDetail(r domain.GroupRule) string {
	var b strings.Builder
	b.WriteString("Group Rule Detail\n")
	b.WriteString("  id:     ")
	b.WriteString(r.ID)
	b.WriteString("\n")
	b.WriteString("  name:   ")
	b.WriteString(r.Name)
	b.WriteString("\n")
	b.WriteString("  status: ")
	b.WriteString(ruleStatusBadge(r.Status))
	b.WriteString("\n")
	if r.Status == domain.GroupRuleStatusInvalid {
		b.WriteString("  ⚠ INVALID — this rule is broken and memberships are not being applied.\n")
	}
	b.WriteString("\nExpression:\n")
	b.WriteString("  ")
	b.WriteString(r.Expression)
	b.WriteString("\n")
	if len(r.TargetGroupIDs) > 0 {
		b.WriteString("\nTarget groups:\n")
		for _, id := range r.TargetGroupIDs {
			b.WriteString("  - ")
			b.WriteString(id)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n⚠ Deactivating this rule would remove all memberships it created.\n")
	b.WriteString("   This action is disabled in read-only mode.\n")
	return b.String()
}

// --- Cmd factories -----------------------------------------------------------

func fetchRulesCmd(port domain.GroupRulesPort) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		iter, err := port.List(ctx, domain.GroupRulesQuery{Limit: 200})
		if err != nil {
			return rulesErrMsg{err: err}
		}
		defer iter.Close()
		var out []domain.GroupRule
		for {
			r, hasMore, err := iter.Next(ctx)
			if err != nil {
				return rulesErrMsg{err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, r)
		}
		return rulesLoadedMsg{rules: out}
	}
}

var (
	_ tea.Model = ListModel{}
	_ tea.Model = DetailModel{}
)
