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
//
// 2026-06-17 — the dashboard pivoted off Okta-enumeration counts
// (the per-resource Total Users / Groups / Apps cards consistently
// tripped the management-category rate limit on real tenants
// because Okta deliberately doesn't ship a "count only" endpoint).
// Activity / Posture now derive from System Log queries, which:
//   - live in their own rate-limit bucket (no interference with
//     :users / :groups / :apps navigation),
//   - cost ONE API call per card regardless of tenant size, and
//   - surface what operators actually look at every morning ("what
//     changed overnight?") rather than absolute totals.
type CardID int

const (
	CardActivity CardID = iota
	CardPosture
	CardHealth
	CardEvents
)

// cardOrder is the tab focus cycle. Driven by Tab / Shift-Tab.
var cardOrder = []CardID{
	CardActivity, CardPosture, CardHealth, CardEvents,
}

// Cache keys for the headline scalars the home screen rolls into
// dashboard.Cache.History so subsequent sessions render "+X ↑ vs 1d"
// deltas. Kept narrow on purpose — every new key bloats the cache
// file. Pick metrics where the day-over-day signal is meaningful.
const (
	cacheKeyActivitySignIns        = "activity-signins-24h"
	cacheKeyPostureFailedSignIns   = "posture-failed-signins-7d"
	cacheKeyPostureSensitiveWrites = "posture-sensitive-writes-7d"
)

// cardState tracks per-card lifecycle independently — fetching,
// last error, last successful observation. The View reads these to
// paint the freshness stamp + spinner / error glyph.
//
// requested flips true once a fetch for this card has been
// dispatched in the current session — the home screen uses it to
// avoid re-firing fetches on every Tab cycle. The operator can
// always force a refresh with R.
type cardState struct {
	loading   bool
	requested bool
	err       error

	// Activity card payload — single window summary derived from
	// one /api/v1/logs query; sampled flips when the response hit
	// logsSampleSize and there are likely more events.
	activity    ActivityMetrics
	hasActivity bool
	sampled     bool

	// Posture-specific state — derived from System Log too (7d
	// admin-role / token / policy / rule mutation counts).
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
		focus:           CardActivity,
		cards:           map[CardID]*cardState{},
		secondaryWindow: 1 * time.Hour, // start cheap; `t` widens
	}
	for _, c := range cardOrder {
		m.cards[c] = &cardState{}
	}
	m.seedFromCache()
	return m
}

