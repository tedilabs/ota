// Package simpleres implements the read-only "list + Pretty/JSON/YAML
// detail" pattern that smaller Okta resources collapse onto. Used by
// network-zones / authorization-servers / api-tokens /
// administrators (all of which only need to be inspected, not
// mutated, in the MVP). Heavier surfaces (Users / Groups / Policies)
// keep their dedicated packages because they need actions, extras
// boxes, and per-resource quirks the scaffold can't generalize.
package simpleres

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
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// Lister is the per-resource fetch callback the scaffold drives. It
// returns every row in the resource (small lists, < 1000 entries —
// the heavy paged surfaces have their own packages). ctx carries the
// network deadline; the scaffold short-circuits when ctx is done.
type Lister[T any] func(ctx context.Context) ([]T, error)

// Column describes one list-view column. Format renders the cell
// text from the row; Width is the minimum cell width (0 = flex on
// the leftover budget). Header is shown in muted bold across the
// column header row.
type Column[T any] struct {
	Header string
	Width  int
	Flex   bool
	Format func(T) string
}

// Spec is the per-resource configuration the scaffold reads from.
// The minimum a resource has to provide is:
//
//   - Title       — chrome label (e.g. "Network Zones").
//   - List        — fetch callback.
//   - ID          — stable per-row identifier (drives RowChanged
//                   highlight + filter dedupe).
//   - FilterMatch — return true when a row matches the operator's
//                   `/`-filter substring (already lowercased).
//   - Columns     — list-view column specs.
//   - Pretty      — Pretty-tab body for the open-detail surface.
//
// Optional callbacks (StatusBadge, Actions, etc.) can be added
// later without changing scaffold callers.
type Spec[T any] struct {
	Title       string
	List        Lister[T]
	ID          func(T) string
	FilterMatch func(item T, needle string) bool
	Columns     []Column[T]
	Pretty      func(T) string
	// JSON / YAML — overrides for the raw tabs. When nil the scaffold
	// marshals the row directly with encoding/json + yaml.v3, which
	// is enough for transparent shapes (zones / authorization servers
	// / api-tokens / administrators all serialize cleanly).
	JSON func(T) string
	YAML func(T) string
}

// Deps bundles the cross-cutting scaffold inputs every list / detail
// surface in ota receives — clock, logger, keymap, terminal size,
// auto-refresh interval, and an optional seed slice for tests.
type Deps[T any] struct {
	Spec            Spec[T]
	Clock           clock.Clock
	Logger          *slog.Logger
	Keys            keys.ResolvedMap
	Width           int
	Height          int
	RefreshInterval time.Duration
	Initial         []T
}

// loadedMsg / errMsg / *TickMsg follow the same shape every list
// screen uses. The scaffold defines its own type so the App Shell's
// generic stat interfaces don't conflate ticks across resources.
type loadedMsg[T any] struct{ items []T }
type errMsg struct{ err error }
type refreshTickMsg struct{ gen int }
type spinnerTickMsg struct{}

// Model implements the read-only list + detail surface. State shape
// mirrors authenticators.ListModel — same cursor / filter / detail-
// tab semantics so the operator's mental model carries across every
// resource the scaffold powers.
type Model[T any] struct {
	deps        Deps[T]
	items       []T
	cursor      int
	filter      string
	filtering   bool
	opened      bool
	detail      T
	detailTab   shared.DetailTab
	detailRaw   shared.DetailTab
	detailCur   shared.BodyCursor
	lastErr     error
	width       int
	height      int
	viewportTop int

	lastUpdated  time.Time
	refreshGen   int
	loaded       bool
	spinnerFrame int
	fetching     bool
}

// New constructs a scaffold model. When the deps carry an Initial
// slice (test seeding) the loading spinner is suppressed.
func New[T any](deps Deps[T]) Model[T] {
	m := Model[T]{
		deps:   deps,
		items:  deps.Initial,
		width:  deps.Width,
		height: deps.Height,
	}
	if len(m.items) > 0 || deps.Spec.List == nil {
		m.loaded = true
	}
	return m
}

