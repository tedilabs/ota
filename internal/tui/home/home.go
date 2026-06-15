// Package home renders the Okta admin-style dashboard surface that
// ota boots into. Each "card" owns its own data fetcher + freshness
// stamp so the page paints fast from cache and back-fills as the
// individual Okta API responses land. Phase 1 (2026-05-04) ships the
// frame + card grid + focus navigation; data wiring follows in
// Phase 2 (counts), 3 (Δ + sparklines), 4 (activity / posture),
// 5 (recent critical events), 6 (time-window toggle).
package home

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// Deps wires the home dashboard to the same outbound ports the
// resource list/detail screens use — each card's fetcher is built
// from these in subsequent phases. Width/Height + RefreshInterval
// follow the convention every other list screen uses so the chrome
// behaviour stays uniform.
type Deps struct {
	Users          domain.UsersPort
	Groups         domain.GroupsPort
	GroupRules     domain.GroupRulesPort
	Policies       domain.PoliciesPort
	Apps           domain.AppsPort
	Authenticators domain.AuthenticatorsPort
	Logs           domain.LogsPort
	APITokens      domain.APITokensPort
	Administrators domain.AdministratorsPort

	OrgURL string
	Clock  clock.Clock
	Logger *slog.Logger
	Keys   keys.ResolvedMap
	Width  int
	Height int

	// RefreshInterval triggers a background re-fetch of every card.
	// Zero disables auto-refresh; the operator can still hit `R`.
	RefreshInterval time.Duration
}

// CardID enumerates the focusable cards on the dashboard. Tab /
// Shift-Tab + j/k cycle through them; Enter drills to the matching
// resource screen.
type CardID int

const (
	CardUsers CardID = iota
	CardGroups
	CardApps
	CardActivity
	CardPosture
	CardHealth
	CardEvents
)

// cardOrder is the tab focus cycle. Driven by Tab / Shift-Tab.
// Visual layout uses a different traversal (rows L→R, top→bottom)
// described in renderGrid.
var cardOrder = []CardID{
	CardUsers, CardGroups, CardApps,
	CardActivity,
	CardPosture, CardHealth,
	CardEvents,
}

// Model is the home dashboard screen.
type Model struct {
	deps   Deps
	width  int
	height int

	focus  CardID
	cursor int // index into cardOrder

	lastUpdated time.Time
	refreshGen  int
}

// New constructs an empty dashboard. Cards render placeholder copy
// until Phase 2 wires the per-card fetchers.
func New(deps Deps) Model {
	return Model{
		deps:   deps,
		width:  deps.Width,
		height: deps.Height,
		focus:  CardUsers,
	}
}

// Init is currently a no-op — Phase 2 will fan out one cmd per card.
func (m Model) Init() tea.Cmd { return nil }

// Update handles window sizing + focus navigation. Card-specific
// data messages will land here in later phases.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	km = shared.NormalizeArrowKey(km)
	switch km.Type {
	case tea.KeyTab:
		m.advanceFocus(1)
		return m, nil
	case tea.KeyShiftTab:
		m.advanceFocus(-1)
		return m, nil
	case tea.KeyEnter:
		// Drill into the resource screen the focused card represents.
		if target, ok := drillTargetFor(m.focus); ok {
			return m, openResourceCmd(target)
		}
		return m, nil
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			m.advanceFocus(1)
		case "k":
			m.advanceFocus(-1)
		case "g":
			m.cursor = 0
			m.focus = cardOrder[0]
		case "G":
			m.cursor = len(cardOrder) - 1
			m.focus = cardOrder[m.cursor]
		case "r", "R":
			// Phase 2: triggers re-fetch. Phase 1 just bumps the
			// generation counter so the test scaffolding can observe.
			m.refreshGen++
		}
	}
	return m, nil
}

func (m *Model) advanceFocus(d int) {
	n := len(cardOrder)
	m.cursor = (m.cursor + d + n) % n
	m.focus = cardOrder[m.cursor]
}

// drillTargetFor maps a card to the resource screen name the App
// Shell understands (forwarded via shared.SwitchScreenMsg). Cards
// without a drill target (Activity sparkline, Health) return false.
func drillTargetFor(c CardID) (string, bool) {
	switch c {
	case CardUsers:
		return "users", true
	case CardGroups:
		return "groups", true
	case CardApps:
		return "apps", true
	case CardEvents:
		return "logs", true
	}
	return "", false
}

// openResourceCmd emits shared.OpenScreenMsg — the App Shell maps
// that through screenFromName and PUSHES onto the nav stack so Esc
// returns to the dashboard the operator came from.
func openResourceCmd(target string) tea.Cmd {
	return func() tea.Msg {
		return shared.OpenScreenMsg{Target: target}
	}
}

// LastUpdated implements app.LastUpdatedStater.
func (m Model) LastUpdated() time.Time { return m.lastUpdated }

// EscapeWillAct — the home screen has no internal modal state, so
// Esc at the root frame is always free to fire the quit confirm.
func (m Model) EscapeWillAct() bool { return false }

// StatusBadges contributes a tiny FOCUS badge to the chrome's
// status row so the operator can confirm which card has Tab focus
// without parsing the dashboard for `▸`. Useful in screen-reader /
// mono terminals.
func (m Model) StatusBadges() []shared.ChromeBadge {
	return []shared.ChromeBadge{
		{Key: "FOCUS", Value: cardLabel(m.focus)},
	}
}

