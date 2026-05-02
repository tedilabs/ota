// Package authenticators renders the Okta Authenticators list and
// detail surface (#F1 v0.2.5). Authenticators are the org-level
// methods (password / email / phone / okta_verify / webauthn / etc.)
// operators enable for end-user enrollment.
package authenticators

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// Deps bundles dependencies shared by the Authenticators screen.
type Deps struct {
	Port   domain.AuthenticatorsPort
	Clock  clock.Clock
	Logger *slog.Logger
	Keys   keys.ResolvedMap
	Width  int
	Height int
	// RefreshInterval drives the auto-refresh tick. Zero disables.
	RefreshInterval time.Duration
	// InitialAuthenticators lets tests seed without invoking the port.
	InitialAuthenticators []domain.Authenticator
}

// authsLoadedMsg / authsErrMsg / *TickMsg follow the shape every
// list screen uses (cf. apps / rules / users) so the App Shell's
// generic FetchingStater / LastUpdatedStater interfaces work without
// extra wiring.
type authsLoadedMsg struct{ items []domain.Authenticator }
type authsErrMsg struct{ err error }
type authsRefreshTickMsg struct{ gen int }
type authsHighlightTickMsg struct{}
type authsSpinnerTickMsg struct{}

// AuthDetailTab aliases shared.DetailTab — same Pretty / JSON / YAML
// triad every other detail surface uses.
type AuthDetailTab = shared.DetailTab

const (
	AuthDetailTabPretty = shared.DetailTabPretty
	AuthDetailTabJSON   = shared.DetailTabJSON
	AuthDetailTabYAML   = shared.DetailTabYAML
)

// ListModel renders the Authenticators screen.
type ListModel struct {
	deps        Deps
	items       []domain.Authenticator
	cursor      int
	filter      string
	filtering   bool
	opened      bool
	detail      domain.Authenticator
	detailTab   AuthDetailTab
	detailRawReturn AuthDetailTab
	detailCursor    shared.BodyCursor // #F5 v0.2.5
	lastErr     error
	width       int
	height      int
	viewportTop int

	lastUpdated  time.Time
	refreshGen   int
	loaded       bool
	spinnerFrame int
	fetching     bool
	changedAt    map[string]time.Time
	failedAt     map[string]time.Time
}

// NewListModel constructs an Authenticators list. Seeded models skip
// the loading spinner.
func NewListModel(deps Deps) ListModel {
	m := ListModel{
		deps:   deps,
		items:  deps.InitialAuthenticators,
		width:  deps.Width,
		height: deps.Height,
	}
	if len(m.items) > 0 || deps.Port == nil {
		m.loaded = true
	}
	return m
}

func (m ListModel) Init() tea.Cmd {
	var fetch tea.Cmd
	if len(m.items) == 0 && m.deps.Port != nil {
		fetch = fetchAuthsCmd(m.deps.Port)
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

func (m ListModel) LastUpdated() time.Time { return m.lastUpdated }
func (m ListModel) Fetching() bool         { return m.fetching }

// EscapeWillAct reports whether Esc has work to do.
func (m ListModel) EscapeWillAct() bool {
	return m.filtering || m.opened || m.filter != ""
}

// Filtering / Filter / Count satisfy the App Shell's state interfaces.
func (m ListModel) Filtering() bool { return m.filtering }
func (m ListModel) Filter() string  { return m.filter }
func (m ListModel) Count() (visible, total int) {
	return len(m.visible()), len(m.items)
}

// Selected returns the cursor's authenticator (or the open detail
// when in detail mode). Used by the App Shell's `l` shortcut to feed
// the actor identifier into the Logs query.
func (m ListModel) SelectedID() string {
	if m.opened {
		return m.detail.ID
	}
	if rows := m.visible(); m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].ID
	}
	return ""
}