// Init fetches the initial snapshot + schedules the auto-refresh
// tick. Returns nil when the scaffold has no port wired (test path).
func (m Model[T]) Init() tea.Cmd {
	var fetch tea.Cmd
	if len(m.items) == 0 && m.deps.Spec.List != nil {
		fetch = m.fetchCmd()
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

func (m Model[T]) fetchCmd() tea.Cmd {
	list := m.deps.Spec.List
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		items, err := list(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return loadedMsg[T]{items: items}
	}
}

func (m Model[T]) scheduleRefreshTickCmd() tea.Cmd {
	if m.deps.RefreshInterval <= 0 || m.deps.Spec.List == nil {
		return nil
	}
	return shared.ScheduleRefreshTickCmd(m.deps.RefreshInterval,
		refreshTickMsg{gen: m.refreshGen})
}

// LastUpdated implements app.LastUpdatedStater for the chrome
// "freshness" stamp.
func (m Model[T]) LastUpdated() time.Time { return m.lastUpdated }

// Fetching implements app.FetchingStater so the chrome can render
// the in-flight indicator.
func (m Model[T]) Fetching() bool { return m.fetching }

// Filtering implements app.FilterStater for the floating filter box.
func (m Model[T]) Filtering() bool      { return m.filtering }
func (m Model[T]) FilterInput() string  { return m.filter }
func (m Model[T]) Filter() string       { return m.filter }
func (m Model[T]) FilterPrompt() string { return "/" }

// Count returns visible / total for the chrome upper divider.
func (m Model[T]) Count() (visible, total int) {
	return len(m.visible()), len(m.items)
}

// EscapeWillAct lets the App Shell tell whether Esc has work to do
// (silent Esc would otherwise feel broken).
func (m Model[T]) EscapeWillAct() bool {
	return m.filtering || m.opened || m.filter != ""
}

func (m Model[T]) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

// visible returns the items slice with the current `/`-filter applied.
func (m Model[T]) visible() []T {
	if m.filter == "" {
		return m.items
	}
	needle := strings.ToLower(m.filter)
	out := make([]T, 0, len(m.items))
	for _, it := range m.items {
		if m.deps.Spec.FilterMatch != nil && m.deps.Spec.FilterMatch(it, needle) {
			out = append(out, it)
		}
	}
	return out
}

// Update wires the standard list / detail key handler. Mirrors the
// authenticators pattern so cross-resource navigation feels
// uniform — `/` filter, j/k navigate, Enter / d open detail, Tab /
// Shift-Tab cycle Pretty/JSON/YAML, r toggles Raw, Esc backs out.
func (m Model[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.loaded {
			return m, shared.ScheduleSpinnerTickCmd(spinnerTickMsg{})
		}
		return m, nil
	case loadedMsg[T]:
		m.items = msg.items
		m.lastErr = nil
		m.loaded = true
		m.fetching = false
		m.lastUpdated = m.now()
		if m.cursor >= len(m.items) {
			m.cursor = 0
		}
		return m, nil
	case errMsg:
		m.lastErr = msg.err
		m.loaded = true
		m.fetching = false
		return m, nil
	case refreshTickMsg:
		if msg.gen != m.refreshGen || m.deps.Spec.List == nil {
			return m, nil
		}
		m.fetching = true
		return m, tea.Batch(m.fetchCmd(), m.scheduleRefreshTickCmd())
	case spinnerTickMsg:
		if !shared.BumpSpinner(m.loaded, &m.spinnerFrame) {
			return m, nil
		}
		return m, shared.ScheduleSpinnerTickCmd(spinnerTickMsg{})
	case shared.RefreshScreenMsg:
		if m.deps.Spec.List == nil {
			return m, nil
		}
		m.fetching = true
		return m, m.fetchCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model[T]) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.opened && !m.filtering && km.Type == tea.KeyRunes {
		km = shared.NormalizeArrowKey(km)
	}

	if m.opened {
		return m.handleDetailKey(km)
	}
	if m.filtering {
		switch km.Type {
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
			m.filter += string(km.Runes)
			return m, nil
		}
		return m, nil
	}

	if km.Type == tea.KeyEsc && m.filter != "" {
		m.filter = ""
		m.cursor = 0
		return m, nil
	}

	rows := m.visible()
	switch km.Type {
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(rows) {
			m.detail = rows[m.cursor]
			m.opened = true
			m.detailTab = shared.DetailTabPretty
			m.detailCur = shared.BodyCursor{}
		}
		return m, nil
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "/":
			m.filtering = true
			m.filter = ""
			return m, nil
		case "j":
			if m.cursor < len(rows)-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
			m.viewportTop = 0
		case "G":
			if n := len(rows); n > 0 {
				m.cursor = n - 1
			}
		case "d":
			if m.cursor >= 0 && m.cursor < len(rows) {
				m.detail = rows[m.cursor]
				m.opened = true
				m.detailTab = shared.DetailTabPretty
				m.detailCur = shared.BodyCursor{}
			}
		case "r":
			if m.deps.Spec.List != nil {
				m.fetching = true
				return m, m.fetchCmd()
			}
		}
	}
	return m, nil
}

func (m Model[T]) handleDetailKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	viewport := m.detailViewportRows()
	total := len(m.detailLines())
	switch km.Type {
	case tea.KeyEsc:
		if m.detailCur.Visual {
			m.detailCur.CancelVisual()
			return m, nil
		}
		m.opened = false
		m.detailTab = shared.DetailTabPretty
		m.detailRaw = shared.DetailTabPretty
		m.detailCur = shared.BodyCursor{}
		return m, nil
	case tea.KeyTab:
		m.detailTab = shared.NextTab(m.detailTab)
		m.detailCur = shared.BodyCursor{}
		return m, nil
	case tea.KeyShiftTab:
		m.detailTab = shared.PrevTab(m.detailTab)
		m.detailCur = shared.BodyCursor{}
		return m, nil
	case tea.KeyCtrlF:
		m.detailCur.PageDown(viewport, total)
		return m, nil
	case tea.KeyCtrlB:
		m.detailCur.PageUp(viewport)
		return m, nil
	case tea.KeyCtrlD:
		m.detailCur.HalfPageDown(viewport, total)
		return m, nil
	case tea.KeyCtrlU:
		m.detailCur.HalfPageUp(viewport)
		return m, nil
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "r":
			m.detailTab, m.detailRaw = shared.ToggleRawTab(m.detailTab, m.detailRaw)
			m.detailCur = shared.BodyCursor{}
			return m, nil
		case "j":
			m.detailCur.Down(total)
		case "k":
			m.detailCur.Up()
		case "g":
			m.detailCur.GoTop()
		case "G":
			m.detailCur.GoBottom(total)
		case "v", "V":
			if m.detailCur.Visual {
				m.detailCur.CancelVisual()
			} else {
				m.detailCur.StartVisual()
			}
		case "y":
			return m, shared.YankCmd(m.detailCur, m.detailLines(), m.deps.Spec.Title+" Detail")
		}
	}
	return m, nil
}

