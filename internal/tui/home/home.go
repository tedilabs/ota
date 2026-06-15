// Package home renders the Okta admin-style dashboard surface that
// ota boots into. Each "card" owns its own data fetcher + freshness
// stamp so the page paints fast from cache and back-fills as the
// individual Okta API responses land. Phase 1 (2026-05-04) shipped
// the frame + card grid + focus navigation; Phase 2 wires the
// Users / Groups / Apps count fetchers + disk-backed snapshot
// cache for instant first-paint across sessions. Phase 3 adds
// Δ-vs-7d. Phase 4 wires Activity / Posture / Health. Phase 5
// wires Recent Critical Events.
package home

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/dashboard"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// Deps wires the home dashboard to the same outbound ports the
// resource list/detail screens use — each card's fetcher is built
// from these. Width/Height + RefreshInterval follow the convention
// every other list screen uses so the chrome behaviour stays
// uniform.
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

	// Cache backs the per-card snapshot persisted across sessions.
	// Nil falls back to in-memory only (tests / cache-dir failure).
	Cache *dashboard.Cache
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
var cardOrder = []CardID{
	CardUsers, CardGroups, CardApps,
	CardActivity,
	CardPosture, CardHealth,
	CardEvents,
}

// cardState tracks per-card lifecycle independently — fetching, last
// error, last successful observation. The View reads these to paint
// the freshness stamp + spinner / error glyph.
type cardState struct {
	loading bool
	err     error
	counts  dashboard.Counts
	// hasCounts distinguishes "we observed an empty list" (Total=0
	// but real) from "we haven't fetched yet" (Total=0 and stale).
	hasCounts bool
}

// Model is the home dashboard screen.
type Model struct {
	deps   Deps
	width  int
	height int

	focus  CardID
	cursor int // index into cardOrder

	cards map[CardID]*cardState

	lastUpdated time.Time
	refreshGen  int
}

// New constructs the dashboard. Cards seed from cache (so the first
// render isn't all "loading…"); Init fans out background re-fetches
// to keep them fresh.
func New(deps Deps) Model {
	m := Model{
		deps:   deps,
		width:  deps.Width,
		height: deps.Height,
		focus:  CardUsers,
		cards:  map[CardID]*cardState{},
	}
	for _, c := range cardOrder {
		m.cards[c] = &cardState{}
	}
	m.seedFromCache()
	return m
}

// seedFromCache populates each card with the last persisted counts
// so the first paint isn't empty. Network fetches in Init override
// these once they land.
func (m *Model) seedFromCache() {
	if m.deps.Cache == nil {
		return
	}
	for _, pair := range []struct {
		card CardID
		key  string
	}{
		{CardUsers, dashboard.CardUsers},
		{CardGroups, dashboard.CardGroups},
		{CardApps, dashboard.CardApps},
	} {
		if c, ok := m.deps.Cache.Get(pair.key); ok {
			s := m.cards[pair.card]
			s.counts = c
			s.hasCounts = true
		}
	}
}

// Init fans out one cmd per card with a port to call. Cards without
// ports (or in Phase 4/5 that aren't yet wired) stay seeded from
// cache. Phase 2 wires Users / Groups / Apps.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, c := range cardOrder {
		if cmd := m.fetchCmdFor(c); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// Refresh tick chain — fires once after RefreshInterval, then
	// followFetchedMsg-style chains itself on each tick.
	if tick := m.scheduleRefreshTickCmd(); tick != nil {
		cmds = append(cmds, tick)
	}
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

// fetchCmdFor returns the cmd that loads `c`'s counts, or nil when
// the card isn't fetchable in this phase. The returned msg is a
// per-card cardLoadedMsg that Update folds back into m.cards.
func (m Model) fetchCmdFor(c CardID) tea.Cmd {
	switch c {
	case CardUsers:
		if m.deps.Users == nil {
			return nil
		}
		port := m.deps.Users
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			counts, err := countUsers(ctx, port, m.now())
			return cardLoadedMsg{card: CardUsers, counts: counts, err: err}
		}
	case CardGroups:
		if m.deps.Groups == nil {
			return nil
		}
		port := m.deps.Groups
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			counts, err := countGroups(ctx, port, m.now())
			return cardLoadedMsg{card: CardGroups, counts: counts, err: err}
		}
	case CardApps:
		if m.deps.Apps == nil {
			return nil
		}
		port := m.deps.Apps
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			counts, err := countApps(ctx, port, m.now())
			return cardLoadedMsg{card: CardApps, counts: counts, err: err}
		}
	}
	return nil
}

// cardLoadedMsg is the result one card fetcher delivers back to
// Update. Phase 4 will add cardLoadedActivityMsg etc. with their
// own payload shapes; keeping per-card msg types means the type
// switch in Update reads as documentation.
type cardLoadedMsg struct {
	card   CardID
	counts dashboard.Counts
	err    error
}