// SelectedLogQuery returns the search text the Logs screen should
// pre-populate when the operator presses `l` from this resource. We
// use the authenticator's Name (more readable than the opaque ID).
func (m ListModel) SelectedLogQuery() string {
	if m.opened {
		return m.detail.Name
	}
	if rows := m.visible(); m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].Name
	}
	return ""
}

func (m ListModel) scheduleRefreshTickCmd() tea.Cmd {
	if m.deps.Port == nil {
		return nil
	}
	return shared.ScheduleRefreshTickCmd(m.deps.RefreshInterval,
		authsRefreshTickMsg{gen: m.refreshGen})
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.loaded {
			return m, shared.ScheduleSpinnerTickCmd(authsSpinnerTickMsg{})
		}
		return m, nil
	case authsLoadedMsg:
		flash := shared.LoadDiff(&m.loaded, &m.lastUpdated, &m.changedAt,
			m.items, msg.items, m.now(),
			func(a domain.Authenticator) string { return a.ID }, authTrackedEqual)
		m.items = msg.items
		m.lastErr = nil
		m.fetching = false
		if flash {
			return m, shared.ScheduleHighlightTickCmd(authsHighlightTickMsg{})
		}
		return m, nil
	case authsErrMsg:
		m.lastErr = msg.err
		m.loaded = true
		m.fetching = false
		return m, nil
	case authsHighlightTickMsg:
		now := m.now()
		if shared.HasFreshHighlights(m.changedAt, now) ||
			shared.HasFreshHighlights(m.failedAt, now) {
			return m, shared.ScheduleHighlightTickCmd(authsHighlightTickMsg{})
		}
		return m, nil
	case authsSpinnerTickMsg:
		if !shared.BumpSpinner(m.loaded, &m.spinnerFrame) {
			return m, nil
		}
		return m, shared.ScheduleSpinnerTickCmd(authsSpinnerTickMsg{})
	case authsRefreshTickMsg:
		if msg.gen != m.refreshGen || m.deps.Port == nil {
			return m, nil
		}
		m.fetching = true
		return m, tea.Batch(fetchAuthsCmd(m.deps.Port), m.scheduleRefreshTickCmd())
	case shared.RefreshScreenMsg:
		if m.deps.Port == nil {
			return m, nil
		}
		m.fetching = true
		return m, fetchAuthsCmd(m.deps.Port)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ListModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.opened && !m.filtering && msg.Type == tea.KeyRunes {
		msg = shared.NormalizeArrowKey(msg)
	}

	if m.opened {
		switch msg.Type {
		case tea.KeyEsc:
			// #F5 v0.2.5 — Esc cancels visual mode first.
			if m.detailCursor.Visual {
				m.detailCursor.CancelVisual()
				return m, nil
			}
			m.opened = false
			m.detail = domain.Authenticator{}
			m.detailTab = AuthDetailTabPretty
			m.detailRawReturn = AuthDetailTabPretty
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
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "r":
				m.detailTab, m.detailRawReturn = shared.ToggleRawTab(m.detailTab, m.detailRawReturn)
				m.detailCursor = shared.BodyCursor{}
			case "j":
				m.detailCursor.Down(len(authDetailLines(m.detail, m.detailTab)))
			case "k":
				m.detailCursor.Up()
			case "g":
				m.detailCursor.Top()
			case "G":
				m.detailCursor.Bottom(len(authDetailLines(m.detail, m.detailTab)))
			case "v", "V":
				if m.detailCursor.Visual {
					m.detailCursor.CancelVisual()
				} else {
					m.detailCursor.StartVisual()
				}
			case "y":
				return m, shared.YankCmd(m.detailCursor, authDetailLines(m.detail, m.detailTab), "Authenticator Detail")
			case "l":
				// #F2 / #F4 v0.2.5 — open Logs scoped to events
				// targeting this authenticator's ID.
				if id := m.detail.ID; id != "" {
					return m, openLogsForCmd(`target.id eq "` + id + `"`)
				}
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

	if msg.Type == tea.KeyEsc && m.filter != "" {
		m.filter = ""
		m.cursor = 0
		return m, nil
	}

	rows := m.visible()
	switch msg.Type {
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(rows) {
			m.detail = rows[m.cursor]
			m.opened = true
		}
		return m, nil
	case tea.KeyRunes:
		switch string(msg.Runes) {
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
			if len(rows) > 0 {
				m.cursor = len(rows) - 1
			}
		case "/":
			m.filtering = true
			m.filter = ""
		case "l":
			// #F2 / #F4 v0.2.5 — open Logs scoped to events
			// targeting this authenticator's ID.
			if m.cursor >= 0 && m.cursor < len(rows) {
				if id := rows[m.cursor].ID; id != "" {
					return m, openLogsForCmd(`target.id eq "` + id + `"`)
				}
			}
		}
	}
	return m, nil
}

// openLogsForCmd asks the App Shell to open Logs scoped to a server
// filter expression (#F2 / #F4 v0.2.5).
func openLogsForCmd(filter string) tea.Cmd {
	return func() tea.Msg { return shared.OpenLogsMsg{Filter: filter} }
}

func (m ListModel) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

func (m ListModel) visible() []domain.Authenticator {
	if m.filter == "" {
		return m.items
	}
	out := make([]domain.Authenticator, 0, len(m.items))
	needle := strings.ToLower(m.filter)
	for _, a := range m.items {
		hay := strings.ToLower(a.Name + "\x00" + a.Key + "\x00" + string(a.Type))
		if strings.Contains(hay, needle) {
			out = append(out, a)
		}
	}
	return out
}

func (m ListModel) View() string {
	if m.opened {
		return renderAuthDetailTabbedWithCursor(m.detail, m.detailTab, m.detailCursor, m.chromeContentWidth()-2)
	}
	if m.lastErr != nil {
		return "Authenticators  (error)\n" + shared.ErrorPanel("authenticators", m.lastErr)
	}
	tk := activeTokens()
	if !m.loaded {
		return shared.LoadingPlaceholder(m.spinnerFrame, "Loading…",
			m.chromeContentWidth(), shared.ListBodyRowBudget(m.height), tk)
	}
	rows := m.visible()
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(formatColumns("STATUS", "TYPE", "KEY", "NAME", "UPDATED")))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(rows), shared.ListBodyRowBudget(m.height))
	budget := end - top
	rowTarget := m.chromeContentWidth() - 2
	now := m.now()
	for i := top; i < end; i++ {
		row := renderAuthRow(rows[i], now)
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		changed := shared.IsRowChanged(m.changedAt, rows[i].ID, now)
		tone := shared.RowToneNone
		if shared.IsRowChanged(m.failedAt, rows[i].ID, now) {
			tone = shared.RowToneFailed
		}
		b.WriteString(shared.RenderRowCursorTone(prefix+row, rowTarget, i == m.cursor, string(rows[i].Status), changed, tone, tk))
		b.WriteString(shared.AppendScrollbarSuffix(i-top, top, budget, len(rows), tk))
		b.WriteByte('\n')
	}
	return b.String()
}

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

