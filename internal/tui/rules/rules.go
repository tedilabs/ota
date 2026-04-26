// Package rules implements the Group Rules list/detail Screen Models
// (SCR-030, SCR-031). INVALID state is rendered with an [INVALID] badge per
// REQ-R03 AC-2 so operators spot broken rules immediately.
package rules

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
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
	lastErr   error
	width     int
}

// NewListModel constructs a ListModel.
func NewListModel(deps Deps) ListModel {
	return ListModel{deps: deps, rules: deps.InitialRules, width: deps.Width}
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

	if msg.Type == tea.KeyRunes {
		switch string(msg.Runes) {
		case "/":
			m.filtering = true
			m.filter = ""
			return m, nil
		case "j":
			m.cursor++
			return m, nil
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
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
		return renderRuleDetail(m.detail)
	}
	if m.lastErr != nil {
		return "Group Rules  (error)\n" + shared.ErrorPanel("group rules", m.lastErr)
	}

	tk := activeTokens()
	rows := m.visible()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Group Rules  %d of %d", len(rows), len(m.rules)))
	if m.filter != "" {
		b.WriteString(" · q=\"" + m.filter + "\"")
	}
	b.WriteByte('\n')
	if m.filtering {
		b.WriteString("filter: " + m.filter + "\n")
	}
	b.WriteString(m.formatRulesColumns("STATUS", "NAME", "TARGETS", "UPDATED"))
	b.WriteByte('\n')
	for i, r := range rows {
		row := m.renderRulesRow(r, m.now(), tk)
		if i == m.cursor {
			row = "> " + row
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

// formatRulesColumns lays out the 4 columns (TUI_DESIGN §15.4) with
// responsive drop:
//
//   - W ≥ 120 : all 4 columns
//   - 100..119: drop UPDATED
//   - 90..99  : STATUS + NAME + TARGETS only
//   - 80..89  : STATUS + NAME only
//   - <80     : STATUS + NAME only (NAME trimmed)
func (m ListModel) formatRulesColumns(status, name, targets, updated string) string {
	w := m.width
	const (
		wStatus  = 14
		wName    = 30
		wTargets = 22
		wUpdated = 10
	)
	switch {
	case w >= 120 || w == 0:
		return padRight(status, wStatus) + "  " + padRight(name, wName) + "  " +
			padRight(shared.Truncate(targets, wTargets), wTargets) + "  " +
			padLeft(updated, wUpdated)
	case w >= 100:
		return padRight(status, wStatus) + "  " + padRight(name, wName) + "  " +
			padRight(shared.Truncate(targets, wTargets), wTargets)
	case w >= 90:
		return padRight(status, wStatus) + "  " + padRight(name, wName) + "  " +
			padRight(shared.Truncate(targets, 18), 18)
	case w >= 80:
		return padRight(status, wStatus) + "  " + padRight(name, max(0, w-18))
	default:
		return padRight(status, wStatus) + "  " + padRight(name, max(0, w-18))
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

// padRight / padLeft mirror the Users/Groups helpers.
func padRight(s string, width int) string {
	w := visibleLen(s)
	if w >= width {
		return shared.Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func padLeft(s string, width int) string {
	w := visibleLen(s)
	if w >= width {
		return shared.Truncate(s, width)
	}
	return strings.Repeat(" ", width-w) + s
}

func visibleLen(s string) int {
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

func (m ListModel) visible() []domain.GroupRule {
	if m.filter == "" {
		return m.rules
	}
	needle := strings.ToLower(m.filter)
	out := make([]domain.GroupRule, 0, len(m.rules))
	for _, r := range m.rules {
		if strings.Contains(strings.ToLower(r.Name), needle) {
			out = append(out, r)
		}
	}
	return out
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
func (m DetailModel) View() string { return renderRuleDetail(m.rule) }

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
