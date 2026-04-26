package users

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
	lastErr    error
	// width is the most recent terminal width seen via WindowSizeMsg. Drives
	// responsive column drop per TUI_DESIGN §15.2.
	width int
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
	return ListModel{deps: deps, users: deps.InitialUsers, width: deps.Width}
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
		// MVP: opening the detail view also ends the list program for tests
		// that use teatest.FinalOutput. The App Shell will replace this with
		// a proper screen transition when the router is wired in v0.2.
		return m, tea.Quit
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ListModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case keys.IDNavSelect:
		sel := m.selected()
		if sel == nil {
			return m, nil
		}
		return m, openUserCmd(m.deps.Port, sel.ID)
	}
	return m, nil
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
		return "User Detail\n" + m.detailUser.Profile.Login + "\n"
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
	b.WriteString(m.renderUsersHeader(tk))
	b.WriteByte('\n')
	for i, u := range rows {
		row := m.renderUsersRow(u, m.now(), tk)
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

// renderUsersHeader returns the column header row, width-aware (TUI_DESIGN
// §15.2 responsive drop).
func (m ListModel) renderUsersHeader(_ shared.Tokens) string {
	return m.formatUsersColumns("STATUS", "LOGIN", "DISPLAY NAME", "LAST LOGIN", "CHANGED")
}

// renderUsersRow formats a single User row, width-aware.
func (m ListModel) renderUsersRow(u domain.User, now time.Time, tk shared.Tokens) string {
	status := shared.UserStatusBadge(string(u.Status), tk).Render(tk)
	display := strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName)
	if display == "" {
		display = u.Profile.DisplayName
	}
	last := shared.RelativeTime(u.LastLogin, now)
	changed := shared.RelativeTime(u.StatusChanged, now)
	return m.formatUsersColumns(status, u.Profile.Login, display, last, changed)
}

// formatUsersColumns lays out STATUS / LOGIN / DISPLAY NAME / LAST LOGIN /
// CHANGED with TUI_DESIGN §15.2 responsive drop:
//
//   - W ≥ 120 : all 5 columns
//   - 100..119: drop CHANGED
//   - 90..99  : drop CHANGED + DISPLAY NAME
//   - 80..89  : keep STATUS + LOGIN + LAST LOGIN
//   - <80     : STATUS + LOGIN only
//
// width 0 (no WindowSizeMsg yet) renders the full layout — first frame
// uses the chrome's default 85-col fallback so callers see all columns even
// before tea reports terminal size.
func (m ListModel) formatUsersColumns(status, login, display, lastLogin, changed string) string {
	w := m.width
	const (
		wStatus  = 14
		wLogin   = 28
		wDisplay = 18
		wLast    = 10
		wChanged = 8
	)
	switch {
	case w >= 120 || w == 0:
		return padRight(status, wStatus) + "  " + padRight(login, wLogin) + "  " +
			padRight(display, wDisplay) + "  " + padLeft(lastLogin, wLast) + "  " +
			padLeft(changed, wChanged)
	case w >= 100:
		return padRight(status, wStatus) + "  " + padRight(login, wLogin) + "  " +
			padRight(display, wDisplay) + "  " + padLeft(lastLogin, wLast)
	case w >= 90:
		return padRight(status, wStatus) + "  " + padRight(login, wLogin) + "  " +
			padLeft(lastLogin, wLast)
	case w >= 80:
		return padRight(status, wStatus) + "  " + padRight(login, 22) + "  " +
			padLeft(lastLogin, wLast)
	default:
		return padRight(status, wStatus) + "  " + padRight(login, max(0, w-18))
	}
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// padRight left-aligns s within width using spaces (or truncates with "…").
func padRight(s string, width int) string {
	if visibleLen(s) > width {
		return shared.Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-visibleLen(s))
}

// padLeft right-aligns s within width using leading spaces.
func padLeft(s string, width int) string {
	if visibleLen(s) > width {
		return shared.Truncate(s, width)
	}
	return strings.Repeat(" ", width-visibleLen(s)) + s
}

// visibleLen counts visible cells, ignoring ANSI escapes. We re-implement a
// minimal CSI stripper here to avoid a circular import; chrome.go has the
// canonical version.
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
		// rune-naive count (sufficient for the ASCII + box-drawing alphabet
		// our cells use)
		_, size := decodeRune(s[i:])
		count++
		i += size
	}
	return count
}

func decodeRune(s string) (r rune, size int) {
	if len(s) == 0 {
		return 0, 0
	}
	b := s[0]
	switch {
	case b < 0x80:
		return rune(b), 1
	case b < 0xC0:
		return rune(b), 1
	case b < 0xE0:
		if len(s) < 2 {
			return rune(b), 1
		}
		return rune(b), 2
	case b < 0xF0:
		if len(s) < 3 {
			return rune(b), 1
		}
		return rune(b), 3
	default:
		if len(s) < 4 {
			return rune(b), 1
		}
		return rune(b), 4
	}
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
// Profile.Login) to m.users.
func (m ListModel) visible() []domain.User {
	if m.filter == "" {
		return m.users
	}
	needle := strings.ToLower(m.filter)
	out := make([]domain.User, 0, len(m.users))
	for _, u := range m.users {
		if strings.Contains(strings.ToLower(u.Profile.Login), needle) {
			out = append(out, u)
		}
	}
	return out
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
