// Package logs implements the System Logs search/tail/detail Screen Models
// (SCR-050, SCR-051). Tail mode renders a `[TAIL 7s] ▶` indicator per
// REQ-R05 AC-3.
package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// Deps bundles dependencies shared by Logs screens.
type Deps struct {
	Service *service.LogsService
	// Tail is the stateful tail loop (REQ-R05 AC-2/AC-3). When present, `s`
	// toggles tail mode and PollInterval() feeds the indicator.
	Tail   *service.LogsTail
	Clock  clock.Clock
	Logger *slog.Logger
	Keys   keys.ResolvedMap
	Width  int
	Height int
	// InitialEvents lets tests seed without invoking the service.
	InitialEvents []domain.LogEvent
	// RefreshInterval drives the `f` follow-mode auto-fetch tick
	// (issue #177 v0.1.16). Default 5s when zero.
	RefreshInterval time.Duration
}

// --- List / Search (SCR-050) -------------------------------------------------

// logsLoadedMsg delivers the first page of a History fetch. `after`
// is the next-page cursor (empty when the page already covered the
// window) the operator advances via the load-older sentinel.
// (#F3 v0.2.5)
type logsLoadedMsg struct {
	events []domain.LogEvent
	after  string
}

// logsOlderLoadedMsg delivers a follow-up "load older" page. The
// model prepends these events to the buffer and updates the cursor
// to the new oldest visible row. (#F3 v0.2.5)
type logsOlderLoadedMsg struct {
	events []domain.LogEvent
	after  string
}

// LoadedForTest synthesises the (unexported) logsLoadedMsg so black-box
// tests can deliver a deterministic event slice through Update without
// running the network fetch path.
func LoadedForTest(events []domain.LogEvent) tea.Msg {
	return logsLoadedMsg{events: events}
}

// logsErrMsg surfaces a fetch failure (TUI_DESIGN §17).
type logsErrMsg struct{ err error }

// SearchModel is SCR-050.
// LogDetailTab indexes the visible tab on a log-event detail screen
// (issue #135). The Pretty tab keeps the v0.1.0 curated layout; JSON
// and YAML render the full Raw payload from Okta with shared syntax
// highlighting so operators can pull any field they need.
// LogDetailTab is an alias of shared.DetailTab (#A4 v0.2.4).
type LogDetailTab = shared.DetailTab

const (
	LogDetailTabPretty = shared.DetailTabPretty
	LogDetailTabJSON   = shared.DetailTabJSON
	LogDetailTabYAML   = shared.DetailTabYAML
)

type SearchModel struct {
	deps         Deps
	events       []domain.LogEvent
	cursor       int
	tail         bool
	follow       bool
	pollInterval time.Duration
	opened       bool
	detail       domain.LogEvent
	// detailTab tracks the active Pretty / JSON / YAML tab on the
	// log-event detail screen (issue #135). Tab cycles, `r` jumps to
	// JSON and back to the previous non-JSON tab.
	detailTab       LogDetailTab
	detailRawReturn LogDetailTab
	detailCursor    shared.BodyCursor // #F5 v0.2.5
	lastErr      error
	width        int
	height       int
	viewportTop  int
	ggChord      shared.GChord
	// timeRange is the active history window (issue #116). Default 30m;
	// 1h / 3h / 12h / 24h selectable via `1`, `3`, `c`, `e` shortcuts.
	timeRange time.Duration
	// filter / filtering carry the `/` incremental filter state
	// (issue #153). Filter narrows visible events by substring
	// match on eventType / actor / displayMsg / outcome / IP.
	filter    string
	filtering bool
	// query / queryInput drive the server-side full-text search
	// against /api/v1/logs?q=<text> (issue #185 v0.2.1). `Q`
	// opens the prompt: typing edits queryInput, Enter commits
	// to query + re-fetches the history window with the new q,
	// Esc cancels. `query` persists across follow ticks so the
	// auto-refresh stream stays scoped to the operator's search.
	// Distinct from the local `/` filter — this one runs on
	// Okta's side, mirroring the dashboard's Search input.
	query        string
	queryInput   string
	queryEditing bool
	// serverFilter / serverFilterInput drive the server-side filter
	// state — Okta's `filter=` parameter (#F4 v0.2.5). Distinct
	// from `query` (q=, free-text search) and `filter` (the local
	// substring narrow). `F` opens the prompt; the cross-resource
	// `l` shortcut writes serverFilter directly with an ID-keyed
	// expression like `target.id eq "00uABC"`.
	serverFilter        string
	serverFilterInput   string
	serverFilterEditing bool
	// followSince is the cursor used by the auto-refresh tick when
	// follow mode is on (issue #177 v0.1.16). Each tick fetches the
	// slice of events `published > followSince` and advances the
	// cursor to the highest observed published timestamp + 1ms so
	// subsequent ticks never re-emit a row that's already on screen.
	// Zero means "first tick — seed from the most recent event in
	// the loaded history window".
	followSince time.Time
	// followGen counter prevents stale tick Cmds from triggering
	// fetches after the operator toggled follow off and back on, or
	// changed the time range. Each new generation invalidates the
	// in-flight tick by mismatch.
	followGen int
	// lastUpdated stamps the most recent successful fetch — surfaced
	// via LastUpdated() so the App Shell can stamp it into the
	// chrome's upper-divider right slot (issue #177).
	lastUpdated time.Time
	// loaded flips true once the first logsLoadedMsg / logsErrMsg
	// arrives; before then View renders a spinner (issue #194 v0.2.4).
	loaded       bool
	spinnerFrame int
	fetching     bool // #U10 v0.2.4 — fetch in flight (history window or follow tick)

	// nextPageAfter is the cursor (Okta's `after` param) returned by
	// the most recent History fetch. Non-empty when the API window
	// has more events older than what's currently in m.events. Drives
	// the load-older sentinel + Enter handler. (#F3 v0.2.5)
	nextPageAfter string
	// loadingOlder is set while a load-older fetch is in flight so the
	// sentinel renders "Loading…" instead of accepting another Enter.
	loadingOlder bool
}

// Fetching implements app.FetchingStater (#U10 v0.2.4).
func (m SearchModel) Fetching() bool { return m.fetching }

// logsSpinnerTickMsg advances the loading spinner frame (issue #194
// v0.2.4).
type logsSpinnerTickMsg struct{}

// OpenForFilterMsg routes a cross-screen `l`-shortcut request from
// the App Shell into the Logs list (#F2 / #F4 v0.2.5). The model
// overwrites m.serverFilter with the supplied Okta filter
// expression and re-fetches the history window. Empty Filter just
// re-fetches without scope.
type OpenForFilterMsg struct{ Filter string }

// NewSearchModel constructs a SearchModel with defaults (tail off, follow on,
// poll interval 10s per issue #179 v0.1.17). When deps.RefreshInterval is set,
// it overrides the default. Falls back to deps.Tail's adaptive interval when
// neither is provided.
func NewSearchModel(deps Deps) SearchModel {
	interval := 10 * time.Second
	if deps.RefreshInterval > 0 {
		interval = deps.RefreshInterval
	} else if deps.Tail != nil {
		interval = deps.Tail.PollInterval()
	}
	m := SearchModel{
		deps:         deps,
		events:       deps.InitialEvents,
		follow:       true,
		pollInterval: interval,
		width:        deps.Width,
		height:       deps.Height,
		timeRange:    30 * time.Minute,
	}
	if len(m.events) > 0 || deps.Service == nil {
		m.loaded = true
	}
	return m
}

