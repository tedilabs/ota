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
	"strconv"
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
	// RefreshInterval drives the auto-refresh tick (issue #177
	// v0.1.16). Zero disables auto-refresh.
	RefreshInterval time.Duration
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
	// groupNames caches id→name lookups for the TARGETS column
	// (issue #163). Populated lazily on first render via a Cmd.
	groupNames        map[string]string
	groupNamesFetched bool

	// lastUpdated stamps the most recent successful list fetch (issue
	// #177 v0.1.16); refreshGen invalidates stale ticks.
	lastUpdated time.Time
	refreshGen  int
	// changedAt — per-row "just changed" stamps for the RowChanged
	// flash on refresh (issue #193 v0.2.3).
	changedAt map[string]time.Time
	// loaded flips true once the first rulesLoadedMsg / rulesErrMsg
	// arrives; before then View renders a spinner (issue #194 v0.2.4).
	loaded       bool
	spinnerFrame int
	fetching     bool                 // #U10 v0.2.4
	failedAt     map[string]time.Time // #U11 v0.2.4

	// detailTargetsFocused / detailTargetCur drive the TARGETS
	// drill-down cursor on the Rule Detail surface (#G4 / U7 v0.2.4).
	// `]` enters focused mode; j/k cycle among target groups; Enter
	// drills into the focused target's Group Detail; Esc / `[` exits.
	detailTargetsFocused bool
	detailTargetCur      int
}

// Fetching implements app.FetchingStater (#U10 v0.2.4).
func (m ListModel) Fetching() bool { return m.fetching }

// rulesRefreshTickMsg fires the auto-refresh tick (issue #177).
type rulesRefreshTickMsg struct{ gen int }

// rulesHighlightTickMsg keeps the View re-rendering while at least one
// row is still inside shared.HighlightWindow (issue #193 v0.2.3).
type rulesHighlightTickMsg struct{}

// rulesSpinnerTickMsg advances the loading spinner frame (issue #194
// v0.2.4).
type rulesSpinnerTickMsg struct{}

// RuleDetailTab is an alias of shared.DetailTab so the canonical
// Pretty/JSON/YAML tab order + labels live in one place (#A4 v0.2.4).
// Profile / Raw aliases survive for v0.1.1 callers.
type RuleDetailTab = shared.DetailTab

const (
	RuleDetailTabPretty = shared.DetailTabPretty
	RuleDetailTabJSON   = shared.DetailTabJSON
	RuleDetailTabYAML   = shared.DetailTabYAML
)

const (
	RuleDetailTabProfile = RuleDetailTabPretty
	RuleDetailTabRaw     = RuleDetailTabJSON
)

var ruleDetailTabLabels = shared.DetailTabLabels
var ruleDetailTabCount = shared.DetailTabCount

// NewListModel constructs a ListModel.
func NewListModel(deps Deps) ListModel {
	m := ListModel{
		deps:   deps,
		rules:  deps.InitialRules,
		width:  deps.Width,
		height: deps.Height,
	}
	if len(m.rules) > 0 || deps.Port == nil {
		m.loaded = true
	}
	return m
}

// Init fetches the rules list on entry and schedules the first
// auto-refresh tick (issue #177 v0.1.16).
func (m ListModel) Init() tea.Cmd {
	var fetch tea.Cmd
	if len(m.rules) == 0 && m.deps.Port != nil {
		fetch = fetchRulesCmd(m.deps.Port)
	}
	tick := m.scheduleRefreshTickCmd()
	switch {
	case fetch != nil && tick != nil:
		return tea.Batch(fetch, tick)
	case fetch != nil:
		return fetch
	case tick != nil:
		return tick
	}
	return nil
}

// LastUpdated implements app.LastUpdatedStater (issue #177 v0.1.16).
func (m ListModel) LastUpdated() time.Time { return m.lastUpdated }

// StatusBadges publishes Rules screen state (v0.2.0): SORT cycle,
// FILTER echo, hscroll offset.
func (m ListModel) StatusBadges() []shared.ChromeBadge {
	var out []shared.ChromeBadge
	if m.sortBy != SortNone && m.sortDir != SortOff {
		out = append(out, shared.ChromeBadge{Key: "SORT", Value: rulesSortBadge(m.sortBy, m.sortDir)})
	}
	if m.filter != "" {
		out = append(out, shared.ChromeBadge{Key: "FILTER", Value: m.filter})
	}
	if m.hScroll > 0 {
		out = append(out, shared.ChromeBadge{Key: "hscroll", Value: strconv.Itoa(m.hScroll), Tone: shared.BadgeMuted})
	}
	return out
}

