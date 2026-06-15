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
	"github.com/tedilabs/ota/internal/oktastatus"
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
//
// requested flips true once a fetch for this card has been
// dispatched in the current session — the home screen uses it to
// avoid re-firing fetches on every Tab cycle. The operator can
// always force a refresh with R.
//
// The sampled flag indicates the count came from a single-page
// sample rather than a full enumeration; the card renders an "≈"
// prefix in that case so the operator knows the number is a lower
// bound, not a precise total.
type cardState struct {
	loading   bool
	requested bool
	err       error
	counts    dashboard.Counts
	sampled   bool
	hasCounts bool

	// Activity-specific state. The Activity card fans out TWO
	// fetches (one short + one longer window) so we accumulate
	// them as they land.
	activity24h    ActivityMetrics
	activity7d     ActivityMetrics
	hasActivity24h bool
	hasActivity7d  bool

	// Posture-specific state.
	posture    PostureMetrics
	hasPosture bool

	// Events-specific state. cursor indexes events when the
	// Events card is focused — Enter drills into Logs scoped to
	// the highlighted event's actor or target.
	events       []CriticalEvent
	hasEvents    bool
	eventsCursor int
}

// Model is the home dashboard screen.
type Model struct {
	deps   Deps
	width  int
	height int

	focus  CardID
	cursor int // index into cardOrder

	cards map[CardID]*cardState

	// health is pushed from the App Shell via UpdateHealthMsg —
	// the chrome already tracks the underlying signals (Okta
	// status probe, rate-limit monitor, last fetch), so the home
	// screen reuses them instead of re-fetching.
	health    HealthSnapshot
	hasHealth bool

	// activityWindow controls the SECOND column on the Activity
	// card (the first is always 24h since the sparkline depends
	// on it). `t` cycles through 7d → 30d → 7d. The selected
	// window's fetcher re-fires on cycle.
	secondaryWindow time.Duration

	lastUpdated time.Time
	refreshGen  int
}

// New constructs the dashboard. Cards seed from cache (so the first
// render isn't all "loading…"); Init fans out background re-fetches
// to keep them fresh.
func New(deps Deps) Model {
	m := Model{
		deps:            deps,
		width:           deps.Width,
		height:          deps.Height,
		focus:           CardUsers,
		cards:           map[CardID]*cardState{},
		secondaryWindow: 1 * time.Hour, // start cheap; `t` widens
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

// Init kicks ONE fetch — the focused card. The rest fire lazily on
// first focus to keep Okta rate-limit budget intact (the previous
// eager-fanout-everything-on-boot approach hammered every category
// at once and consistently tripped the 60/min cap). The seeded
// cache values render under the focused card immediately so the
// dashboard still has numbers on screen during the round-trip.
//
// Auto-refresh is DISABLED by default — every other resource
// surface in ota learned the same lesson with the Logs follow flag
// (see v0.2.5 commit). Operators that want continuous polling can
// hit `R`; that's a single explicit gesture instead of a 10s
// background tick fanned out across 8+ paginated endpoints.
func (m Model) Init() tea.Cmd {
	cmd, _ := m.ensureFetch(m.focus)
	return cmd
}

// ensureFetch dispatches the fetcher for `c` exactly once per
// session. Subsequent calls return nil so Tab-cycling doesn't
// re-spam the network. The bool return reports whether a fetch
// was actually scheduled — used by Update's focus-change handler.
func (m Model) ensureFetch(c CardID) (tea.Cmd, bool) {
	st := m.cards[c]
	if st == nil {
		st = &cardState{}
		m.cards[c] = st
	}
	if st.requested {
		return nil, false
	}
	cmd := m.fetchCmdFor(c)
	if cmd == nil {
		return nil, false
	}
	st.requested = true
	st.loading = true
	return cmd, true
}

// fetchCmdFor returns the cmd that loads `c`'s data, or nil when
// the card isn't fetchable. Each card costs exactly one API call
// against its category — single-page sampling keeps Okta rate-
// limit budget intact even when the operator R-spams.
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
			counts, sampled, err := countUsers(ctx, port, m.now())
			return cardLoadedMsg{card: CardUsers, counts: counts, sampled: sampled, err: err}
		}
	case CardGroups:
		if m.deps.Groups == nil {
			return nil
		}
		port := m.deps.Groups
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			counts, sampled, err := countGroups(ctx, port, m.now())
			return cardLoadedMsg{card: CardGroups, counts: counts, sampled: sampled, err: err}
		}
	case CardApps:
		if m.deps.Apps == nil {
			return nil
		}
		port := m.deps.Apps
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			counts, sampled, err := countApps(ctx, port, m.now())
			return cardLoadedMsg{card: CardApps, counts: counts, sampled: sampled, err: err}
		}
	case CardActivity:
		// Activity now uses a SINGLE window — Phase 6's two-window
		// design fired two log fetches concurrently which doubled
		// the logs rate-limit burn for marginal extra value. `t`
		// still cycles the window choice but only one fetch fires
		// per refresh.
		if m.deps.Logs == nil {
			return nil
		}
		port := m.deps.Logs
		now := m.now()
		win := activityWindow{
			label:     windowLabel(m.activityWindowSize()),
			since:     m.activityWindowSize(),
			withSpark: true,
		}
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			am, sampled, err := countActivity(ctx, port, now, win)
			return activityLoadedMsg{window: win.label, metrics: am, sampled: sampled, err: err}
		}
	case CardPosture:
		// Posture fans out across multiple ports inside countPosture;
		// gate only on having SOMETHING wired so a partial deps
		// still renders a partial card.
		if m.deps.Administrators == nil && m.deps.APITokens == nil &&
			m.deps.GroupRules == nil && m.deps.Authenticators == nil {
			return nil
		}
		deps := m.deps
		now := m.now()
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			p := countPosture(ctx, deps, now)
			return postureLoadedMsg{metrics: p}
		}
	case CardEvents:
		if m.deps.Logs == nil {
			return nil
		}
		port := m.deps.Logs
		now := m.now()
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			events, err := fetchCriticalEvents(ctx, port, now, 6)
			return eventsLoadedMsg{events: events, err: err}
		}
	}
	return nil
}