// LastUpdated implements app.LastUpdatedStater so the chrome's upper
// divider right slot can stamp "updated 12:34:56 UTC" each tick
// (issue #177 v0.1.16). Zero before the first successful fetch so
// the chrome leaves the slot empty.
func (m SearchModel) LastUpdated() time.Time { return m.lastUpdated }

// StatusBadges publishes the Logs screen's transient state to the
// chrome's v0.2.0 status row: time range, tail/follow toggles. The
// inline status lines this replaces used to consume 2 body rows
// every screen height — Logs reclaims them while making the same
// state visible alongside the other screens' badges.
func (m SearchModel) StatusBadges() []shared.ChromeBadge {
	out := []shared.ChromeBadge{
		{Key: "RANGE", Value: timeRangeLabel(m.timeRange)},
	}
	if m.tail {
		out = append(out, shared.ChromeBadge{
			Key:   "TAIL",
			Value: fmt.Sprintf("%ds", int(m.pollInterval/time.Second)),
			Tone:  shared.BadgeSuccess,
		})
	} else {
		out = append(out, shared.ChromeBadge{
			Key:   "TAIL",
			Value: "off",
			Tone:  shared.BadgeMuted,
		})
	}
	if m.follow {
		out = append(out, shared.ChromeBadge{Key: "FOLLOW", Tone: shared.BadgeSuccess})
	} else {
		out = append(out, shared.ChromeBadge{Key: "FOLLOW", Value: "off", Tone: shared.BadgeMuted})
	}
	if m.query != "" {
		out = append(out, shared.ChromeBadge{Key: "Q", Value: m.query})
	}
	if m.serverFilter != "" {
		// #F4 v0.2.5 — surface the active server-side filter
		// expression so operators see what's scoping the stream.
		out = append(out, shared.ChromeBadge{Key: "F", Value: m.serverFilter})
	}
	if m.filter != "" {
		out = append(out, shared.ChromeBadge{Key: "/", Value: m.filter})
	}
	// #F3 v0.2.5 — surface the load-older state so operators know
	// whether the API has more pages waiting (sentinel reachable via
	// `k` from row 0).
	if m.canLoadOlder() {
		out = append(out, shared.ChromeBadge{
			Key:   "MORE",
			Value: "k+Enter",
			Tone:  shared.BadgeSuccess,
		})
	}
	return out
}

// EscapeWillAct reports whether Esc will do something on the Logs
// screen — clear filter / leave detail / cancel filtering / clear
// the active server query. False when nothing is active so the App
// Shell surfaces the unified `nothing to close` toast instead of
// forwarding a silent Esc.
func (m SearchModel) EscapeWillAct() bool {
	return m.filtering || m.queryEditing || m.serverFilterEditing || m.opened ||
		m.filter != "" || m.query != "" || m.serverFilter != ""
}

// TimeRange reports the active history window — exposed for tests and
// other models that need to mirror the operator's selection.
func (m SearchModel) TimeRange() time.Duration { return m.timeRange }

// Count returns the visible/total event counts for the App Shell's
// upper divider (issue #136). With the `/` filter applied (#153)
// `visible` reflects the post-filter row count.
func (m SearchModel) Count() (visible, total int) {
	return len(m.visible()), len(m.events)
}

// Filtering / Filter implement app.FilterStater so the App Shell can
// render the floating `/` input box and stamp the active filter
// into the chrome's upper divider (issues #123 + #153).
func (m SearchModel) Filtering() bool { return m.filtering }
func (m SearchModel) Filter() string  { return m.filter }

// QueryEditing / QueryInput implement app.QueryStater so the App
// Shell renders the floating `Q` server-search input (issue #185).
func (m SearchModel) QueryEditing() bool { return m.queryEditing }
func (m SearchModel) QueryInput() string { return m.queryInput }

// ServerFilterEditing / ServerFilterInput implement
// app.ServerFilterStater so the App Shell can render the floating
// `F`-prefixed input box (#F4 v0.2.5).
func (m SearchModel) ServerFilterEditing() bool { return m.serverFilterEditing }
func (m SearchModel) ServerFilterInput() string { return m.serverFilterInput }

// visible returns the event slice filtered by the active `/` query.
// Substring match (case-insensitive) against eventType / displayMsg
// / actor display+alternateID / outcome / IP — the surface most
// operators search on.
func (m SearchModel) visible() []domain.LogEvent {
	if m.filter == "" {
		return m.events
	}
	needle := strings.ToLower(m.filter)
	out := make([]domain.LogEvent, 0, len(m.events))
	for _, e := range m.events {
		hay := strings.ToLower(strings.Join([]string{
			e.EventType,
			e.DisplayMsg,
			e.Actor.DisplayName,
			e.Actor.AlternateID,
			string(e.Outcome.Result),
			e.Outcome.Reason,
			e.Client.IPAddress,
		}, "\x00"))
		if strings.Contains(hay, needle) {
			out = append(out, e)
		}
	}
	return out
}

// Init fetches the history list and kicks off the follow-mode
// auto-refresh tick (issue #177 v0.1.16). When the model already
// carries seeded events, only the tick fires — useful for tests
// that pre-populate via InitialEvents.
func (m SearchModel) Init() tea.Cmd {
	var fetch tea.Cmd
	if len(m.events) == 0 && m.deps.Service != nil {
		fetch = fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, m.query, m.serverFilter)
	}
	tick := m.scheduleFollowTickCmd()
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

