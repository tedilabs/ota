// Package logs implements the System Logs search/tail/detail Screen Models
// (SCR-050, SCR-051). Tail mode renders a `[TAIL 7s] ▶` indicator per
// REQ-R05 AC-3.
package logs

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
}

// --- List / Search (SCR-050) -------------------------------------------------

type logsLoadedMsg struct{ events []domain.LogEvent }

// LoadedForTest synthesises the (unexported) logsLoadedMsg so black-box
// tests can deliver a deterministic event slice through Update without
// running the network fetch path.
func LoadedForTest(events []domain.LogEvent) tea.Msg {
	return logsLoadedMsg{events: events}
}

// logsErrMsg surfaces a fetch failure (TUI_DESIGN §17).
type logsErrMsg struct{ err error }

// SearchModel is SCR-050.
type SearchModel struct {
	deps         Deps
	events       []domain.LogEvent
	cursor       int
	tail         bool
	follow       bool
	pollInterval time.Duration
	opened       bool
	detail       domain.LogEvent
	lastErr      error
	width        int
	height       int
	viewportTop  int
	ggChord      shared.GChord
	// timeRange is the active history window (issue #116). Default 30m;
	// 1h / 3h / 12h / 24h selectable via `1`, `3`, `c`, `e` shortcuts.
	timeRange time.Duration
}

// NewSearchModel constructs a SearchModel with defaults (tail off, follow on,
// poll interval 7s per REQ-R05 AC-2). When deps.Tail is set, the initial
// interval reflects the tail's current adaptive state.
func NewSearchModel(deps Deps) SearchModel {
	interval := 7 * time.Second
	if deps.Tail != nil {
		interval = deps.Tail.PollInterval()
	}
	return SearchModel{
		deps:         deps,
		events:       deps.InitialEvents,
		follow:       true,
		pollInterval: interval,
		width:        deps.Width,
		height:       deps.Height,
		timeRange:    30 * time.Minute,
	}
}

// TimeRange reports the active history window — exposed for tests and
// other models that need to mirror the operator's selection.
func (m SearchModel) TimeRange() time.Duration { return m.timeRange }

// Init fetches the history list.
func (m SearchModel) Init() tea.Cmd {
	if len(m.events) > 0 || m.deps.Service == nil {
		return nil
	}
	return fetchHistoryWindowCmd(m.deps.Service, m.timeRange)
}