// cardLoadedMsg is the result one count-card fetcher delivers back
// to Update. Activity / Posture use their own msg types with
// per-card payload shapes — keeps the type switch in Update
// reading as documentation.
type cardLoadedMsg struct {
	card    CardID
	counts  dashboard.Counts
	sampled bool
	err     error
}

// activityLoadedMsg is the per-window result from countActivity.
// Two of these land per Activity refresh (24h + 7d) and Update
// folds each into m.cards[CardActivity] without blocking the other
// window.
type activityLoadedMsg struct {
	window  string // "1h" / "6h" / "24h" — see windowLabel.
	metrics ActivityMetrics
	sampled bool
	err     error
}

// postureLoadedMsg delivers the Risk & Governance snapshot.
type postureLoadedMsg struct {
	metrics PostureMetrics
}

// eventsLoadedMsg delivers the latest critical-events batch.
type eventsLoadedMsg struct {
	events []CriticalEvent
	err    error
}

// refreshTickMsg is the legacy auto-refresh signal. The home
// screen no longer subscribes to a tick chain — every fetch is
// lazy + operator-triggered — but the type stays so any in-flight
// tick from a pre-fix session lands in the Update default case
// without crashing.
type refreshTickMsg struct{ gen int }

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
			st.sampled = msg.sampled
			st.hasCounts = true
			m.persist(msg.card, msg.counts)
		}
		m.lastUpdated = m.now()
		return m, nil
	case activityLoadedMsg:
		st := m.cards[CardActivity]
		if st == nil {
			st = &cardState{}
			m.cards[CardActivity] = st
		}
		st.loading = false
		st.err = msg.err
		if msg.err == nil {
			st.activity24h = msg.metrics
			st.hasActivity24h = true
			st.sampled = msg.sampled
		}
		m.lastUpdated = m.now()
		return m, nil
	case postureLoadedMsg:
		st := m.cards[CardPosture]
		if st == nil {
			st = &cardState{}
			m.cards[CardPosture] = st
		}
		st.loading = false
		st.posture = msg.metrics
		st.hasPosture = true
		m.lastUpdated = m.now()
		return m, nil
	case UpdateHealthMsg:
		m.health = msg.Snapshot
		m.hasHealth = true
		m.lastUpdated = m.now()
		return m, nil
	case eventsLoadedMsg:
		st := m.cards[CardEvents]
		if st == nil {
			st = &cardState{}
			m.cards[CardEvents] = st
		}
		st.loading = false
		st.err = msg.err
		if msg.err == nil {
			st.events = msg.events
			st.hasEvents = true
			if st.eventsCursor >= len(st.events) {
				st.eventsCursor = 0
			}
		}
		m.lastUpdated = m.now()
		return m, nil
	case refreshTickMsg:
		// Drop — auto-refresh was retired (the 10s tick was
		// hammering rate-limit budget across multiple categories).
		// `R` and shared.RefreshScreenMsg cover manual reload.
		return m, nil
	case shared.RefreshScreenMsg:
		return m, m.refreshAllCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// refreshAllCmd bumps the generation + re-fires fetchers for every