// Update handles keys: `s` toggles tail, `f` toggles follow, j/k navigates,
// Enter opens detail (REQ-R05 AC-3).
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.loaded {
			return m, shared.ScheduleSpinnerTickCmd(logsSpinnerTickMsg{})
		}
		return m, nil
	case logsLoadedMsg:
		// #F3 v0.2.5 — API returns DESCENDING (newest first); re-sort
		// ASC for terminal-tail display (oldest top, newest bottom).
		// `after` carries the next-page cursor for the load-older
		// sentinel.
		m.events = sortLogsAscending(msg.events)
		m.nextPageAfter = msg.after
		m.loadingOlder = false
		m.lastErr = nil
		m.loaded = true
		m.fetching = false
		// Logs render newest-at-bottom, so the operator's mental model
		// is: open the screen → land on the most recent entry, scroll
		// upward (k) to view older ones. Park the cursor on the last
		// row whenever a fresh batch arrives (issue #127).
		if n := len(m.events); n > 0 {
			m.cursor = n - 1
		} else {
			m.cursor = 0
		}
		m.viewportTop = 0
		// Seed follow-mode cursor from the freshest history event so
		// the next tick fetches strictly newer rows (issue #177).
		if n := len(m.events); n > 0 {
			latest := m.events[0].Published
			for _, e := range m.events[1:] {
				if e.Published.After(latest) {
					latest = e.Published
				}
			}
			m.followSince = latest.Add(time.Millisecond)
		} else {
			m.followSince = time.Now()
		}
		m.lastUpdated = time.Now()
		return m, nil
	case logsOlderLoadedMsg:
		// #F3 v0.2.5 — older page fetched on demand. Prepend (after
		// re-sorting both halves) and adjust the cursor + viewport
		// down by the number of new rows so the operator's "current
		// row" stays visually anchored.
		newer := m.events
		older := sortLogsAscending(msg.events)
		// Dedupe by ID in case the page boundary overlaps.
		seen := make(map[string]struct{}, len(newer))
		for _, e := range newer {
			seen[e.UUID] = struct{}{}
		}
		fresh := older[:0]
		for _, e := range older {
			if _, dup := seen[e.UUID]; !dup {
				fresh = append(fresh, e)
			}
		}
		shift := len(fresh)
		m.events = append(fresh, newer...)
		m.nextPageAfter = msg.after
		m.loadingOlder = false
		m.fetching = false
		// #F3 v0.2.5 — operator was on the sentinel (cursor=-1)
		// triggering this fetch. Park them on row 0 of the prepended
		// page (the new oldest data row) so they immediately see the
		// older content rather than staring at the sentinel again.
		// For any other prior cursor position, shift down so the
		// "current row" stays anchored to the same event.
		if m.cursor == sentinelRow {
			m.cursor = 0
		} else if shift > 0 {
			m.cursor += shift
			m.viewportTop += shift
		}
		m.lastUpdated = time.Now()
		return m, nil
	case logsErrMsg:
		m.lastErr = msg.err
		m.loaded = true
		m.fetching = false
		m.loadingOlder = false
		return m, nil
	case logsSpinnerTickMsg:
		if !shared.BumpSpinner(m.loaded, &m.spinnerFrame) {
			return m, nil
		}
		return m, shared.ScheduleSpinnerTickCmd(logsSpinnerTickMsg{})
	case shared.RefreshScreenMsg:
		// Issue #192 v0.2.3 — out-of-band refresh from the App Shell.
		if m.deps.Service == nil {
			return m, nil
		}
		m.fetching = true
		return m, fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, m.query, m.serverFilter)
	case OpenForFilterMsg:
		// #F2 / #F4 v0.2.5 — `l` shortcut from another resource.
		// Overwrite the server-side filter, clear any prior q so the
		// resource scope is what's visible, reset cursor, re-fetch.
		m.serverFilter = strings.TrimSpace(msg.Filter)
		m.serverFilterInput = ""
		m.serverFilterEditing = false
		m.query = ""
		m.queryInput = ""
		m.queryEditing = false
		m.cursor = 0
		m.viewportTop = 0
		m.followGen++
		if m.deps.Service == nil {
			return m, nil
		}
		m.fetching = true
		return m, tea.Batch(
			fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, m.query, m.serverFilter),
			m.scheduleFollowTickCmd(),
		)
	case followTickMsg:
		// Stale tick — operator toggled follow off and back on, or
		// changed the time range. Drop the tick and let the new
		// generation's tick chain take over.
		if msg.gen != m.followGen || !m.follow {
			return m, nil
		}
		m.fetching = true
		return m, m.followFetchCmd()
	case followFetchedMsg:
		m.fetching = false
		if msg.gen != m.followGen || !m.follow {
			// Stale result — schedule the next tick if follow is
			// still on so the loop resumes naturally on the
			// current generation.
			return m, m.scheduleFollowTickCmd()
		}
		// Issue #179 v0.1.17 — dedupe by UUID before appending. The
		// since-cursor advance is best-effort: Okta's `since` is
		// inclusive AND the upstream fetch can race with the
		// history reload (range key, `r` refresh, initial Init
		// batch). Without the dedup, a tick fired while history
		// was still resolving could re-emit rows that history
		// just delivered. UUID is the canonical event identifier
		// — it survives across replicas, sort orders, and any
		// truncation of the published timestamp's precision.
		if len(msg.events) > 0 {
			seen := make(map[string]struct{}, len(m.events))
			for _, e := range m.events {
				if e.UUID != "" {
					seen[e.UUID] = struct{}{}
				}
			}
			fresh := msg.events[:0]
			for _, e := range msg.events {
				if e.UUID != "" {
					if _, ok := seen[e.UUID]; ok {
						continue
					}
					seen[e.UUID] = struct{}{}
				}
				fresh = append(fresh, e)
			}
			if len(fresh) > 0 {
				m.events = append(m.events, fresh...)
				if !m.opened && !m.filtering {
					m.cursor = len(m.events) - 1
				}
			}
		}
		m.followSince = msg.nextSince
		m.lastUpdated = msg.at
		return m, m.scheduleFollowTickCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m SearchModel) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Arrow keys map to Vim-style runes (issue #159) — but NOT
	// while the filter prompt is in input mode, where arrow keys
	// should still pass through (the `j` rune is meaningful as
	// content there).
	if !m.filtering {
		km = shared.NormalizeArrowKey(km)
	}
	// Filter-input mode (issue #153). `/` opens the prompt; the
	// chrome renders the floating box. Enter applies, Esc cancels,
	// Backspace backs out a char, runes append.
	if m.filtering {
		switch km.Type {
		case tea.KeyEnter:
			m.filtering = false
			if n := len(m.visible()); n > 0 {
				m.cursor = n - 1
			} else {
				m.cursor = 0
			}
			m.viewportTop = 0
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
			m.filter += string(km.Runes)
			return m, nil
		}
		return m, nil
	}
	// #F4 v0.2.5 — server-side filter input mode. Operator typed
	// `F`; subsequent keys edit serverFilterInput. Enter commits +
	// fires the history fetch with the new filter; Esc cancels.
	if m.serverFilterEditing {
		switch km.Type {
		case tea.KeyEnter:
			m.serverFilterEditing = false
			m.serverFilter = strings.TrimSpace(m.serverFilterInput)
			m.serverFilterInput = ""
			m.cursor = 0
			m.viewportTop = 0
			m.followGen++
			if m.deps.Service != nil {
				return m, tea.Batch(
					fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, m.query, m.serverFilter),
					m.scheduleFollowTickCmd(),
				)
			}
			return m, nil
		case tea.KeyEsc:
			m.serverFilterEditing = false
			m.serverFilterInput = ""
			return m, nil
		case tea.KeyBackspace:
			if n := len(m.serverFilterInput); n > 0 {
				m.serverFilterInput = m.serverFilterInput[:n-1]
			}
			return m, nil
		case tea.KeyRunes:
			m.serverFilterInput += string(km.Runes)
			return m, nil
		}
		return m, nil
	}

	// v0.2.1 #185 — server-side query input mode. Operator typed
	// `Q`; subsequent keys edit queryInput. Enter commits + fires
	// fetchHistoryWindowQueryCmd; Esc cancels and reverts.
	if m.queryEditing {
		switch km.Type {
		case tea.KeyEnter:
			m.queryEditing = false
			m.query = strings.TrimSpace(m.queryInput)
			m.queryInput = ""
			m.cursor = 0
			m.viewportTop = 0
			// v0.2.2 #186: bump followGen so any in-flight tick
			// (captured the previous m.query value at scheduling
			// time) is dropped when its result lands. The fresh
			// history fetch + the next tick will both use the new
			// query value.
			m.followGen++
			if m.deps.Service != nil {
				return m, tea.Batch(
					fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, m.query, m.serverFilter),
					m.scheduleFollowTickCmd(),
				)
			}
			return m, nil
		case tea.KeyEsc:
			m.queryEditing = false
			m.queryInput = ""
			return m, nil
		case tea.KeyBackspace:
			if n := len(m.queryInput); n > 0 {
				m.queryInput = m.queryInput[:n-1]
			}
			return m, nil
		case tea.KeyRunes:
			m.queryInput += string(km.Runes)
			return m, nil
		}
		return m, nil
	}

	// Esc on the list with a stuck filter clears it (issue #131
	// pattern, ported to logs).
	if !m.opened && km.Type == tea.KeyEsc && m.filter != "" {
		m.filter = ""
		m.cursor = 0
		m.viewportTop = 0
		return m, nil
	}
	// Esc on the list with an active server query clears it and
	// re-fetches the unfiltered window (issue #185 v0.2.1). Mirrors
	// the local-filter Esc semantics so the operator's mental model
	// stays "Esc undoes the most recent narrowing". Bumps followGen
	// (v0.2.2 #186) so a stale tick captured with the previous query
	// is dropped when its result lands.
	if !m.opened && km.Type == tea.KeyEsc && m.query != "" {
		m.query = ""
		m.cursor = 0
		m.viewportTop = 0
		m.followGen++
		if m.deps.Service != nil {
			return m, tea.Batch(
				fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, "", m.serverFilter),
				m.scheduleFollowTickCmd(),
			)
		}
		return m, nil
	}
	// #F4 v0.2.5 — Esc on the list with an active server filter clears
	// it and re-fetches without scope, mirroring the Q clear path.
	if !m.opened && km.Type == tea.KeyEsc && m.serverFilter != "" {
		m.serverFilter = ""
		m.cursor = 0
		m.viewportTop = 0
		m.followGen++
		if m.deps.Service != nil {
			return m, tea.Batch(
				fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, m.query, ""),
				m.scheduleFollowTickCmd(),
			)
		}
		return m, nil
	}

	// Detail-mode keys (issue #135). Esc backs out, Tab / Shift-Tab
	// cycle through Pretty / JSON / YAML, `r` jumps to / from JSON
	// against the previously-visited non-JSON tab.
	if m.opened {
		viewport := m.detailViewportRows()
		total := len(logDetailLines(m.detail, m.detailTab))
		switch km.Type {
		case tea.KeyEsc:
			// #F5 v0.2.5 — Esc cancels visual mode first.
			if m.detailCursor.Visual {
				m.detailCursor.CancelVisual()
				return m, nil
			}
			m.opened = false
			m.detail = domain.LogEvent{}
			m.detailTab = LogDetailTabPretty
			m.detailRawReturn = LogDetailTabPretty
			m.detailCursor = shared.BodyCursor{}
			return m, nil
		case tea.KeyTab:
			m.detailTab = shared.NextTab(m.detailTab)
			m.detailCursor = shared.BodyCursor{}
			return m, nil
		case tea.KeyShiftTab:
			m.detailTab = shared.PrevTab(m.detailTab)
			m.detailCursor = shared.BodyCursor{}
			return m, nil
		case tea.KeyCtrlF:
			m.detailCursor.PageDown(viewport, total)
			return m, nil
		case tea.KeyCtrlB:
			m.detailCursor.PageUp(viewport)
			return m, nil
		case tea.KeyCtrlD:
			m.detailCursor.HalfPageDown(viewport, total)
			return m, nil
		case tea.KeyCtrlU:
			m.detailCursor.HalfPageUp(viewport)
			return m, nil
		case tea.KeyEnter:
			// #G5 / U7 v0.2.4 — drill into the actor's User Detail.
			if id := logActorDrillID(m.detail); id != "" {
				return m, openUserDetailCmd(id)
			}
			return m, nil
		case tea.KeyRunes:
			switch string(km.Runes) {
			case "r":
				m.detailTab, m.detailRawReturn = shared.ToggleRawTab(m.detailTab, m.detailRawReturn)
				m.detailCursor = shared.BodyCursor{}
				return m, nil
			case "j":
				m.detailCursor.Down(total)
				return m, nil
			case "k":
				m.detailCursor.Up()
				return m, nil
			case "g":
				m.detailCursor.GoTop()
				return m, nil
			case "G":
				m.detailCursor.GoBottom(total)
				return m, nil
			case "v", "V":
				if m.detailCursor.Visual {
					m.detailCursor.CancelVisual()
				} else {
					m.detailCursor.StartVisual()
				}
				return m, nil
			case "y":
				return m, shared.YankCmd(m.detailCursor, logDetailLines(m.detail, m.detailTab), "Log Detail")
			}
		}
		return m, nil
	}
	rows := m.visible()
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	case tea.KeyCtrlF:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor = clampLogIdx(m.cursor+page, len(rows))
	case tea.KeyCtrlB:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor = clampLogIdx(m.cursor-page, len(rows))
	case tea.KeyCtrlD:
		page := shared.ListBodyRowBudget(m.height) / 2
		if page < 1 {
			page = 5
		}
		m.cursor = clampLogIdx(m.cursor+page, len(rows))
	case tea.KeyCtrlU:
		page := shared.ListBodyRowBudget(m.height) / 2
		if page < 1 {
			page = 5
		}
		m.cursor = clampLogIdx(m.cursor-page, len(rows))
	case tea.KeyEnter:
		// #F3 v0.2.5 — Enter on the load-older sentinel fetches the
		// next older page; Enter on a real row opens detail.
		if m.cursor == sentinelRow && m.canLoadOlder() && !m.loadingOlder {
			m.loadingOlder = true
			m.fetching = true
			return m, fetchOlderPageCmd(m.deps.Service, m.timeRange, m.query, m.serverFilter, m.nextPageAfter)
		}
		if m.cursor >= 0 && m.cursor < len(rows) {
			m.detail = rows[m.cursor]
			m.opened = true
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "/":
			// Open the `/` filter prompt (issue #153). The chrome
			// renders the floating input box once Filtering() flips.
			m.ggChord.Reset()
			m.filtering = true
			m.filter = ""
		case "g":
			if m.ggChord.Press(m.now()) {
				m.cursor = 0
				m.viewportTop = 0
			}
		case "G":
			m.ggChord.Reset()
			if n := len(rows); n > 0 {
				m.cursor = n - 1
			}
		case "s":
			m.ggChord.Reset()
			m.tail = !m.tail
		case "f":
			m.ggChord.Reset()
			m.follow = !m.follow
			// Issue #177 v0.1.16: bump generation so any in-flight
			// tick from the previous follow session is invalidated;
			// kick off a fresh tick chain when toggling on.
			m.followGen++
			if m.follow {
				return m, m.scheduleFollowTickCmd()
			}
			return m, nil
		case "r":
			// Manual refresh — operator wants the current window
			// re-fetched (e.g., after firing a write op elsewhere).
			m.ggChord.Reset()
			if m.deps.Service != nil {
				return m, fetchHistoryWindowQueryCmd(m.deps.Service, m.timeRange, m.query, m.serverFilter)
			}
		case "Q":
			// v0.2.1 #185 — server-side full-text search prompt.
			// Mirrors the Okta web dashboard's Search input: type a
			// term, Enter re-fetches the history window with q=<term>
			// against /api/v1/logs. Distinct from the local `/`
			// filter which narrows already-loaded events client-side.
			m.ggChord.Reset()
			m.queryEditing = true
			m.queryInput = m.query
		case "F":
			// #F4 v0.2.5 — server-side filter expression prompt.
			// Pre-populates with the current serverFilter so operators
			// can extend / edit a filter set by `l` from another
			// resource (e.g., add `and eventType eq "user.session.start"`).
			m.ggChord.Reset()
			m.serverFilterEditing = true
			m.serverFilterInput = m.serverFilter
		case "j":
			m.ggChord.Reset()
			// #F3 v0.2.5 — `j` from the sentinel row falls back to row 0.
			if m.cursor == sentinelRow {
				m.cursor = 0
			} else if m.cursor < len(rows)-1 {
				m.cursor++
			}
		case "k":
			m.ggChord.Reset()
			// #F3 v0.2.5 — `k` from row 0 lands on the sentinel when
			// older logs are available; from sentinel it stays put.
			if m.cursor > 0 {
				m.cursor--
			} else if m.cursor == 0 && m.canLoadOlder() {
				m.cursor = sentinelRow
			} else if m.cursor == 0 {
				// At the top with no older page to load. Tell the
				// operator explicitly so they don't wonder whether
				// the keypath is broken (#F3 v0.2.5 follow-up).
				return m, emitNoMoreLogsToast(m.timeRange)
			}
		case "d":
			m.ggChord.Reset()
			if m.cursor >= 0 && m.cursor < len(rows) {
				m.detail = rows[m.cursor]
				m.opened = true
			}
		// History window shortcuts (issue #116). Each refetches with the
		// new bound. Operator presses again any time to refresh.
		case "0":
			m.ggChord.Reset()
			return m.setRange(30 * time.Minute)
		case "1":
			m.ggChord.Reset()
			return m.setRange(1 * time.Hour)
		case "3":
			m.ggChord.Reset()
			return m.setRange(3 * time.Hour)
		case "c":
			m.ggChord.Reset()
			return m.setRange(12 * time.Hour)
		case "e":
			m.ggChord.Reset()
			return m.setRange(24 * time.Hour)
		}
	}
	return m, nil
}