// detailLines flattens the active tab body into the line slice the
// BodyCursor navigates against.
func (m Model[T]) detailLines() []string {
	var raw string
	switch m.detailTab {
	case shared.DetailTabJSON:
		raw = m.renderJSON()
	case shared.DetailTabYAML:
		raw = m.renderYAML()
	default:
		if m.deps.Spec.Pretty != nil {
			raw = m.deps.Spec.Pretty(m.detail)
		}
	}
	return strings.Split(strings.TrimRight(raw, "\n"), "\n")
}

func (m Model[T]) renderJSON() string {
	if m.deps.Spec.JSON != nil {
		return m.deps.Spec.JSON(m.detail)
	}
	buf, err := json.MarshalIndent(m.detail, "", "  ")
	if err != nil {
		return "(json error: " + err.Error() + ")"
	}
	return shared.HighlightJSON(string(buf), activeTokens())
}

func (m Model[T]) renderYAML() string {
	if m.deps.Spec.YAML != nil {
		return m.deps.Spec.YAML(m.detail)
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m.detail); err != nil {
		return "(yaml error: " + err.Error() + ")"
	}
	_ = enc.Close()
	return shared.HighlightYAML(strings.TrimRight(buf.String(), "\n"), activeTokens())
}

// detailViewportRows reserves 3 rows for the in-body header (title +
// tab bar + divider) when sizing the body cursor's window.
func (m Model[T]) detailViewportRows() int {
	return shared.DetailBodyRowBudget(m.height, 3)
}

// chromeContentWidth returns the body width minus the chrome's
// scrollbar gutter — cells available per data row.
func (m Model[T]) chromeContentWidth() int {
	w := m.width
	if w <= 0 {
		w = shared.ChromeWidth
	}
	if w < 80 {
		w = 80
	}
	return w - 3
}

