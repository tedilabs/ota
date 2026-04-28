package users

import (
	"context"
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
	// RefreshInterval drives the auto-refresh tick (issue #177
	// v0.1.16). Zero disables auto-refresh.
	RefreshInterval time.Duration
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
	// hScroll is the horizontal column offset (issue #122). 0 keeps the
	// row aligned to the leftmost column; `l` advances the slice right
	// when the natural row exceeds the viewport, `h` retreats. Tracked
	// per ListModel so each list keeps its own state independently.
	hScroll int

	// detailGroups / detailApps carry the lazy-fetched assigned-groups
	// and assigned-apps lists for the open user (issue #168). Rendered
	// as two extra sections beneath the 2-col Pretty layout. Per-user
	// keying (detailExtrasUser) prevents a stale fetch from a
	// previously-opened user clobbering the current detail.
	detailGroups       []domain.Group
	detailGroupsErr    error
	detailGroupsLoaded bool
	detailApps         []domain.AppLink
	detailAppsErr      error
	detailAppsLoaded   bool
	detailExtrasUser   string
	// detailExtrasFocused flips when the operator hops from the info
	// grid into the Groups+Apps boxes (issue #174 v0.1.15). When false
	// j/k moves the info grid line cursor (detailLine); when true j/k
	// drives a single linear cursor that flows from the first Groups
	// row through to the last Apps row and wraps. `]` toggles in (and
	// `]`/`[` while inside the boxes jump to the other column's first
	// row); Esc both exits Visual and exits the boxes back to the
	// info grid.
	detailExtrasFocused bool
	// detailExtrasCur is the linear cursor inside the extras region:
	// 0..len(groups)-1 maps to Groups rows; len(groups)..total-1 maps
	// to Apps rows. The View renders a single highlight on whichever
	// box owns the cursor — never both at once.
	detailExtrasCur int
	// detailGroupsTop / detailAppsTop hold the per-box scroll
	// offsets so each scrollbar tracks independently as the linear
	// cursor moves between columns.
	detailGroupsTop int
	detailAppsTop   int

	// lastUpdated stamps the most recent successful list fetch so
	// the chrome's upper-divider right slot reads "updated 12:34:56
	// UTC" (issue #177 v0.1.16). Zero before the first fetch.
	lastUpdated time.Time
	// refreshGen guards against stale refresh-tick Cmds firing
	// after the operator switched screens or the model was rebuilt.
	refreshGen int

}

// usersLoadedMsg delivers the result of the initial fetch.
type usersLoadedMsg struct{ users []domain.User }

// usersErrMsg delivers a fetch failure to the model so the View can surface
// it via the inline error panel (TUI_DESIGN §17.1 / Phase 6d-6).
type usersErrMsg struct{ err error }

// usersRefreshTickMsg fires when the auto-refresh ticker (issue #177
// v0.1.16) should re-fetch the user list. `gen` matches the model's
// `refreshGen` so a screen switch or reload invalidates in-flight
// ticks (no fetch fires on a model that's been swapped out).
type usersRefreshTickMsg struct{ gen int }

// userOpenedMsg delivers the result of a detail fetch.
type userOpenedMsg struct{ user domain.User }

// userDetailGroupsLoadedMsg / userDetailGroupsErrMsg deliver the
// result of the per-user assigned-groups fetch the detail view
// renders below the 2-col Pretty layout (issue #168). The userID is
// round-tripped so a stale fetch from a previously-opened detail
// can't overwrite the current one.
type userDetailGroupsLoadedMsg struct {
	userID string
	groups []domain.Group
}
type userDetailGroupsErrMsg struct {
	userID string
	err    error
}

// userDetailAppsLoadedMsg / userDetailAppsErrMsg are the apps
// counterpart of the messages above.
type userDetailAppsLoadedMsg struct {
	userID string
	apps   []domain.AppLink
}
type userDetailAppsErrMsg struct {
	userID string
	err    error
}

// NewListModel constructs a ListModel.
func NewListModel(deps Deps) ListModel {
	return ListModel{
		deps:   deps,
		users:  deps.InitialUsers,
		width:  deps.Width,
		height: deps.Height,
	}
}