// setRange swaps the active history window and triggers a re-fetch.
// Resets cursor / viewport so the new (potentially much smaller) result
// set renders from the top, then jumps to the bottom in fetchHistoryCmd
// receipt path so the newest entry is on screen.
func (m SearchModel) setRange(window time.Duration) (tea.Model, tea.Cmd) {
	m.timeRange = window
	m.cursor = 0
	m.viewportTop = 0
	if m.deps.Service == nil {
		// No backing service (test harness with InitialEvents only) —
		// leave the seeded events alone.
		return m, nil
	}
	return m, fetchHistoryWindowQueryCmd(m.deps.Service, window, m.query, m.serverFilter)
}

// View renders SCR-050 (TUI_DESIGN §15.6 / §16.8). Columns:
// WHEN / SEV / EVENTTYPE / ACTOR / OUTCOME / IP. Tail indicator is contributed
// to the first line so operators see the tail state at a glance (REQ-R05
// AC-3).
// detailViewportRows returns the row budget the body cursor scrolls
// inside on the Log Event Detail surface (#F5 v0.2.5). 4 rows
// reserved for the title + tab bar + divider + footer hint when the
// actor drill-down is shown.
func (m SearchModel) detailViewportRows() int {
	header := 3
	if logActorDrillID(m.detail) != "" {
		header++
	}
	return shared.DetailBodyRowBudget(m.height, header)
}

