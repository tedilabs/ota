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
	"gopkg.in/yaml.v3"

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
	// RefreshInterval drives the auto-refresh tick (issue #177
	// v0.1.16). Zero disables auto-refresh.
	RefreshInterval time.Duration
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
	height          int
	viewportTop     int
	// sortBy / sortDir track the active column sort cycle (TUI_DESIGN §3.5).
	sortBy  SortKey
	sortDir SortDir
	ggChord shared.GChord
	// hScroll — horizontal column offset (issue #122). h/l move the
	// column slice when the natural row exceeds the viewport.
	hScroll int
	// detailMembers is the lazy-loaded member list rendered on the
	// Members detail tab (issue #142). Nil until the operator visits
	// the tab; once loaded, future Tab cycles re-show without
	// re-fetching.
	detailMembers       []domain.User
	detailMembersGroup  string // group ID the cached list belongs to
	detailMembersErr    error
	detailMembersLoaded bool

	// lastUpdated stamps the most recent successful list fetch (issue
	// #177 v0.1.16). Surfaced via LastUpdated() for the chrome.
	lastUpdated time.Time
	// refreshGen guards against stale refresh-tick Cmds firing
	// after the model was rebuilt or a refresh cycle was forced.
	refreshGen int
}

// groupsRefreshTickMsg fires the auto-refresh tick (issue #177
// v0.1.16). gen matches refreshGen; mismatches are dropped.
type groupsRefreshTickMsg struct{ gen int }

// GroupDetailTab indexes the Group Detail tab bar. v0.1.2 collapsed the
// placeholder tabs into the three structural views (Pretty / JSON / YAML).
// GroupDetailTabProfile and GroupDetailTabRaw are kept as compile-time
// aliases so v0.1.1 callers keep working.
type GroupDetailTab int

const (
	GroupDetailTabPretty GroupDetailTab = iota
	GroupDetailTabJSON
	GroupDetailTabYAML
	// GroupDetailTabMembers lists the users that belong to this group
	// (issue #142). Populated lazily — entering the tab fires a Cmd
	// against GroupsPort.Members; the result lands on the model via
	// groupMembersLoadedMsg.
	GroupDetailTabMembers
)

const (
	GroupDetailTabProfile = GroupDetailTabPretty
	GroupDetailTabRaw     = GroupDetailTabJSON
)

var groupDetailTabLabels = []string{"Pretty", "JSON", "YAML", "Members"}
var groupDetailTabCount = GroupDetailTab(len(groupDetailTabLabels))

// groupsErrMsg surfaces a fetch failure to View() (TUI_DESIGN §17).
type groupsErrMsg struct{ err error }

// groupMembersLoadedMsg / groupMembersErrMsg deliver the result of a
// Group Detail Members tab fetch (issue #142).
type groupMembersLoadedMsg struct {
	groupID string
	members []domain.User
}
type groupMembersErrMsg struct {
	groupID string
	err     error
}

// OpenDetailByIDMsg is the cross-screen drill-down request: another
// screen (User Detail Groups row Enter — issue #171) routes through
// the App Shell which forwards this msg into the Groups list. The
// list fetches the group by ID and, on success, sets m.detail +
// m.opened so the View flips to the detail surface immediately.
type OpenDetailByIDMsg struct {
	ID string
}

// groupOpenedByIDMsg / groupOpenByIDErrMsg deliver the result of an
// OpenDetailByIDMsg-triggered fetch.
type groupOpenedByIDMsg struct{ group domain.Group }
type groupOpenByIDErrMsg struct{ err error }

// NewListModel constructs a ListModel.
func NewListModel(deps Deps) ListModel {
	return ListModel{
		deps:   deps,
		groups: deps.InitialGroups,
		width:  deps.Width,
		height: deps.Height,
	}
}