// EscapeWillAct reports whether Esc has work to do.
func (m ListModel) EscapeWillAct() bool {
	return m.filtering || m.opened || m.filter != ""
}

func rulesSortBadge(key SortKey, dir SortDir) string {
	name := ""
	switch key {
	case SortStatus:
		name = "status"
	case SortName:
		name = "name"
	default:
		return ""
	}
	switch dir {
	case SortAsc:
		return name + "↑"
	case SortDesc:
		return name + "↓"
	}
	return name
}

// scheduleRefreshTickCmd returns the auto-refresh tea.Tick.
func (m ListModel) scheduleRefreshTickCmd() tea.Cmd {
	if m.deps.Port == nil {
		return nil
	}
	return shared.ScheduleRefreshTickCmd(m.deps.RefreshInterval,
		rulesRefreshTickMsg{gen: m.refreshGen})
}

// Update handles key input and fetch results.
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.loaded {
			return m, shared.ScheduleSpinnerTickCmd(rulesSpinnerTickMsg{})
		}
		return m, nil
	case rulesLoadedMsg:
		flash := shared.LoadDiff(&m.loaded, &m.lastUpdated, &m.changedAt,
			m.rules, msg.rules, m.now(),
			func(r domain.GroupRule) string { return r.ID }, ruleTrackedEqual)
		m.rules = msg.rules
		m.lastErr = nil
		m.fetching = false
		// Once rules are loaded, fire a Cmd that resolves target
		// group IDs → names so TARGETS reads as a list of human
		// names rather than 00g_… opaque IDs (issue #163).
		var nameFetch tea.Cmd
		if !m.groupNamesFetched {
			m.groupNamesFetched = true
			ids := collectTargetGroupIDs(m.rules)
			if len(ids) > 0 && m.deps.Groups != nil {
				nameFetch = fetchGroupNamesCmd(m.deps.Groups, ids)
			}
		}
		var highlight tea.Cmd
		if flash {
			highlight = shared.ScheduleHighlightTickCmd(rulesHighlightTickMsg{})
		}
		switch {
		case nameFetch != nil && highlight != nil:
			return m, tea.Batch(nameFetch, highlight)
		case nameFetch != nil:
			return m, nameFetch
		case highlight != nil:
			return m, highlight
		}
		return m, nil
	case rulesHighlightTickMsg:
		now := m.now()
		if shared.HasFreshHighlights(m.changedAt, now) ||
			shared.HasFreshHighlights(m.failedAt, now) {
			return m, shared.ScheduleHighlightTickCmd(rulesHighlightTickMsg{})
		}
		return m, nil
	case rulesSpinnerTickMsg:
		if !shared.BumpSpinner(m.loaded, &m.spinnerFrame) {
			return m, nil
		}
		return m, shared.ScheduleSpinnerTickCmd(rulesSpinnerTickMsg{})
	case rulesErrMsg:
		m.lastErr = msg.err
		m.loaded = true
		m.fetching = false
		return m, nil
	case rulesRefreshTickMsg:
		if msg.gen != m.refreshGen || m.deps.Port == nil {
			return m, nil
		}
		m.fetching = true
		return m, tea.Batch(fetchRulesCmd(m.deps.Port), m.scheduleRefreshTickCmd())
	case shared.RefreshScreenMsg:
		if m.deps.Port == nil {
			return m, nil
		}
		m.fetching = true
		return m, fetchRulesCmd(m.deps.Port)
	case shared.ActionFailedMsg:
		if msg.TargetID == "" {
			return m, nil
		}
		if m.failedAt == nil {
			m.failedAt = map[string]time.Time{}
		}
		m.failedAt[msg.TargetID] = m.now()
		return m, shared.ScheduleHighlightTickCmd(rulesHighlightTickMsg{})
	case groupNamesLoadedMsg:
		if m.groupNames == nil {
			m.groupNames = map[string]string{}
		}
		for k, v := range msg.names {
			m.groupNames[k] = v
		}
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
	// Arrow keys map to Vim-style runes (issue #159).
	msg = shared.NormalizeArrowKey(msg)
	// Detail mode (TUI_DESIGN §3.6 + §15.7): Esc returns to the list; Tab /
	// Shift-Tab cycle through tabs; `r` toggles the Raw tab against the
	// last-visited non-Raw tab.
	if m.opened {
		switch msg.Type {
		case tea.KeyEsc:
			// #G4 v0.2.4 — Esc backs out of TARGETS focus first,
			// then closes detail (mirrors User/Group Detail extras
			// semantics).
			if m.detailTargetsFocused {
				m.detailTargetsFocused = false
				m.detailTargetCur = 0
				return m, nil
			}
			m.opened = false
			m.detail = domain.GroupRule{}
			m.detailTab = RuleDetailTabProfile
			m.detailRawReturn = RuleDetailTabProfile
			return m, nil
		case tea.KeyTab:
			m.detailTab = shared.NextTab(m.detailTab)
			return m, nil
		case tea.KeyShiftTab:
			m.detailTab = shared.PrevTab(m.detailTab)
			return m, nil
		case tea.KeyEnter:
			// #G4 / U7 v0.2.4 — drill into the focused target's
			// Group Detail. No-op when targets aren't focused so the
			// keystroke doesn't accidentally re-open this rule.
			if m.detailTargetsFocused {
				ids := m.detail.TargetGroupIDs
				if cur := m.detailTargetCur; cur >= 0 && cur < len(ids) {
					if id := ids[cur]; id != "" {
						return m, openGroupDetailCmd(id)
					}
				}
			}
			return m, nil
		case tea.KeyRunes:
			runes := string(msg.Runes)
			switch runes {
			case "]":
				// Enter or advance the TARGETS cursor.
				ids := m.detail.TargetGroupIDs
				if !m.detailTargetsFocused {
					if len(ids) == 0 {
						return m, nil
					}
					m.detailTargetsFocused = true
					m.detailTargetCur = 0
				} else if len(ids) > 0 {
					m.detailTargetCur = (m.detailTargetCur + 1) % len(ids)
				}
				return m, nil
			case "[":
				// Exit TARGETS focus back to the body.
				if m.detailTargetsFocused {
					m.detailTargetsFocused = false
					m.detailTargetCur = 0
				}
				return m, nil
			case "j":
				if m.detailTargetsFocused {
					if n := len(m.detail.TargetGroupIDs); n > 0 {
						m.detailTargetCur = (m.detailTargetCur + 1) % n
					}
					return m, nil
				}
			case "k":
				if m.detailTargetsFocused {
					if n := len(m.detail.TargetGroupIDs); n > 0 {
						m.detailTargetCur = (m.detailTargetCur - 1 + n) % n
					}
					return m, nil
				}
			case "r":
				m.detailTab, m.detailRawReturn = shared.ToggleRawTab(m.detailTab, m.detailRawReturn)
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
		// #G4 / U7 v0.2.4 — pass the TARGETS focus state so the
		// renderer marks the focused target with `▸` and surfaces a
		// footer hint.
		focusIdx := -1
		if m.detailTargetsFocused {
			focusIdx = m.detailTargetCur
		}
		return renderRuleDetailTabbedWithFocus(m.detail, m.detailTab, focusIdx, m.groupNames)
	}
	if m.lastErr != nil {
		return "Group Rules  (error)\n" + shared.ErrorPanel("group rules", m.lastErr)
	}

	tk := activeTokens()
	if !m.loaded {
		return shared.LoadingPlaceholder(m.spinnerFrame, "Loading…",
			m.chromeContentWidth(), shared.ListBodyRowBudget(m.height), tk)
	}
	rows := m.visible()

	var b strings.Builder
	// Resource label, count, and filter all live in the chrome's
	// upper divider (issues #133 + #136); the body opens straight
	// with the column header.
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatRulesColumns(
		rulesSortLabel("STATUS", m.sortBy, SortStatus, m.sortDir, tk),
		rulesSortLabel("NAME", m.sortBy, SortName, m.sortDir, tk),
		"EXPRESSION",
		"TARGETS",
		"UPDATED",
	)))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(rows), shared.ListBodyRowBudget(m.height))
	budget := end - top
	rowTarget := m.chromeContentWidth() - 2
	now := m.now()
	for i := top; i < end; i++ {
		row := m.renderRulesRow(rows[i], now, tk)
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		// v0.2.0 #182 — unified cursor pipeline.
		// v0.2.3 #193 — flash RowChanged for rows whose tracked
		// fields just changed during a refresh.
		// #U11 v0.2.4 — flash RowDanger on a recent failed action.
		changed := shared.IsRowChanged(m.changedAt, rows[i].ID, now)
		tone := shared.RowToneNone
		if shared.IsRowChanged(m.failedAt, rows[i].ID, now) {
			tone = shared.RowToneFailed
		}
		b.WriteString(shared.RenderRowCursorTone(prefix+row, rowTarget, i == m.cursor, string(rows[i].Status), changed, tone, tk))
		b.WriteString(shared.AppendScrollbarSuffix(i-top, top, budget, len(rows), tk))
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
	targets := m.formatTargets(r.TargetGroupIDs)
	updated := shared.RelativeTime(&r.LastUpdated, now)
	if r.LastUpdated.IsZero() {
		updated = "—"
	}
	expr := r.Expression
	if expr == "" {
		expr = "—"
	}
	return m.formatRulesColumns(status, r.Name, expr, targets, updated)
}