func (m SearchModel) View() string {
	if m.opened {
		return renderLogDetailTabbedWithCursor(m.detail, m.detailTab, m.detailCursor,
			m.chromeContentWidth()-2, m.detailViewportRows())
	}
	if m.lastErr != nil {
		return "Logs  (error)\n" + shared.ErrorPanel("events", m.lastErr)
	}

	tk := activeTokens()
	if !m.loaded {
		return shared.LoadingPlaceholder(m.spinnerFrame, "Loading…",
			m.chromeContentWidth(), shared.ListBodyRowBudget(m.height), tk)
	}
	now := m.now()

	var b strings.Builder
	// v0.2.0: tail / follow / range / filter all moved to the chrome
	// status row via StatusBadges(); the body opens straight with the
	// column header so we reclaim 2 rows of event data on every screen
	// height. Hint line ("0=30m 1=1h …") moves into the help overlay.
	// 2-cell cursor gutter on the header keeps it aligned with data rows.
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatLogsColumns(
		"PUBLISHED", "SEV", "MESSAGE", "ACTOR TYPE", "ACTOR", "OUTCOME", "IP", "WHEN",
	)))
	b.WriteByte('\n')
	rows := m.visible()
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(rows), shared.ListBodyRowBudget(m.height))
	budget := end - top
	rowTarget := m.chromeContentWidth() - 2
	// #F3 v0.2.5 — when the next-page cursor is set, render a
	// navigable sentinel ABOVE row 0. cursor==sentinelRow places
	// the `▸` marker on the sentinel itself; Enter on the sentinel
	// fires fetchOlderPageCmd. Sentinel only renders when the
	// viewport top is already row 0 (operator has scrolled up to
	// meet the oldest data).
	showSentinel := m.canLoadOlder() && top == 0
	if showSentinel {
		hint := "↑  Press Enter to load older logs"
		if m.loadingOlder {
			hint = "⠋  Loading older logs…"
		}
		prefix := "  "
		line := tk.Muted.Render(hint)
		if m.cursor == sentinelRow {
			prefix = "▸ "
			line = tk.Accent.Render(hint)
		}
		// Pad to rowTarget so the cursor highlight extends across
		// the full body width like a normal row.
		full := shared.PadOrTruncateVisible(prefix+line, rowTarget)
		if m.cursor == sentinelRow {
			full = tk.RowCursor.Render(shared.StripCSI(full))
		}
		b.WriteString(full)
		b.WriteByte('\n')
	}
	for i := top; i < end; i++ {
		row := m.renderLogsRow(rows[i], now, tk)
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		// v0.2.0 #182 — unified cursor pipeline.
		b.WriteString(shared.RenderRowCursor(prefix+row, rowTarget, i == m.cursor, "", false, tk))
		b.WriteString(shared.AppendScrollbarSuffix(i-top, top, budget, len(rows), tk))
		b.WriteByte('\n')
	}
	return b.String()
}

// canLoadOlder reports whether the load-older sentinel + Enter
// handler are active — gated by the next-page cursor and the
// service being wired (#F3 v0.2.5).
func (m SearchModel) canLoadOlder() bool {
	return m.nextPageAfter != "" && m.deps.Service != nil
}

// sentinelRow is the cursor index assigned to the load-older
// sentinel — sits one slot above the real data rows so `k` from
// row 0 lands on it (#F3 v0.2.5).
const sentinelRow = -1

