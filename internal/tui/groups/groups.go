// Package groups implements the Groups list/detail/members Screen Models
// (SCR-020, SCR-021). See docs/TUI_DESIGN.md §4.
package groups

import (
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

// SortKey identifies the column the user has selected via Shift+letter
// (TUI_DESIGN §3.5a). Groups only honours SortName in MVP; other Shift
// chords are no-ops at the handler level.
type SortKey int

const (
	SortNone SortKey = iota
	SortName
)

// SortDir is the on/off cycle direction (off → asc → desc → off).
type SortDir int

const (
	SortOff SortDir = iota
	SortAsc
	SortDesc
)

// Deps bundles dependencies shared by Groups screens.
type Deps struct {
	Port    domain.GroupsPort
	Clock   clock.Clock
	Logger  *slog.Logger
	Keys    keys.ResolvedMap
	Width   int
	Height  int
	// InitialGroups lets tests seed without invoking the port.
	InitialGroups []domain.Group
}

// --- List --------------------------------------------------------------------

// groupsLoadedMsg delivers the result of the initial fetch.
type groupsLoadedMsg struct{ groups []domain.Group }

// ListModel is SCR-020.
type ListModel struct {
	deps      Deps
	groups    []domain.Group
	cursor    int
	filter    string
	filtering bool
	opened    bool
	detail    domain.Group
	// detailTab tracks the active Detail tab while m.opened is true
	// (TUI_DESIGN §15.7 v1.2.0).
	detailTab GroupDetailTab
	// detailRawReturn is the tab `r` jumped from (Raw toggle target).
	detailRawReturn GroupDetailTab
	lastErr         error
	width           int
	// sortBy / sortDir track the active column sort cycle (TUI_DESIGN §3.5).
	sortBy  SortKey
	sortDir SortDir
}

// GroupDetailTab indexes the Group Detail tab bar (TUI_DESIGN §15.7 v1.2.0).
type GroupDetailTab int

const (
	GroupDetailTabProfile GroupDetailTab = iota
	GroupDetailTabMembers
	GroupDetailTabApps
	GroupDetailTabRules
	GroupDetailTabRaw
)

var groupDetailTabLabels = []string{"Profile", "Members", "Apps", "Rules", "Raw"}
var groupDetailTabCount = GroupDetailTab(len(groupDetailTabLabels))

// groupsErrMsg surfaces a fetch failure to View() (TUI_DESIGN §17).
type groupsErrMsg struct{ err error }

// NewListModel constructs a ListModel.
func NewListModel(deps Deps) ListModel {
	return ListModel{deps: deps, groups: deps.InitialGroups, width: deps.Width}
}

// Init fetches the groups list on entry (REQ-R02 AC-1).
func (m ListModel) Init() tea.Cmd {
	if len(m.groups) > 0 || m.deps.Port == nil {
		return nil
	}
	return fetchGroupsCmd(m.deps.Port)
}

// Update handles key input and fetch results.
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case groupsLoadedMsg:
		m.groups = msg.groups
		m.lastErr = nil
		return m, nil
	case groupsErrMsg:
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
			m.detail = domain.Group{}
			m.detailTab = GroupDetailTabProfile
			m.detailRawReturn = GroupDetailTabProfile
			return m, nil
		case tea.KeyTab:
			m.detailTab = (m.detailTab + 1) % groupDetailTabCount
			return m, nil
		case tea.KeyShiftTab:
			m.detailTab = (m.detailTab + groupDetailTabCount - 1) % groupDetailTabCount
			return m, nil
		case tea.KeyRunes:
			if string(msg.Runes) == "r" {
				if m.detailTab == GroupDetailTabRaw {
					m.detailTab = m.detailRawReturn
				} else {
					m.detailRawReturn = m.detailTab
					m.detailTab = GroupDetailTabRaw
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
		case "N":
			// §3.5a — Groups: Shift+N sorts by NAME. Other Shift chords are
			// no-ops on Groups (S / L / C have no matching column).
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

// cycleSort advances the sort state per TUI_DESIGN §3.5: same key cycles
// off → asc → desc → off; a different key resets cursor and starts at asc.
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

// View renders SCR-020 (TUI_DESIGN §15.3 / §16.5).
//
// Columns: TYPE / NAME / DESCRIPTION / UPDATED / TAGS, two-space gutters.
// TAGS column carries [RULE] (DynamicTargeted), [SYS] (BUILT_IN), [LARGE]
// (Everyone-style). The chrome (HeaderBar / StatusBar) is composed by the App
// Shell.
func (m ListModel) View() string {
	if m.opened {
		return renderGroupDetailTabbed(m.detail, m.detailTab)
	}
	if m.lastErr != nil {
		return "Groups  (error)\n" + shared.ErrorPanel("groups", m.lastErr)
	}

	tk := activeTokens()
	rows := m.visible()

	var b strings.Builder
	count := groupsCounter(len(rows), len(m.groups))
	b.WriteString("Groups  ")
	b.WriteString(count)
	if m.filter != "" {
		b.WriteString(" · q=\"" + m.filter + "\"")
	}
	b.WriteByte('\n')
	if m.filtering {
		b.WriteString("filter: " + m.filter + "\n")
	}
	b.WriteString(m.formatGroupsColumns(
		"TYPE",
		groupsSortLabel("NAME", m.sortBy, SortName, m.sortDir),
		"DESCRIPTION",
		"UPDATED",
		"TAGS",
	))
	b.WriteByte('\n')
	for i, g := range rows {
		row := m.renderGroupsRow(g, m.now(), tk)
		if i == m.cursor {
			row = "> " + row
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}
	return b.String()
}

// renderGroupsRow formats one group as a row.
func (m ListModel) renderGroupsRow(g domain.Group, now time.Time, tk shared.Tokens) string {
	typeBadge := shared.GroupTypeBadge(string(g.Type), tk).Mono
	updated := shared.RelativeTime(&g.LastUpdated, now)
	if g.LastUpdated.IsZero() {
		updated = "—"
	}
	tags := groupTags(g)
	return m.formatGroupsColumns(typeBadge, g.Profile.Name, g.Profile.Description, updated, tags)
}

// groupTags concatenates the badge column for a group.
func groupTags(g domain.Group) string {
	var parts []string
	if g.DynamicTargeted {
		parts = append(parts, "[RULE]")
	}
	if g.Type == domain.GroupTypeBuiltIn {
		parts = append(parts, "[SYS]", "[LARGE]")
	}
	return strings.Join(parts, "")
}

// groupsColumnSpecs returns the §15.0a.3 column definitions in declaration
// order: TYPE, NAME, DESCRIPTION, UPDATED, TAGS. Drop priorities (low first):
// TAGS (1) → DESCRIPTION (2) → UPDATED (3); TYPE / NAME never drop.
func groupsColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "TYPE", Kind: shared.ColumnFixed, Min: 4, DropPriority: 0},
		{Title: "NAME", Kind: shared.ColumnFlex, Min: 18, Weight: 2, DropPriority: 0},
		{Title: "DESCRIPTION", Kind: shared.ColumnFlex, Min: 16, Weight: 2, DropPriority: 2},
		{Title: "UPDATED", Kind: shared.ColumnFixed, Min: 10, DropPriority: 3, AlignRight: true},
		{Title: "TAGS", Kind: shared.ColumnFlex, Min: 10, Weight: 1, DropPriority: 1},
	}
}

// formatGroupsColumns lays out TYPE / NAME / DESCRIPTION / UPDATED / TAGS
// per the TUI_DESIGN §15.0a Min/Weight + DropPriority model.
func (m ListModel) formatGroupsColumns(cells ...string) string {
	specs := groupsColumnSpecs()
	innerWidth := m.groupsInnerWidth()
	widths := shared.LayoutColumns(specs, innerWidth, 2)

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

// groupsInnerWidth mirrors users.usersInnerWidth — body width after the
// chrome border (2), left padding (1), and cursor gutter (2).
func (m ListModel) groupsInnerWidth() int {
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

// groupsCounter returns "N of M" when no filter, else "N of M".
func groupsCounter(visible, total int) string {
	return strconvI(visible) + " of " + strconvI(total)
}

func strconvI(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// activeTokens picks the right token set per NO_COLOR.
func activeTokens() shared.Tokens {
	if shared.MonochromeEnabled() {
		return shared.Monochrome()
	}
	return shared.Dark()
}

// now returns the current time, preferring the injected clock.
func (m ListModel) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

func (m ListModel) visible() []domain.Group {
	var out []domain.Group
	if m.filter == "" {
		out = append(out, m.groups...)
	} else {
		needle := strings.ToLower(m.filter)
		out = make([]domain.Group, 0, len(m.groups))
		for _, g := range m.groups {
			if strings.Contains(strings.ToLower(g.Profile.Name), needle) {
				out = append(out, g)
			}
		}
	}
	if m.sortBy != SortNone && m.sortDir != SortOff {
		sortGroupsByKey(out, m.sortBy, m.sortDir)
	}
	return out
}

// sortGroupsByKey applies a stable sort honouring §3.5a Groups: only
// SortName is mapped (case-insensitive sort on Profile.Name).
func sortGroupsByKey(xs []domain.Group, key SortKey, dir SortDir) {
	if key != SortName {
		return
	}
	sort.SliceStable(xs, func(i, j int) bool {
		ai := strings.ToLower(xs[i].Profile.Name)
		bj := strings.ToLower(xs[j].Profile.Name)
		if dir == SortDesc {
			return ai > bj
		}
		return ai < bj
	})
}

// groupsSortLabel appends "↑" / "↓" to title when the active key matches.
func groupsSortLabel(title string, active, key SortKey, dir SortDir) string {
	if active != key || dir == SortOff {
		return title
	}
	switch dir {
	case SortAsc:
		return title + "↑"
	case SortDesc:
		return title + "↓"
	}
	return title
}

func (m ListModel) selected() *domain.Group {
	vs := m.visible()
	if m.cursor < 0 || m.cursor >= len(vs) {
		return nil
	}
	return &vs[m.cursor]
}

// --- Detail ------------------------------------------------------------------

// DetailModel is SCR-021.
type DetailModel struct {
	deps  Deps
	group domain.Group
}

// NewDetailModel constructs a DetailModel.
func NewDetailModel(deps Deps, g domain.Group) DetailModel {
	return DetailModel{deps: deps, group: g}
}

// Init implements tea.Model.
func (m DetailModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m DetailModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

// View renders the group detail with profile + type + membership hint.
func (m DetailModel) View() string { return renderGroupDetailTabbed(m.group, GroupDetailTabProfile) }

// renderGroupDetailTabbed wraps the legacy single-surface view with a
// §15.7 v1.2.0 tab bar. The Profile tab body remains the v0.1.0 layout;
// the Raw tab serialises domain.Group as JSON. Other tabs (Members /
// Apps / Rules) surface a placeholder note until v0.2 fills them.
func renderGroupDetailTabbed(g domain.Group, active GroupDetailTab) string {
	var b strings.Builder
	b.WriteString("Group Detail\n")
	b.WriteString(renderGroupTabBar(active))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 78))
	b.WriteByte('\n')
	switch active {
	case GroupDetailTabRaw:
		b.WriteString(renderGroupRawTab(g))
	default:
		b.WriteString(renderGroupDetail(g))
	}
	return b.String()
}

func renderGroupTabBar(active GroupDetailTab) string {
	var parts []string
	for i, label := range groupDetailTabLabels {
		if GroupDetailTab(i) == active {
			parts = append(parts, "["+label+"]")
		} else {
			parts = append(parts, "[ "+label+" ]")
		}
	}
	return strings.Join(parts, " ")
}

// renderGroupRawTab returns the §15.7 v1.2.0 Raw JSON tab content.
// Groups carry no PII so no mask wrapping is needed; the marshal is
// straight from the domain projection.
func renderGroupRawTab(g domain.Group) string {
	out := groupJSONShape{
		ID:              g.ID,
		Type:            string(g.Type),
		DynamicTargeted: g.DynamicTargeted,
		Profile: groupProfileShape{
			Name:        g.Profile.Name,
			Description: g.Profile.Description,
		},
		Created:     formatJSONTime(g.Created),
		LastUpdated: formatJSONTime(g.LastUpdated),
	}
	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "(raw render error: " + err.Error() + ")\n"
	}
	return string(buf) + "\n"
}

// groupJSONShape is a stable projection of domain.Group used by the Raw
// tab. Field order is fixed so the golden file doesn't depend on Go map
// iteration order.
type groupJSONShape struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	DynamicTargeted bool              `json:"dynamicTargeted"`
	Profile         groupProfileShape `json:"profile"`
	Created         string            `json:"created,omitempty"`
	LastUpdated     string            `json:"lastUpdated,omitempty"`
}

type groupProfileShape struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func formatJSONTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func renderGroupDetail(g domain.Group) string {
	var b strings.Builder
	b.WriteString("Group Detail\n")
	b.WriteString("  id:          ")
	b.WriteString(g.ID)
	b.WriteString("\n")
	b.WriteString("  name:        ")
	b.WriteString(g.Profile.Name)
	b.WriteString("\n")
	b.WriteString("  type:        ")
	b.WriteString(string(g.Type))
	b.WriteString("\n")
	if g.Profile.Description != "" {
		b.WriteString("  description: ")
		b.WriteString(g.Profile.Description)
		b.WriteString("\n")
	}
	if g.DynamicTargeted {
		b.WriteString("  [RULE]       targeted by one or more Group Rules\n")
	}
	if g.Type == domain.GroupTypeBuiltIn {
		b.WriteString("  [LARGE]      built-in group — member list may be very large\n")
	}
	b.WriteString("\nMembers tab: press `l` or Tab (not implemented in MVP stub).\n")
	return b.String()
}

// --- Cmd factories -----------------------------------------------------------

func fetchGroupsCmd(port domain.GroupsPort) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		iter, err := port.List(ctx, domain.GroupsQuery{Limit: 200})
		if err != nil {
			return groupsErrMsg{err: err}
		}
		defer iter.Close()
		var out []domain.Group
		for {
			g, hasMore, err := iter.Next(ctx)
			if err != nil {
				return groupsErrMsg{err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, g)
		}
		return groupsLoadedMsg{groups: out}
	}
}

var (
	_ tea.Model = ListModel{}
	_ tea.Model = DetailModel{}
)