// card the operator has ALREADY visited (i.e. requested = true).
// Cards that haven't been focused yet stay untouched — refreshing
// them too would defeat the lazy-fetch rate-limit win. Hooked to
// `R` + shared.RefreshScreenMsg.
func (m *Model) refreshAllCmd() tea.Cmd {
	m.refreshGen++
	var cmds []tea.Cmd
	for _, c := range cardOrder {
		st := m.cards[c]
		if st == nil || !st.requested {
			continue
		}
		if cmd := m.fetchCmdFor(c); cmd != nil {
			cmds = append(cmds, cmd)
			st.loading = true
		}
	}
	if len(cmds) == 0 {
		// Even at the very first frame R should kick the focused
		// card so the operator sees data move.
		cmd, _ := m.ensureFetch(m.focus)
		if cmd != nil {
			return cmd
		}
		return nil
	}
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
		cmd := m.advanceFocus(1)
		return m, cmd
	case tea.KeyShiftTab:
		cmd := m.advanceFocus(-1)
		return m, cmd
	case tea.KeyEnter:
		// When the Events card is focused, Enter drills into the
		// highlighted event's actor's user-detail (via Logs scoped
		// filter). Otherwise the rest of the cards use the
		// generic drillTargetFor mapping.
		if m.focus == CardEvents {
			if cmd := m.drillEventCmd(); cmd != nil {
				return m, cmd
			}
		}
		if target, ok := drillTargetFor(m.focus); ok {
			return m, openResourceCmd(target)
		}
		return m, nil
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			// j inside the Events card moves between events; outside,
			// it moves card focus. Keeps the dashboard's keymap
			// consistent with how Users / Logs lists work.
			if m.focus == CardEvents && m.cards[CardEvents] != nil && m.cards[CardEvents].hasEvents {
				st := m.cards[CardEvents]
				if st.eventsCursor < len(st.events)-1 {
					st.eventsCursor++
				}
				return m, nil
			}
			cmd := m.advanceFocus(1)
			return m, cmd
		case "k":
			if m.focus == CardEvents && m.cards[CardEvents] != nil && m.cards[CardEvents].hasEvents {
				st := m.cards[CardEvents]
				if st.eventsCursor > 0 {
					st.eventsCursor--
				}
				return m, nil
			}
			cmd := m.advanceFocus(-1)
			return m, cmd
		case "g":
			if m.focus == CardEvents && m.cards[CardEvents] != nil {
				m.cards[CardEvents].eventsCursor = 0
				return m, nil
			}
			m.cursor = 0
			m.focus = cardOrder[0]
			cmd, _ := m.ensureFetch(m.focus)
			return m, cmd
		case "G":
			if m.focus == CardEvents && m.cards[CardEvents] != nil {
				st := m.cards[CardEvents]
				if len(st.events) > 0 {
					st.eventsCursor = len(st.events) - 1
				}
				return m, nil
			}
			m.cursor = len(cardOrder) - 1
			m.focus = cardOrder[m.cursor]
			cmd, _ := m.ensureFetch(m.focus)
			return m, cmd
		case "r", "R":
			return m, m.refreshAllCmd()
		case "t":
			// Cycle the Activity card's window: 1h → 6h → 24h → 1h.
			// Re-fire the fetcher with the new window so the next
			// render reflects the change. Wider windows are more
			// expensive on the logs rate-limit category, so the
			// operator opts in explicitly.
			switch m.secondaryWindow {
			case 1 * time.Hour:
				m.secondaryWindow = 6 * time.Hour
			case 6 * time.Hour:
				m.secondaryWindow = 24 * time.Hour
			default:
				m.secondaryWindow = 1 * time.Hour
			}
			if st := m.cards[CardActivity]; st != nil {
				st.hasActivity24h = false
				st.loading = true
			}
			return m, m.fetchCmdFor(CardActivity)
		}
	}
	return m, nil
}