// Update handles keys: `s` toggles tail, `f` toggles follow, j/k navigates,
// Enter opens detail (REQ-R05 AC-3).
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case logsLoadedMsg:
		m.events = msg.events
		m.lastErr = nil
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
		return m, nil
	case logsErrMsg:
		m.lastErr = msg.err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m SearchModel) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc inside detail takes precedence so operators always have a way
	// back to the list (TUI_DESIGN §3.6 / §3.6a Note).
	if m.opened && km.Type == tea.KeyEsc {
		m.opened = false
		m.detail = domain.LogEvent{}
		return m, nil
	}
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	case tea.KeyCtrlF:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor = clampLogIdx(m.cursor+page, len(m.events))
	case tea.KeyCtrlB:
		page := shared.ListBodyRowBudget(m.height)
		if page <= 0 {
			page = 10
		}
		m.cursor = clampLogIdx(m.cursor-page, len(m.events))
	case tea.KeyCtrlD:
		page := shared.ListBodyRowBudget(m.height) / 2
		if page < 1 {
			page = 5
		}
		m.cursor = clampLogIdx(m.cursor+page, len(m.events))
	case tea.KeyCtrlU:
		page := shared.ListBodyRowBudget(m.height) / 2
		if page < 1 {
			page = 5
		}
		m.cursor = clampLogIdx(m.cursor-page, len(m.events))
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.events) {
			m.detail = m.events[m.cursor]
			m.opened = true
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "g":
			if m.ggChord.Press(m.now()) {
				m.cursor = 0
				m.viewportTop = 0
			}
		case "G":
			m.ggChord.Reset()
			if n := len(m.events); n > 0 {
				m.cursor = n - 1
			}
		case "s":
			m.ggChord.Reset()
			m.tail = !m.tail
		case "f":
			m.ggChord.Reset()
			m.follow = !m.follow
		case "r":
			// Manual refresh — operator wants the current window
			// re-fetched (e.g., after firing a write op elsewhere).
			m.ggChord.Reset()
			if m.deps.Service != nil {
				return m, fetchHistoryWindowCmd(m.deps.Service, m.timeRange)
			}
		case "j":
			m.ggChord.Reset()
			if m.cursor < len(m.events)-1 {
				m.cursor++
			}
		case "k":
			m.ggChord.Reset()
			if m.cursor > 0 {
				m.cursor--
			}
		case "d":
			m.ggChord.Reset()
			if m.cursor >= 0 && m.cursor < len(m.events) {
				m.detail = m.events[m.cursor]
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
	return m, fetchHistoryWindowCmd(m.deps.Service, window)
}

// View renders SCR-050 (TUI_DESIGN §15.6 / §16.8). Columns:
// WHEN / SEV / EVENTTYPE / ACTOR / OUTCOME / IP. Tail indicator is contributed
// to the first line so operators see the tail state at a glance (REQ-R05
// AC-3).
func (m SearchModel) View() string {
	if m.opened {
		return renderLogDetail(m.detail)
	}
	if m.lastErr != nil {
		return "Logs  (error)\n" + shared.ErrorPanel("events", m.lastErr)
	}

	tk := activeTokens()
	now := m.now()

	var b strings.Builder
	// Resource label moved to chrome's upper divider (issue #133); the
	// body now surfaces just the live state — time-range window and
	// tail/follow toggles — so operators can read the controls at a
	// glance without the redundant "System Logs" prefix.
	b.WriteString("[")
	b.WriteString(timeRangeLabel(m.timeRange))
	b.WriteString("]  ")
	b.WriteString(tailIndicator(m.tail, m.pollInterval, m.follow))
	b.WriteByte('\n')
	// 2-cell cursor gutter on the header keeps it aligned with data rows.
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatLogsColumns("WHEN", "SEV", "EVENTTYPE", "ACTOR", "OUTCOME", "IP")))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(m.events), shared.ListBodyRowBudget(m.height))
	for i := top; i < end; i++ {
		row := m.renderLogsRow(m.events[i], now, tk)
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

// renderLogsRow formats one event row.
func (m SearchModel) renderLogsRow(e domain.LogEvent, now time.Time, tk shared.Tokens) string {
	when := shared.RelativeTime(&e.Published, now)
	if e.Published.IsZero() {
		when = "—"
	}
	sev := shared.SeverityBadge(string(e.Severity), tk).Render(tk)
	outcome := string(e.Outcome.Result)
	if outcome == "" {
		outcome = "—"
	}
	actor := e.Actor.DisplayName
	if actor == "" {
		actor = e.Actor.AlternateID
	}
	return m.formatLogsColumns(when, sev, e.EventType, actor, outcome, e.Client.IPAddress)
}

// formatLogsColumns lays out 6 columns (TUI_DESIGN §15.6) with responsive
// drop:
//
//   - W ≥ 120 : all 6 columns
//   - 100..119: drop IP
//   - 90..99  : drop IP + OUTCOME
//   - 80..89  : drop IP + OUTCOME + ACTOR
//   - <80     : WHEN + SEV + EVENTTYPE
func (m SearchModel) formatLogsColumns(when, sev, etype, actor, outcome, ip string) string {
	w := m.width
	const (
		wWhen    = 12
		wSev     = 8
		wEvent   = 24
		wActor   = 18
		wOutcome = 9
		wIP      = 15
	)
	switch {
	case w >= 120 || w == 0:
		return padRightLog(when, wWhen) + "  " + padRightLog(sev, wSev) + "  " +
			padRightLog(shared.Truncate(etype, wEvent), wEvent) + "  " +
			padRightLog(shared.Truncate(actor, wActor), wActor) + "  " +
			padRightLog(outcome, wOutcome) + "  " + padRightLog(ip, wIP)
	case w >= 100:
		return padRightLog(when, wWhen) + "  " + padRightLog(sev, wSev) + "  " +
			padRightLog(shared.Truncate(etype, wEvent), wEvent) + "  " +
			padRightLog(shared.Truncate(actor, wActor), wActor) + "  " +
			padRightLog(outcome, wOutcome)
	case w >= 90:
		return padRightLog(when, wWhen) + "  " + padRightLog(sev, wSev) + "  " +
			padRightLog(shared.Truncate(etype, wEvent), wEvent) + "  " +
			padRightLog(shared.Truncate(actor, wActor), wActor)
	case w >= 80:
		return padRightLog(when, wWhen) + "  " + padRightLog(sev, wSev) + "  " +
			padRightLog(shared.Truncate(etype, wEvent), wEvent)
	default:
		return padRightLog(when, wWhen) + "  " + padRightLog(sev, wSev) + "  " +
			padRightLog(shared.Truncate(etype, max(0, w-22)), max(0, w-22))
	}
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

// tailIndicator returns a two-segment status string surfacing both the
// `s`-tail and `f`-follow toggles independently. Each segment is always
// rendered so operators can see which key flipped what state — the
// previous "TAIL OFF only" form hid follow's effect when tail was off.
func tailIndicator(tail bool, interval time.Duration, follow bool) string {
	tailSeg := "[TAIL OFF]"
	if tail {
		tailSeg = fmt.Sprintf("[TAIL %ds]", int(interval/time.Second))
	}
	followSeg := "[FOLLOW]"
	if !follow {
		followSeg = "[PAUSED]"
	}
	return tailSeg + " " + followSeg
}

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
// the full event UUID.
func (m DetailModel) View() string { return renderLogDetail(m.event) }

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

// --- Cmd factories -----------------------------------------------------------

func fetchHistoryCmd(svc *service.LogsService) tea.Cmd {
	return fetchHistoryWindowCmd(svc, 30*time.Minute)
}

func fetchHistoryWindowCmd(svc *service.LogsService, window time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		iter, err := svc.Search(ctx, svc.HistoryQueryWindow(window))
		if err != nil {
			return logsErrMsg{err: err}
		}
		defer iter.Close()
		var out []domain.LogEvent
		for {
			e, hasMore, err := iter.Next(ctx)
			if err != nil {
				return logsErrMsg{err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, e)
		}
		return logsLoadedMsg{events: out}
	}
}

var (
	_ tea.Model = SearchModel{}
	_ tea.Model = DetailModel{}
)