// seedFromCache is a no-op today — the previous count-card seeding
// went away with the count cards themselves. Kept as a hook so the
// Activity card's "rolling daily Δ" feature can re-attach to the
// dashboard.Cache snapshot later without churning the constructor
// signature.
func (m *Model) seedFromCache() {}

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
// the card isn't fetchable. Activity / Posture / Events each cost
// exactly one /api/v1/logs call against the logs rate-limit bucket
// — Workforce tenants get ~120 req/min on that bucket alone so the
// dashboard runs comfortably even with R-spam. Health is pushed
// from the App Shell (zero API cost).
func (m Model) fetchCmdFor(c CardID) tea.Cmd {
	switch c {
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
		// Posture is derived from a single 7d /api/v1/logs walk —
		// the dashboard never fans out across resource ports for
		// this card. Logs is its own rate-limit category so the
		// per-resource screens stay unaffected.
		if m.deps.Logs == nil {
			return nil
		}
		port := m.deps.Logs
		now := m.now()
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			p := countPosture(ctx, port, now)
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
	case activityLoadedMsg:
		st := m.cards[CardActivity]
		if st == nil {
			st = &cardState{}
			m.cards[CardActivity] = st
		}
		st.loading = false
		st.err = msg.err
		if msg.err == nil {
			st.activity = msg.metrics
			st.hasActivity = true
			st.sampled = msg.sampled
			// Roll today's 24h sign-in headline into the cache so
			// next session paints a "+X ↑ vs 1d" delta. Only the
			// 24h window goes in — 1h / 6h windows are exploratory.
			if msg.window == "24h" && m.deps.Cache != nil {
				_ = m.deps.Cache.Put(cacheKeyActivitySignIns, dashboard.Counts{
					Total:      msg.metrics.SignIns,
					ObservedAt: m.now(),
				})
			}
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
		if m.deps.Cache != nil {
			_ = m.deps.Cache.Put(cacheKeyPostureFailedSignIns, dashboard.Counts{
				Total:      msg.metrics.FailedSignIns7d,
				ObservedAt: m.now(),
			})
			_ = m.deps.Cache.Put(cacheKeyPostureSensitiveWrites, dashboard.Counts{
				Total:      msg.metrics.SensitiveWrites7d,
				ObservedAt: m.now(),
			})
		}
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
				st.hasActivity = false
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
// Shell understands (forwarded via shared.OpenScreenMsg). Activity
// drills into Logs scoped to the active window; Events handles its
// own row-level drill in drillEventCmd. Posture / Health don't
// have a single drill target — operator uses `:users`, `:logs`,
// `:administrators` to navigate from there.
func drillTargetFor(c CardID) (string, bool) {
	switch c {
	case CardActivity, CardEvents:
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
	// 3 rows: Activity on top (the headline), Posture + Health
	// side-by-side, Events anchoring the bottom. Activity gets the
	// full chrome width because it carries the most data.
	row1 := m.renderRow(width, tk, m.renderActivityCard(tk))
	row2 := m.renderRow(width, tk, m.renderPostureCard(tk), m.renderHealthCard(tk))
	row3 := m.renderRow(width, tk, m.renderEventsCard(tk))
	return row1 + "\n\n" + row2 + "\n\n" + row3
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
	if st == nil || !st.hasActivity {
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
	sectionHeader := func(s string) string {
		return tk.Accent.Render(s)
	}
	a := st.activity
	lines := []string{header}
	if len(a.HourlyBuckets) > 0 {
		spark := dashboard.NormalizeSparkline(a.HourlyBuckets)
		lines = append(lines, tk.Muted.Render("hourly sign-ins ")+tk.Accent.Render(dashboard.RenderSparkline(spark)))
	}
	lines = append(lines,
		sectionHeader("identity"),
		metricRow("Sign-ins",         a.SignIns,        nil),
		metricRow("Failed sign-ins",  a.FailedSignIns,  func(n int) bool { return n > 100 }),
		metricRow("Account lockouts", a.AccountLocks,   func(n int) bool { return n > 0 }),
		metricRow("MFA resets",       a.MFAResets,      nil),
		sectionHeader("admin"),
		metricRow("Admin actions",    a.AdminActions,   nil),
		metricRow("Role changes",     a.RoleChanges,    nil),
		metricRow("API token writes", a.APITokenWrites, func(n int) bool { return n > 0 }),
		metricRow("Policy mutations", a.PolicyMutations, nil),
		sectionHeader("lifecycle"),
		metricRow("User creates",     a.UserCreates,    nil),
		metricRow("User deletes",     a.UserDeletes,    func(n int) bool { return n > 0 }),
		metricRow("User suspends",    a.UserSuspends,   nil),
		metricRow("App assign +",     a.AppAssignAdds,  nil),
		metricRow("App assign −",     a.AppAssignRemoves, nil),
	)
	// Delta footer only kicks in for the 24h window — the 1h/6h
	// windows are exploratory + we only roll the 24h sign-in total
	// into cache history.
	if winLabel == "24h" && m.deps.Cache != nil {
		if footer := m.renderDeltaFooter(tk, cacheKeyActivitySignIns, "sign-ins"); footer != "" {
			lines = append(lines, "", footer)
		}
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
	row := func(icon, msg, severity string) string {
		s := tk.FG
		switch severity {
		case "ok":
			s = tk.Success
		case "warn":
			s = tk.Warning
		case "danger":
			s = tk.Danger
		case "muted":
			s = tk.Muted
		}
		return s.Render(icon) + " " + tk.FG.Render(msg)
	}
	prefix := ""
	if p.Sampled {
		prefix = "≈ "
	}

	lines := []string{header}
	lines = append(lines, tk.Muted.Render("7-day window"))

	// Failed sign-in pressure — absolute count + share of total.
	switch {
	case p.FailedSignIns7d == 0 && p.SignIns7d > 0:
		lines = append(lines, row("✓", "no failed sign-ins (7d)", "ok"))
	case p.FailedSignIns7d > 0 && p.SignIns7d > 0:
		share := percentOf(p.FailedSignIns7d, p.SignIns7d)
		sev := "warn"
		if share >= 0.20 {
			sev = "danger"
		}
		lines = append(lines, row("⚠",
			prefix+formatThousands(p.FailedSignIns7d)+" failed sign-ins (7d) — "+formatPercent(share),
			sev))
	}

	// Account lockouts — anything > 0 is investigable.
	if p.AccountLocks7d > 0 {
		sev := "warn"
		if p.AccountLocks7d > 25 {
			sev = "danger"
		}
		lines = append(lines, row("⚠",
			prefix+formatThousands(p.AccountLocks7d)+" account lockouts (7d)",
			sev))
	}

	// MFA resets — destructive, but routine in small numbers.
	if p.MFAResets7d > 0 {
		sev := "muted"
		if p.MFAResets7d > 10 {
			sev = "warn"
		}
		lines = append(lines, row("·",
			prefix+formatThousands(p.MFAResets7d)+" MFA factor resets (7d)",
			sev))
	}

	// Admin sprawl + sensitive-write pressure.
	if p.SensitiveWrites7d > 0 {
		lines = append(lines, row("·",
			prefix+formatThousands(p.SensitiveWrites7d)+" sensitive admin writes (7d)",
			"muted"))
	}
	switch {
	case p.DistinctAdminActors7d > 10:
		lines = append(lines, row("⚠",
			formatThousands(p.DistinctAdminActors7d)+" distinct admin actors (review sprawl)",
			"warn"))
	case p.DistinctAdminActors7d > 0:
		lines = append(lines, row("·",
			formatThousands(p.DistinctAdminActors7d)+" distinct admin actors",
			"muted"))
	}

	// Offboarding / deletes — destructive lifecycle moves.
	if p.UserDeletes7d > 0 {
		sev := "warn"
		if p.UserDeletes7d > 25 {
			sev = "danger"
		}
		lines = append(lines, row("⚠",
			prefix+formatThousands(p.UserDeletes7d)+" user deletes (7d)",
			sev))
	}
	if p.AppRemoves7d > 0 {
		lines = append(lines, row("·",
			prefix+formatThousands(p.AppRemoves7d)+" app deactivations / deletes (7d)",
			"muted"))
	}
	if p.UserSuspends7d > 0 {
		lines = append(lines, row("·",
			prefix+formatThousands(p.UserSuspends7d)+" user suspends (7d)",
			"muted"))
	}

	if p.Err != "" {
		lines = append(lines, tk.Warning.Render("· "+p.Err))
	}
	if len(lines) == 2 {
		lines = append(lines, tk.Muted.Render("· nothing to surface yet"))
	}
	if m.deps.Cache != nil {
		if footer := m.renderDeltaFooter(tk, cacheKeyPostureFailedSignIns, "failed sign-ins"); footer != "" {
			lines = append(lines, "", footer)
		}
	}
	return strings.Join(lines, "\n")
}

// renderDeltaFooter renders a "Δ +X vs 1d / Δ −Y vs 7d" muted strip
// based on the cache history for the given key. Returns empty when
// there's nothing to compare against (fresh cache / first run) so
// the card layout doesn't blow out with a permanent "uncompared"
// placeholder.
func (m Model) renderDeltaFooter(tk shared.Tokens, key, label string) string {
	if m.deps.Cache == nil {
		return ""
	}
	now := m.now()
	d1 := dashboard.DeltaFor(m.deps.Cache, key, now, 1)
	d7 := dashboard.DeltaFor(m.deps.Cache, key, now, 7)
	if !d1.Compared && !d7.Compared {
		return ""
	}
	cells := []string{tk.Muted.Render(label + " Δ ")}
	if d1.Compared {
		cells = append(cells, formatDeltaCell(d1.Diff, "1d", tk))
	}
	if d7.Compared {
		cells = append(cells, formatDeltaCell(d7.Diff, "7d", tk))
	}
	return strings.Join(cells, "  ")
}

// formatDeltaCell stamps "+12 ↑ (1d)" / "−4 ↓ (7d)" / "  0 = (1d)"
// with a severity-tinted glyph so the eye snaps to direction.
func formatDeltaCell(diff int, window string, tk shared.Tokens) string {
	glyph := "="
	style := tk.Muted
	sign := ""
	val := diff
	switch {
	case diff > 0:
		glyph = "↑"
		style = tk.Warning
		sign = "+"
	case diff < 0:
		glyph = "↓"
		style = tk.Success
		val = -diff
		sign = "−"
	}
	return style.Render(sign+formatThousands(val)+" "+glyph) + tk.Muted.Render(" ("+window+")")
}

// percentOf returns hits / total as a 0..1 fraction. Returns 0 when
// total == 0 so callers don't need to guard divide-by-zero.
func percentOf(hits, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// formatPercent renders a 0..1 fraction as a "12%" string.
func formatPercent(frac float64) string {
	p := frac * 100
	if p < 1 {
		return "<1%"
	}
	return strconv.Itoa(int(p+0.5)) + "%"
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