// formatTargets joins target group IDs for the TARGETS column,
// resolving IDs to group names via the in-process cache when
// available (issue #163). Falls back to the raw ID when the cache
// has no entry.
func (m ListModel) formatTargets(ids []string) string {
	if len(ids) == 0 {
		return "—"
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := m.groupNames[id]; ok {
			out = append(out, name)
		} else {
			out = append(out, id)
		}
	}
	return strings.Join(out, ", ")
}

// rulesColumnSpecs returns the column definitions:
// STATUS, NAME, EXPRESSION, TARGETS, UPDATED. Drop priorities (low
// first): EXPRESSION (1) → TARGETS (2) → UPDATED (3); STATUS / NAME
// never drop.
func rulesColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "STATUS", Kind: shared.ColumnFixed, Min: 10, DropPriority: 0, AlignCenter: true},
		{Title: "NAME", Kind: shared.ColumnFlex, Min: 22, Weight: 2, DropPriority: 0},
		{Title: "EXPRESSION", Kind: shared.ColumnFlex, Min: 24, Weight: 3, DropPriority: 1},
		{Title: "TARGETS", Kind: shared.ColumnFlex, Min: 16, Weight: 2, DropPriority: 2},
		{Title: "UPDATED", Kind: shared.ColumnFixed, Min: 10, DropPriority: 3, AlignRight: true},
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
// chrome border (2), left padding (1), cursor gutter (2), and the
// 2-cell scrollbar gutter (issue #173).
func (m ListModel) rulesInnerWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	inner := w - 2 - 1 - 2 - 2
	if inner < 20 {
		inner = 20
	}
	return inner
}