// emitNoMoreLogsToast tells the operator that `k` from row 0 ran
// out of older history within the active window — most often
// because the API returned every event in one page. Hint suggests
// extending the range so they don't think the keypath is broken.
func emitNoMoreLogsToast(window time.Duration) tea.Cmd {
	return func() tea.Msg {
		return shared.ToastMsg{
			Text:  fmt.Sprintf("no older logs in the last %s — try a longer range (1 / 3 / c / e)", timeRangeLabel(window)),
			Level: shared.ToastInfo,
			Until: time.Now().Add(4 * time.Second),
		}
	}
}

// chromeContentWidth returns the body cells the chrome reserves per
// row, used to land the scrollbar gutter flush against the right
// border (issue #173).
func (m SearchModel) chromeContentWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	return w - 3
}

// renderLogsRow formats one event row in the issue #158 column
// order: PUBLISHED > SEV > MESSAGE > ACTOR TYPE > ACTOR > OUTCOME
// > IP > WHEN. MESSAGE prefers DisplayMsg (the human-friendly
// summary Okta provides) and falls back to EventType when empty.
// OUTCOME concatenates result + reason so "FAILURE - bad password"
// reads inline.
func (m SearchModel) renderLogsRow(e domain.LogEvent, now time.Time, tk shared.Tokens) string {
	published := "—"
	if !e.Published.IsZero() {
		published = e.Published.UTC().Format("2006-01-02 15:04:05")
	}
	sev := shared.SeverityBadge(string(e.Severity), tk).Render(tk)
	message := e.DisplayMsg
	if message == "" {
		message = e.EventType
	}
	actorType := string(e.Actor.Type)
	if actorType == "" {
		actorType = "—"
	}
	actor := e.Actor.DisplayName
	if actor == "" {
		actor = e.Actor.AlternateID
	}
	if actor == "" {
		actor = "—"
	}
	outcome := string(e.Outcome.Result)
	if outcome == "" {
		outcome = "—"
	}
	if e.Outcome.Reason != "" {
		outcome = outcome + " — " + e.Outcome.Reason
	}
	ip := e.Client.IPAddress
	if ip == "" {
		ip = "—"
	}
	when := shared.RelativeTime(&e.Published, now)
	if e.Published.IsZero() {
		when = "—"
	}
	return m.formatLogsColumns(published, sev, message, actorType, actor, outcome, ip, when)
}

// formatLogsColumns lays out the 8-column row in the issue #158
// order using the shared column-spec system (#157), so widths
// auto-fit observed data the same way Users does.
func (m SearchModel) formatLogsColumns(cells ...string) string {
	specs := logsColumnSpecs()
	specs = shared.ShrinkSpecsToFit(specs, m.observedColumnWidths())
	widths := m.logsWidths(specs)
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

// logsColumnSpecs — issue #158 order. Drop priorities degrade from
// the right so PUBLISHED + MESSAGE stay visible longest.
func logsColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "PUBLISHED", Kind: shared.ColumnFixed, Min: 19, DropPriority: 0},
		{Title: "SEV", Kind: shared.ColumnFixed, Min: 5, DropPriority: 0, AlignCenter: true},
		{Title: "MESSAGE", Kind: shared.ColumnFlex, Min: 24, Weight: 3, DropPriority: 0},
		{Title: "ACTOR TYPE", Kind: shared.ColumnFixed, Min: 12, DropPriority: 4},
		{Title: "ACTOR", Kind: shared.ColumnFlex, Min: 16, Weight: 1, DropPriority: 5},
		{Title: "OUTCOME", Kind: shared.ColumnFlex, Min: 12, Weight: 1, DropPriority: 3},
		{Title: "IP", Kind: shared.ColumnFixed, Min: 13, DropPriority: 2},
		{Title: "WHEN", Kind: shared.ColumnFixed, Min: 8, DropPriority: 1, AlignRight: true},
	}
}

// logsWidths picks the layout — tight first, hScroll fallback.
func (m SearchModel) logsWidths(specs []shared.ColumnSpec) []int {
	inner := m.logsInnerWidth()
	if w := shared.LayoutColumnsTight(specs, inner, 2); w != nil {
		return w
	}
	return shared.LayoutColumnsHScroll(specs, inner, 2, 0)
}

func (m SearchModel) logsInnerWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	// chrome border (2) + left padding (1) + cursor gutter (2) +
	// scrollbar gutter (2: " ▌"/" │", v0.1.15 issue #173).
	inner := w - 2 - 1 - 2 - 2
	if inner < 20 {
		inner = 20
	}
	return inner
}

