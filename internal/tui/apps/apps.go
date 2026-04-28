// Package apps implements the Okta Applications type-select, list,
// and detail screens (issue #166). Mirrors the Policies pattern: a
// Wrapper picks an app type then swaps to a typed list.
package apps

import (
	"context"
	"encoding/json"
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

// Deps bundles dependencies shared by Apps screens.
type Deps struct {
	Port   domain.AppsPort
	Clock  clock.Clock
	Logger *slog.Logger
	Keys   keys.ResolvedMap
	Width  int
	Height int
	// RefreshInterval drives the auto-refresh tick (issue #177
	// v0.1.16). Zero disables auto-refresh.
	RefreshInterval time.Duration
	// InitialApps lets tests seed without invoking the port.
	InitialApps []domain.App
}

// --- Type Select -------------------------------------------------------------

// TypeSelectModel lets the user pick an AppType before fetching.
type TypeSelectModel struct {
	deps   Deps
	types  []domain.AppType
	cursor int
	picked domain.AppType
	done   bool
}

func NewTypeSelectModel(deps Deps) TypeSelectModel {
	return TypeSelectModel{
		deps: deps,
		types: []domain.AppType{
			domain.AppTypeSAML,
			domain.AppTypeOIDC,
			domain.AppTypeBookmark,
			domain.AppTypeSWA,
			domain.AppTypeSCIM,
			domain.AppTypeOther,
		},
	}
}

func (m TypeSelectModel) Init() tea.Cmd { return nil }

func (m TypeSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	km = shared.NormalizeArrowKey(km)
	switch km.Type {
	case tea.KeyCtrlC:
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.types) {
			m.picked = m.types[m.cursor]
			m.done = true
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			if m.cursor < len(m.types)-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

func (m TypeSelectModel) View() string {
	var b strings.Builder
	b.WriteString("Select App Type:\n")
	for i, t := range m.types {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		b.WriteString(prefix)
		b.WriteString(appTypeLabel(t))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m TypeSelectModel) Picked() (domain.AppType, bool) { return m.picked, m.done }

// appTypeLabel returns the human-friendly label for the picker.
func appTypeLabel(t domain.AppType) string {
	switch t {
	case domain.AppTypeSAML:
		return "SAML 2.0  (saml-app)"
	case domain.AppTypeOIDC:
		return "OpenID Connect  (oidc-app)"
	case domain.AppTypeBookmark:
		return "Bookmark  (bookmark-app)"
	case domain.AppTypeSWA:
		return "SWA / Auto-login  (swa-app)"
	case domain.AppTypeSCIM:
		return "SCIM  (scim-app)"
	case domain.AppTypeOther:
		return "Other  (other-app)"
	}
	return string(t)
}

// --- List --------------------------------------------------------------------

type appsLoadedMsg struct{ apps []domain.App }
type appsErrMsg struct{ err error }

// OpenDetailByIDMsg routes a cross-screen drill-down request into the
// Apps Wrapper (issue #171: User Detail Apps row Enter). The Wrapper
// fetches the app, infers its AppType from the SignOnMode, and flips
// directly into a typed list with detail mode open.
type OpenDetailByIDMsg struct {
	ID string
}

// appOpenedByIDMsg / appOpenByIDErrMsg deliver the result of an
// OpenDetailByIDMsg-triggered fetch back to the Wrapper.
type appOpenedByIDMsg struct{ app domain.App }
type appOpenByIDErrMsg struct{ err error }

// fetchAppByIDCmd resolves an App via AppsPort.Get for cross-screen
// drill-down (issue #171). The Wrapper consumes the result.
func fetchAppByIDCmd(port domain.AppsPort, id string) tea.Cmd {
	return func() tea.Msg {
		if port == nil {
			return appOpenByIDErrMsg{err: domain.ErrNotFound}
		}
		ctx := context.Background()
		a, err := port.Get(ctx, id)
		if err != nil {
			return appOpenByIDErrMsg{err: err}
		}
		return appOpenedByIDMsg{app: a}
	}
}

// ListModel renders apps of a single type.
type ListModel struct {
	deps        Deps
	appType     domain.AppType
	apps        []domain.App
	cursor      int
	filter      string
	filtering   bool
	opened      bool
	detail      domain.App
	detailTab   AppDetailTab
	lastErr     error
	width       int
	height      int
	viewportTop int
	ggChord     shared.GChord
	// lastUpdated stamps the most recent successful list fetch (issue
	// #177 v0.1.16); refreshGen invalidates stale ticks.
	lastUpdated time.Time
	refreshGen  int
}

// appsRefreshTickMsg fires the auto-refresh tick (issue #177).
type appsRefreshTickMsg struct{ gen int }

// AppDetailTab indexes the detail tab bar — Pretty / JSON / YAML
// (matches the rest of the detail surfaces in ota).
type AppDetailTab int

const (
	AppDetailTabPretty AppDetailTab = iota
	AppDetailTabJSON
	AppDetailTabYAML
)

var appDetailTabLabels = []string{"Pretty", "JSON", "YAML"}
var appDetailTabCount = AppDetailTab(len(appDetailTabLabels))

func NewListModel(deps Deps, t domain.AppType) ListModel {
	return ListModel{
		deps:    deps,
		appType: t,
		apps:    deps.InitialApps,
		width:   deps.Width,
		height:  deps.Height,
	}
}

func (m ListModel) Init() tea.Cmd {
	var fetch tea.Cmd
	if len(m.apps) == 0 && m.deps.Port != nil {
		fetch = fetchAppsCmd(m.deps.Port, m.appType)
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

// scheduleRefreshTickCmd returns the auto-refresh tea.Tick.
func (m ListModel) scheduleRefreshTickCmd() tea.Cmd {
	if m.deps.RefreshInterval <= 0 || m.deps.Port == nil {
		return nil
	}
	gen := m.refreshGen
	return tea.Tick(m.deps.RefreshInterval, func(time.Time) tea.Msg {
		return appsRefreshTickMsg{gen: gen}
	})
}

// Filtering / Filter / Count satisfy the App Shell's state interfaces.
func (m ListModel) Filtering() bool { return m.filtering }
func (m ListModel) Filter() string  { return m.filter }
func (m ListModel) Count() (visible, total int) {
	return len(m.visible()), len(m.apps)
}

func (m ListModel) visible() []domain.App {
	if m.filter == "" {
		return m.apps
	}
	needle := strings.ToLower(m.filter)
	out := make([]domain.App, 0, len(m.apps))
	for _, a := range m.apps {
		hay := strings.ToLower(a.Label + "\x00" + a.Name + "\x00" + a.SignOnMode)
		if strings.Contains(hay, needle) {
			out = append(out, a)
		}
	}
	return out
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case appsLoadedMsg:
		m.apps = msg.apps
		m.lastErr = nil
		m.lastUpdated = m.now()
		return m, nil
	case appsErrMsg:
		m.lastErr = msg.err
		return m, nil
	case appsRefreshTickMsg:
		if msg.gen != m.refreshGen || m.deps.Port == nil {
			return m, nil
		}
		return m, tea.Batch(fetchAppsCmd(m.deps.Port, m.appType), m.scheduleRefreshTickCmd())
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ListModel) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	if km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	if !m.filtering {
		km = shared.NormalizeArrowKey(km)
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
	if !m.opened && km.Type == tea.KeyEsc && m.filter != "" {
		m.filter = ""
		m.cursor = 0
		m.viewportTop = 0
		return m, nil
	}
	if m.opened {
		switch km.Type {
		case tea.KeyEsc:
			m.opened = false
			m.detail = domain.App{}
			m.detailTab = AppDetailTabPretty
			return m, nil
		case tea.KeyTab:
			m.detailTab = (m.detailTab + 1) % appDetailTabCount
			return m, nil
		case tea.KeyShiftTab:
			m.detailTab = (m.detailTab + appDetailTabCount - 1) % appDetailTabCount
			return m, nil
		case tea.KeyRunes:
			if string(km.Runes) == "r" {
				if m.detailTab == AppDetailTabJSON {
					m.detailTab = AppDetailTabPretty
				} else {
					m.detailTab = AppDetailTabJSON
				}
				return m, nil
			}
		}
		return m, nil
	}
	rows := m.visible()
	switch km.Type {
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(rows) {
			m.detail = rows[m.cursor]
			m.opened = true
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "/":
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
		case "j":
			m.ggChord.Reset()
			if m.cursor < len(rows)-1 {
				m.cursor++
			}
		case "k":
			m.ggChord.Reset()
			if m.cursor > 0 {
				m.cursor--
			}
		case "d":
			m.ggChord.Reset()
			if m.cursor >= 0 && m.cursor < len(rows) {
				m.detail = rows[m.cursor]
				m.opened = true
			}
		}
	}
	return m, nil
}

func (m ListModel) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

// View renders the list (or the detail when opened).
func (m ListModel) View() string {
	if m.opened {
		return renderAppDetailTabbed(m.detail, m.detailTab)
	}
	if m.lastErr != nil {
		return "Apps  (error)\n" + shared.ErrorPanel("Apps", m.lastErr)
	}
	tk := activeTokens()
	now := m.now()
	rows := m.visible()
	var b strings.Builder
	b.WriteString("› ")
	b.WriteString(appTypeLabel(m.appType))
	b.WriteByte('\n')
	b.WriteString("  ")
	b.WriteString(tk.Header.Render(m.formatColumns("STATUS", "LABEL", "NAME", "SIGN-ON MODE", "UPDATED")))
	b.WriteByte('\n')
	top, end := shared.WindowBounds(m.cursor, m.viewportTop, len(rows), shared.ListBodyRowBudget(m.height))
	budget := end - top
	rowTarget := m.chromeContentWidth() - 2
	for i := top; i < end; i++ {
		row := m.renderRow(rows[i], now, tk)
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		composed := prefix + row
		composed = shared.PadOrTruncateVisible(composed, rowTarget)
		switch {
		case i == m.cursor:
			composed = tk.Accent.Render(composed)
		default:
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

// chromeContentWidth returns the body cells the chrome reserves per
// row — used to land the scrollbar gutter flush against the right
// border (issue #173).
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

func (m ListModel) renderRow(a domain.App, now time.Time, tk shared.Tokens) string {
	status := shared.AppStatusBadge(string(a.Status), tk).Render(tk)
	updated := shared.RelativeTime(&a.LastUpdated, now)
	if a.LastUpdated.IsZero() {
		updated = "—"
	}
	signOn := a.SignOnMode
	if signOn == "" {
		signOn = "—"
	}
	return m.formatColumns(status, a.Label, a.Name, signOn, updated)
}

func (m ListModel) formatColumns(cells ...string) string {
	specs := appsColumnSpecs()
	specs = shared.ShrinkSpecsToFit(specs, m.observedColumnWidths())
	widths := m.appsWidths(specs)
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

func appsColumnSpecs() []shared.ColumnSpec {
	return []shared.ColumnSpec{
		{Title: "STATUS", Kind: shared.ColumnFixed, Min: 10, DropPriority: 0, AlignCenter: true},
		{Title: "LABEL", Kind: shared.ColumnFlex, Min: 24, Weight: 3, DropPriority: 0},
		{Title: "NAME", Kind: shared.ColumnFlex, Min: 22, Weight: 2, DropPriority: 2},
		{Title: "SIGN-ON MODE", Kind: shared.ColumnFixed, Min: 16, DropPriority: 1},
		{Title: "UPDATED", Kind: shared.ColumnFixed, Min: 10, DropPriority: 3, AlignRight: true},
	}
}

func (m ListModel) appsWidths(specs []shared.ColumnSpec) []int {
	inner := m.appsInnerWidth()
	if w := shared.LayoutColumnsTight(specs, inner, 2); w != nil {
		return w
	}
	return shared.LayoutColumnsHScroll(specs, inner, 2, 0)
}

func (m ListModel) appsInnerWidth() int {
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

func (m ListModel) observedColumnWidths() []int {
	rows := m.visible()
	if len(rows) == 0 {
		return nil
	}
	now := m.now()
	tk := activeTokens()
	out := make([]int, 5)
	for _, a := range rows {
		status := shared.AppStatusBadge(string(a.Status), tk).Render(tk)
		updated := shared.RelativeTime(&a.LastUpdated, now)
		if a.LastUpdated.IsZero() {
			updated = "—"
		}
		signOn := a.SignOnMode
		if signOn == "" {
			signOn = "—"
		}
		cells := []string{status, a.Label, a.Name, signOn, updated}
		for i, c := range cells {
			if w := shared.VisibleWidth(c); w > out[i] {
				out[i] = w
			}
		}
	}
	return out
}

func activeTokens() shared.Tokens {
	if shared.MonochromeEnabled() {
		return shared.Monochrome()
	}
	return shared.Dark()
}

// --- Detail ------------------------------------------------------------------

func renderAppDetailTabbed(a domain.App, active AppDetailTab) string {
	var b strings.Builder
	b.WriteString("App Detail\n")
	b.WriteString(renderAppTabBar(active))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 78))
	b.WriteByte('\n')
	switch active {
	case AppDetailTabJSON:
		b.WriteString(renderAppJSONTab(a))
	case AppDetailTabYAML:
		b.WriteString(renderAppYAMLTab(a))
	default:
		b.WriteString(renderAppPretty(a))
	}
	return b.String()
}

func renderAppTabBar(active AppDetailTab) string {
	parts := make([]string, 0, len(appDetailTabLabels))
	for i, label := range appDetailTabLabels {
		if AppDetailTab(i) == active {
			parts = append(parts, "["+label+"]")
		} else {
			parts = append(parts, "[ "+label+" ]")
		}
	}
	return strings.Join(parts, " ")
}

func renderAppPretty(a domain.App) string {
	const keyWidth = 16
	var b strings.Builder
	b.WriteString(shared.KVRow("id", a.ID, keyWidth))
	b.WriteByte('\n')
	b.WriteString(shared.KVRow("name", a.Name, keyWidth))
	b.WriteByte('\n')
	b.WriteString(shared.KVRow("label", a.Label, keyWidth))
	b.WriteByte('\n')
	b.WriteString(shared.KVRow("type", string(a.Type), keyWidth))
	b.WriteByte('\n')
	b.WriteString(shared.KVRow("signOnMode", a.SignOnMode, keyWidth))
	b.WriteByte('\n')
	b.WriteString(shared.KVRow("status", string(a.Status), keyWidth))
	b.WriteByte('\n')
	if !a.Created.IsZero() {
		b.WriteString(shared.KVRow("created", a.Created.UTC().Format(time.RFC3339), keyWidth))
		b.WriteByte('\n')
	}
	if !a.LastUpdated.IsZero() {
		b.WriteString(shared.KVRow("lastUpdated", a.LastUpdated.UTC().Format(time.RFC3339), keyWidth))
		b.WriteByte('\n')
	}
	return b.String()
}

func renderAppJSONTab(a domain.App) string {
	body := prettyAppJSON(a)
	return shared.HighlightJSON(body, activeTokens()) + "\n"
}

func renderAppYAMLTab(a domain.App) string {
	body := prettyAppJSON(a)
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
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

func prettyAppJSON(a domain.App) string {
	if len(a.Raw) > 0 {
		var v any
		if err := json.Unmarshal(a.Raw, &v); err == nil {
			if buf, err := json.MarshalIndent(v, "", "  "); err == nil {
				return string(buf)
			}
		}
	}
	curated := map[string]any{
		"id":          a.ID,
		"name":        a.Name,
		"label":       a.Label,
		"status":      string(a.Status),
		"signOnMode":  a.SignOnMode,
		"type":        string(a.Type),
		"created":     formatJSONTime(a.Created),
		"lastUpdated": formatJSONTime(a.LastUpdated),
	}
	buf, err := json.MarshalIndent(curated, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(buf)
}

func formatJSONTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// --- Cmd factories -----------------------------------------------------------

func fetchAppsCmd(port domain.AppsPort, t domain.AppType) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		iter, err := port.List(ctx, domain.AppsQuery{Type: t, Limit: 200})
		if err != nil {
			return appsErrMsg{err: err}
		}
		defer iter.Close()
		var out []domain.App
		for {
			a, hasMore, err := iter.Next(ctx)
			if err != nil {
				return appsErrMsg{err: err}
			}
			if !hasMore {
				break
			}
			out = append(out, a)
		}
		return appsLoadedMsg{apps: out}
	}
}

var (
	_ tea.Model = TypeSelectModel{}
	_ tea.Model = ListModel{}
)