// chromeContentWidth returns the body cells the chrome reserves per
// row, used to pad the row out before the scrollbar suffix lands
// flush against the right border (issue #173).
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

// Count returns the visible/total counts for the App Shell's upper
// divider (issue #136).
func (m ListModel) Count() (visible, total int) {
	return len(m.visible()), len(m.rules)
}

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

// Actions publishes the resource-specific actions surfaced by the
// `a` action menu (issue #188 v0.2.2). Group Rules expose three
// lifecycle ops via Okta's API.
func (m ListModel) Actions() []shared.ActionItem {
	return []shared.ActionItem{
		{ID: "activate", Label: "Activate rule", Hint: "INACTIVE → ACTIVE (re-evaluates expression)"},
		{ID: "deactivate", Label: "Deactivate rule", Hint: "ACTIVE → INACTIVE"},
		{ID: "delete", Label: "Delete rule", Hint: "INACTIVE only — permanent"},
	}
}

// RunAction emits a shared.RunRuleActionMsg back into the App Shell
// when the operator picks an item from the `a` menu.
func (m ListModel) RunAction(id string) (tea.Model, tea.Cmd) {
	return m, func() tea.Msg { return shared.RunRuleActionMsg{Kind: id} }
}

// SelectedRule surfaces the active rule (the open detail target
// while m.opened is true, otherwise the cursor row) so the App Shell
// can hand it to lifecycle confirmation modals (issue #188).
func (m ListModel) SelectedRule() (domain.GroupRule, bool) {
	if m.opened {
		if m.detail.ID != "" {
			return m.detail, true
		}
	}
	if r := m.selected(); r != nil {
		return *r, true
	}
	return domain.GroupRule{}, false
}