// observedColumnWidths returns the widest cell width per column
// across the visible event slice. Powers ShrinkSpecsToFit so logs
// auto-fit observed data the same way Users does.
func (m SearchModel) observedColumnWidths() []int {
	rows := m.visible()
	if len(rows) == 0 {
		return nil
	}
	now := m.now()
	tk := activeTokens()
	out := make([]int, 8)
	for _, e := range rows {
		published := "—"
		if !e.Published.IsZero() {
			published = e.Published.UTC().Format("2006-01-02 15:04:05")
		}
		sev := shared.SeverityBadge(string(e.Severity), tk).Render(tk)
		message := e.DisplayMsg
		if message == "" {
			message = e.EventType
		}
		actorType := string(e.Actor.Type)
		if actorType == "" {
			actorType = "—"
		}
		actor := e.Actor.DisplayName
		if actor == "" {
			actor = e.Actor.AlternateID
		}
		if actor == "" {
			actor = "—"
		}
		outcome := string(e.Outcome.Result)
		if outcome == "" {
			outcome = "—"
		}
		if e.Outcome.Reason != "" {
			outcome = outcome + " — " + e.Outcome.Reason
		}
		ip := e.Client.IPAddress
		if ip == "" {
			ip = "—"
		}
		when := shared.RelativeTime(&e.Published, now)
		if e.Published.IsZero() {
			when = "—"
		}
		cells := []string{published, sev, message, actorType, actor, outcome, ip, when}
		for i, c := range cells {
			if w := shared.VisibleWidth(c); w > out[i] {
				out[i] = w
			}
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// padRightLog left-aligns s to width using spaces (or truncates with "…").
// Uses a CSI-aware visible-length check so styled cells (badges) align.
func padRightLog(s string, width int) string {
	w := visibleLenLog(s)
	if w >= width {
		return shared.Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func visibleLenLog(s string) int {
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

// activeTokens picks the right token set per NO_COLOR.
func activeTokens() shared.Tokens {
	if shared.MonochromeEnabled() {
		return shared.Monochrome()
	}
	return shared.Dark()
}

// now returns the injected clock or wall time.
func (m SearchModel) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

// clampLogIdx pins idx to [0, total-1]. Empty list returns 0.
func clampLogIdx(idx, total int) int {
	if total == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= total {
		return total - 1
	}
	return idx
}

// timeRangeLabel renders the active history window for the header
// (issue #116). 30m / 1h / 3h / 12h / 24h.
func timeRangeLabel(d time.Duration) string {
	switch d {
	case 30 * time.Minute:
		return "Last 30m"
	case 1 * time.Hour:
		return "Last 1h"
	case 3 * time.Hour:
		return "Last 3h"
	case 12 * time.Hour:
		return "Last 12h"
	case 24 * time.Hour:
		return "Last 24h"
	}
	if d == 0 {
		return "All"
	}
	return "Last " + d.String()
}

// renderTailState / renderFollowState / tailIndicator removed in
// v0.2.0 (#182): tail / follow state surfaces via the chrome's
// transient status row through StatusBadges() rather than inline
// body lines. timeRangeLabel survives because StatusBadges still
// formats the active range there.

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04:05Z")
}

// --- Detail (SCR-051) --------------------------------------------------------

// DetailModel is SCR-051.
type DetailModel struct {
	deps  Deps
	event domain.LogEvent
}

// NewDetailModel constructs a DetailModel.
func NewDetailModel(deps Deps, e domain.LogEvent) DetailModel {
	return DetailModel{deps: deps, event: e}
}

// Init implements tea.Model.
func (m DetailModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m DetailModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

// View renders the structured sections (Actor/Target/Client/Outcome) plus
// the full event UUID. Renders the Pretty tab — the standalone
// DetailModel is used by tests that don't carry a tab cursor.
func (m DetailModel) View() string { return renderLogDetailTabbed(m.event, LogDetailTabPretty) }

// renderLogDetailTabbed dispatches to the active tab body. Pretty is
// the curated v0.1.0 layout; JSON dumps the full Raw payload from
// Okta with shared syntax highlighting; YAML reformats the same
// payload at 2-space indent.
func renderLogDetailTabbed(e domain.LogEvent, active LogDetailTab) string {
	return renderLogDetailTabbedWithCursor(e, active, shared.BodyCursor{}, 0, 0)
}

// logDetailLines returns the active tab's body as a flat line slice
// — the unit BodyCursor navigates and yanks against (#F5 v0.2.5).
func logDetailLines(e domain.LogEvent, active LogDetailTab) []string {
	var raw string
	switch active {
	case LogDetailTabJSON:
		raw = renderLogJSONTab(e)
	case LogDetailTabYAML:
		raw = renderLogYAMLTab(e)
	default:
		raw = renderLogDetail(e)
	}
	return strings.Split(strings.TrimRight(raw, "\n"), "\n")
}

// renderLogDetailTabbedWithCursor is the cursor-aware variant used by
// the live View. cursor.Line marks the focused row with `▸ ` + the
// RowCursor tint; visual range shares the same RowCursor tint without
// the marker (#F5 v0.2.5). height clips the body slice so the chrome
// doesn't truncate cursor rows that scrolled off-screen.
func renderLogDetailTabbedWithCursor(e domain.LogEvent, active LogDetailTab, cursor shared.BodyCursor, width, height int) string {
	tk := activeTokens()
	var b strings.Builder
	b.WriteString("Log Event\n")
	barWidth := width
	if barWidth <= 0 {
		barWidth = 78
	}
	b.WriteString(shared.RenderDetailTabBar(active, barWidth, tk))
	b.WriteByte('\n')
	if width <= 0 {
		// Fall back to plain rendering when called without a width
		// (e.g., legacy callers / DetailModel.View()).
		switch active {
		case LogDetailTabJSON:
			b.WriteString(renderLogJSONTab(e))
		case LogDetailTabYAML:
			b.WriteString(renderLogYAMLTab(e))
		default:
			b.WriteString(renderLogDetail(e))
		}
	} else {
		lines := logDetailLines(e, active)
		rendered := cursor.RenderViewport(lines, width, height, tk)
		b.WriteString(shared.JoinLines(rendered))
	}
	// #G5 / U7 v0.2.4 — surface the actor drill-down affordance.
	if logActorDrillID(e) != "" {
		b.WriteByte('\n')
		b.WriteString("Enter to open the actor's user detail")
	}
	return b.String()
}

// renderLogJSONTab emits the full event payload (LogEvent.Raw — what
// Okta returned over the wire) with shared.HighlightJSON applied so
// keys / strings / numbers / booleans get their colour tokens.
// Falls back to a curated projection when Raw is empty so unit tests
// without a wire fixture still get a useful body.
func renderLogJSONTab(e domain.LogEvent) string {
	body := prettyJSONForLog(e)
	return shared.HighlightJSON(body, activeTokens()) + "\n"
}

// renderLogYAMLTab decodes the same payload as the JSON tab and
// re-emits it as YAML at 2-space indent (issue #109 carried over).
func renderLogYAMLTab(e domain.LogEvent) string {
	raw := prettyJSONForLog(e)
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	if err := enc.Close(); err != nil {
		return "(yaml render error: " + err.Error() + ")\n"
	}
	return shared.HighlightYAML(strings.TrimRight(buf.String(), "\n"), activeTokens()) + "\n"
}

// prettyJSONForLog returns a 2-space-indent JSON string for the
// supplied event. Prefers LogEvent.Raw (Okta's wire payload) so
// operators see the canonical fields; falls back to a curated
// projection when Raw is empty (e.g. seeded test fixtures).
func prettyJSONForLog(e domain.LogEvent) string {
	if len(e.Raw) > 0 {
		var v any
		if err := json.Unmarshal(e.Raw, &v); err == nil {
			if buf, err := json.MarshalIndent(v, "", "  "); err == nil {
				return string(buf)
			}
		}
	}
	curated := map[string]any{
		"uuid":       e.UUID,
		"published":  e.Published.UTC().Format(time.RFC3339),
		"severity":   string(e.Severity),
		"eventType":  e.EventType,
		"displayMsg": e.DisplayMsg,
		"actor": map[string]any{
			"id":          e.Actor.ID,
			"type":        string(e.Actor.Type),
			"displayName": e.Actor.DisplayName,
			"alternateId": e.Actor.AlternateID,
		},
		"client": map[string]any{
			"ipAddress": e.Client.IPAddress,
			"userAgent": e.Client.UserAgent,
		},
		"outcome": map[string]any{
			"result": string(e.Outcome.Result),
			"reason": e.Outcome.Reason,
		},
	}
	if len(e.Targets) > 0 {
		ts := make([]map[string]any, 0, len(e.Targets))
		for _, t := range e.Targets {
			ts = append(ts, map[string]any{
				"id":          t.ID,
				"type":        t.Type,
				"displayName": t.DisplayName,
				"alternateId": t.AlternateID,
			})
		}
		curated["targets"] = ts
	}
	buf, err := json.MarshalIndent(curated, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(buf)
}

func renderLogDetail(e domain.LogEvent) string {
	var b strings.Builder
	b.WriteString("Log Event\n")
	b.WriteString("  uuid:       ")
	b.WriteString(e.UUID)
	b.WriteString("\n")
	b.WriteString("  published:  ")
	b.WriteString(formatTime(e.Published))
	b.WriteString("\n")
	b.WriteString("  severity:   ")
	b.WriteString(string(e.Severity))
	b.WriteString("\n")
	b.WriteString("  eventType:  ")
	b.WriteString(e.EventType)
	b.WriteString("\n")
	if e.DisplayMsg != "" {
		b.WriteString("  message:    ")
		b.WriteString(e.DisplayMsg)
		b.WriteString("\n")
	}
	b.WriteString("\nActor\n")
	b.WriteString("  id:          ")
	b.WriteString(e.Actor.ID)
	b.WriteString("\n")
	b.WriteString("  display:     ")
	b.WriteString(e.Actor.DisplayName)
	b.WriteString("\n")
	b.WriteString("  type:        ")
	b.WriteString(string(e.Actor.Type))
	b.WriteString("\n")
	if e.Actor.AlternateID != "" {
		b.WriteString("  alternateId: ")
		b.WriteString(e.Actor.AlternateID)
		b.WriteString("\n")
	}
	if len(e.Targets) > 0 {
		b.WriteString("\nTargets\n")
		for _, t := range e.Targets {
			b.WriteString("  - ")
			b.WriteString(t.DisplayName)
			b.WriteString(" (")
			b.WriteString(t.Type)
			b.WriteString(")\n")
		}
	}
	b.WriteString("\nOutcome: ")
	b.WriteString(string(e.Outcome.Result))
	if e.Outcome.Reason != "" {
		b.WriteString(" — ")
		b.WriteString(e.Outcome.Reason)
	}
	b.WriteString("\n")
	if e.Client.IPAddress != "" {
		b.WriteString("Client:  ")
		b.WriteString(e.Client.IPAddress)
		b.WriteString("\n")
	}
	return b.String()
}

// followTickMsg fires when the auto-refresh ticker (issue #177
// v0.1.16) should poll the next slice of events while follow mode is
// on. `gen` matches the model's `followGen` so toggling follow off
// invalidates in-flight ticks (no fetch fires after the operator
// pressed `f` to stop following).
type followTickMsg struct {
	gen int
}

// followFetchedMsg delivers the result of a single follow-mode tail
// poll: the slice of new events (already filtered by `since`), the
// next-since cursor to use for the following tick, and the wall-clock
// fetch time for the chrome's upper-divider stamp.
type followFetchedMsg struct {
	gen       int
	events    []domain.LogEvent
	nextSince time.Time
	at        time.Time
}

// scheduleFollowTickCmd returns a tea.Tick Cmd that fires a
// followTickMsg after the configured interval. Returns nil when
// follow is off OR the interval is zero so callers can chain it
// safely.
func (m SearchModel) scheduleFollowTickCmd() tea.Cmd {
	if !m.follow {
		return nil
	}
	return shared.ScheduleRefreshTickCmd(m.pollInterval,
		followTickMsg{gen: m.followGen})
}

// followFetchCmd issues one tail-style poll and returns a
// followFetchedMsg. Uses LogsTail.Poll when available (cursor-based
// incremental fetch — REQ-R05 AC-2 since-cursor); falls back to a
// service.Search for tests / scenarios with no Tail injected.
func (m SearchModel) followFetchCmd() tea.Cmd {
	gen := m.followGen
	since := m.followSince
	q := m.query // v0.2.2 #186 — carry the active server query
	tail := m.deps.Tail
	svc := m.deps.Service
	return func() tea.Msg {
		now := time.Now()
		ctx := context.Background()
		// Cursor seed: first tick after history fetch. Use 1ms past
		// the most-recent event we've already shown so the next
		// poll never re-emits a row.
		if since.IsZero() {
			since = now
		}
		query := domain.LogsQuery{
			Since:     &since,
			Q:         q,
			SortOrder: domain.SortAscending,
			Limit:     1000,
		}
		if tail != nil {
			events, nextSince, err := tail.Poll(ctx, query)
			if err != nil {
				return logsErrMsg{err: err}
			}
			if nextSince.IsZero() {
				nextSince = since
			}
			return followFetchedMsg{
				gen:       gen,
				events:    events,
				nextSince: nextSince,
				at:        now,
			}
		}
		// Fallback path — no Tail wired (test harness etc.).
		if svc == nil {
			return followFetchedMsg{gen: gen, events: nil, nextSince: since, at: now}
		}
		iter, err := svc.Search(ctx, query)
		if err != nil {
			return logsErrMsg{err: err}
		}
		defer iter.Close()
		var out []domain.LogEvent
		nextSince := since
		for {
			e, hasMore, err := iter.Next(ctx)
			if err != nil {
				return logsErrMsg{err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, e)
			if e.Published.After(nextSince) {
				nextSince = e.Published
			}
		}
		if len(out) > 0 {
			nextSince = nextSince.Add(time.Millisecond)
		}
		return followFetchedMsg{gen: gen, events: out, nextSince: nextSince, at: now}
	}
}

// --- Cmd factories -----------------------------------------------------------

func fetchHistoryCmd(svc *service.LogsService) tea.Cmd {
	return fetchHistoryWindowCmd(svc, 30*time.Minute)
}

func fetchHistoryWindowCmd(svc *service.LogsService, window time.Duration) tea.Cmd {
	return fetchHistoryWindowQueryCmd(svc, window, "", "")
}

// fetchHistoryWindowQueryCmd is the v0.2.1 (#185) variant that pushes
// server-side scoping into the history fetch. #F3 v0.2.5 — uses
// LogsService.SearchPage so one page returns; the operator drives
// older pages via Enter on the sentinel. #F4 v0.2.5 — accepts both
// q (free-text search) and filter (Okta filter expression). They
// compose: filter scopes to specific actor/target IDs, q narrows
// further by substring.
func fetchHistoryWindowQueryCmd(svc *service.LogsService, window time.Duration, q, filter string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		query := svc.HistoryQueryWindow(window)
		query.Q = q
		query.Filter = filter
		page, err := svc.SearchPage(ctx, query)
		if err != nil {
			return logsErrMsg{err: err}
		}
		return logsLoadedMsg{events: page.Events, after: page.After}
	}
}

// fetchOlderPageCmd issues a "load older" request using the cached
// next-page cursor. Called when the operator presses Enter on the
// load-older sentinel at the top of the buffer (#F3 v0.2.5).
func fetchOlderPageCmd(svc *service.LogsService, window time.Duration, q, filter, after string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		query := svc.HistoryQueryWindow(window)
		query.Q = q
		query.Filter = filter
		query.After = after
		page, err := svc.SearchPage(ctx, query)
		if err != nil {
			return logsErrMsg{err: err}
		}
		return logsOlderLoadedMsg{events: page.Events, after: page.After}
	}
}

// logActorDrillID returns the best identifier to feed
// shared.OpenUserDetailMsg with. Prefers AlternateID (the login,
// most useful for human inspection) and falls back to the actor ID
// when the alternate is unset. Empty when the event has no actor or
// the actor isn't a User type. Issue #G5 / U7 v0.2.4.
func logActorDrillID(e domain.LogEvent) string {
	if e.Actor.Type != domain.ActorTypeUser {
		return ""
	}
	if id := e.Actor.AlternateID; id != "" {
		return id
	}
	return e.Actor.ID
}

// openUserDetailCmd is the cross-screen drill-down emitter for the
// Log Detail actor row (#G5 / U7 v0.2.4). The App Shell forwards to
// the Users list, which opens detail by ID.
func openUserDetailCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenUserDetailMsg{ID: id} }
}

// sortLogsAscending returns a new slice sorted by Published ASC so
// the renderer can lay it out oldest-top / newest-bottom regardless
// of what sortOrder the API used for the fetch (#F3 v0.2.5).
func sortLogsAscending(events []domain.LogEvent) []domain.LogEvent {
	out := make([]domain.LogEvent, len(events))
	copy(out, events)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Published.Before(out[j].Published)
	})
	return out
}

var (
	_ tea.Model = SearchModel{}
	_ tea.Model = DetailModel{}
)