// activityWindowSize returns the duration the Activity card is
// configured to summarize. Defaults to 1h on construction; `t`
// cycles through 6h / 24h.
func (m Model) activityWindowSize() time.Duration {
	if m.secondaryWindow > 0 {
		return m.secondaryWindow
	}
	return time.Hour
}

// windowLabel renders the Activity-card window label. Prefers
// hours up to and including 24h (so 24h reads as "24h", not "1d"),
// then days from 2d onward.
func windowLabel(d time.Duration) string {
	hours := int(d.Hours())
	if hours <= 24 {
		return strconv.Itoa(hours) + "h"
	}
	days := hours / 24
	return strconv.Itoa(days) + "d"
}

// drillEventCmd opens Logs filtered to the actor (or target when
// the event has no human actor) of the highlighted critical event.
// Returns nil when the Events card has no data yet so Enter falls
// through to the generic drillTargetFor("logs") path.
func (m Model) drillEventCmd() tea.Cmd {
	st := m.cards[CardEvents]
	if st == nil || !st.hasEvents || len(st.events) == 0 {
		return nil
	}
	ev := st.events[st.eventsCursor]
	filter := ""
	switch {
	case ev.ActorID != "":
		filter = `actor.id eq "` + ev.ActorID + `"`
	case ev.TargetID != "":
		filter = `target.id eq "` + ev.TargetID + `"`
	default:
		filter = `eventType eq "` + ev.EventType + `"`
	}
	return func() tea.Msg {
		return shared.OpenLogsMsg{Filter: filter}
	}
}

// advanceFocus moves the cursor through cardOrder and returns the
// (possibly-nil) lazy fetch cmd for the newly-focused card.
// Returning the cmd from Update lets the operator see the
// "loading…" state when they Tab onto a never-fetched card.
func (m *Model) advanceFocus(d int) tea.Cmd {
	n := len(cardOrder)
	m.cursor = (m.cursor + d + n) % n
	m.focus = cardOrder[m.cursor]
	cmd, _ := m.ensureFetch(m.focus)
	return cmd
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
		m.renderActivityCard(tk),
	)
	row3 := m.renderRow(width, tk,
		m.renderPostureCard(tk),
		m.renderHealthCard(tk),
	)
	row4 := m.renderRow(width, tk,
		m.renderEventsCard(tk),
	)
	return row1 + "\n\n" + row2 + "\n\n" + row3 + "\n\n" + row4
}

// renderEventsCard surfaces the last N high-severity System Log
// entries with relative timestamps + event type + actor. When the
// Events card is focused, the highlighted row carries the
// RowCursor tint + a `▸` glyph so Enter's drill target is obvious.
func (m Model) renderEventsCard(tk shared.Tokens) string {
	st := m.cards[CardEvents]
	focused := m.focus == CardEvents
	titleStyle := tk.Muted.Bold(true)
	if focused {
		titleStyle = tk.Accent.Bold(true)
	}
	header := titleStyle.Render("Recent Critical Events (last 1h)")
	if st == nil || !st.hasEvents {
		body := tk.Muted.Render("Tab to fetch · R to refresh")
		if st != nil && st.loading {
			body = tk.Muted.Render("loading…")
		}
		if st != nil && st.err != nil {
			body = tk.Danger.Render("err: " + truncate(st.err.Error(), 60))
		}
		return header + "\n" + body
	}
	if len(st.events) == 0 {
		return header + "\n" + tk.Success.Render("✓ nothing critical in the last hour")
	}
	now := m.now()
	lines := []string{header}
	for i, ev := range st.events {
		isCursor := focused && i == st.eventsCursor
		prefix := "  "
		if isCursor {
			prefix = "▸ "
		}
		when := rpad(relativeAge(now, ev.When), 8)
		eventType := rpad(ev.EventType, 36)
		actor := ev.ActorLogin
		if actor == "" {
			actor = ev.ActorID
		}
		raw := prefix + tk.Muted.Render(when) + " " + tk.Accent.Render(eventType) + " " + tk.FG.Render(actor)
		if isCursor {
			raw = tk.RowCursor.Render(shared.StripCSI(prefix + when + " " + eventType + " " + actor))
		}
		lines = append(lines, raw)
	}
	if focused {
		lines = append(lines, "", tk.Muted.Render("Enter → Logs scoped to the highlighted event's actor"))
	}
	return strings.Join(lines, "\n")
}