// View renders the full dashboard grid. Width-aware: collapses to a
// single column when the terminal is too narrow to host the 3-up row.
func (m Model) View() string {
	tk := activeTokens()
	width := m.chromeContentWidth()
	if width <= 0 {
		width = shared.ChromeWidth - 3
	}

	var b strings.Builder
	b.WriteString(m.renderHeader(width, tk))
	b.WriteByte('\n')
	b.WriteString(m.renderGrid(width, tk))
	return b.String()
}

// renderHeader stamps "Home · <org> · <ts>" across the top of the
// body so the operator sees what tenant they're looking at at a
// glance.
func (m Model) renderHeader(width int, tk shared.Tokens) string {
	stamp := m.now().UTC().Format("2006-01-02 15:04 UTC")
	left := tk.Header.Render("Home")
	mid := tk.Muted.Render(" · " + orgShortName(m.deps.OrgURL))
	right := tk.Muted.Render(stamp)

	line := left + mid
	gap := width - shared.VisibleWidth(line) - shared.VisibleWidth(right)
	if gap < 1 {
		gap = 1
	}
	return line + strings.Repeat(" ", gap) + right
}

// renderGrid lays out the dashboard rows. Each row is a horizontal
// composition of 1–3 card boxes.
func (m Model) renderGrid(width int, tk shared.Tokens) string {
	row1 := m.renderRow(width, tk,
		m.renderCard(CardUsers, "Users", "(counts wire in Phase 2)"),
		m.renderCard(CardGroups, "Groups", "(counts wire in Phase 2)"),
		m.renderCard(CardApps, "Apps", "(counts wire in Phase 2)"),
	)
	row2 := m.renderRow(width, tk,
		m.renderCard(CardActivity, "Activity (last 24h · 7d)", "(activity wires in Phase 4)"),
	)
	row3 := m.renderRow(width, tk,
		m.renderCard(CardPosture, "Posture & Risk", "(posture wires in Phase 4)"),
		m.renderCard(CardHealth, "Health", "(health wires in Phase 4)"),
	)
	row4 := m.renderRow(width, tk,
		m.renderCard(CardEvents, "Recent Critical Events", "(events wire in Phase 5)"),
	)
	return row1 + "\n\n" + row2 + "\n\n" + row3 + "\n\n" + row4
}

// renderRow joins card panels side-by-side, evenly dividing width
// (1-cell gutter). When the row would collapse below per-card
// minWidth, falls back to a vertical stack so each card still has
// readable horizontal space.
func (m Model) renderRow(width int, tk shared.Tokens, cards ...string) string {
	const minCardWidth = 28
	const gutter = 1
	n := len(cards)
	if n == 0 {
		return ""
	}
	per := (width - gutter*(n-1)) / n
	if per < minCardWidth && n > 1 {
		return strings.Join(cards, "\n")
	}
	// Side-by-side compose: split each card into its own line slice,
	// pad each line to `per` cells, then zip.
	lineSlices := make([][]string, n)
	maxLines := 0
	for i, c := range cards {
		lines := strings.Split(c, "\n")
		for j, l := range lines {
			lines[j] = shared.PadOrTruncateVisible(l, per)
		}
		lineSlices[i] = lines
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	for i := range lineSlices {
		for len(lineSlices[i]) < maxLines {
			lineSlices[i] = append(lineSlices[i], strings.Repeat(" ", per))
		}
	}
	var b strings.Builder
	for row := 0; row < maxLines; row++ {
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(lineSlices[i][row])
		}
		if row < maxLines-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderCard draws one labeled bordered card. Focus state is
// indicated by an accent border + bolded title; unfocused cards
// stay muted so the operator's eye snaps to the active one.
func (m Model) renderCard(id CardID, label, body string) string {
	tk := activeTokens()
	focused := id == m.focus
	titleStyle := tk.Muted.Bold(true)
	if focused {
		titleStyle = tk.Accent.Bold(true)
	}
	title := titleStyle.Render(label)
	bodyStyle := tk.FG
	if !focused {
		bodyStyle = tk.Muted
	}
	bodyLines := strings.Split(body, "\n")
	for i, l := range bodyLines {
		bodyLines[i] = bodyStyle.Render(l)
	}
	return title + "\n" + strings.Join(bodyLines, "\n")
}

// cardLabel maps a CardID back to a short label for the status row.
func cardLabel(c CardID) string {
	switch c {
	case CardUsers:
		return "Users"
	case CardGroups:
		return "Groups"
	case CardApps:
		return "Apps"
	case CardActivity:
		return "Activity"
	case CardPosture:
		return "Posture"
	case CardHealth:
		return "Health"
	case CardEvents:
		return "Events"
	}
	return "—"
}

// orgShortName extracts the tenant hostname from the OrgURL so the
// header reads "acme.okta.com" rather than the full URL. Falls back
// to the raw OrgURL when parsing fails.
func orgShortName(orgURL string) string {
	s := strings.TrimSpace(orgURL)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	return s
}

func (m Model) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

// chromeContentWidth mirrors the helper every list screen uses —
// reserves the chrome's scrollbar gutter so the dashboard never
// overflows the right border.
func (m Model) chromeContentWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	return w - 3
}

// activeTokens picks the right token set per active theme.
func activeTokens() shared.Tokens {
	return shared.PickTheme(shared.ResolveTheme(""))
}

// FocusedCard returns the currently focused card — exported so
// tests can assert the focus cycle without poking internals.
func (m Model) FocusedCard() CardID { return m.focus }

// CardCount returns the number of focusable cards (drives the
// modulus in advanceFocus + lets tests verify the cycle wraps).
func (m Model) CardCount() int { return len(cardOrder) }

// _ = strconv keeps the import live until Phase 2 wires the count
// renders (each card body will format integer metrics).
var _ = strconv.Itoa