// ruleTrackedEqual reports whether two GroupRule snapshots match on
// every field the list View renders. Tracked fields: status, name,
// expression, target group IDs, lastUpdated.
func ruleTrackedEqual(a, b domain.GroupRule) bool {
	if a.Status != b.Status {
		return false
	}
	if a.Name != b.Name || a.Expression != b.Expression {
		return false
	}
	if !stringSlicesEqual(a.TargetGroupIDs, b.TargetGroupIDs) {
		return false
	}
	if !a.LastUpdated.Equal(b.LastUpdated) {
		return false
	}
	return true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	return renderRuleDetailTabbedWithFocus(r, active, -1, nil)
}

// renderRuleDetailTabbedWithFocus is the focus-aware variant: when
// focusIdx ≥ 0 and the active tab is Pretty, the corresponding
// TARGETS row is marked with `▸` and a footer hint surfaces the
// drill-down affordance (#G4 / U7 v0.2.4). groupNames resolves IDs
// to human names when populated.
func renderRuleDetailTabbedWithFocus(r domain.GroupRule, active RuleDetailTab, focusIdx int, groupNames map[string]string) string {
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
		b.WriteString(renderRuleDetailWithFocus(r, focusIdx, groupNames))
	}
	if active == RuleDetailTabPretty && len(r.TargetGroupIDs) > 0 {
		b.WriteByte('\n')
		if focusIdx >= 0 {
			b.WriteString("Enter to drill into focused target group · j/k cycle · [ exits · Esc closes detail")
		} else {
			b.WriteString("] focuses TARGETS for drill-down")
		}
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

// renderRuleDetailWithFocus renders the same body as renderRuleDetail
// but marks the focused target group row with `▸` and resolves IDs
// to human names via groupNames when available (#G4 / U7 v0.2.4).
func renderRuleDetailWithFocus(r domain.GroupRule, focusIdx int, groupNames map[string]string) string {
	if focusIdx < 0 && groupNames == nil {
		return renderRuleDetail(r)
	}
	var b strings.Builder
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
		for i, id := range r.TargetGroupIDs {
			marker := "  - "
			if i == focusIdx {
				marker = "▸ - "
			}
			b.WriteString(marker)
			label := id
			if name, ok := groupNames[id]; ok && name != "" {
				label = name + "  (" + id + ")"
			}
			b.WriteString(label)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n⚠ Deactivating this rule would remove all memberships it created.\n")
	b.WriteString("   This action is disabled in read-only mode.\n")
	return b.String()
}

// groupNamesLoadedMsg delivers the id→name resolution for the
// TARGETS column (issue #163).
type groupNamesLoadedMsg struct{ names map[string]string }

// collectTargetGroupIDs deduplicates target group IDs across all
// rules so the resolution Cmd issues exactly one Get per group.
func collectTargetGroupIDs(rules []domain.GroupRule) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, r := range rules {
		for _, id := range r.TargetGroupIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// fetchGroupNamesCmd resolves a batch of group IDs to display names
// via GroupsPort.Get. Errors per-id are silently dropped — the
// caller renders the bare ID for any unresolved entry.
func fetchGroupNamesCmd(port domain.GroupsPort, ids []string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		out := make(map[string]string, len(ids))
		for _, id := range ids {
			g, err := port.Get(ctx, id)
			if err != nil {
				continue
			}
			if g.Profile.Name != "" {
				out[id] = g.Profile.Name
			}
		}
		return groupNamesLoadedMsg{names: out}
	}
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

// openGroupDetailCmd asks the App Shell to switch to Groups and open
// the matching group's Detail by ID — used by Rule Detail's TARGETS
// drill-down (#G4 / U7 v0.2.4).
func openGroupDetailCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenGroupDetailMsg{ID: id} }
}

var (
	_ tea.Model = ListModel{}
	_ tea.Model = DetailModel{}
)