// Init fetches the groups list on entry (REQ-R02 AC-1) and schedules
// the first auto-refresh tick (issue #177 v0.1.16).
func (m ListModel) Init() tea.Cmd {
	var fetch tea.Cmd
	if len(m.groups) == 0 && m.deps.Port != nil {
		fetch = fetchGroupsCmd(m.deps.Port)
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

// scheduleRefreshTickCmd returns a tea.Tick that fires
// groupsRefreshTickMsg after RefreshInterval. nil disables.
func (m ListModel) scheduleRefreshTickCmd() tea.Cmd {
	if m.deps.RefreshInterval <= 0 || m.deps.Port == nil {
		return nil
	}
	gen := m.refreshGen
	return tea.Tick(m.deps.RefreshInterval, func(time.Time) tea.Msg {
		return groupsRefreshTickMsg{gen: gen}
	})
}

// Update handles key input and fetch results.
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case groupsLoadedMsg:
		m.groups = msg.groups
		m.lastErr = nil
		m.lastUpdated = m.now()
		return m, nil
	case groupsErrMsg:
		m.lastErr = msg.err
		return m, nil
	case groupsRefreshTickMsg:
		if msg.gen != m.refreshGen || m.deps.Port == nil {
			return m, nil
		}
		return m, tea.Batch(fetchGroupsCmd(m.deps.Port), m.scheduleRefreshTickCmd())
	case groupMembersLoadedMsg:
		// Only accept if it matches the currently-open group — a stale
		// fetch from a previously-opened detail must not overwrite.
		if m.opened && m.detail.ID == msg.groupID {
			m.detailMembers = msg.members
			m.detailMembersGroup = msg.groupID
			m.detailMembersErr = nil
			m.detailMembersLoaded = true
		}
		return m, nil
	case groupMembersErrMsg:
		if m.opened && m.detail.ID == msg.groupID {
			m.detailMembersErr = msg.err
			m.detailMembersLoaded = true
		}
		return m, nil
	case OpenDetailByIDMsg:
		// Issue #171: another screen requested drill-down by ID.
		// Fetch the group and surface the detail surface inline.
		if msg.ID == "" || m.deps.Port == nil {
			return m, nil
		}
		return m, fetchGroupByIDCmd(m.deps.Port, msg.ID)
	case groupOpenedByIDMsg:
		m.detail = msg.group
		m.opened = true
		m.detailTab = GroupDetailTabProfile
		m.detailRawReturn = GroupDetailTabProfile
		m.detailMembers = nil
		m.detailMembersGroup = ""
		m.detailMembersErr = nil
		m.detailMembersLoaded = false
		return m, nil
	case groupOpenByIDErrMsg:
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
	// Arrow keys map to Vim-style runes so the rune switch below
	// handles both (issue #159).
	msg = shared.NormalizeArrowKey(msg)
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
			m.detailMembers = nil
			m.detailMembersGroup = ""
			m.detailMembersErr = nil
			m.detailMembersLoaded = false
			return m, nil
		case tea.KeyTab:
			m.detailTab = (m.detailTab + 1) % groupDetailTabCount
			return m.maybeFetchMembers()
		case tea.KeyShiftTab:
			m.detailTab = (m.detailTab + groupDetailTabCount - 1) % groupDetailTabCount
			return m.maybeFetchMembers()
		case tea.KeyRunes:
			runes := string(msg.Runes)
			if runes == "m" {
				// Direct shortcut to the Members tab (issue #142).
				m.detailTab = GroupDetailTabMembers
				return m.maybeFetchMembers()
			}
			if runes == "r" {
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

	// Esc on the list (after the `/` prompt has closed) clears any
	// active filter (issue #131).
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
			// `gg` chord (TUI_DESIGN §3.2). First `g` arms; second within
			// the chord window jumps to the top.
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
			specs := groupsColumnSpecs()
			max := shared.MaxHScroll(specs, m.groupsInnerWidth(), 2)
			if m.hScroll < max {
				m.hScroll++
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
		return renderGroupDetailTabbed(m.detail, m.detailTab,
			m.detailMembers, m.detailMembersLoaded, m.detailMembersErr)
	}
	if m.lastErr != nil {
		return "Groups  (error)\n" + shared.ErrorPanel("groups", m.lastErr)
	}

	tk := activeTokens()
	rows := m.visible()

	var b strings.Builder
	// Resource label, count, and filter all live in the chrome's
	// upper divider (issues #133 + #136); the body opens straight
	// with the column header.
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatGroupsColumns(
		"TYPE",
		groupsSortLabel("NAME", m.sortBy, SortName, m.sortDir, tk),
		"DESCRIPTION",
		"MEMBERS",
		"UPDATED",
	)))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(rows), shared.ListBodyRowBudget(m.height))
	budget := end - top
	rowTarget := m.chromeContentWidth() - 2
	for i := top; i < end; i++ {
		row := m.renderGroupsRow(rows[i], m.now(), tk)
		var composed string
		if i == m.cursor {
			composed = "▸ " + row
		} else {
			composed = "  " + row
		}
		composed = shared.PadOrTruncateVisible(composed, rowTarget)
		if i == m.cursor {
			composed = tk.Accent.Render(composed)
		}
		b.WriteString(composed)
		b.WriteString(shared.AppendScrollbarSuffix(i-top, top, budget, len(rows), tk))
		b.WriteByte('\n')
	}
	return b.String()
}