func formatColumns(status, typ, key, name, updated string) string {
	return fmt.Sprintf("%-10s  %-18s  %-22s  %-30s  %-12s", status, typ, key, name, updated)
}

func renderAuthRow(a domain.Authenticator, now time.Time) string {
	updated := shared.RelativeTime(&a.LastUpdated, now)
	if a.LastUpdated.IsZero() {
		updated = "—"
	}
	return formatColumns(string(a.Status), string(a.Type), a.Key, a.Name, updated)
}

// authTrackedEqual reports whether two authenticator snapshots match
// on every field the list View renders.
func authTrackedEqual(a, b domain.Authenticator) bool {
	if a.Status != b.Status {
		return false
	}
	if a.Type != b.Type || a.Key != b.Key || a.Name != b.Name {
		return false
	}
	if !a.LastUpdated.Equal(b.LastUpdated) {
		return false
	}
	return true
}

// authDetailLines returns the active tab's body as a flat line
// slice — the unit shared.BodyCursor navigates and yanks against
// (#F5 v0.2.5).
func authDetailLines(a domain.Authenticator, active AuthDetailTab) []string {
	var raw string
	switch active {
	case AuthDetailTabJSON:
		raw = renderAuthJSON(a)
	case AuthDetailTabYAML:
		raw = renderAuthYAML(a)
	default:
		raw = renderAuthPretty(a)
	}
	return strings.Split(strings.TrimRight(raw, "\n"), "\n")
}