// View renders the list or detail. Detail-mode renders through the
// canonical RenderDetailTabBar helper so navigation reads identical
// across every resource.
func (m Model[T]) View() string {
	if m.opened {
		return m.renderDetail()
	}
	tk := activeTokens()
	if m.lastErr != nil {
		return m.deps.Spec.Title + "  (error)\n" + shared.ErrorPanel(strings.ToLower(m.deps.Spec.Title), m.lastErr)
	}
	if !m.loaded {
		return shared.LoadingPlaceholder(m.spinnerFrame, "Loading…",
			m.chromeContentWidth(), shared.ListBodyRowBudget(m.height), tk)
	}
	rows := m.visible()
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatColumns(m.headerCells())))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(rows), shared.ListBodyRowBudget(m.height))
	budget := end - top
	rowTarget := m.chromeContentWidth() - 2
	for i := top; i < end; i++ {
		row := m.formatColumns(m.rowCells(rows[i]))
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		b.WriteString(shared.RenderRowCursor(prefix+row, rowTarget, i == m.cursor, "", false, tk))
		b.WriteString(shared.AppendScrollbarSuffix(i-top, top, budget, len(rows), tk))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderDetail composes the title + tab bar + body for the open
// detail surface.
func (m Model[T]) renderDetail() string {
	tk := activeTokens()
	width := m.chromeContentWidth() - 2
	var b strings.Builder
	b.WriteString(m.deps.Spec.Title + " Detail\n")
	bw := width
	if bw <= 0 {
		bw = 78
	}
	b.WriteString(shared.RenderDetailTabBar(m.detailTab, bw, tk))
	b.WriteByte('\n')
	if width <= 0 {
		switch m.detailTab {
		case shared.DetailTabJSON:
			b.WriteString(m.renderJSON())
		case shared.DetailTabYAML:
			b.WriteString(m.renderYAML())
		default:
			if m.deps.Spec.Pretty != nil {
				b.WriteString(m.deps.Spec.Pretty(m.detail))
			}
		}
		return b.String()
	}
	rendered := m.detailCur.RenderViewport(m.detailLines(), width, m.detailViewportRows(), tk)
	b.WriteString(shared.JoinLines(rendered))
	return b.String()
}

// headerCells returns the column header text per Spec.Columns.
func (m Model[T]) headerCells() []string {
	out := make([]string, len(m.deps.Spec.Columns))
	for i, c := range m.deps.Spec.Columns {
		out[i] = c.Header
	}
	return out
}

// rowCells renders one row's column values.
func (m Model[T]) rowCells(it T) []string {
	out := make([]string, len(m.deps.Spec.Columns))
	for i, c := range m.deps.Spec.Columns {
		if c.Format != nil {
			out[i] = c.Format(it)
		}
	}
	return out
}

// formatColumns lays out column cells with 2-space gutters and the
// per-column width (flex columns absorb the remaining width budget,
// fixed-width columns render at exactly their declared minimum).
func (m Model[T]) formatColumns(cells []string) string {
	cols := m.deps.Spec.Columns
	if len(cols) == 0 || len(cells) != len(cols) {
		return strings.Join(cells, "  ")
	}
	specs := make([]shared.ColumnSpec, len(cols))
	for i, c := range cols {
		minW := c.Width
		if minW < 1 {
			minW = 1
		}
		kind := shared.ColumnFixed
		weight := 0
		if c.Flex {
			kind = shared.ColumnFlex
			weight = 1
		}
		specs[i] = shared.ColumnSpec{
			Title:  c.Header,
			Kind:   kind,
			Min:    minW,
			Weight: weight,
		}
	}
	widths := shared.LayoutColumns(specs, m.chromeContentWidth()-2, 2)
	var b strings.Builder
	first := true
	for i, cell := range cells {
		if widths[i] == 0 {
			continue
		}
		if !first {
			b.WriteString("  ")
		}
		first = false
		b.WriteString(shared.PadOrTruncateVisible(cell, widths[i]))
	}
	return b.String()
}

// Snapshot exposes the current item slice so tests + the action
// menu (when later added) can introspect state.
func (m Model[T]) Snapshot() []T {
	out := make([]T, len(m.items))
	copy(out, m.items)
	return out
}

// CursorItem returns the item the cursor is parked on (zero-value
// when the list is empty).
func (m Model[T]) CursorItem() T {
	rows := m.visible()
	var zero T
	if m.cursor < 0 || m.cursor >= len(rows) {
		return zero
	}
	return rows[m.cursor]
}

// activeTokens picks the token set per the active theme. Routed
// through PickTheme so monochrome / light / high-contrast all work.
func activeTokens() shared.Tokens {
	return shared.PickTheme(shared.ResolveTheme(""))
}

// formatTimeRel formats a time as a relative duration ("2h ago"),
// fallback on dash for zero values. Helper for column callbacks.
func formatTimeRel(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return shared.RelativeTime(&t, now)
}

// SortByID is a convenience for callers that want a deterministic
// list — the scaffold itself doesn't sort.
func SortByID[T any](items []T, id func(T) string) {
	sort.SliceStable(items, func(i, j int) bool {
		return id(items[i]) < id(items[j])
	})
}

var _ = formatTimeRel // keep helper exported for future column callbacks