// chromeContentWidth returns the body cells the chrome reserves per
// row — width minus chrome left border, left padding, and right
// border. The list pads each row out to this width minus 2 cells so
// the scrollbar gutter (" ▌"/" │") sits flush against the chrome's
// right border (issue #173).
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

// renderGroupsRow formats one group as a row.
func (m ListModel) renderGroupsRow(g domain.Group, now time.Time, tk shared.Tokens) string {
	// v0.1.12 surfaces the full type label (OKTA / APP / BUILT_IN)
	// instead of the one-letter mono so operators don't have to
	// memorize the legend (issue #160).
	typeBadge := shared.GroupTypeBadge(string(g.Type), tk).Label
	updated := shared.RelativeTime(&g.LastUpdated, now)
	if g.LastUpdated.IsZero() {
		updated = "—"
	}
	// MEMBERS — populated when the list query enabled expand=stats
	// (issue #161). nil means the API didn't surface a count, so
	// render "—" rather than a misleading "0".
	members := "—"
	if g.MemberCount != nil {
		members = itoaG(*g.MemberCount)
	}
	return m.formatGroupsColumns(typeBadge, g.Profile.Name, g.Profile.Description, members, updated)
}

// groupsColumnSpecs returns the column definitions for the Groups
// list. v0.1.7 dropped the TAGS column (issue #141): the user pointed
// out Groups don't actually carry a tags attribute — the column was
// surfacing derived [RULE] / [SYS] flags under a name that promised
// something else. Either fold those flags into TYPE later or expose
// them on Group Detail; for now the list is just the four core
// fields.
func groupsColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "TYPE", Kind: shared.ColumnFixed, Min: 8, DropPriority: 0},
		{Title: "NAME", Kind: shared.ColumnFlex, Min: 18, Weight: 2, DropPriority: 0},
		{Title: "DESCRIPTION", Kind: shared.ColumnFlex, Min: 16, Weight: 2, DropPriority: 2},
		{Title: "MEMBERS", Kind: shared.ColumnFixed, Min: 7, DropPriority: 4, AlignRight: true},
		{Title: "UPDATED", Kind: shared.ColumnFixed, Min: 10, DropPriority: 3, AlignRight: true},
	}
}