// renderAuthDetailTabbed renders the Pretty / JSON / YAML triad
// (legacy / no-cursor entrypoint).
func renderAuthDetailTabbed(a domain.Authenticator, active AuthDetailTab) string {
	return renderAuthDetailTabbedWithCursor(a, active, shared.BodyCursor{}, 0)
}

// renderAuthDetailTabbedWithCursor highlights the focused row + any
// visual selection (#F5 v0.2.5).
func renderAuthDetailTabbedWithCursor(a domain.Authenticator, active AuthDetailTab, cursor shared.BodyCursor, width int) string {
	var b strings.Builder
	b.WriteString("Authenticator Detail\n")
	b.WriteString(renderAuthTabBar(active))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 78))
	b.WriteByte('\n')
	if width <= 0 {
		switch active {
		case AuthDetailTabJSON:
			b.WriteString(renderAuthJSON(a))
		case AuthDetailTabYAML:
			b.WriteString(renderAuthYAML(a))
		default:
			b.WriteString(renderAuthPretty(a))
		}
		return b.String()
	}
	tk := activeTokens()
	rendered := cursor.RenderLines(authDetailLines(a, active), width, tk)
	b.WriteString(shared.JoinLines(rendered))
	return b.String()
}

func renderAuthTabBar(active AuthDetailTab) string {
	var b strings.Builder
	for i, label := range shared.DetailTabLabels {
		if AuthDetailTab(i) == active {
			b.WriteString("[" + label + "] ")
		} else {
			b.WriteString(" " + label + "  ")
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func renderAuthPretty(a domain.Authenticator) string {
	var b strings.Builder
	b.WriteString("  id:       " + a.ID + "\n")
	b.WriteString("  name:     " + a.Name + "\n")
	b.WriteString("  type:     " + string(a.Type) + "\n")
	b.WriteString("  key:      " + a.Key + "\n")
	b.WriteString("  status:   " + string(a.Status) + "\n")
	b.WriteString("  provider: " + string(a.Provider) + "\n")
	if !a.Created.IsZero() {
		b.WriteString("  created:  " + a.Created.UTC().Format(time.RFC3339) + "\n")
	}
	if !a.LastUpdated.IsZero() {
		b.WriteString("  updated:  " + a.LastUpdated.UTC().Format(time.RFC3339) + "\n")
	}
	return b.String()
}

func renderAuthJSON(a domain.Authenticator) string {
	out, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "(JSON encode failed: " + err.Error() + ")"
	}
	return string(out)
}

func renderAuthYAML(a domain.Authenticator) string {
	out, err := yaml.Marshal(a)
	if err != nil {
		return "(YAML encode failed: " + err.Error() + ")"
	}
	return string(out)
}

func fetchAuthsCmd(port domain.AuthenticatorsPort) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		items, err := port.List(ctx)
		if err != nil {
			return authsErrMsg{err: err}
		}
		return authsLoadedMsg{items: items}
	}
}

// activeTokens picks the active theme. Mirrors the App Shell helper.
func activeTokens() shared.Tokens {
	return shared.PickTheme(shared.ResolveTheme(""))
}

var _ tea.Model = ListModel{}