// renderActivityCard surfaces a single-window event summary with an
// optional sparkline. The window is `t`-toggleable (1h / 6h / 24h)
// — wider windows are more expensive on the logs rate-limit
// category, so they're opt-in. The `≈` prefix appears when the
// sample hit logsSampleSize and there are likely more events the
// fetcher didn't drain.
func (m Model) renderActivityCard(tk shared.Tokens) string {
	st := m.cards[CardActivity]
	focused := m.focus == CardActivity
	titleStyle := tk.Muted.Bold(true)
	if focused {
		titleStyle = tk.Accent.Bold(true)
	}
	winLabel := windowLabel(m.activityWindowSize())
	header := titleStyle.Render("Activity (" + winLabel + ")")
	if focused {
		header = header + tk.Muted.Render("   (t cycles 1h/6h/24h)")
	}
	if st == nil || !st.hasActivity24h {
		body := tk.Muted.Render("press Tab to fetch · or R to load now")
		if st != nil && st.loading {
			body = tk.Muted.Render("loading…")
		}
		if st != nil && st.err != nil {
			body = tk.Danger.Render("err: " + truncate(st.err.Error(), 60))
		}
		return header + "\n" + body
	}
	prefix := ""
	if st.sampled {
		prefix = "≈ "
	}
	metricRow := func(label string, n int, alarmFn func(int) bool) string {
		left := tk.Muted.Render(shared.PadOrTruncateVisible(label, 20))
		val := prefix + formatThousands(n)
		style := tk.FG
		if alarmFn != nil && alarmFn(n) {
			style = tk.Danger
		}
		return left + style.Render(val)
	}
	lines := []string{
		header,
		metricRow("Sign-ins",        st.activity24h.SignIns,       nil),
		metricRow("Failed sign-ins", st.activity24h.FailedSignIns, func(n int) bool { return n > 100 }),
		metricRow("Account lockouts", st.activity24h.AccountLocks, func(n int) bool { return n > 0 }),
		metricRow("MFA resets",       st.activity24h.MFAResets,    nil),
		metricRow("Admin actions",    st.activity24h.AdminActions, nil),
		metricRow("User creates",     st.activity24h.UserCreates,  nil),
	}
	if len(st.activity24h.HourlyBuckets) > 0 {
		spark := dashboard.NormalizeSparkline(st.activity24h.HourlyBuckets)
		lines = append(lines, "", tk.Muted.Render("hourly sign-ins ")+tk.Accent.Render(dashboard.RenderSparkline(spark)))
	}
	return strings.Join(lines, "\n")
}