// Init kicks off the initial List call (REQ-R01 AC-1) and schedules
// the first auto-refresh tick (issue #177 v0.1.16). When the model
// is seeded with InitialUsers the fetch is skipped but the tick
// still fires — operators get fresh data on the configured cadence.
func (m ListModel) Init() tea.Cmd {
	var fetch tea.Cmd
	if len(m.users) == 0 && m.deps.Port != nil {
		fetch = fetchUsersCmd(m.deps.Port)
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

// scheduleRefreshTickCmd returns a tea.Tick that fires usersRefreshTickMsg
// after the configured interval. Returns nil when auto-refresh is
// disabled (RefreshInterval == 0) or the port isn't wired.
func (m ListModel) scheduleRefreshTickCmd() tea.Cmd {
	if m.deps.RefreshInterval <= 0 || m.deps.Port == nil {
		return nil
	}
	gen := m.refreshGen
	return tea.Tick(m.deps.RefreshInterval, func(time.Time) tea.Msg {
		return usersRefreshTickMsg{gen: gen}
	})
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
		m.lastUpdated = m.now()
		return m, nil
	case usersErrMsg:
		m.lastErr = msg.err
		return m, nil
	case usersRefreshTickMsg:
		// Stale tick (model rebuilt / generation bumped) — drop.
		if msg.gen != m.refreshGen || m.deps.Port == nil {
			return m, nil
		}
		// Re-fetch + reschedule. Using tea.Batch keeps the tick
		// chain alive even when the fetch is in flight so a slow
		// API doesn't pause the loop.
		return m, tea.Batch(fetchUsersCmd(m.deps.Port), m.scheduleRefreshTickCmd())
	case userOpenedMsg:
		m.detailUser = msg.user
		m.opened = true
		// Issue #168 — kick off the assigned-groups + assigned-apps
		// fetches so the extra sections render below the Pretty
		// layout. Reset cached results so a previous detail's data
		// doesn't flash before the new fetches return.
		if m.detailExtrasUser != msg.user.ID {
			m.detailExtrasUser = msg.user.ID
			m.detailGroups = nil
			m.detailGroupsErr = nil
			m.detailGroupsLoaded = false
			m.detailApps = nil
			m.detailAppsErr = nil
			m.detailAppsLoaded = false
		}
		if m.deps.Port != nil {
			return m, tea.Batch(
				fetchUserGroupsCmd(m.deps.Port, msg.user.ID),
				fetchUserAppLinksCmd(m.deps.Port, msg.user.ID),
			)
		}
		return m, nil
	case userDetailGroupsLoadedMsg:
		if m.opened && m.detailUser.ID == msg.userID {
			m.detailGroups = msg.groups
			m.detailGroupsErr = nil
			m.detailGroupsLoaded = true
		}
		return m, nil
	case userDetailGroupsErrMsg:
		if m.opened && m.detailUser.ID == msg.userID {
			m.detailGroupsErr = msg.err
			m.detailGroupsLoaded = true
		}
		return m, nil
	case userDetailAppsLoadedMsg:
		if m.opened && m.detailUser.ID == msg.userID {
			m.detailApps = msg.apps
			m.detailAppsErr = nil
			m.detailAppsLoaded = true
		}
		return m, nil
	case userDetailAppsErrMsg:
		if m.opened && m.detailUser.ID == msg.userID {
			m.detailAppsErr = msg.err
			m.detailAppsLoaded = true
		}
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
			// v0.1.15 issue #174: when focus is inside the
			// Groups+Apps boxes, Esc returns to the info grid
			// instead of closing the whole detail surface — gives
			// the operator a way back without an Enter mis-step.
			if m.detailExtrasFocused {
				m.detailExtrasFocused = false
				return m, nil
			}
			m.opened = false
			m.detailUser = domain.User{}
			m.detailTab = DetailTabProfile
			m.detailRawReturn = DetailTabProfile
			m.detailLine = 0
			m.detailVisualAnchor = 0
			// Issue #168: clear cached groups/apps so the next user
			// fetches fresh.
			m.detailExtrasUser = ""
			m.detailGroups = nil
			m.detailGroupsErr = nil
			m.detailGroupsLoaded = false
			m.detailApps = nil
			m.detailAppsErr = nil
			m.detailAppsLoaded = false
			m.detailExtrasFocused = false
			m.detailExtrasCur = 0
			m.detailGroupsTop = 0
			m.detailAppsTop = 0
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
		case tea.KeyEnter:
			// Issue #171 — drill-down on the Groups / Apps boxes:
			// Enter on a focused row asks the App Shell to switch
			// to the matching screen and open detail by ID. v0.1.15
			// uses a linear cursor that flows across both boxes:
			// any cursor position < len(groups) selects a group;
			// the rest selects an app.
			if m.detailExtrasFocused {
				if cur := m.detailExtrasCur; cur >= 0 && cur < len(m.detailGroups) {
					id := m.detailGroups[cur].ID
					if id != "" {
						return m, openGroupDetailCmd(id)
					}
				} else if appIdx := m.detailExtrasCur - len(m.detailGroups); appIdx >= 0 && appIdx < len(m.detailApps) {
					id := m.detailApps[appIdx].ID
					if id != "" {
						return m, openAppDetailCmd(id)
					}
				}
			}
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
				// v0.1.15 issue #174: j on the info grid scrolls the
				// line cursor; j inside the Groups+Apps boxes drives
				// a single linear cursor that flows across both
				// boxes (and wraps around at the end).
				if m.detailExtrasFocused {
					if total := m.detailExtrasTotal(); total > 0 {
						m.detailExtrasCur = (m.detailExtrasCur + 1) % total
					}
				} else {
					lines := m.detailBodyLines()
					if m.detailLine < len(lines)-1 {
						m.detailLine++
					}
				}
			case "k":
				if m.detailExtrasFocused {
					if total := m.detailExtrasTotal(); total > 0 {
						m.detailExtrasCur = (m.detailExtrasCur - 1 + total) % total
					}
				} else {
					if m.detailLine > 0 {
						m.detailLine--
					}
				}
			case "]":
				// v0.1.15 issue #174: `]` enters the boxes when on
				// the info grid; once inside, jumps to the start of
				// the Apps column. `[` is the symmetric back jump.
				if !m.detailExtrasFocused {
					m.detailExtrasFocused = true
					m.detailExtrasCur = 0
				} else {
					m.detailExtrasCur = len(m.detailGroups)
					if m.detailExtrasCur >= m.detailExtrasTotal() {
						m.detailExtrasCur = 0
					}
				}
			case "[":
				if m.detailExtrasFocused {
					m.detailExtrasCur = 0
				}
			case "g":
				if m.detailExtrasFocused {
					m.detailExtrasCur = 0
				} else {
					m.detailLine = 0
				}
			case "G":
				if m.detailExtrasFocused {
					if total := m.detailExtrasTotal(); total > 0 {
						m.detailExtrasCur = total - 1
					}
				} else {
					lines := m.detailBodyLines()
					if len(lines) > 0 {
						m.detailLine = len(lines) - 1
					}
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

	// Esc on a list with an active filter clears the filter and
	// restores the full row set (issue #131). The `/` input is closed
	// by Enter — at that point m.filtering is false but m.filter still
	// drives the visible() projection. Without this, operators had no
	// way to escape a filter besides backspacing through it.
	if msg.Type == tea.KeyEsc && m.filter != "" {
		m.filter = ""
		m.cursor = 0
		m.viewportTop = 0
		return m, nil
	}

	// Vim page nav (TUI_DESIGN §3.2). Ctrl-f / Ctrl-b move a full page,
	// Ctrl-d / Ctrl-u move half a page. Page size mirrors the body
	// row budget so the cursor lands in the same relative spot after a
	// jump.
	switch msg.Type {
	case tea.KeyCtrlF:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor += page
		return m.clampedCursor(), nil
	case tea.KeyCtrlB:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor -= page
		return m.clampedCursor(), nil
	case tea.KeyCtrlD:
		page := shared.ListBodyRowBudget(m.height) / 2
		if page <= 0 {
			page = 5
		}
		m.cursor += page
		return m.clampedCursor(), nil
	case tea.KeyCtrlU:
		page := shared.ListBodyRowBudget(m.height) / 2
		if page <= 0 {
			page = 5
		}
		m.cursor -= page
		return m.clampedCursor(), nil
	}

	// Horizontal scroll (issue #122 + #159). `l` / Right advance the
	// column slice; `h` / Left retreat. Clamped to [0, MaxHScroll].
	if msg.Type == tea.KeyRight || (msg.Type == tea.KeyRunes && string(msg.Runes) == "l") {
		m.hScroll = m.clampHScroll(m.hScroll + 1)
		return m, nil
	}
	if msg.Type == tea.KeyLeft || (msg.Type == tea.KeyRunes && string(msg.Runes) == "h") {
		if m.hScroll > 0 {
			m.hScroll--
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

	var b strings.Builder
	// Resource label, count, and filter all live in the chrome's
	// upper divider now (issues #133 + #136); the body opens straight
	// with the column header. 2-cell cursor gutter on the header
	// keeps it aligned with data rows.
	b.WriteString("  ")
	b.WriteString(m.renderUsersHeader(tk))
	b.WriteByte('\n')

	// Compute the slice of rows that fits in the body. Without windowing,
	// large user lists render every row into the body string and the chrome
	// header scrolls off-screen — the user reported this directly. The
	// budget keeps the chrome's top border + context line visible by
	// reserving header / hint / filter rows from the body height.
	top, end := m.windowBounds(len(rows))
	budget := end - top
	contentW := m.chromeContentWidth()
	rowTarget := contentW - 2 // leave room for " ▌" / " │"
	for i := top; i < end; i++ {
		row := m.renderUsersRow(rows[i], m.now(), tk)
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		composed := prefix + row
		// Pad to rowTarget BEFORE styling so the cursor / status
		// background tint covers the full row including the trailing
		// fill cells, not just the column data.
		composed = shared.PadOrTruncateVisible(composed, rowTarget)
		switch {
		case i == m.cursor:
			// Cursor takes priority — the operator needs to see
			// where they are. The status badge in the row already
			// conveys the abnormal state.
			composed = tk.Accent.Render(composed)
		default:
			// Issue #155: non-cursor row gets a status-driven bg
			// tint when STATUS is abnormal so LOCKED_OUT /
			// SUSPENDED / PASSWORD_EXPIRED / DEPROVISIONED pop.
			// Strip inner ANSI first so the bg style applies
			// uniformly across the row.
			if rowStyle, ok := shared.RowStyleForStatus(string(rows[i].Status), tk); ok {
				composed = rowStyle.Render(shared.StripCSI(composed))
			}
		}
		b.WriteString(composed)
		b.WriteString(shared.AppendScrollbarSuffix(i-top, top, budget, len(rows), tk))
		b.WriteByte('\n')
	}
	return b.String()
}

// Filtering reports whether the operator is in `/` filter input mode.
// Implements app.FilterStater so the App Shell can render the floating
// filter box (issue #123).
func (m ListModel) Filtering() bool { return m.filtering }

// Filter returns the current filter string (echoed in the floating box).
func (m ListModel) Filter() string { return m.filter }

// Count returns the visible/total counts so the App Shell can stamp
// "N of M" into the chrome's upper divider (issue #136). Implements
// app.Counter.
func (m ListModel) Count() (visible, total int) {
	return len(m.visible()), len(m.users)
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

// clampedCursor pins m.cursor inside [0, len(visible)-1] and returns
// the model so the caller can `return m.clampedCursor(), nil` succinctly.
// Centralised so Ctrl-f / Ctrl-b / Ctrl-d / Ctrl-u all share the same
// boundary behaviour.
func (m ListModel) clampedCursor() ListModel {
	vis := m.visible()
	if m.cursor < 0 {
		m.cursor = 0
	}
	if n := len(vis); n > 0 && m.cursor >= n {
		m.cursor = n - 1
	}
	return m
}

// renderDetailWithCursor wraps DetailModel.View() with a line-cursor +
// optional Vim Visual highlight and a transient toast (e.g. "yanked 5
// lines"). The DetailModel itself is stateless across renders; the
// cursor / visual state lives on ListModel so the data flow stays
// idempotent.
func (m ListModel) renderDetailWithCursor() string {
	tk := activeTokens()
	body := m.newDetail().View()
	// Issue #168: append assigned-groups + assigned-apps sections
	// beneath the Pretty tab body so operators see what the user is
	// in / sees on their dashboard without leaving the detail surface.
	// Only the Pretty tab gets the extras — JSON / YAML stay raw.
	if m.detailTab == DetailTabPretty {
		body = body + "\n" + m.renderDetailExtras(tk)
	}
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
	//
	// Issue #139: pad each highlighted line with trailing spaces up to
	// the widest body line before styling, so the bg tint extends across
	// the full row instead of stopping at the cell content (e.g.
	// `"login":` would be highlighted but the rest of the row stayed
	// flat). The pad width is the max visible width of the SELECTABLE
	// body so the highlight is uniform across cursor moves.
	maxBodyWidth := 0
	for i := headerLines; i < len(lines); i++ {
		if w := shared.VisibleWidth(lines[i]); w > maxBodyWidth {
			maxBodyWidth = w
		}
	}
	// Issue #146: lipgloss's outer Style.Render emits its own
	// foreground/background prefix and a `\x1b[0m` suffix, but inner
	// ANSI resets emitted by the syntax highlighter (one per styled
	// segment) wipe the outer bg too. The result was a row highlight
	// that only covered text BEFORE the first inner reset — the key
	// portion of `"login": "alice@acme.com"` would tint, the value
	// after the colon stayed flat. Strip inner ANSI from the cursor
	// row so RowHighlight produces a uniform bg tint across the
	// whole line. The cost is losing syntax colour on the active row,
	// which is a fair trade for unambiguous cursor visibility.
	highlight := func(line string) string {
		plain := shared.StripCSI(line)
		w := shared.VisibleWidth(plain)
		if w < maxBodyWidth {
			plain = plain + strings.Repeat(" ", maxBodyWidth-w)
		}
		return tk.RowHighlight.Render(plain)
	}
	for i, line := range lines {
		switch {
		case i < headerLines:
			b.WriteString(line)
		case m.detailVisual && i >= start && i <= end:
			b.WriteString(highlight(line))
		case i == cursor:
			b.WriteString(highlight(line))
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
	// Pipe the live terminal width into Deps so the detail view's
	// 2-column layout sizes to the current window (issue #149).
	deps := m.deps
	deps.Width = m.width
	deps.Height = m.height
	d := NewDetailModel(deps, m.detailUser).WithActiveTab(m.detailTab)
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
	if m.detailTab == DetailTabPretty {
		body = body + "\n" + m.renderDetailExtras(activeTokens())
	}
	all := strings.Split(body, "\n")
	const headerLines = 3
	if len(all) <= headerLines {
		return nil
	}
	return all[headerLines:]
}

// renderDetailExtras builds the side-by-side Groups + Apps boxes
// rendered beneath the 2-col Pretty info layout (issues #168 +
// #170 + #174). Each box is a rounded-border widget with:
//   - title bar carrying the section name + "(N)" count
//   - scrollable content window (j/k advances when focused)
//   - vertical scrollbar (▒ thumb on a │ track) on the right edge
//   - focus halo: when EITHER box owns the linear cursor, only
//     that box lights up — never both at once (issue #174)
//
// Both boxes carry the same height so the chrome's body-row
// budget stays predictable — even a user with 200 groups won't
// push the chrome's top border off the screen.
//
// v0.1.15 (#174): a single linear cursor (detailExtrasCur) flows
// across both boxes. When the cursor sits inside the Groups
// range, the Groups box has focus + highlight; when it advances
// past len(groups) the Apps box takes over. j wraps from the last
// Apps row back to the first Groups row; k wraps the other
// direction.
func (m ListModel) renderDetailExtras(tk shared.Tokens) string {
	innerHeight := m.detailExtrasBoxHeight()
	colW := m.detailExtrasColWidth()

	groupsItems := m.formatGroupsItems(tk)
	appsItems := m.formatAppsItems(tk)

	groupsTitle := "Groups"
	if m.detailGroupsLoaded {
		groupsTitle = groupsTitle + "  (" + itoaSimple(len(m.detailGroups)) + ")"
	}
	appsTitle := "Apps"
	if m.detailAppsLoaded {
		appsTitle = appsTitle + "  (" + itoaSimple(len(m.detailApps)) + ")"
	}

	groupsFocus := m.detailExtrasFocused && m.detailExtrasCur < len(m.detailGroups)
	appsFocus := m.detailExtrasFocused && m.detailExtrasCur >= len(m.detailGroups)
	// Cursor row inside each box — outside its range it's set to a
	// negative value so renderScrollBox skips the highlight.
	groupsRow := -1
	appsRow := -1
	if groupsFocus {
		groupsRow = m.detailExtrasCur
	}
	if appsFocus {
		appsRow = m.detailExtrasCur - len(m.detailGroups)
	}

	groupsTop := clampScrollTop(maxInt(groupsRow, 0), m.detailGroupsTop, innerHeight, len(groupsItems))
	appsTop := clampScrollTop(maxInt(appsRow, 0), m.detailAppsTop, innerHeight, len(appsItems))

	left := renderScrollBox(
		groupsTitle,
		groupsItems,
		groupsFocus,
		groupsRow,
		groupsTop,
		innerHeight,
		colW,
		tk,
	)
	right := renderScrollBox(
		appsTitle,
		appsItems,
		appsFocus,
		appsRow,
		appsTop,
		innerHeight,
		colW,
		tk,
	)
	hint := tk.Muted.Render(
		"  ]  enter / next box   [  back to top   j/k  scroll (wraps)   Enter  open detail   Esc  exit boxes")
	return composeColumns(left, right, colW) + "\n" + hint
}

// detailExtrasTotal returns the number of selectable rows in the
// combined Groups+Apps cursor space — used by j/k wrap arithmetic.
// Loading / error / empty placeholders contribute 0 so the cursor
// stays at 0 until real data arrives.
func (m ListModel) detailExtrasTotal() int {
	g := 0
	if m.detailGroupsLoaded && m.detailGroupsErr == nil {
		g = len(m.detailGroups)
	}
	a := 0
	if m.detailAppsLoaded && m.detailAppsErr == nil {
		a = len(m.detailApps)
	}
	return g + a
}

// formatGroupsItems returns the bare row strings for the Groups
// box — `[TYPE]  name`. Loading / error / empty surface as a
// single muted row so the box never renders blank.
func (m ListModel) formatGroupsItems(tk shared.Tokens) []string {
	switch {
	case m.detailGroupsErr != nil:
		return []string{tk.Danger.Render("error: " + m.detailGroupsErr.Error())}
	case !m.detailGroupsLoaded:
		return []string{tk.Muted.Render("loading groups…")}
	case len(m.detailGroups) == 0:
		return []string{tk.Muted.Render("(no groups)")}
	}
	out := make([]string, 0, len(m.detailGroups))
	for _, g := range m.detailGroups {
		out = append(out, tk.Muted.Render("["+string(g.Type)+"]")+"  "+g.Profile.Name)
	}
	return out
}

// formatAppsItems returns the bare row strings for the Apps box.
// Each row is `Label  (appName)` so operators see both the
// dashboard label and the canonical Okta app name.
func (m ListModel) formatAppsItems(tk shared.Tokens) []string {
	switch {
	case m.detailAppsErr != nil:
		return []string{tk.Danger.Render("error: " + m.detailAppsErr.Error())}
	case !m.detailAppsLoaded:
		return []string{tk.Muted.Render("loading apps…")}
	case len(m.detailApps) == 0:
		return []string{tk.Muted.Render("(no apps assigned)")}
	}
	out := make([]string, 0, len(m.detailApps))
	for _, a := range m.detailApps {
		label := a.Label
		if label == "" {
			label = a.AppName
		}
		row := label
		if a.AppName != "" && a.AppName != label {
			row = row + "  " + tk.Muted.Render("("+a.AppName+")")
		}
		out = append(out, row)
	}
	return out
}

// detailExtrasBoxHeight returns the content-row count for each extras
// box — sized to consume whatever vertical space the info grid leaves
// in the body. v0.1.15 (#174) replaces the old fixed 18-row cap with
// dynamic measurement: render the info grid, count its lines, and
// subtract from the body budget to derive the box height. Tall
// terminals now use their full vertical real estate instead of
// leaving 30+ rows of dead space at the bottom.
//
// A floor of 5 rows guarantees a usable single-row view even on
// short terminals; the box's own scrollbar handles overflow.
func (m ListModel) detailExtrasBoxHeight() int {
	const minRows = 5
	available := shared.ListBodyRowBudget(m.height)

	// Measure the info grid's actual line count so the boxes adapt
	// to user-shaped content (Identity / Status / Address / …).
	infoLines := strings.Count(m.newDetail().View(), "\n") + 1
	// Box widget overhead: 1 row top border + 1 row bottom border
	// + 1 row of hint footer + 1 row of breathing room above the
	// boxes. The detail header (User Detail label, tab bar,
	// divider) is already counted inside infoLines.
	const overhead = 4
	rows := available - infoLines - overhead
	if rows < minRows {
		return minRows
	}
	return rows
}

// detailExtrasColWidth picks the per-box width. Splits the body
// inner width evenly across the two boxes minus a 2-cell gutter.
func (m ListModel) detailExtrasColWidth() int {
	w := m.usersInnerWidth()
	per := (w - 2) / 2
	if per < 30 {
		per = 30
	}
	return per
}

// clampScrollTop slides the scroll window so the cursor stays
// visible. Pure helper, no model state.
func clampScrollTop(cursor, scrollTop, height, total int) int {
	if total <= height {
		return 0
	}
	if cursor < scrollTop {
		return cursor
	}
	if cursor >= scrollTop+height {
		return cursor - height + 1
	}
	if scrollTop+height > total {
		return total - height
	}
	if scrollTop < 0 {
		return 0
	}
	return scrollTop
}

// renderScrollBox draws a single rounded-border box of fixed
// (height + 2) rows × (width) cells with title, content window,
// and scrollbar. Returns a multi-line string ready for
// composeColumns. The focused state lights up the border so the
// operator sees where j/k routes to.
func renderScrollBox(
	title string,
	items []string,
	focused bool,
	cursor, scrollTop, height, width int,
	tk shared.Tokens,
) string {
	if width < 12 {
		width = 12
	}
	if height < 1 {
		height = 1
	}
	// Border + scrollbar reserve 3 cells from inner content.
	contentW := width - 4
	if contentW < 4 {
		contentW = 4
	}

	borderStyle := tk.Muted
	if focused {
		borderStyle = tk.Header
	}

	titleStr := title
	if focused {
		titleStr = tk.Accent.Render(title)
	}

	top := borderStyle.Render("╭─ ") + titleStr + " " + borderStyle.Render(strings.Repeat("─", maxInt(0, width-5-shared.VisibleWidth(title)))+"╮")
	bottom := borderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")

	var lines []string
	lines = append(lines, top)
	for r := 0; r < height; r++ {
		idx := scrollTop + r
		row := ""
		if idx < len(items) {
			row = items[idx]
		}
		row = padOrTruncateVisible(row, contentW)
		if focused && idx == cursor && idx < len(items) {
			row = tk.RowHighlight.Render(row)
		}
		// Scrollbar marker: thumb (▌) when this row is inside the
		// active scroll window, track (│) otherwise. Position
		// scales with cursor / total. Hidden when everything fits.
		bar := " "
		if len(items) > height {
			thumbStart, thumbEnd := scrollbarRange(scrollTop, height, len(items))
			if r >= thumbStart && r <= thumbEnd {
				bar = tk.Accent.Render("▌")
			} else {
				bar = tk.Muted.Render("│")
			}
		}
		lines = append(lines, borderStyle.Render("│ ")+row+" "+bar+borderStyle.Render("│"))
	}
	lines = append(lines, bottom)
	return strings.Join(lines, "\n")
}

// scrollbarRange maps (scrollTop, height, total) to the inclusive
// row-index range of the scrollbar thumb. Always returns at least
// a single-row thumb so the operator sees their position.
func scrollbarRange(scrollTop, height, total int) (start, end int) {
	if total <= height {
		return 0, height - 1
	}
	scale := float64(height) / float64(total)
	thumbStart := int(float64(scrollTop) * scale)
	thumbLen := int(float64(height) * scale)
	if thumbLen < 1 {
		thumbLen = 1
	}
	thumbEnd := thumbStart + thumbLen - 1
	if thumbEnd >= height {
		thumbEnd = height - 1
		thumbStart = thumbEnd - thumbLen + 1
		if thumbStart < 0 {
			thumbStart = 0
		}
	}
	return thumbStart, thumbEnd
}

// padOrTruncateVisible pads / truncates s to exactly width visible
// cells, honouring inner ANSI styling. The shared.VisibleWidth
// helper already accounts for runewidth so CJK-wide glyphs don't
// drift.
func padOrTruncateVisible(s string, width int) string {
	w := shared.VisibleWidth(s)
	if w == width {
		return s
	}
	if w > width {
		return shared.Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

// contextLine returns "" — the resource label, count, and filter all
// live in the chrome's upper divider now (issues #133 + #136). Kept
// as a method so existing callers compile unchanged; their `\n` after
// it gives the view a one-row gap above the column header.
func (m ListModel) contextLine(visible []domain.User) string {
	return ""
}

// renderUsersHeader returns the column header row, width-aware
// (issue #145 column lineup): LOGIN, NICKNAME, DIVISION, DEPARTMENT,
// TITLE, STATUS, LAST LOGIN, LAST UPDATED. The whole row is wrapped
// in tk.Header (bold + accent) for issue #137. Sort glyphs survive
// the outer wrap.
func (m ListModel) renderUsersHeader(tk shared.Tokens) string {
	row := m.formatUsersColumns(
		usersSortLabel("LOGIN", m.sortBy, SortName, m.sortDir, tk),
		"NICKNAME",
		"DIVISION",
		"DEPARTMENT",
		"TITLE",
		usersSortLabel("STATUS", m.sortBy, SortStatus, m.sortDir, tk),
		usersSortLabel("LAST LOGIN", m.sortBy, SortLastLogin, m.sortDir, tk),
		usersSortLabel("LAST UPDATED", m.sortBy, SortCreated, m.sortDir, tk),
	)
	return tk.Header.Render(row)
}

// usersSortLabel appends a coloured ↑ / ↓ to title when this column is
// the active sort column. The glyph is green for asc, red for desc
// (issue #118) so operators spot the active column at a glance.
// SortNone / SortOff renders the label unchanged.
func usersSortLabel(title string, active, key SortKey, dir SortDir, tk shared.Tokens) string {
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

// renderUsersRow formats a single User row, width-aware.
func (m ListModel) renderUsersRow(u domain.User, now time.Time, tk shared.Tokens) string {
	status := shared.UserStatusBadge(string(u.Status), tk).Render(tk)
	lastLogin := shared.RelativeTime(u.LastLogin, now)
	lastUpdated := shared.RelativeTime(&u.LastUpdated, now)
	dash := func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	}
	// Order matches usersColumnSpecs: LOGIN first so it lands left of
	// the eye, identity attrs next, status mid-row, timestamps right.
	return m.formatUsersColumns(
		u.Profile.Login,
		dash(u.Profile.NickName),
		dash(u.Profile.Division),
		dash(u.Profile.Department),
		dash(u.Profile.Title),
		status,
		lastLogin,
		lastUpdated,
	)
}

// usersColumnSpecs is the column lineup the user pinned in #145:
//
//	LOGIN > NICKNAME > DIVISION > DEPARTMENT > TITLE > STATUS >
//	LAST LOGIN > LAST UPDATED
//
// EMPLOYEE# was dropped at the same time. Drop priority degrades
// from the right so the LOGIN identity stays visible longest:
//
//	1  LAST UPDATED
//	2  LAST LOGIN
//	3  TITLE
//	4  DEPARTMENT
//	5  DIVISION
//	6  NICKNAME
//	0  LOGIN, STATUS — never dropped (essentials)
//
// Min widths are placeholders — ShrinkSpecsToFit overrides them with
// max(header, observed-data-width) every render so the layout
// honours the actual fetched data.
func usersColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "LOGIN", Kind: shared.ColumnFlex, Min: 18, Weight: 3, DropPriority: 0},
		{Title: "NICKNAME", Kind: shared.ColumnFlex, Min: 10, Weight: 1, DropPriority: 6},
		{Title: "DIVISION", Kind: shared.ColumnFlex, Min: 10, Weight: 1, DropPriority: 5},
		{Title: "DEPARTMENT", Kind: shared.ColumnFlex, Min: 10, Weight: 1, DropPriority: 4},
		{Title: "TITLE", Kind: shared.ColumnFlex, Min: 12, Weight: 2, DropPriority: 3},
		{Title: "STATUS", Kind: shared.ColumnFixed, Min: 16, DropPriority: 0, AlignCenter: true},
		{Title: "LAST LOGIN", Kind: shared.ColumnFixed, Min: 10, DropPriority: 2, AlignRight: true},
		{Title: "LAST UPDATED", Kind: shared.ColumnFixed, Min: 12, DropPriority: 1, AlignRight: true},
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
	specs := m.scaledUsersSpecs()
	widths := m.usersWidths(specs)

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

// scaledUsersSpecs returns the column specs after auto-fit shrink and
// the sort-glyph bump — both layout decisions need to agree across the
// header row, the data rows, and the hScroll clamp.
func (m ListModel) scaledUsersSpecs() []shared.ColumnSpec {
	specs := usersColumnSpecs()
	specs = shared.ShrinkSpecsToFit(specs, m.observedColumnWidths())
	if i := usersSortColumnIdx(m.sortBy); i >= 0 && m.sortDir != SortOff {
		specs[i].Min++
	}
	return specs
}

// usersWidths picks the width slice for a render. Three-stage fall-back:
//
//  1. Tight (issue #138): try to fit every column at its observed-data
//     width. No flex puffing — leftover space stays empty at end of
//     row. The user wants tight columns; LOGIN should never grow past
//     the longest login it actually contains.
//  2. hScroll: when the tight layout overflows, switch to the
//     hScroll-aware packing so h / l can pan across columns.
//
// The previous "LayoutColumns then puff flex" path was actively
// hostile to operators staring at 30-char-wide LOGIN columns full of
// trailing whitespace.
func (m ListModel) usersWidths(specs []shared.ColumnSpec) []int {
	inner := m.usersInnerWidth()
	if w := shared.LayoutColumnsTight(specs, inner, 2); w != nil {
		return w
	}
	return shared.LayoutColumnsHScroll(specs, inner, 2, m.hScroll)
}

// clampHScroll bounds the horizontal scroll cursor to [0, MaxHScroll]
// using the same scaled specs the renderer reads, so h/l can never
// step into an empty row even after auto-fit / sort-glyph bumps shift
// the column widths.
func (m ListModel) clampHScroll(want int) int {
	if want < 0 {
		return 0
	}
	specs := m.scaledUsersSpecs()
	max := shared.MaxHScroll(specs, m.usersInnerWidth(), 2)
	if want > max {
		return max
	}
	return want
}

// observedColumnWidths returns the largest cell width seen in each
// column across the currently visible rows. Powers ShrinkSpecsToFit so
// every column gets exactly the width its data demands. Order must
// match usersColumnSpecs: LOGIN, NICKNAME, DIVISION, DEPARTMENT,
// TITLE, STATUS, LAST LOGIN, LAST UPDATED.
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
			u.Profile.Login,
			dash(u.Profile.NickName),
			dash(u.Profile.Division),
			dash(u.Profile.Department),
			dash(u.Profile.Title),
			statusBadge,
			shared.RelativeTime(u.LastLogin, now),
			shared.RelativeTime(&u.LastUpdated, now),
		}
		for i, c := range cells {
			if w := visibleCellWidth(c); w > out[i] {
				out[i] = w
			}
		}
	}
	return out
}

// visibleCellWidth returns the rendered cell width of s, delegating
// to the shared package's CSI-aware stripper. Earlier versions tried
// to hand-roll the escape skip and miscounted because the CSI
// introducer `[` (0x5b) sits inside the 0x40-0x7e final-byte range —
// it exited escape mode on the introducer itself, then counted the
// SGR parameters (digits, semicolons, `m`) as visible cells. The
// resulting over-estimate kept ShrinkSpecsToFit from actually
// shrinking columns, leaving rows wider than the viewport (issue #128).
func visibleCellWidth(s string) int {
	return shared.VisibleWidth(s)
}

// usersSortColumnIdx maps a SortKey to its index in usersColumnSpecs.
// Issue #145 column order (v0.1.17 reverts the GROUPS / APPS columns
// added in #178 — they fanned out 2N extra API calls per list load
// and were the largest single contributor to rate-limit warnings;
// the per-user counts still surface inside User Detail's Groups +
// Apps boxes which are lazily fetched only when the operator opens
// the detail surface):
//
//	0 LOGIN · 1 NICKNAME · 2 DIVISION · 3 DEPARTMENT · 4 TITLE ·
//	5 STATUS · 6 LAST LOGIN · 7 LAST UPDATED
func usersSortColumnIdx(k SortKey) int {
	switch k {
	case SortName:
		return 0
	case SortStatus:
		return 5
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
	// chrome border (2) + left padding (1) + cursor gutter (2 for "> "/"  ")
	// + scrollbar gutter (2: 1-cell gap + 1-cell bar) reserves room for
	// the right-edge scrollbar (v0.1.15 issue #173). Subtracting from
	// the width returned to the column layout means LayoutColumns picks
	// a tighter set, leaving 2 cells of clean space at the end of the
	// row for the scrollbar to render flush against the chrome border.
	inner := w - 2 - 1 - 2 - 2
	if inner < 20 {
		inner = 20
	}
	return inner
}

// chromeContentWidth returns the body cells the chrome reserves for
// list content per row — width minus the chrome's left border, left
// padding, and right border. The list pads each row out to this
// width minus 2 (for " ▌"/" │") so the scrollbar lands flush against
// the right border regardless of how wide the columns end up
// rendering.
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

// Actions publishes the resource-specific actions surfaced by the
// `a` action menu (issue #175 v0.1.15). Three Users lifecycle
// operations: reset password, unlock, reset MFA factors. The IDs
// match the `:` palette commands so the App Shell can route both
// entry points through the same confirmation flow.
func (m ListModel) Actions() []shared.ActionItem {
	return []shared.ActionItem{
		{ID: "reset-password", Label: "Reset password", Hint: "send the standard reset email"},
		{ID: "unlock", Label: "Unlock account", Hint: "clear LOCKED_OUT state"},
		{ID: "reset-factors", Label: "Reset MFA factors", Hint: "destructive — forces re-enrollment"},
	}
}

// RunAction emits a shared.RunUserActionMsg back into the App Shell
// when the operator picks an item from the `a` menu. The App Shell
// already routes this into openActionConfirm so the y/N gate fires
// for every destructive op (issue #125 + #175).
func (m ListModel) RunAction(id string) (tea.Model, tea.Cmd) {
	return m, func() tea.Msg { return shared.RunUserActionMsg{Kind: id} }
}

// SelectedUser surfaces the active user (the open detail target while
// `m.opened` is true, otherwise the cursor row) so the App Shell can
// hand it to lifecycle confirmation modals (issue #125). Implements
// the app.SelectedUserStater interface.
func (m ListModel) SelectedUser() (domain.User, bool) {
	if m.opened {
		if m.detailUser.ID != "" {
			return m.detailUser, true
		}
	}
	if u := m.selected(); u != nil {
		return *u, true
	}
	return domain.User{}, false
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

// fetchUserGroupsCmd lazily loads the user's assigned groups for
// the User Detail Groups section (issue #168).
func fetchUserGroupsCmd(port domain.UsersPort, userID string) tea.Cmd {
	return func() tea.Msg {
		groups, err := port.ListGroups(context.Background(), userID)
		if err != nil {
			return userDetailGroupsErrMsg{userID: userID, err: err}
		}
		return userDetailGroupsLoadedMsg{userID: userID, groups: groups}
	}
}

// fetchUserAppLinksCmd lazily loads the user's assigned apps for
// the User Detail Apps section (issue #168).
func fetchUserAppLinksCmd(port domain.UsersPort, userID string) tea.Cmd {
	return func() tea.Msg {
		links, err := port.ListAppLinks(context.Background(), userID)
		if err != nil {
			return userDetailAppsErrMsg{userID: userID, err: err}
		}
		return userDetailAppsLoadedMsg{userID: userID, apps: links}
	}
}

// openGroupDetailCmd / openAppDetailCmd emit the cross-screen drill-down
// requests the App Shell routes to the Groups / Apps screens (issue
// #171). The shared.OpenGroup/AppDetailMsg types live in the shared
// package to keep the users → app import boundary clean.
func openGroupDetailCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenGroupDetailMsg{ID: id} }
}

func openAppDetailCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenAppDetailMsg{ID: id} }
}

var _ tea.Model = ListModel{}