// refreshTickMsg fires the auto-refresh chain. Carries a gen value
// so a tick from a previous chain (e.g. operator hit R in the
// middle of an interval) gets dropped instead of double-fetching.
type refreshTickMsg struct{ gen int }

func (m Model) scheduleRefreshTickCmd() tea.Cmd {
	if m.deps.RefreshInterval <= 0 {
		return nil
	}
	return shared.ScheduleRefreshTickCmd(m.deps.RefreshInterval,
		refreshTickMsg{gen: m.refreshGen})
}

// Update handles window sizing + focus navigation + per-card fetch
// results.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case cardLoadedMsg:
		st := m.cards[msg.card]
		if st == nil {
			st = &cardState{}
			m.cards[msg.card] = st
		}
		st.loading = false
		st.err = msg.err
		if msg.err == nil {
			st.counts = msg.counts
			st.hasCounts = true
			m.persist(msg.card, msg.counts)
		}
		m.lastUpdated = m.now()
		return m, nil
	case refreshTickMsg:
		if msg.gen != m.refreshGen {
			return m, nil
		}
		var cmds []tea.Cmd
		for _, c := range cardOrder {
			if cmd := m.fetchCmdFor(c); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		cmds = append(cmds, m.scheduleRefreshTickCmd())
		return m, tea.Batch(cmds...)
	case shared.RefreshScreenMsg:
		return m, m.refreshAllCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// refreshAllCmd bumps the generation + fires every fetcher at once.
// Hooked to `R` + shared.RefreshScreenMsg.
func (m *Model) refreshAllCmd() tea.Cmd {
	m.refreshGen++
	var cmds []tea.Cmd
	for _, c := range cardOrder {
		if cmd := m.fetchCmdFor(c); cmd != nil {
			cmds = append(cmds, cmd)
			if st := m.cards[c]; st != nil {
				st.loading = true
			}
		}
	}
	cmds = append(cmds, m.scheduleRefreshTickCmd())
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

// persist writes a card's latest counts to the cache so the next
// session boots with fresh-feeling data.
func (m Model) persist(card CardID, counts dashboard.Counts) {
	if m.deps.Cache == nil {
		return
	}
	key, ok := cacheKeyFor(card)
	if !ok {
		return
	}
	_ = m.deps.Cache.Put(key, counts)
}

// cacheKeyFor maps the CardID to the stable string key the cache
// persists under. Cards without a cache key (Activity / Posture /
// Health / Events — those wire later) return false.
func cacheKeyFor(c CardID) (string, bool) {
	switch c {
	case CardUsers:
		return dashboard.CardUsers, true
	case CardGroups:
		return dashboard.CardGroups, true
	case CardApps:
		return dashboard.CardApps, true
	}
	return "", false
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
			return m, m.refreshAllCmd()
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
// Shell understands (forwarded via shared.OpenScreenMsg). Cards
// without a drill target return false.
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

// StatusBadges contributes a FOCUS chip to the chrome's status row.
func (m Model) StatusBadges() []shared.ChromeBadge {
	return []shared.ChromeBadge{
		{Key: "FOCUS", Value: cardLabel(m.focus)},
	}
}

// View renders the full dashboard grid.
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

func (m Model) renderGrid(width int, tk shared.Tokens) string {
	row1 := m.renderRow(width, tk,
		m.renderCountCard(CardUsers, "Users", []string{"ACTIVE", "PROVISIONED", "SUSPENDED", "LOCKED_OUT", "DEPROVISIONED"}),
		m.renderCountCard(CardGroups, "Groups", []string{"OKTA_GROUP", "APP_GROUP", "BUILT_IN"}),
		m.renderCountCard(CardApps, "Apps", []string{"ACTIVE", "INACTIVE"}),
	)
	row2 := m.renderRow(width, tk,
		m.renderPlaceholder(CardActivity, "Activity (last 24h · 7d)", "wires in Phase 4 — sign-ins, failures, lockouts, MFA resets"),
	)
	row3 := m.renderRow(width, tk,
		m.renderPlaceholder(CardPosture, "Posture & Risk", "wires in Phase 4 — super-admins, expiring tokens, inactive users"),
		m.renderPlaceholder(CardHealth, "Health", "wires in Phase 4 — Okta status, rate-limit headroom, last fetch age"),
	)
	row4 := m.renderRow(width, tk,
		m.renderPlaceholder(CardEvents, "Recent Critical Events", "wires in Phase 5 — last N high-severity log events"),
	)
	return row1 + "\n\n" + row2 + "\n\n" + row3 + "\n\n" + row4
}

// renderCountCard renders one Users/Groups/Apps card with the big
// number, freshness stamp, and per-status breakdown rows. statusKeys
// dictates the order of the breakdown — keeping it explicit means
// "ACTIVE" always sits above "DEPROVISIONED" regardless of map
// iteration order.
func (m Model) renderCountCard(id CardID, title string, statusKeys []string) string {
	tk := activeTokens()
	st := m.cards[id]
	focused := id == m.focus

	titleStyle := tk.Muted.Bold(true)
	if focused {
		titleStyle = tk.Accent.Bold(true)
	}
	header := titleStyle.Render(title)

	if st == nil || !st.hasCounts {
		body := tk.Muted.Render("loading…")
		if st != nil && st.err != nil {
			body = tk.Danger.Render("err: " + truncate(st.err.Error(), 40))
		}
		return header + "\n" + body
	}

	bigStyle := tk.FG.Bold(true)
	if focused {
		bigStyle = tk.Accent.Bold(true)
	}
	big := bigStyle.Render(formatThousands(st.counts.Total))

	// "5m ago" freshness stamp (Phase 3 will fold Δ-vs-7d here too).
	age := tk.Muted.Render(relativeAge(m.now(), st.counts.ObservedAt))
	if st.loading {
		age = tk.Muted.Render("refreshing…")
	}

	lines := []string{header, big + "  " + age, ""}
	// Sort breakdown: explicit-order keys first, then any extras
	// observed but not enumerated.
	seen := map[string]struct{}{}
	for _, k := range statusKeys {
		v, ok := st.counts.ByStatus[k]
		if !ok && st.counts.BySubtype != nil {
			v, ok = st.counts.BySubtype[k]
		}
		if !ok {
			continue
		}
		seen[k] = struct{}{}
		lines = append(lines, formatStatusRow(k, v, tk))
	}
	// Append unknown keys for visibility, sorted alphabetically.
	extras := make([]string, 0)
	addExtras := func(src map[string]int) {
		for k := range src {
			if _, hit := seen[k]; hit {
				continue
			}
			extras = append(extras, k)
		}
	}
	addExtras(st.counts.ByStatus)
	addExtras(st.counts.BySubtype)
	sort.Strings(extras)
	for _, k := range extras {
		v, ok := st.counts.ByStatus[k]
		if !ok {
			v = st.counts.BySubtype[k]
		}
		lines = append(lines, formatStatusRow(k, v, tk))
	}
	return strings.Join(lines, "\n")
}

func formatStatusRow(label string, count int, tk shared.Tokens) string {
	labelCell := shared.PadOrTruncateVisible(strings.ToLower(label), 16)
	val := formatThousands(count)
	pad := 8 - shared.VisibleWidth(val)
	if pad < 0 {
		pad = 0
	}
	return tk.Muted.Render(labelCell) + strings.Repeat(" ", pad) + tk.FG.Render(val)
}

// renderPlaceholder is the temporary card body for Activity /
// Posture / Health / Events until Phase 4-5 lands. Reads visually
// distinct from "loading…" so the operator knows nothing's pending
// fetch — the card just isn't wired yet.
func (m Model) renderPlaceholder(id CardID, title, note string) string {
	tk := activeTokens()
	focused := id == m.focus
	titleStyle := tk.Muted.Bold(true)
	if focused {
		titleStyle = tk.Accent.Bold(true)
	}
	return titleStyle.Render(title) + "\n" + tk.Muted.Render("· "+note)
}

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

func orgShortName(orgURL string) string {
	s := strings.TrimSpace(orgURL)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "(unknown tenant)"
	}
	return s
}

func (m Model) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

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

func activeTokens() shared.Tokens {
	return shared.PickTheme(shared.ResolveTheme(""))
}

// FocusedCard returns the currently focused card.
func (m Model) FocusedCard() CardID { return m.focus }

// CardCount returns the number of focusable cards.
func (m Model) CardCount() int { return len(cardOrder) }

// CardCountsFor exposes the cached counts for a card so tests +
// future overlays can read the live numbers without poking the
// model's unexported state.
func (m Model) CardCountsFor(id CardID) (dashboard.Counts, bool) {
	st := m.cards[id]
	if st == nil || !st.hasCounts {
		return dashboard.Counts{}, false
	}
	return st.counts, true
}

// --- formatting helpers ------------------------------------------------------

// formatThousands stamps "12,438" so big tenant numbers stay
// scannable. Uses comma regardless of locale (operator-facing TUI
// — consistency beats l10n).
func formatThousands(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return "-" + formatThousands(-n)
	}
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// relativeAge renders "5m ago" / "2h ago" / "3d ago" — keeps cards
// honest about freshness without burning a column on absolute
// timestamps.
func relativeAge(now, observed time.Time) string {
	if observed.IsZero() {
		return "—"
	}
	d := now.Sub(observed)
	switch {
	case d < 0:
		return "just now"
	case d < time.Minute:
		return strconv.Itoa(int(d.Seconds())) + "s ago"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h ago"
	default:
		return strconv.Itoa(int(d.Hours())/24) + "d ago"
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