// formatGroupsColumns lays out TYPE / NAME / DESCRIPTION / UPDATED / TAGS
// per the TUI_DESIGN §15.0a Min/Weight + DropPriority model. When the
// natural row overflows the viewport, switch to LayoutColumnsHScroll so
// h/l can pan across columns instead of dropping them.
func (m ListModel) formatGroupsColumns(cells ...string) string {
	specs := groupsColumnSpecs()
	innerWidth := m.groupsInnerWidth()
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

// groupsInnerWidth mirrors users.usersInnerWidth — body width after the
// chrome border (2), left padding (1), cursor gutter (2), and the
// 2-cell scrollbar gutter (issue #173).
func (m ListModel) groupsInnerWidth() int {
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

// groupsSortLabel appends a coloured ↑ / ↓ to title when the active key
// matches. asc → green, desc → red (issue #118).
func groupsSortLabel(title string, active, key SortKey, dir SortDir, tk shared.Tokens) string {
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
	return len(m.visible()), len(m.groups)
}

// cursorBy moves the cursor by delta rows, clamped to the visible range.
// Used by Ctrl-f/b/d/u page nav handlers (issue #119).
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

// max returns the larger of a and b. Local copy keeps the file
// dependency-free and matches the style users/list.go already uses.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
func (m DetailModel) View() string {
	return renderGroupDetailTabbed(m.group, GroupDetailTabProfile, nil, false, nil)
}

// renderGroupDetailTabbed wraps the legacy single-surface view with a
// §15.7 v1.2.0 tab bar. Pretty / JSON / YAML tabs render off the
// passed group; the Members tab renders the (lazily fetched) member
// list passed in via members + loaded.
func renderGroupDetailTabbed(g domain.Group, active GroupDetailTab, members []domain.User, loaded bool, memberErr error) string {
	var b strings.Builder
	b.WriteString("Group Detail\n")
	b.WriteString(renderGroupTabBar(active))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 78))
	b.WriteByte('\n')
	switch active {
	case GroupDetailTabJSON:
		b.WriteString(renderGroupRawTab(g))
	case GroupDetailTabYAML:
		b.WriteString(renderGroupYAMLTab(g))
	case GroupDetailTabMembers:
		b.WriteString(renderGroupMembersTab(members, loaded, memberErr))
	default:
		b.WriteString(renderGroupDetail(g))
	}
	return b.String()
}

// renderGroupMembersTab renders the lazily-fetched member list for
// the Members detail tab (issue #142). Three states:
//
//   - !loaded && err == nil: "loading…"
//   - err != nil: ErrorPanel with the failure detail
//   - loaded: a column table of members (status badge + login)
func renderGroupMembersTab(members []domain.User, loaded bool, err error) string {
	if err != nil {
		return shared.ErrorPanel("group members", err) + "\n"
	}
	if !loaded {
		return "loading members…\n"
	}
	if len(members) == 0 {
		return "(group has no members)\n"
	}
	tk := activeTokens()
	var b strings.Builder
	b.WriteString("Members  ")
	b.WriteString(itoaG(len(members)))
	b.WriteByte('\n')
	for _, u := range members {
		status := shared.UserStatusBadge(string(u.Status), tk).Render(tk)
		login := u.Profile.Login
		if login == "" {
			login = u.ID
		}
		b.WriteString("  ")
		b.WriteString(status)
		b.WriteString("  ")
		b.WriteString(login)
		b.WriteByte('\n')
	}
	return b.String()
}