// renderPostureCard surfaces risk + governance signals with a
// one-glyph status icon (✓ / ⚠ / ✗) per row so the eye snaps to
// rows that need attention.
func (m Model) renderPostureCard(tk shared.Tokens) string {
	st := m.cards[CardPosture]
	focused := m.focus == CardPosture
	titleStyle := tk.Muted.Bold(true)
	if focused {
		titleStyle = tk.Accent.Bold(true)
	}
	header := titleStyle.Render("Posture & Risk")
	if st == nil || !st.hasPosture {
		body := tk.Muted.Render("Tab to fetch · R to refresh")
		if st != nil && st.loading {
			body = tk.Muted.Render("loading…")
		}
		if st != nil && st.err != nil {
			body = tk.Danger.Render("err: " + truncate(st.err.Error(), 60))
		}
		return header + "\n" + body
	}
	p := st.posture
	row := func(icon, msg string, style any) string {
		s := tk.FG
		switch ss := style.(type) {
		case string:
			switch ss {
			case "ok":
				s = tk.Success
			case "warn":
				s = tk.Warning
			case "danger":
				s = tk.Danger
			case "muted":
				s = tk.Muted
			}
		}
		return s.Render(icon) + " " + tk.FG.Render(msg)
	}

	lines := []string{header}

	// Super admins — > 5 is widely cited as a least-privilege smell.
	switch {
	case p.SuperAdmins == 0 && p.TotalAdmins > 0:
		lines = append(lines, row("✓", "0 SUPER_ADMINs (least-privilege)", "ok"))
	case p.SuperAdmins > 5:
		lines = append(lines, row("⚠", formatThousands(p.SuperAdmins)+" SUPER_ADMINs (review)", "warn"))
	case p.TotalAdmins > 0:
		lines = append(lines, row("·", formatThousands(p.SuperAdmins)+" SUPER_ADMINs", "muted"))
	}

	// Expiring tokens.
	switch {
	case p.ExpiringTokens7d > 0:
		lines = append(lines, row("⚠", formatThousands(p.ExpiringTokens7d)+" API tokens expire <7d", "warn"))
	case p.TotalTokens > 0:
		lines = append(lines, row("✓", "no tokens expiring this week", "ok"))
	}

	// Invalid group rules.
	switch {
	case p.InvalidGroupRules > 0:
		lines = append(lines, row("✗", formatThousands(p.InvalidGroupRules)+" INVALID group rules", "danger"))
	case p.TotalGroupRules > 0:
		lines = append(lines, row("✓", "0 invalid group rules", "ok"))
	}

	// Inactive authenticators (read-only signal — operator decides).
	if p.TotalAuthenticators > 0 {
		if p.InactiveAuthenticators > 0 {
			lines = append(lines, row("·",
				formatThousands(p.InactiveAuthenticators)+" of "+formatThousands(p.TotalAuthenticators)+" authenticators INACTIVE",
				"muted"))
		}
	}

	// Partial-fetch errors.
	for _, e := range p.Errs {
		lines = append(lines, tk.Warning.Render("· "+e))
	}

	if len(lines) == 1 {
		lines = append(lines, tk.Muted.Render("· nothing to surface yet"))
	}
	return strings.Join(lines, "\n")
}

// renderHealthCard shows live-system signals the App Shell already
// tracks for its title bar — surfaced here so an operator landing
// on the home screen sees them without scanning the chrome.
func (m Model) renderHealthCard(tk shared.Tokens) string {
	focused := m.focus == CardHealth
	titleStyle := tk.Muted.Bold(true)
	if focused {
		titleStyle = tk.Accent.Bold(true)
	}
	header := titleStyle.Render("Health")
	if !m.hasHealth {
		return header + "\n" + tk.Muted.Render("warming up…")
	}
	h := m.health

	lines := []string{header}

	// Okta status.com signal.
	switch h.OktaStatus.Indicator {
	case oktastatus.IndicatorOperational:
		lines = append(lines, tk.Success.Render("✓")+" "+tk.FG.Render("Okta API operational"))
	case oktastatus.IndicatorUnknown:
		lines = append(lines, tk.Muted.Render("·")+" "+tk.Muted.Render("status: unknown"))
	case oktastatus.IndicatorMaintenance:
		lines = append(lines, tk.Muted.Render("⏱")+" "+tk.Muted.Render("Okta maintenance: "+h.OktaStatus.Indicator.Label()))
	default:
		lines = append(lines, tk.Warning.Render("⚠")+" "+tk.Warning.Render("Okta status: "+h.OktaStatus.Indicator.Label()))
	}

	// Rate-limit headroom — show the worst-case bucket.
	if worst, pct := worstRateLimit(h.RateLimits); worst.Category != "" {
		label := "RL headroom: " + formatPct(pct) + " (" + worst.Category + ")"
		switch {
		case pct < 10:
			lines = append(lines, tk.Danger.Render("✗ ")+tk.Danger.Render(label))
		case pct < 25:
			lines = append(lines, tk.Warning.Render("⚠ ")+tk.Warning.Render(label))
		default:
			lines = append(lines, tk.Success.Render("✓ ")+tk.FG.Render(label))
		}
	}

	// Last fetch age — useful when auto-refresh is off and the
	// numbers might be hours-stale.
	if !h.LastFetchAt.IsZero() {
		lines = append(lines, tk.Muted.Render("⏱ Last fetch: ")+tk.FG.Render(relativeAge(m.now(), h.LastFetchAt)))
	}

	if h.APIRecorderCount > 0 {
		lines = append(lines, tk.Muted.Render("· API timeline: ")+tk.FG.Render(formatThousands(h.APIRecorderCount))+tk.Muted.Render(" calls"))
	}
	return strings.Join(lines, "\n")
}

