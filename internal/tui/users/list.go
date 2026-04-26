package users

import (
	"context"
	"fmt"
	"strconv"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// SortKey identifies the column the user has selected via Shift+letter.
// SortNone is the default — rows render in fetch order.
type SortKey int

// Sort keys for the Users list (TUI_DESIGN §3.5a).
const (
	SortNone SortKey = iota
	SortStatus
	SortName // Users → Profile.Login
	SortLastLogin
	SortCreated // Users → StatusChanged (fallback Created per §3.5a)
)

// SortDir is the on/off cycle direction (off → asc → desc → off).
type SortDir int

const (
	SortOff SortDir = iota
	SortAsc
	SortDesc
)

// Deps bundles ListModel's dependencies (CONVENTIONS §8.1).
type Deps struct {
	Port   domain.UsersPort
	Clock  clock.Clock
	Logger *slog.Logger
	Keys   keys.ResolvedMap
	Width  int
	Height int
	// InitialUsers is an optional seed for tests (instead of a SetUsers setter).
	InitialUsers []domain.User
}

// ListModel is the SCR-010 Users list.
type ListModel struct {
	deps       Deps
	users      []domain.User
	cursor     int
	filter     string
	filtering  bool // `/` prompt open
	opened     bool // detail view active
	detailUser domain.User
	// detailTab tracks the active Detail tab while m.opened is true.
	// Profile (DetailTabProfile) is the entry tab per TUI_DESIGN §3.6.
	detailTab DetailTab
	// detailRawReturn is the tab to fall back to when the operator
	// presses `r` while already on Raw — see TUI_DESIGN §3.6 r-toggle.
	detailRawReturn DetailTab
	// detailLine is the active line cursor inside the detail body.
	// Drives j/k navigation and the anchor for Vim Visual selection.
	detailLine int
	// detailVisual / detailVisualAnchor power line-based Visual mode.
	// `v` enters Visual, j/k extend, `y` copies, Esc cancels.
	detailVisual       bool
	detailVisualAnchor int
	// detailToast is a transient one-line message shown above the body
	// (e.g. "5 lines copied"). Cleared on the next key press.
	detailToast string
	// detailUnmasked is the per-field PII unmask flag set, persisted on
	// the ListModel so it survives DetailModel reconstruction every render
	// (issue #115). Toggled by :unmask <field> / :mask palette commands.
	detailUnmasked  map[string]bool
	lastErr         error
	// width is the most recent terminal width seen via WindowSizeMsg. Drives
	// responsive column drop per TUI_DESIGN §15.2.
	width int
	// height is the most recent terminal height. Used (with the chrome
	// reservation) to compute how many rows the body can show without
	// pushing the chrome header off-screen.
	height int
	// viewportTop is the index of the first row currently rendered. Slides
	// with the cursor to keep the selection inside the visible window.
	viewportTop int
	// sortBy / sortDir track the active column sort cycle (TUI_DESIGN §3.5).
	// SortNone / SortOff means render rows in fetch order.
	sortBy  SortKey
	sortDir SortDir
	// ggChord captures the Vim `gg` two-press chord — see shared.GChord.
	ggChord shared.GChord
}

// usersLoadedMsg delivers the result of the initial fetch.
type usersLoadedMsg struct{ users []domain.User }

// usersErrMsg delivers a fetch failure to the model so the View can surface
// it via the inline error panel (TUI_DESIGN §17.1 / Phase 6d-6).
type usersErrMsg struct{ err error }

// userOpenedMsg delivers the result of a detail fetch.
type userOpenedMsg struct{ user domain.User }

// NewListModel constructs a ListModel.
func NewListModel(deps Deps) ListModel {
	return ListModel{
		deps:   deps,
		users:  deps.InitialUsers,
		width:  deps.Width,
		height: deps.Height,
	}
}

// Init kicks off the initial List call (REQ-R01 AC-1).
func (m ListModel) Init() tea.Cmd {
	if len(m.users) > 0 || m.deps.Port == nil {
		return nil
	}
	return fetchUsersCmd(m.deps.Port)
}

// Update handles key presses, the list fetch Msg, and the detail fetch Msg.
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case usersLoadedMsg:
		m.users = msg.users
		m.lastErr = nil
		return m, nil
	case usersErrMsg:
		m.lastErr = msg.err
		return m, nil
	case userOpenedMsg:
		m.detailUser = msg.user
		m.opened = true
		// v0.1.1: detail mode is rendered inline by the same ListModel — see
		// View()'s `m.opened` branch. The earlier tea.Quit shortcut, used by
		// teatest harnesses to drain final output, is gone now that operators
		// can press `d` repeatedly and Esc back to the list. v0.2 will replace
		// this with App Shell OpenResourceMsg routing.
		return m, nil
	case shared.UnmaskFieldMsg:
		// :unmask <field> from the App Shell palette (issue #115). Only
		// honoured while detail mode is active — masking outside the
		// detail surface has nothing to flip.
		if m.opened && msg.Field != "" {
			if m.detailUnmasked == nil {
				m.detailUnmasked = map[string]bool{}
			}
			m.detailUnmasked[msg.Field] = true
		}
		return m, nil
	case shared.MaskAllMsg:
		m.detailUnmasked = nil
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ListModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl-c: hard quit. Mirrors groups/rules — when ListModel runs as the
	// teatest root (no App Shell wrapping it) Ctrl-c is the only way to
	// drain teatest's FinalOutput. The App Shell intercepts Ctrl-c earlier
	// in production and routes it to the QuitConfirm overlay.
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	// Detail mode (TUI_DESIGN §3.6): Esc returns to the list; Tab /
	// Shift-Tab cycle through tabs; `r` toggles the Raw tab against the
	// last-visited non-Raw tab so a second press returns the operator to
	// where they came from. Line-cursor + Visual mode (v / V / y) live
	// alongside the tab navigation.
	if m.opened {
		// Any keypress dismisses the previous "5 lines copied" toast.
		m.detailToast = ""
		switch msg.Type {
		case tea.KeyEsc:
			if m.detailVisual {
				// Cancel Visual without leaving detail mode.
				m.detailVisual = false
				return m, nil
			}
			m.opened = false
			m.detailUser = domain.User{}
			m.detailTab = DetailTabProfile
			m.detailRawReturn = DetailTabProfile
			m.detailLine = 0
			m.detailVisualAnchor = 0
			return m, nil
		case tea.KeyTab:
			m.detailTab = (m.detailTab + 1) % detailTabCount
			m.detailLine = 0
			m.detailVisual = false
			return m, nil
		case tea.KeyShiftTab:
			m.detailTab = (m.detailTab + detailTabCount - 1) % detailTabCount
			m.detailLine = 0
			m.detailVisual = false
			return m, nil
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "r":
				if m.detailTab == DetailTabRaw {
					m.detailTab = m.detailRawReturn
				} else {
					m.detailRawReturn = m.detailTab
					m.detailTab = DetailTabRaw
				}
				m.detailLine = 0
				m.detailVisual = false
			case "j":
				lines := m.detailBodyLines()
				if m.detailLine < len(lines)-1 {
					m.detailLine++
				}
			case "k":
				if m.detailLine > 0 {
					m.detailLine--
				}
			case "g":
				m.detailLine = 0
			case "G":
				lines := m.detailBodyLines()
				if len(lines) > 0 {
					m.detailLine = len(lines) - 1
				}
			case "v", "V":
				if m.detailVisual {
					m.detailVisual = false
				} else {
					m.detailVisual = true
					m.detailVisualAnchor = m.detailLine
				}
			case "y":
				lines := m.detailBodyLines()
				if len(lines) == 0 {
					return m, nil
				}
				start, end := m.detailLine, m.detailLine
				if m.detailVisual {
					start, end = m.detailVisualAnchor, m.detailLine
					if start > end {
						start, end = end, start
					}
				}
				selected := strings.Join(lines[start:end+1], "\n")
				if err := clipboard.WriteAll(selected); err != nil {
					m.detailToast = "yank failed: " + err.Error()
				} else {
					n := end - start + 1
					unit := "line"
					if n != 1 {
						unit = "lines"
					}
					m.detailToast = "yanked " + itoaSimple(n) + " " + unit
				}
				m.detailVisual = false
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

	// Vim navigation: `gg` jumps to top, `G` to bottom. Detected here
	// because keys.Resolve binds them as a chord ("g g") that classify()
	// can't represent as a single rune. Any non-`g` keypress resets the
	// chord arming below.
	if msg.Type == tea.KeyRunes && string(msg.Runes) == "g" {
		if m.ggChord.Press(m.now()) {
			m.cursor = 0
			m.viewportTop = 0
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes && string(msg.Runes) == "G" {
		m.ggChord.Reset()
		if vis := m.visible(); len(vis) > 0 {
			m.cursor = len(vis) - 1
		}
		return m, nil
	}
	m.ggChord.Reset()

	switch m.classify(msg) {
	case keys.IDSearchOpen:
		m.filtering = true
		m.filter = ""
		return m, nil
	case keys.IDNavDown:
		m.cursor++
		return m, nil
	case keys.IDNavUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case keys.IDNavSelect, keys.IDActionDetail:
		// `Enter` and `d` share the inline detail flow (TUI_DESIGN §3.6).
		// Both fetch the full user and surface the detail view; v0.1.1
		// keeps the routing inside ListModel (Option A) — App Shell-level
		// OpenResourceMsg routing arrives in v0.2.
		sel := m.selected()
		if sel == nil {
			return m, nil
		}
		return m, openUserCmd(m.deps.Port, sel.ID)
	case keys.IDSortStatus:
		m.cycleSort(SortStatus)
		return m, nil
	case keys.IDSortName:
		m.cycleSort(SortName)
		return m, nil
	case keys.IDSortLastLogin:
		m.cycleSort(SortLastLogin)
		return m, nil
	case keys.IDSortCreated:
		m.cycleSort(SortCreated)
		return m, nil
	}
	return m, nil
}

// cycleSort advances the sort state per TUI_DESIGN §3.5:
//   - same key as the active column → off → asc → desc → off
//   - new key → reset cursor + start at SortAsc on the new column
//
// Pressing a different sort key always discards the previous column's
// direction immediately (single-active-sort invariant).
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

// classify resolves a tea.KeyMsg through the injected Deps.Keys map (REQ-C03
// AC-2). When Deps.Keys is empty it falls back to the built-in defaults so
// the screen still works in standalone teatest harnesses.
func (m ListModel) classify(msg tea.KeyMsg) keys.ID {
	resolved := m.deps.Keys
	if len(resolved) == 0 {
		resolved, _, _ = keys.Resolve(nil)
	}
	switch msg.Type {
	case tea.KeyDown:
		return keys.IDNavDown
	case tea.KeyUp:
		return keys.IDNavUp
	case tea.KeyEnter:
		return keys.IDNavSelect
	}
	if msg.Type == tea.KeyRunes {
		return resolved.Reverse()[string(msg.Runes)]
	}
	return resolved.Reverse()[msg.String()]
}

// View renders SCR-010 (TUI_DESIGN §15.2 / §16.1). Output is a column-aligned
// table — chrome (HeaderBar / StatusBar) is contributed by the App Shell.
//
// Layout (NO_COLOR, 5 columns, 80-cell budget):
//
//	STATUS         LOGIN                       DISPLAY NAME       LAST LOGIN  CHANGED
//	[+] ACTIVE     alice@acme.com              Alice Smith            2h ago   14d ago
//	...
func (m ListModel) View() string {
	if m.opened {
		return m.renderDetailWithCursor()
	}

	if m.lastErr != nil {
		return renderUsersError(m.lastErr)
	}

	tk := activeTokens()

	rows := m.visible()
	hint := m.contextLine(rows)

	var b strings.Builder
	b.WriteString(hint)
	b.WriteByte('\n')
	if m.filtering {
		b.WriteString("filter: " + m.filter)
		b.WriteByte('\n')
	}
	// Header carries the same 2-cell cursor gutter every data row uses so
	// column titles align with their values (issue #107).
	b.WriteString("  ")
	b.WriteString(m.renderUsersHeader(tk))
	b.WriteByte('\n')

	// Compute the slice of rows that fits in the body. Without windowing,
	// large user lists render every row into the body string and the chrome
	// header scrolls off-screen — the user reported this directly. The
	// budget keeps the chrome's top border + context line visible by
	// reserving header / hint / filter rows from the body height.
	top, end := m.windowBounds(len(rows))
	for i := top; i < end; i++ {
		row := m.renderUsersRow(rows[i], m.now(), tk)
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

// DetailLine returns the active line cursor inside the detail body.
// Exported so tests can assert cursor movement without depending on a
// visual marker — v0.1.3-1 dropped the ▸ prefix to avoid shifting the
// cursor row's indent, leaving the highlight as style-only (stripped by
// testfx.PinTestEnvironment under NO_COLOR).
func (m ListModel) DetailLine() int { return m.detailLine }

// DetailVisualActive reports whether Visual selection is currently in
// progress (`v` was pressed and `Esc` / `y` haven't ended it yet).
func (m ListModel) DetailVisualActive() bool { return m.detailVisual }

// renderDetailWithCursor wraps DetailModel.View() with a line-cursor +
// optional Vim Visual highlight and a transient toast (e.g. "yanked 5
// lines"). The DetailModel itself is stateless across renders; the
// cursor / visual state lives on ListModel so the data flow stays
// idempotent.
func (m ListModel) renderDetailWithCursor() string {
	tk := activeTokens()
	body := m.newDetail().View()
	lines := strings.Split(body, "\n")

	// The first three lines (header / tab bar / divider) are not
	// selectable — Visual mode only operates on the body.
	const headerLines = 3
	cursor := m.detailLine + headerLines
	anchor := m.detailVisualAnchor + headerLines
	start, end := cursor, cursor
	if m.detailVisual {
		start, end = anchor, cursor
		if start > end {
			start, end = end, start
		}
	}

	var b strings.Builder
	if m.detailToast != "" {
		b.WriteString(tk.Header.Render(m.detailToast))
		b.WriteByte('\n')
	}
	if m.detailVisual {
		b.WriteString(tk.Warning.Render("-- VISUAL --"))
		b.WriteByte('\n')
	}
	// Highlight cursor / Visual range with style only — no character prefix
	// so columns stay aligned with the surrounding lines (issue #106). The
	// shared RowHighlight token adds a background tint over the bold accent
	// so the cursor row reads at-a-glance even from the corner of the eye
	// (issue #112).
	cursorStyle := tk.RowHighlight
	for i, line := range lines {
		switch {
		case i < headerLines:
			b.WriteString(line)
		case m.detailVisual && i >= start && i <= end:
			b.WriteString(cursorStyle.Render(line))
		case i == cursor:
			b.WriteString(cursorStyle.Render(line))
		default:
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	footer := tk.Muted.Render("<j/k> nav · <v> visual · <y> yank · <Tab> tabs · <Esc> back")
	b.WriteString(footer)
	return b.String()
}

// newDetail constructs a DetailModel for the current detailUser and
// applies the persistent ListModel state (active tab + per-field unmask
// flags) so :unmask survives across renders. Centralised so the detail
// view and detailBodyLines stay in lockstep.
func (m ListModel) newDetail() DetailModel {
	d := NewDetailModel(m.deps, m.detailUser).WithActiveTab(m.detailTab)
	for field, on := range m.detailUnmasked {
		if on {
			d.ToggleUnmask(field)
		}
	}
	return d
}

// detailBodyLines returns the body of the active tab as a slice of lines,
// excluding the three-line header (User Detail / tab bar / divider) so j/k
// navigation only counts content rows.
func (m ListModel) detailBodyLines() []string {
	body := m.newDetail().View()
	all := strings.Split(body, "\n")
	const headerLines = 3
	if len(all) <= headerLines {
		return nil
	}
	return all[headerLines:]
}

// itoaSimple is a tiny strconv.Itoa shim used by handleKey's toast string
// (avoids importing strconv elsewhere in list.go for one usage).
func itoaSimple(n int) string { return strconv.Itoa(n) }

// windowBounds returns the [top, end) slice of rows that should render
// given the current cursor position and viewportTop. Delegates to the
// shared helper so Groups and Rules use the same algorithm.
func (m ListModel) windowBounds(total int) (int, int) {
	return shared.WindowBounds(m.cursor, m.viewportTop, total, shared.ListBodyRowBudget(m.height))
}

// contextLine renders the "Users · 5 of N · q="..."" line (TUI_DESIGN §15.2
// ContextBar). The chrome's ContextBar will eventually consume these counts,
// but rendering them here keeps screens self-contained for tests that drive
// the model directly.
func (m ListModel) contextLine(visible []domain.User) string {
	count := fmt.Sprintf("%d of %d", len(visible), len(m.users))
	if m.filter != "" {
		count = count + ` · q="` + m.filter + `"`
	}
	return "Users  " + count
}

// renderUsersHeader returns the column header row, width-aware
// (TUI_DESIGN §15.0a / §15.2 v1.3). v0.1.3 columns:
//
//	STATUS · LOGIN · TITLE · DIVISION · EMPLOYEE# · NICKNAME ·
//	LAST LOGIN · CHANGED
//
// DISPLAY NAME and DEPARTMENT moved out of the default row at the user's
// request — they're still on the Pretty / JSON / YAML detail tabs.
//
// The active sort column carries an `↑` (asc) or `↓` (desc) indicator
// appended to its label per §15.2 v1.2.0.
func (m ListModel) renderUsersHeader(_ shared.Tokens) string {
	return m.formatUsersColumns(
		usersSortLabel("STATUS", m.sortBy, SortStatus, m.sortDir),
		usersSortLabel("LOGIN", m.sortBy, SortName, m.sortDir),
		"TITLE",
		"DIVISION",
		"EMPLOYEE#",
		"NICKNAME",
		usersSortLabel("LAST LOGIN", m.sortBy, SortLastLogin, m.sortDir),
		usersSortLabel("CHANGED", m.sortBy, SortCreated, m.sortDir),
	)
}

// usersSortLabel appends "↑" / "↓" to title when active is the same key as
// the column. SortNone / SortOff renders the label unchanged.
func usersSortLabel(title string, active, key SortKey, dir SortDir) string {
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

// renderUsersRow formats a single User row, width-aware.
func (m ListModel) renderUsersRow(u domain.User, now time.Time, tk shared.Tokens) string {
	status := shared.UserStatusBadge(string(u.Status), tk).Render(tk)
	last := shared.RelativeTime(u.LastLogin, now)
	changed := shared.RelativeTime(u.StatusChanged, now)
	dash := func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	}
	return m.formatUsersColumns(
		status,
		u.Profile.Login,
		dash(u.Profile.Title),
		dash(u.Profile.Division),
		dash(u.Profile.EmployeeNumber),
		dash(u.Profile.NickName),
		last,
		changed,
	)
}

// usersColumnSpecs returns the v0.1.3 column definitions:
//
//	STATUS, LOGIN, TITLE, DIVISION, EMPLOYEE#, NICKNAME, LAST LOGIN, CHANGED
//
// Drop priority (lowest = drops first) keeps the user-requested fields
// visible as long as possible while still degrading gracefully on narrow
// terminals:
//
//	1  CHANGED
//	2  LAST LOGIN
//	3  NICKNAME
//	4  EMPLOYEE#
//	5  DIVISION
//	6  TITLE
//	0  STATUS, LOGIN — never dropped (essentials)
func usersColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "STATUS", Kind: shared.ColumnFixed, Min: 14, DropPriority: 0},
		{Title: "LOGIN", Kind: shared.ColumnFlex, Min: 22, Weight: 3, DropPriority: 0},
		{Title: "TITLE", Kind: shared.ColumnFlex, Min: 14, Weight: 2, DropPriority: 6},
		{Title: "DIVISION", Kind: shared.ColumnFlex, Min: 10, Weight: 1, DropPriority: 5},
		{Title: "EMPLOYEE#", Kind: shared.ColumnFixed, Min: 10, DropPriority: 4},
		{Title: "NICKNAME", Kind: shared.ColumnFlex, Min: 10, Weight: 1, DropPriority: 3},
		{Title: "LAST LOGIN", Kind: shared.ColumnFixed, Min: 10, DropPriority: 2, AlignRight: true},
		{Title: "CHANGED", Kind: shared.ColumnFixed, Min: 8, DropPriority: 1, AlignRight: true},
	}
}

// formatUsersColumns lays out STATUS / LOGIN / DISPLAY NAME / LAST LOGIN /
// CHANGED / DEPARTMENT per the TUI_DESIGN §15.0a Min/Weight + DropPriority
// model. Cells beyond the supplied list (e.g., DEPARTMENT before the User
// model carries it) are rendered as "—".
//
// The active sort column gets +1 Min to reserve room for its `↑` / `↓`
// indicator (§15.2 v1.2.0: "헤더만 1글자 차지, 본문 cell 폭 영향 없음").
// Without the bump a Min-tight column like LAST LOGIN (10) would clip the
// indicator to "LAST LOGI…".
func (m ListModel) formatUsersColumns(cells ...string) string {
	specs := usersColumnSpecs()
	// Auto-fit (issue #117): shrink first so titles + observed data
	// determine the floor. Apply the sort-glyph bump *after* the shrink
	// so the active column always reserves room for its `↑`/`↓`.
	specs = shared.ShrinkSpecsToFit(specs, m.observedColumnWidths())
	if i := usersSortColumnIdx(m.sortBy); i >= 0 && m.sortDir != SortOff {
		specs[i].Min++
	}
	widths := shared.LayoutColumns(specs, m.usersInnerWidth(), 2)

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

// observedColumnWidths returns the largest cell width seen in each
// column across the currently visible rows. Powers ShrinkSpecsToFit so
// columns auto-fit data instead of always padding to declared Min.
func (m ListModel) observedColumnWidths() []int {
	rows := m.visible()
	if len(rows) == 0 {
		return nil
	}
	now := m.now()
	tk := activeTokens()
	dash := func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	}
	out := make([]int, 8)
	for _, u := range rows {
		statusBadge := shared.UserStatusBadge(string(u.Status), tk).Render(tk)
		cells := []string{
			statusBadge,
			u.Profile.Login,
			dash(u.Profile.Title),
			dash(u.Profile.Division),
			dash(u.Profile.EmployeeNumber),
			dash(u.Profile.NickName),
			shared.RelativeTime(u.LastLogin, now),
			shared.RelativeTime(u.StatusChanged, now),
		}
		for i, c := range cells {
			if w := visibleCellWidth(c); w > out[i] {
				out[i] = w
			}
		}
	}
	return out
}

// visibleCellWidth approximates the rendered width of a cell after the
// shared chrome's ANSI stripping — falls back to len(s) for ASCII which
// covers our column data today (logins, dates, statuses).
func visibleCellWidth(s string) int {
	// Stay conservative: count runes after stripping ANSI escapes.
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r >= '@' && r <= '~' {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

// usersSortColumnIdx maps a SortKey to its index in usersColumnSpecs.
// Updated for the v0.1.3 column order:
//
//	0 STATUS · 1 LOGIN · 2 TITLE · 3 DIVISION · 4 EMPLOYEE# · 5 NICKNAME ·
//	6 LAST LOGIN · 7 CHANGED
func usersSortColumnIdx(k SortKey) int {
	switch k {
	case SortStatus:
		return 0
	case SortName:
		return 1
	case SortLastLogin:
		return 6
	case SortCreated:
		return 7
	}
	return -1
}

// usersInnerWidth returns the body width available to columns: chrome inner
// (W - 2 borders) minus the 2-cell gutter the chrome adds around the body
// (1 cell left padding + the row's right-edge padding handled by chrome).
//
// width == 0 (no WindowSizeMsg yet) falls back to the chrome's default 85
// so the first frame still shows the standard column set.
func (m ListModel) usersInnerWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	// chrome border (2) + left padding (1) + cursor gutter (2 for "> "/"  ").
	inner := w - 2 - 1 - 2
	if inner < 20 {
		inner = 20
	}
	return inner
}

// renderUsersError builds the inline error panel (TUI_DESIGN §17.1) using
// the shared ErrorPanel helper sourced from errormap.UserMessage(err).
func renderUsersError(err error) string {
	return "Users  (error)\n" + shared.ErrorPanel("users", err)
}

// now returns the current time, preferring the injected clock for tests.
func (m ListModel) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

// activeTokens picks the right token set per NO_COLOR.
func activeTokens() shared.Tokens {
	if shared.MonochromeEnabled() {
		return shared.Monochrome()
	}
	return shared.Dark()
}

// visible applies the active filter (case-insensitive substring match on
// Profile.Login) and active sort (TUI_DESIGN §3.5) to m.users. The filter
// is applied first so sort operates over the visible subset.
func (m ListModel) visible() []domain.User {
	var out []domain.User
	if m.filter == "" {
		// Copy so the sort below doesn't mutate m.users in place.
		out = append(out, m.users...)
	} else {
		needle := strings.ToLower(m.filter)
		out = make([]domain.User, 0, len(m.users))
		for _, u := range m.users {
			if strings.Contains(strings.ToLower(u.Profile.Login), needle) {
				out = append(out, u)
			}
		}
	}
	if m.sortBy != SortNone && m.sortDir != SortOff {
		sortUsersByKey(out, m.sortBy, m.sortDir)
	}
	return out
}

// sortUsersByKey applies a stable sort to xs in place per §3.5a (Users).
// Stability matters: rows sharing a sort-key value must keep their original
// fetch order so operators don't see a confusing reshuffle.
func sortUsersByKey(xs []domain.User, key SortKey, dir SortDir) {
	less := usersComparator(key)
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

// usersComparator returns a "less" function honouring §3.5a's per-column
// rules. Returns nil for keys not applicable to Users (none, in MVP).
func usersComparator(key SortKey) func(a, b domain.User) bool {
	switch key {
	case SortStatus:
		return func(a, b domain.User) bool {
			return userStatusRank(a.Status) < userStatusRank(b.Status)
		}
	case SortName:
		return func(a, b domain.User) bool {
			return strings.ToLower(a.Profile.Login) < strings.ToLower(b.Profile.Login)
		}
	case SortLastLogin:
		// nil is "smallest" — asc places never-logged-in users at the top.
		return func(a, b domain.User) bool {
			return userLastLoginInstant(a).Before(userLastLoginInstant(b))
		}
	case SortCreated:
		// §3.5a: StatusChanged with Created fallback (StatusChanged.IsZero).
		return func(a, b domain.User) bool {
			return userChangedInstant(a).Before(userChangedInstant(b))
		}
	}
	return nil
}

// userStatusRank assigns the §3.5a operational ordering: ACTIVE first so
// most-affected accounts surface ahead of routine ones isn't the goal —
// the rank reflects "what an operator wants to see at the top in a
// healthy → broken cascade".
func userStatusRank(s domain.UserStatus) int {
	switch s {
	case domain.UserStatusActive:
		return 0
	case domain.UserStatusLockedOut:
		return 1
	case domain.UserStatusPasswordExpired:
		return 2
	case domain.UserStatusSuspended:
		return 3
	case domain.UserStatusStaged:
		return 4
	case domain.UserStatusProvisioned:
		return 5
	case domain.UserStatusDeprovisioned:
		return 6
	}
	return 7
}

// userLastLoginInstant returns u.LastLogin or the zero time when nil/zero.
// time.Time's zero value (Jan 1, year 1) is "smaller than" all real
// timestamps — exactly the §3.5a contract for nil-as-smallest.
func userLastLoginInstant(u domain.User) time.Time {
	if u.LastLogin == nil {
		return time.Time{}
	}
	return *u.LastLogin
}

// userChangedInstant returns u.StatusChanged or u.Created when StatusChanged
// is unset. Mirrors the §3.5a Created column contract.
func userChangedInstant(u domain.User) time.Time {
	if u.StatusChanged != nil && !u.StatusChanged.IsZero() {
		return *u.StatusChanged
	}
	return u.Created
}

// selected returns the currently-highlighted user, if any.
func (m ListModel) selected() *domain.User {
	vs := m.visible()
	if m.cursor < 0 || m.cursor >= len(vs) {
		return nil
	}
	return &vs[m.cursor]
}

// fetchUsersCmd drains the Port.List iterator and emits usersLoadedMsg, or
// usersErrMsg on failure (TUI_DESIGN §17 / Phase 6d-6 spec).
func fetchUsersCmd(port domain.UsersPort) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		iter, err := port.List(ctx, domain.UsersQuery{Limit: 200})
		if err != nil {
			return usersErrMsg{err: err}
		}
		defer iter.Close()
		var out []domain.User
		for {
			u, hasMore, err := iter.Next(ctx)
			if err != nil {
				return usersErrMsg{err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, u)
		}
		return usersLoadedMsg{users: out}
	}
}

// openUserCmd fetches the full user and emits userOpenedMsg.
func openUserCmd(port domain.UsersPort, id string) tea.Cmd {
	return func() tea.Msg {
		u, err := port.Get(context.Background(), id)
		if err != nil {
			return userOpenedMsg{user: domain.User{ID: id}}
		}
		return userOpenedMsg{user: u}
	}
}

var _ tea.Model = ListModel{}