// itoaG is a tiny strconv shim local to groups (matches itoa shims
// in the other list packages — keeps strconv out of the import set).
func itoaG(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// renderGroupYAMLTab marshals the same groupJSONShape projection as the
// JSON tab through gopkg.in/yaml.v3, with a 2-space indent (issue #109)
// and shared syntax highlighting (issue #110).
func renderGroupYAMLTab(g domain.Group) string {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(groupJSONShapeFor(g)); err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	if err := enc.Close(); err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	body := strings.TrimRight(buf.String(), "\n")
	return shared.HighlightYAML(body, activeTokens()) + "\n"
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
// straight from the domain projection. v0.1.3 adds syntax highlighting.
func renderGroupRawTab(g domain.Group) string {
	buf, err := json.MarshalIndent(groupJSONShapeFor(g), "", "  ")
	if err != nil {
		return "(raw render error: " + err.Error() + ")\n"
	}
	return shared.HighlightJSON(string(buf), activeTokens()) + "\n"
}

// groupJSONShapeFor centralises the deterministic projection so JSON and
// YAML tabs render identical data.
func groupJSONShapeFor(g domain.Group) groupJSONShape {
	return groupJSONShape{
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
}

// groupJSONShape is a stable projection of domain.Group used by the JSON
// and YAML tabs. Field order is fixed so the golden file doesn't depend
// on Go map iteration order.
type groupJSONShape struct {
	ID              string            `json:"id" yaml:"id"`
	Type            string            `json:"type" yaml:"type"`
	DynamicTargeted bool              `json:"dynamicTargeted" yaml:"dynamicTargeted"`
	Profile         groupProfileShape `json:"profile" yaml:"profile"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	LastUpdated     string            `json:"lastUpdated,omitempty" yaml:"lastUpdated,omitempty"`
}

type groupProfileShape struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
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

// fetchGroupByIDCmd resolves a group via GroupsPort.Get for the
// cross-screen drill-down (issue #171: User Detail Groups row Enter).
// On success returns groupOpenedByIDMsg so the list can flip into
// detail mode.
func fetchGroupByIDCmd(port domain.GroupsPort, id string) tea.Cmd {
	return func() tea.Msg {
		if port == nil {
			return groupOpenByIDErrMsg{err: domain.ErrNotFound}
		}
		ctx := context.Background()
		g, err := port.Get(ctx, id)
		if err != nil {
			return groupOpenByIDErrMsg{err: err}
		}
		return groupOpenedByIDMsg{group: g}
	}
}

// fetchGroupMembersCmd drains GroupsPort.Members for the given group
// and returns a groupMembersLoadedMsg / groupMembersErrMsg. The
// groupID is round-tripped through the message so a stale fetch from
// a previously-opened detail can't overwrite the current one.
func fetchGroupMembersCmd(port domain.GroupsPort, groupID string) tea.Cmd {
	return func() tea.Msg {
		if port == nil {
			return groupMembersErrMsg{groupID: groupID, err: domain.ErrNotFound}
		}
		ctx := context.Background()
		iter, err := port.Members(ctx, domain.GroupMembersQuery{GroupID: groupID, Limit: 200})
		if err != nil {
			return groupMembersErrMsg{groupID: groupID, err: err}
		}
		defer iter.Close()
		var out []domain.User
		for {
			u, hasMore, err := iter.Next(ctx)
			if err != nil {
				return groupMembersErrMsg{groupID: groupID, err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, u)
		}
		return groupMembersLoadedMsg{groupID: groupID, members: out}
	}
}

// maybeFetchMembers fires a Cmd to load the Members tab for the
// currently-open group when (a) the operator is on the Members tab
// and (b) the cache is empty or belongs to a different group. No-op
// otherwise so flipping back to a previously-loaded Members view
// doesn't burn rate-limit budget.
func (m ListModel) maybeFetchMembers() (tea.Model, tea.Cmd) {
	if m.detailTab != GroupDetailTabMembers {
		return m, nil
	}
	if m.detailMembersGroup == m.detail.ID && m.detailMembersLoaded {
		return m, nil
	}
	// Reset cache so the View renders a "loading…" placeholder while
	// the Cmd is in flight.
	m.detailMembersGroup = m.detail.ID
	m.detailMembersLoaded = false
	m.detailMembers = nil
	m.detailMembersErr = nil
	return m, fetchGroupMembersCmd(m.deps.Port, m.detail.ID)
}

var (
	_ tea.Model = ListModel{}
	_ tea.Model = DetailModel{}
)