// rpad right-pads s to n cells (left-aligned cell content).
func rpad(s string, n int) string {
	w := shared.VisibleWidth(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// formatPct rounds the float to int + "%". 78.9 → "79%".
func formatPct(p float64) string {
	return strconv.Itoa(int(p+0.5)) + "%"
}

// renderCountCard renders one Users/Groups/Apps card with the big
// number, freshness stamp, Δ-vs-1d / Δ-vs-7d row, per-status
// breakdown rows, and a 14-day sparkline trend line. statusKeys
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
		body := tk.Muted.Render("Tab to fetch · R to refresh")
		if st != nil && st.loading {
			body = tk.Muted.Render("loading…")
		}
		if st != nil && st.err != nil {
			body = tk.Danger.Render("err: " + truncate(st.err.Error(), 40))
		}
		return header + "\n" + body
	}

	bigStyle := tk.FG.Bold(true)
	if focused {
		bigStyle = tk.Accent.Bold(true)
	}
	totalText := formatThousands(st.counts.Total)
	if st.sampled {
		// Single-page sample — Total is a lower bound. The "≈"
		// prefix signals "more than this" so the operator doesn't
		// read 200 as "exactly 200" on a 50k-user tenant.
		totalText = "≈ " + totalText + "+"
	}
	big := bigStyle.Render(totalText)

	age := tk.Muted.Render(relativeAge(m.now(), st.counts.ObservedAt))
	if st.loading {
		age = tk.Muted.Render("refreshing…")
	}

	lines := []string{header, big + "  " + age}

	// Δ row — both 1d and 7d windows so the operator sees daily
	// churn next to weekly trend. Hidden when neither window has
	// a comparable historical roll yet.
	if cacheKey, ok := cacheKeyFor(id); ok {
		deltaLine := m.renderDeltaRow(cacheKey, tk)
		if deltaLine != "" {
			lines = append(lines, deltaLine)
		}
		// Trend sparkline — 14-day series for the 7d window.
		spark := m.renderTrendSparkline(cacheKey, tk)
		if spark != "" {
			lines = append(lines, spark)
		}
	}

	lines = append(lines, "")

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

// renderDeltaRow assembles the "+47 ↑ (7d)   +3 → (1d)" line.
// Returns empty when neither window has a comparable previous
// roll — typical for the first session before the cache has
// accumulated history.
func (m Model) renderDeltaRow(cacheKey string, tk shared.Tokens) string {
	d7 := dashboard.DeltaFor(m.deps.Cache, cacheKey, m.now(), 7)
	d1 := dashboard.DeltaFor(m.deps.Cache, cacheKey, m.now(), 1)
	if !d7.Compared && !d1.Compared {
		return ""
	}
	parts := []string{}
	if d7.Compared {
		parts = append(parts, formatDeltaCell(d7, "7d", tk))
	}
	if d1.Compared {
		parts = append(parts, formatDeltaCell(d1, "1d", tk))
	}
	return strings.Join(parts, "   ")
}

// formatDeltaCell stamps "+47 ↑ (7d)" with the arrow + sign tinted
// by direction. Flat (Diff == 0) uses → in muted; positive uses ↑
// in accent; negative uses ↓ in warning to draw the eye (an
// admin's "wait, why are users going down?" trigger).
func formatDeltaCell(d dashboard.Delta, label string, tk shared.Tokens) string {
	arrow := "→"
	style := tk.Muted
	sign := ""
	switch {
	case d.Diff > 0:
		arrow = "↑"
		style = tk.Accent
		sign = "+"
	case d.Diff < 0:
		arrow = "↓"
		style = tk.Warning
		// strconv handles the leading minus.
	}
	num := sign + formatThousands(d.Diff)
	return style.Render(num+" "+arrow) + tk.Muted.Render(" ("+label+")")
}

// renderTrendSparkline produces the per-card 14-day sparkline. The
// caller pads to the card width so the trend reads as a horizontal
// band sitting flush with the Δ row above.
func (m Model) renderTrendSparkline(cacheKey string, tk shared.Tokens) string {
	d := dashboard.DeltaFor(m.deps.Cache, cacheKey, m.now(), 7)
	if len(d.Sparkline) < 2 {
		return ""
	}
	bar := dashboard.RenderSparkline(d.Sparkline)
	return tk.Muted.Render("trend ") + tk.Accent.Render(bar)
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
