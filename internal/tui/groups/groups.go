// Package groups implements the Groups list/detail/members Screen Models
// (SCR-020, SCR-021). See docs/TUI_DESIGN.md §4.
package groups

import (
	"context"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
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
	lastErr   error
	width     int
}

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

// View renders SCR-020 (TUI_DESIGN §15.3 / §16.5).
//
// Columns: TYPE / NAME / DESCRIPTION / UPDATED / TAGS, two-space gutters.
// TAGS column carries [RULE] (DynamicTargeted), [SYS] (BUILT_IN), [LARGE]
// (Everyone-style). The chrome (HeaderBar / StatusBar) is composed by the App
// Shell.
func (m ListModel) View() string {
	if m.opened {
		return renderGroupDetail(m.detail)
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
	b.WriteString(m.formatGroupsColumns("TYPE", "NAME", "DESCRIPTION", "UPDATED", "TAGS"))
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

// formatGroupsColumns lays out the 5 columns (TUI_DESIGN §15.3) with
// responsive drop:
//
//   - W ≥ 120 : all 5 columns
//   - 100..119: drop TAGS
//   - 90..99  : drop TAGS + DESCRIPTION
//   - 80..89  : TYPE + NAME + UPDATED only
//   - <80     : TYPE + NAME only
func (m ListModel) formatGroupsColumns(typeBadge, name, desc, updated, tags string) string {
	w := m.width
	const (
		wType    = 4
		wName    = 24
		wDesc    = 28
		wUpdated = 10
		wTags    = 12
	)
	switch {
	case w >= 120 || w == 0:
		return padRight(typeBadge, wType) + "  " + padRight(name, wName) + "  " +
			padRight(shared.Truncate(desc, wDesc), wDesc) + "  " +
			padLeft(updated, wUpdated) + "  " + padRight(tags, wTags)
	case w >= 100:
		return padRight(typeBadge, wType) + "  " + padRight(name, wName) + "  " +
			padRight(shared.Truncate(desc, wDesc), wDesc) + "  " +
			padLeft(updated, wUpdated)
	case w >= 90:
		return padRight(typeBadge, wType) + "  " + padRight(name, wName) + "  " +
			padLeft(updated, wUpdated)
	case w >= 80:
		return padRight(typeBadge, wType) + "  " + padRight(name, 30) + "  " +
			padLeft(updated, wUpdated)
	default:
		return padRight(typeBadge, wType) + "  " + padRight(name, max(0, w-8))
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

// padRight / padLeft mirror the Users helpers — kept package-local to avoid
// exporting layout helpers from shared until we settle on a public API.
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

func (m ListModel) visible() []domain.Group {
	if m.filter == "" {
		return m.groups
	}
	needle := strings.ToLower(m.filter)
	out := make([]domain.Group, 0, len(m.groups))
	for _, g := range m.groups {
		if strings.Contains(strings.ToLower(g.Profile.Name), needle) {
			out = append(out, g)
		}
	}
	return out
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
func (m DetailModel) View() string { return renderGroupDetail(m.group) }

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
