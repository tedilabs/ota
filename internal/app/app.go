package app

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/tui/groups"
	"github.com/tedilabs/ota/internal/tui/logs"
	"github.com/tedilabs/ota/internal/tui/overlay"
	"github.com/tedilabs/ota/internal/tui/policies"
	"github.com/tedilabs/ota/internal/tui/rules"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/users"
	"github.com/tedilabs/ota/internal/version"
)

// Screen identifies the active resource screen (TUI_DESIGN §2.2).
type Screen int

const (
	ScreenUsers Screen = iota
	ScreenGroups
	ScreenRules
	ScreenPolicies
	ScreenLogs
	ScreenUserDetail
	ScreenGroupDetail
	ScreenRuleDetail
	ScreenPolicyDetail
	ScreenLogDetail
)

// String returns the screen's `:command` form (or a detail-view label).
func (s Screen) String() string {
	switch s {
	case ScreenUsers:
		return "users"
	case ScreenGroups:
		return "groups"
	case ScreenRules:
		return "grouprules"
	case ScreenPolicies:
		return "policies"
	case ScreenLogs:
		return "logs"
	case ScreenUserDetail:
		return "user-detail"
	case ScreenGroupDetail:
		return "group-detail"
	case ScreenRuleDetail:
		return "rule-detail"
	case ScreenPolicyDetail:
		return "policy-detail"
	case ScreenLogDetail:
		return "log-detail"
	}
	return "unknown"
}

// ActiveScreenName returns the active screen's label — exported helper so
// tests can assert against the router state without reaching into internals.
func ActiveScreenName(m Model) string { return m.active.String() }

// Overlay identifies the active overlay, if any.
type Overlay int

const (
	OverlayNone Overlay = iota
	OverlayPalette
	OverlayHelp
	OverlayQuitConfirm
	// OverlayActionConfirm — destructive Users lifecycle action gate
	// (issue #125). Holds m.pendingAction until y / Esc / n.
	OverlayActionConfirm
)

// UserActionKind classifies the Users lifecycle action a confirmation
// modal is gating. Matches the three ports added in issue #125 —
// password reset, unlock, MFA factor reset.
type UserActionKind int

const (
	UserActionNone UserActionKind = iota
	UserActionResetPassword
	UserActionUnlock
	UserActionResetFactors
)

// pendingUserAction is the (kind, target) pair the App Shell keeps in
// flight while OverlayActionConfirm is open. Reset back to its zero
// value when the operator confirms or cancels.
type pendingUserAction struct {
	Kind UserActionKind
	User domain.User
}

// SelectedUserStater is implemented by screens that can surface the
// "currently active" user — Users list cursor row, or the open detail
// target. The App Shell reads from it to populate the lifecycle
// confirmation modal (issue #125).
type SelectedUserStater interface {
	SelectedUser() (domain.User, bool)
}

// Deps bundles the App Shell's runtime dependencies.
type Deps struct {
	Services  *service.Bundle
	RateLimit domain.RateLimitPort
	Health    domain.HealthPort
	Keys      keys.ResolvedMap
	Clock     clock.Clock
	Logger    *slog.Logger
	Profile   string
	// OrgURL is the active tenant's URL ("https://acme.okta.com"). Surfaced
	// in the chrome's TitleBar (TUI_DESIGN §15.1).
	OrgURL string

	// Ports are injected raw so child Screen Models can be constructed by
	// the App Shell without exporting fields from the service Bundle.
	UsersPort      domain.UsersPort
	GroupsPort     domain.GroupsPort
	GroupRulesPort domain.GroupRulesPort
	PoliciesPort   domain.PoliciesPort
	LogsPort       domain.LogsPort

	// Optional initial state for tests / direct embedding.
	InitialScreen Screen
}

// Model is ota's top-level tea.Model (App Shell). Hosts the active screen,
// overlays (cmd palette, help, ...), and the global status bar.
type Model struct {
	deps    Deps
	active  Screen
	overlay Overlay

	// screens lazy-caches child Screen Models keyed by Screen enum. Built on
	// demand by ensureScreen so unused screens never allocate.
	screens map[Screen]tea.Model

	// paletteInput captures the `:` prompt buffer while OverlayPalette is open.
	paletteInput string
	// paletteSuggestionIdx points at the currently-highlighted entry in
	// paletteSuggestions(). -1 means "no selection" (Enter applies the
	// raw input). Tab / Shift-Tab cycle the index.
	paletteSuggestionIdx int

	// offline flags the transient statusbar state after a NetworkErrorMsg.
	offline bool

	// helpModel is the screen-aware Help overlay. Non-nil only while
	// overlay == OverlayHelp; instantiated by openHelpMsg using the active
	// screen so `?` shows the keys that actually do something here.
	helpModel *overlay.HelpModel

	// principalLogin is the authenticated Okta user (issue #124). Empty
	// until the /api/v1/users/me probe completes; once populated it
	// renders next to the profile label in the chrome ContextBar.
	principalLogin string
	// principalRequested guards the one-shot /me probe so we never fire
	// it more than once per session (the call is rate-limit-priced and
	// the chrome only needs a single answer).
	principalRequested bool

	// pendingAction holds the Users lifecycle action (issue #125) the
	// operator just queued through `:reset-password` / `:unlock` /
	// `:reset-factors` while the confirmation modal is open. Cleared
	// when the modal closes either way.
	pendingAction pendingUserAction

	// width / height track the current terminal size, updated via
	// tea.WindowSizeMsg. The chrome renders 100% to width (TUI_DESIGN
	// §15.0a v1.2.0): widths >= 80 pass through unchanged so wide
	// terminals fill edge-to-edge; widths < 80 clamp to 80 and rely on
	// the renderer's truncation to keep the layout intact; width == 0
	// (no WindowSizeMsg yet) falls back to shared.ChromeWidth.
	width  int
	height int
}

// New constructs the App Shell. The initial screen is materialized eagerly
// so Init() can return its first Cmd directly.
func New(deps Deps) Model {
	m := Model{
		deps:    deps,
		active:  deps.InitialScreen,
		overlay: OverlayNone,
		screens: map[Screen]tea.Model{},
	}
	if mdl, _ := m.buildScreen(m.active); mdl != nil {
		m.screens[m.active] = mdl
	}
	return m
}

// Init implements tea.Model. Returns the active child's Init Cmd so its
// first fetch (e.g., fetchUsersCmd) starts immediately. The /me probe
// for the chrome's principal slot is kicked off lazily from Update on
// the first incoming message — keeping Init a single Cmd so the App
// Shell stays trivially testable via `init()` → `Update(msg)` round
// trips (no tea.Batch fan-out required).
func (m Model) Init() tea.Cmd {
	if child, ok := m.screens[m.active]; ok {
		return child.Init()
	}
	return nil
}

// kickPrincipalFetch returns the /me probe Cmd the first time it's
// invoked (mutating m so the caller persists the latch flag) and nil
// thereafter. Returns nil when no UsersPort is wired so chrome-only
// tests stay free of background fetches.
func (m *Model) kickPrincipalFetch() tea.Cmd {
	if m.principalRequested || m.deps.UsersPort == nil {
		return nil
	}
	m.principalRequested = true
	return fetchPrincipalCmd(m.deps.UsersPort)
}

// principalLoadedMsg carries the authenticated principal's login back
// into Update so the chrome ContextBar can render it.
type principalLoadedMsg struct{ Login string }

// fetchPrincipalCmd issues GET /api/v1/users/me through the existing
// UsersPort.Get path — Okta accepts "me" as an alias for the token's
// owner — and converts the result into principalLoadedMsg. Failures are
// silenced (the chrome simply omits the principal segment).
func fetchPrincipalCmd(port domain.UsersPort) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		u, err := port.Get(ctx, "me")
		if err != nil {
			return principalLoadedMsg{}
		}
		login := u.Profile.Login
		if login == "" {
			login = u.ID
		}
		return principalLoadedMsg{Login: login}
	}
}

// Update implements tea.Model. Handles global shortcuts (`:`, `?`, Ctrl-c),
// palette input, broadcasts toast/offline/refresh messages, and delegates
// non-routing messages to the active child screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to child Screen Models so they can adapt their column
		// widths / drop rules to the new terminal size. Sized child models
		// re-render with the right number of cells on the next View() call.
		for s, child := range m.screens {
			updated, _ := child.Update(msg)
			m.screens[s] = updated
		}
		// Bubbletea always emits an initial WindowSizeMsg at startup so
		// this is the natural place to kick off the one-shot /me probe
		// (issue #124). Returning the Cmd here keeps Init free of
		// tea.Batch, which keeps the App Shell test helpers
		// (`init() → cmd() → Update(msg)`) trivially correct.
		return m, m.kickPrincipalFetch()
	case tea.KeyMsg:
		return m.handleKey(msg)
	case ErrorMsg:
		return m, toastCmdError(msg)
	case NetworkErrorMsg:
		m.offline = true
		return m, offlineCmd(true)
	case NetworkRestoredMsg:
		m.offline = false
		return m, refreshActiveCmd()
	case OfflineStateMsg:
		m.offline = msg.Offline
		return m, nil
	case principalLoadedMsg:
		m.principalLogin = msg.Login
		return m, nil
	case openCmdPaletteMsg:
		m.overlay = OverlayPalette
		m.paletteInput = ""
		m.paletteSuggestionIdx = -1
		return m, nil
	case openHelpMsg:
		m.overlay = OverlayHelp
		h := overlay.NewHelpModelFor(m.active.String())
		m.helpModel = &h
		return m, nil
	case QuitConfirmRequestMsg:
		m.overlay = OverlayQuitConfirm
		return m, nil
	case ScreenChangeMsg:
		m.active = msg.Target
		m.overlay = OverlayNone
		m.paletteInput = ""
		updated, cmd := m.ensureScreen(m.active)
		return updated, cmd
	case SwitchScreenMsg:
		if s, ok := screenFromName(msg.Target); ok {
			m.active = s
			m.overlay = OverlayNone
			m.paletteInput = ""
			updated, cmd := m.ensureScreen(m.active)
			return updated, cmd
		}
		return m, nil
	case OpenResourceMsg:
		if s, ok := detailScreenFor(msg.Kind); ok {
			m.active = s
			m.overlay = OverlayNone
			updated, cmd := m.ensureScreen(m.active)
			return updated, cmd
		}
		return m, nil
	}
	// Non-routing messages: delegate to the active child screen so background
	// fetch results (usersLoadedMsg, etc.) reach the right Model.
	if m.overlay == OverlayNone {
		if child, ok := m.screens[m.active]; ok {
			updatedChild, cmd := child.Update(msg)
			m.screens[m.active] = updatedChild
			return m, cmd
		}
	}
	return m, nil
}

// screenFromName maps a palette / SwitchScreenMsg screen name to the
// Screen enum. TUI_DESIGN §3.4 v1.2.0 locks in singular, plural,
// hyphenated, and underscored variants alongside the k9s-style short
// codes (u/g/gr/l).
func screenFromName(name string) (Screen, bool) {
	switch strings.ToLower(name) {
	case "user", "users", "u":
		return ScreenUsers, true
	case "group", "groups", "g":
		return ScreenGroups, true
	case "rule", "rules",
		"grouprule", "grouprules",
		"group-rule", "group-rules",
		"group_rule", "group_rules",
		"gr":
		return ScreenRules, true
	case "policy", "policies":
		return ScreenPolicies, true
	case "log", "logs", "l":
		return ScreenLogs, true
	}
	return 0, false
}

// detailScreenFor maps OpenResourceMsg.Kind → detail Screen.
func detailScreenFor(kind string) (Screen, bool) {
	switch strings.ToLower(kind) {
	case "user":
		return ScreenUserDetail, true
	case "group":
		return ScreenGroupDetail, true
	case "rule", "grouprule":
		return ScreenRuleDetail, true
	case "policy":
		return ScreenPolicyDetail, true
	case "log", "logevent":
		return ScreenLogDetail, true
	}
	return 0, false
}

// View implements tea.Model. Composes the global 3-zone chrome (TitleBar /
// ContextBar / MainBody / KeyHints) around the active child Screen's body.
// Overlays render as small panels appended below the body within the box.
func (m Model) View() string {
	tokens := activeTokens()

	body := m.composeBody()
	// Floating palette: when : is open the input box renders ABOVE the
	// body — k9s-style, list still visible behind it (issue #123). Help
	// still owns the full body via composeBody. QuitConfirm + other
	// small overlays continue to append below the body.
	switch {
	case m.overlay == OverlayPalette:
		body = m.renderPaletteBox(tokens) + "\n" + body
	case m.activeChildIsFiltering():
		body = m.renderFilterBox(tokens) + "\n" + body
		fallthrough
	default:
		if m.overlay != OverlayNone {
			if overlay := m.renderOverlayPanel(tokens); overlay != "" {
				if body != "" {
					body = body + "\n" + overlay
				} else {
					body = overlay
				}
			}
		}
	}

	width := clampWidth(m.width)
	bodyLines := clampBodyLines(m.height)
	chrome := shared.ChromeInput{
		Tokens:    tokens,
		Width:     width,
		Brand:     "ota",
		Tenant:    tenantFromOrgURL(m.deps.OrgURL),
		Profile:   m.profileLabel(),
		Principal: m.principalLogin,
		Version:   version.Tag,
		Timezone:  "UTC",
		RateLimit: m.rateLimitState(),
		Resource:  m.resourceLabel(),
		Filter:    m.activeChildFilter(),
		Body:      body,
		BodyLines: bodyLines,
		KeyHints:  m.keyHints(tokens),
		Offline:   m.offline,
	}
	return shared.RenderChrome(chrome)
}

// clampWidth maps a terminal width to the chrome render width.
//
//   - w == 0 (no WindowSizeMsg yet): use shared.ChromeWidth as a fallback so
//     the first frame draws something sensible before tea reports the size.
//   - w < 80: clamp to 80 (the §1.2 minimum supported terminal). The
//     "ota requires minimum 80x24 terminal" branch is handled higher up by
//     the boot screen; once the chrome is engaged we just don't render
//     narrower than 80 cells.
//   - w >= 80: pass through unchanged. The v1.0/v1.1 cap of 200 was dropped
//     in v0.1.1 (§15.0a v1.2.0) so 100% terminal fill works on wide
//     monitors (160, 180, 220, 240+).
func clampWidth(w int) int {
	if w <= 0 {
		return shared.ChromeWidth
	}
	if w < 80 {
		return 80
	}
	return w
}

// clampBodyLines maps a terminal height to a reasonable body-row count.
// The chrome reserves 6 rows (issue #133, k9s-style header): top
// border, TitleBar, upper divider (with embedded resource label),
// status divider, KeyHints, bottom border. The previous ContextBar
// row collapsed into the upper divider so the body now gets +1 row.
func clampBodyLines(h int) int {
	const reserved = 6
	if h <= 0 {
		return 16
	}
	rows := h - reserved
	if rows < 5 {
		return 5
	}
	if rows > 60 {
		return 60
	}
	return rows
}

// composeBody returns the active child Screen's View output, or a loading
// placeholder when no child is registered.
//
// When the Help overlay is open, its modal box replaces the body content so
// the user sees the actual key reference instead of just a footer hint.
// renderOverlayPanel handles the smaller Palette / QuitConfirm overlays
// which compose alongside the screen.
func (m Model) composeBody() string {
	if m.overlay == OverlayHelp && m.helpModel != nil {
		return m.helpModel.View()
	}
	if child, ok := m.screens[m.active]; ok {
		return child.View()
	}
	return "(loading…)"
}

// resourceLabel returns the resource segment shown in the ContextBar.
func (m Model) resourceLabel() string {
	switch m.active {
	case ScreenUsers:
		return "Users"
	case ScreenGroups:
		return "Groups"
	case ScreenRules:
		return "Group Rules"
	case ScreenPolicies:
		return "Policies"
	case ScreenLogs:
		return "Logs"
	case ScreenUserDetail:
		return "Users › detail"
	case ScreenGroupDetail:
		return "Groups › detail"
	case ScreenRuleDetail:
		return "Group Rules › detail"
	case ScreenPolicyDetail:
		return "Policies › detail"
	case ScreenLogDetail:
		return "Logs › detail"
	}
	return m.active.String()
}


// profileLabel returns the env classifier surfaced in the TitleBar.
func (m Model) profileLabel() string {
	return m.deps.Profile
}

// rateLimitState classifies the [RL: ...] badge from the injected port.
// nil port → ok (chrome stays informative even before wiring is complete).
func (m Model) rateLimitState() shared.RateLimitState {
	if m.deps.RateLimit == nil {
		return shared.RateLimitOK
	}
	snaps := m.deps.RateLimit.Snapshots()
	if len(snaps) == 0 {
		return shared.RateLimitOK
	}
	worst := shared.RateLimitOK
	for _, s := range snaps {
		switch {
		case s.Limit > 0 && s.Remaining == 0:
			return shared.RateLimitLimited
		case s.Limit > 0 && float64(s.Remaining)/float64(s.Limit) < 0.2:
			worst = shared.RateLimitWarn
		}
	}
	return worst
}

// keyHints returns the global key palette row, styled per TUI_DESIGN §15.1.
// Keys are highlighted with the Accent token; labels with Muted.
func (m Model) keyHints(tk shared.Tokens) string {
	pairs := []struct{ key, label string }{
		{"<:>", "cmd"},
		{"</>", "search"},
		{"<?>", "help"},
		{"<g>", "top"},
		{"<G>", "bottom"},
		{"<j/k>", "nav"},
		{"<q>", "close"},
	}
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, tk.Accent.Render(p.key)+" "+tk.Muted.Render(p.label))
	}
	return " " + strings.Join(parts, "  ")
}

// renderOverlayPanel composes a tiny footer panel for the active overlay.
// The polished overlay surface (modal box, suggestions, etc.) lives in
// internal/tui/overlay; this is a fallback when the App Shell renders
// overlays directly. Phase 6d-5 will replace this with proper modal models.
func (m Model) renderOverlayPanel(tk shared.Tokens) string {
	switch m.overlay {
	case OverlayPalette:
		// The palette is rendered as a floating box above the body in
		// View(); the footer panel is left empty so it doesn't double up.
		return ""
	case OverlayHelp:
		// Body composition has already replaced the screen with the modal.
		return ""
	case OverlayQuitConfirm:
		return tk.Danger.Render("Quit ota?") + tk.Muted.Render("  (y/N)")
	case OverlayActionConfirm:
		label := userActionLabel(m.pendingAction.Kind)
		login := m.pendingAction.User.Profile.Login
		if login == "" {
			login = m.pendingAction.User.ID
		}
		return tk.Danger.Render(label+" for ") + tk.Accent.Render(login) +
			tk.Danger.Render("?") + tk.Muted.Render("  (y/N)")
	}
	return ""
}

// FilterStater is implemented by list screens that own a `/` incremental
// filter so the App Shell can render the same floating input box used
// for the palette (issue #123).
type FilterStater interface {
	Filtering() bool
	Filter() string
}

// activeChildIsFiltering reports whether the active list child is
// currently in `/` filter input mode.
func (m Model) activeChildIsFiltering() bool {
	child, ok := m.screens[m.active]
	if !ok {
		return false
	}
	fs, ok := child.(FilterStater)
	return ok && fs.Filtering()
}

// activeChildFilter returns the active child's applied filter string
// — empty when no filter is set or the screen doesn't support filters.
// Surfaced in the chrome's upper divider so operators always see what's
// narrowing the visible row set, even after the `/` prompt closes.
func (m Model) activeChildFilter() string {
	child, ok := m.screens[m.active]
	if !ok {
		return ""
	}
	fs, ok := child.(FilterStater)
	if !ok {
		return ""
	}
	return fs.Filter()
}

// renderFilterBox builds the floating input box for `/` filter mode.
// Sibling of renderPaletteBox; same chrome, different prompt and hint.
func (m Model) renderFilterBox(tk shared.Tokens) string {
	const innerWidth = 60
	prompt := tk.Accent.Render("/")
	input := ""
	if child, ok := m.screens[m.active]; ok {
		if fs, ok := child.(FilterStater); ok {
			input = fs.Filter()
		}
	}
	cursor := tk.RowHighlight.Render(" ")
	body := prompt + input + cursor
	hint := tk.Muted.Render("<Enter> apply · <Esc> cancel")
	return lipglossModalBox(body, hint, innerWidth, tk)
}

// renderPaletteBox builds the small framed input box that appears above
// the body when : is open (issue #123). v0.1.5-4 added an inline
// suggestion column under the prompt — Tab cycles, Enter accepts the
// highlight (or the raw input when nothing is highlighted).
func (m Model) renderPaletteBox(tk shared.Tokens) string {
	const innerWidth = 60
	prompt := tk.Accent.Render(":")
	input := m.paletteInput
	cursor := tk.RowHighlight.Render(" ")
	body := prompt + input + cursor

	var b strings.Builder
	b.WriteString(body)
	if sugs := m.paletteSuggestions(); len(sugs) > 0 {
		b.WriteByte('\n')
		max := 6
		if max > len(sugs) {
			max = len(sugs)
		}
		for i := 0; i < max; i++ {
			line := sugs[i]
			if i == m.paletteSuggestionIdx {
				line = tk.RowHighlight.Render("› " + line)
			} else {
				line = tk.Muted.Render("  " + line)
			}
			b.WriteString(line)
			if i < max-1 {
				b.WriteByte('\n')
			}
		}
		if len(sugs) > max {
			b.WriteByte('\n')
			b.WriteString(tk.Muted.Render("  … " + itoaSimple(len(sugs)-max) + " more"))
		}
	}
	hint := tk.Muted.Render("<Tab>/<Shift-Tab> cycle · <Enter> run · <Esc> cancel")
	return lipglossModalBox(b.String(), hint, innerWidth, tk)
}

// itoaSimple is a tiny strconv shim local to app/ so renderPaletteBox
// avoids pulling strconv into a file that doesn't otherwise need it.
func itoaSimple(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// lipglossModalBox renders a small two-line box with body on the first
// line and hint on the second, using lipgloss.RoundedBorder. Centralised
// here so the palette input shares chrome with the future filter prompt.
func lipglossModalBox(body, hint string, innerWidth int, tk shared.Tokens) string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5e81ac")).
		Padding(0, 1).
		Width(innerWidth)
	return border.Render(body + "\n" + hint)
}

// tenantFromOrgURL extracts the host segment from a tenant URL. Handles the
// common forms ("https://acme.okta.com", "acme.okta.com") and falls back to
// the empty string when parsing fails so the chrome simply omits the tenant
// segment rather than crashing.
func tenantFromOrgURL(orgURL string) string {
	if orgURL == "" {
		return ""
	}
	if !strings.Contains(orgURL, "://") {
		return strings.TrimSpace(orgURL)
	}
	parsed, err := url.Parse(orgURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}

// activeTokens picks the token set based on the NO_COLOR env var. Called per
// View() so a runtime toggle (e.g., test pinning) takes effect immediately.
func activeTokens() shared.Tokens {
	if shared.MonochromeEnabled() {
		return shared.Monochrome()
	}
	return shared.Dark()
}

// Active reports the active resource screen (useful for tests / wiring).
func (m Model) Active() Screen { return m.active }

// Overlay reports the active overlay, if any.
func (m Model) Overlay() Overlay { return m.overlay }

// HasScreen reports whether a child Screen Model has been materialized for s.
// Useful for tests of lazy-init behaviour.
func (m Model) HasScreen(s Screen) bool { _, ok := m.screens[s]; return ok }

// ensureScreen builds the active Screen Model if not yet cached and returns
// a Model copy with the new screen + the screen's Init Cmd. When buildScreen
// returns nil (e.g., detail view that hasn't been opened yet), the Model is
// returned unchanged.
func (m Model) ensureScreen(s Screen) (Model, tea.Cmd) {
	if _, ok := m.screens[s]; ok {
		return m, nil
	}
	mdl, cmd := m.buildScreen(s)
	if mdl == nil {
		return m, nil
	}
	if m.screens == nil {
		m.screens = map[Screen]tea.Model{}
	}
	m.screens[s] = mdl
	return m, cmd
}

// buildScreen instantiates a child Screen Model with Deps resolved from the
// App Shell. Width / Height are forwarded so the freshly-built screen
// already knows the terminal size on its first frame — without this the
// chrome's top border scrolls off the moment the operator switches to a
// resource via :cmd (issue #113).
func (m Model) buildScreen(s Screen) (tea.Model, tea.Cmd) {
	switch s {
	case ScreenUsers:
		mdl := users.NewListModel(users.Deps{
			Port:   m.deps.UsersPort,
			Clock:  m.deps.Clock,
			Logger: m.deps.Logger,
			Keys:   m.deps.Keys,
			Width:  m.width,
			Height: m.height,
		})
		return mdl, mdl.Init()
	case ScreenGroups:
		mdl := groups.NewListModel(groups.Deps{
			Port:   m.deps.GroupsPort,
			Clock:  m.deps.Clock,
			Logger: m.deps.Logger,
			Keys:   m.deps.Keys,
			Width:  m.width,
			Height: m.height,
		})
		return mdl, mdl.Init()
	case ScreenRules:
		mdl := rules.NewListModel(rules.Deps{
			Port:   m.deps.GroupRulesPort,
			Groups: m.deps.GroupsPort,
			Clock:  m.deps.Clock,
			Logger: m.deps.Logger,
			Keys:   m.deps.Keys,
			Width:  m.width,
			Height: m.height,
		})
		return mdl, mdl.Init()
	case ScreenPolicies:
		mdl := policies.NewTypeSelectModel(policies.Deps{
			Port:   m.deps.PoliciesPort,
			Clock:  m.deps.Clock,
			Logger: m.deps.Logger,
			Keys:   m.deps.Keys,
			Width:  m.width,
			Height: m.height,
		})
		return mdl, mdl.Init()
	case ScreenLogs:
		var svc *service.LogsService
		var tail *service.LogsTail
		if m.deps.Services != nil {
			svc = m.deps.Services.Logs
			tail = m.deps.Services.LogsTail
		}
		mdl := logs.NewSearchModel(logs.Deps{
			Service: svc,
			Tail:    tail,
			Clock:   m.deps.Clock,
			Logger:  m.deps.Logger,
			Keys:    m.deps.Keys,
			Width:   m.width,
			Height:  m.height,
		})
		return mdl, mdl.Init()
	}
	// Detail views are populated by drill-down handlers; not auto-built.
	return nil, nil
}

// --- Key handling --------------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Palette has input focus while open.
	if m.overlay == OverlayPalette {
		return m.handlePaletteKey(msg)
	}
	if m.overlay == OverlayHelp || m.overlay == OverlayQuitConfirm || m.overlay == OverlayActionConfirm {
		return m.handleOverlayKey(msg)
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, quitConfirmCmd()
	// NOTE: Esc is intentionally NOT handled here. Each child Screen owns
	// what Esc means in its current context — closing detail mode, ending
	// `/` filtering, exiting visual selection, etc. The bottom delegation
	// block forwards it to the active child.
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case ":":
			return m, openCmdPaletteCmd()
		case "?":
			return m, openHelpCmd()
		case "q":
			return m, quitConfirmCmd()
		}
	}
	// Any key the App Shell didn't claim (j/k navigation, Shift+S sort,
	// d / Enter detail, /, Backspace, ...) belongs to the active child
	// Screen. Forward and persist its updated state, otherwise list-level
	// shortcuts silently no-op (the user reported this for `Shift+S`).
	if child, ok := m.screens[m.active]; ok {
		updated, cmd := child.Update(msg)
		m.screens[m.active] = updated
		return m, cmd
	}
	return m, nil
}

func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = OverlayNone
		m.paletteInput = ""
		m.paletteSuggestionIdx = -1
		return m, nil
	case tea.KeyTab:
		// Cycle forward through the suggestion list (issue #121).
		sugs := m.paletteSuggestions()
		if len(sugs) > 0 {
			m.paletteSuggestionIdx = (m.paletteSuggestionIdx + 1) % len(sugs)
		}
		return m, nil
	case tea.KeyShiftTab:
		sugs := m.paletteSuggestions()
		if len(sugs) > 0 {
			if m.paletteSuggestionIdx <= 0 {
				m.paletteSuggestionIdx = len(sugs) - 1
			} else {
				m.paletteSuggestionIdx--
			}
		}
		return m, nil
	case tea.KeyEnter:
		// If a suggestion is highlighted, treat that as the input —
		// operator typed `:us` + Tab + Enter and lands on `:users`.
		input := m.paletteInput
		if m.paletteSuggestionIdx >= 0 {
			if sugs := m.paletteSuggestions(); m.paletteSuggestionIdx < len(sugs) {
				input = sugs[m.paletteSuggestionIdx]
			}
		}
		cmd, target, arg, ok := resolvePaletteCommand(input)
		m.overlay = OverlayNone
		m.paletteInput = ""
		m.paletteSuggestionIdx = -1
		if !ok {
			return m, nil
		}
		switch cmd {
		case paletteCmdScreen:
			return m, screenChangeCmd(target)
		case paletteCmdQuit:
			return m, quitConfirmCmd()
		case paletteCmdUnmask:
			if child, ok := m.screens[m.active]; ok {
				updated, c := child.Update(UnmaskFieldMsg{Field: arg})
				m.screens[m.active] = updated
				return m, c
			}
		case paletteCmdMask:
			if child, ok := m.screens[m.active]; ok {
				updated, c := child.Update(MaskAllMsg{})
				m.screens[m.active] = updated
				return m, c
			}
		case paletteCmdResetPassword:
			return m.openActionConfirm(UserActionResetPassword)
		case paletteCmdUnlock:
			return m.openActionConfirm(UserActionUnlock)
		case paletteCmdResetFactors:
			return m.openActionConfirm(UserActionResetFactors)
		}
		return m, nil
	case tea.KeyBackspace:
		if n := len(m.paletteInput); n > 0 {
			m.paletteInput = m.paletteInput[:n-1]
		}
		m.paletteSuggestionIdx = -1
		return m, nil
	case tea.KeyRunes:
		m.paletteInput += string(msg.Runes)
		m.paletteSuggestionIdx = -1
		return m, nil
	}
	return m, nil
}

// paletteSuggestions returns the autocomplete candidates for the current
// paletteInput (issue #121). A 2-character prefix unlocks the list so a
// typo on the first letter doesn't immediately constrain the operator.
// Suggestions are de-duplicated and sorted alphabetically.
func (m Model) paletteSuggestions() []string {
	prefix := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(m.paletteInput), ":"))
	if len(prefix) < 2 {
		return nil
	}
	all := paletteCommandPool()
	out := make([]string, 0, 8)
	for _, c := range all {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// paletteCommandPool returns every literal command name the palette
// recognises today. Kept inline rather than reflected from screenFromName
// so we can include verbs (`unmask`, `mask`, `quit`) that screenFromName
// itself doesn't enumerate.
func paletteCommandPool() []string {
	return []string{
		"users", "user", "u",
		"groups", "group", "g",
		"rules", "rule",
		"grouprules", "grouprule",
		"group-rules", "group-rule",
		"group_rules", "group_rule", "gr",
		"policies", "policy",
		"logs", "log", "l",
		"unmask", "mask",
		"reset-password", "unlock", "reset-mfa",
		"quit", "exit", "q",
	}
}

func (m Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay: Esc closes, every other key is forwarded to the
	// HelpModel so its internal `/` filter and Tab cycling work.
	if m.overlay == OverlayHelp {
		if msg.Type == tea.KeyEsc {
			m.overlay = OverlayNone
			m.helpModel = nil
			return m, nil
		}
		// `?` toggles closed too, matching the README cheat sheet.
		if msg.Type == tea.KeyRunes && string(msg.Runes) == "?" {
			m.overlay = OverlayNone
			m.helpModel = nil
			return m, nil
		}
		if m.helpModel != nil {
			updated, cmd := m.helpModel.Update(msg)
			if hm, ok := updated.(overlay.HelpModel); ok {
				m.helpModel = &hm
			}
			return m, cmd
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = OverlayNone
		return m, nil
	}
	if m.overlay == OverlayQuitConfirm && msg.Type == tea.KeyRunes {
		switch string(msg.Runes) {
		case "y", "Y":
			return m, tea.Quit
		case "n", "N":
			m.overlay = OverlayNone
		}
	}
	if m.overlay == OverlayActionConfirm && msg.Type == tea.KeyRunes {
		switch string(msg.Runes) {
		case "y", "Y":
			action := m.pendingAction
			m.pendingAction = pendingUserAction{}
			m.overlay = OverlayNone
			return m, runUserActionCmd(m.deps.UsersPort, action)
		case "n", "N":
			m.pendingAction = pendingUserAction{}
			m.overlay = OverlayNone
		}
	}
	return m, nil
}

// --- Palette resolver ----------------------------------------------------

type paletteCmdKind int

const (
	paletteCmdNone paletteCmdKind = iota
	paletteCmdScreen
	paletteCmdQuit
	paletteCmdUnmask
	paletteCmdMask
	// Users lifecycle actions (issue #125). Each opens a confirmation
	// modal targeting the active screen's selected user.
	paletteCmdResetPassword
	paletteCmdUnlock
	paletteCmdResetFactors
)

// UnmaskFieldMsg / MaskAllMsg are re-exported from the shared msgs
// package so callers that only depend on `internal/app` (e.g. tests)
// can still reference them by their App Shell name.
type (
	UnmaskFieldMsg = shared.UnmaskFieldMsg
	MaskAllMsg     = shared.MaskAllMsg
)

// resolvePaletteCommand parses the palette input ("users", ":users",
// "groups", ":q", "unmask mobilePhone", ...) into a kind. Most commands
// resolve to a screen via screenFromName so SwitchScreenMsg and `:` stay
// in sync (TUI_DESIGN §3.4 v1.2.0). PII unmask / mask commands route
// through their own kinds so the App Shell can fan them out to the
// active detail model.
func resolvePaletteCommand(raw string) (kind paletteCmdKind, screen Screen, arg string, ok bool) {
	cmd := strings.TrimSpace(raw)
	cmd = strings.TrimPrefix(cmd, ":")
	// Preserve case-sensitive args for `:unmask <field>` (Okta profile
	// keys are camelCase) but lowercase the verb for matching.
	verb, rest, hasArg := strings.Cut(cmd, " ")
	verb = strings.ToLower(verb)
	rest = strings.TrimSpace(rest)

	switch verb {
	case "quit", "q", "exit":
		return paletteCmdQuit, 0, "", true
	case "mask":
		return paletteCmdMask, 0, "", true
	case "unmask":
		if !hasArg || rest == "" {
			return paletteCmdNone, 0, "", false
		}
		return paletteCmdUnmask, 0, rest, true
	case "reset-password", "reset_password", "resetpassword":
		return paletteCmdResetPassword, 0, "", true
	case "unlock":
		return paletteCmdUnlock, 0, "", true
	case "reset-mfa", "reset-factors", "reset_mfa", "reset_factors", "resetfactors":
		return paletteCmdResetFactors, 0, "", true
	}
	if s, found := screenFromName(strings.ToLower(cmd)); found {
		return paletteCmdScreen, s, "", true
	}
	// `:policies OKTA_SIGN_ON` direct-jump variant.
	if strings.HasPrefix(strings.ToLower(cmd), "policies ") {
		return paletteCmdScreen, ScreenPolicies, "", true
	}
	return paletteCmdNone, 0, "", false
}

// openActionConfirm queues a Users lifecycle action (issue #125) for
// confirmation. Looks up the active screen's selected user, falls back
// to a transient toast when none is available (e.g., the operator
// fired the command from a non-Users screen).
func (m Model) openActionConfirm(kind UserActionKind) (tea.Model, tea.Cmd) {
	child, ok := m.screens[m.active]
	if !ok {
		return m, toastCmdInfo("no active screen")
	}
	stater, ok := child.(SelectedUserStater)
	if !ok {
		return m, toastCmdInfo("action not available on this screen")
	}
	user, ok := stater.SelectedUser()
	if !ok {
		return m, toastCmdInfo("no user selected")
	}
	m.pendingAction = pendingUserAction{Kind: kind, User: user}
	m.overlay = OverlayActionConfirm
	return m, nil
}

// userActionLabel returns a human-readable label for the action kind,
// rendered in the confirmation modal and the post-action toast.
func userActionLabel(k UserActionKind) string {
	switch k {
	case UserActionResetPassword:
		return "Reset password"
	case UserActionUnlock:
		return "Unlock account"
	case UserActionResetFactors:
		return "Reset MFA factors"
	}
	return ""
}

// runUserActionCmd dispatches the active pendingAction against the
// UsersPort and emits a toast with the result. The Cmd returns nil
// when called without a wired UsersPort so tests can drive the flow
// without a network round-trip.
func runUserActionCmd(port domain.UsersPort, action pendingUserAction) tea.Cmd {
	if port == nil {
		return toastCmdInfo("UsersPort not wired — action skipped")
	}
	return func() tea.Msg {
		ctx := context.Background()
		login := action.User.Profile.Login
		if login == "" {
			login = action.User.ID
		}
		switch action.Kind {
		case UserActionResetPassword:
			if _, err := port.ResetPassword(ctx, action.User.ID, true); err != nil {
				return toastErr("reset password failed: " + err.Error())
			}
			return toastInfo("reset password email sent to " + login)
		case UserActionUnlock:
			if err := port.Unlock(ctx, action.User.ID); err != nil {
				return toastErr("unlock failed: " + err.Error())
			}
			return toastInfo("unlocked " + login)
		case UserActionResetFactors:
			if err := port.ResetFactors(ctx, action.User.ID); err != nil {
				return toastErr("reset MFA failed: " + err.Error())
			}
			return toastInfo("MFA factors reset for " + login)
		}
		return nil
	}
}

func toastInfo(text string) ToastMsg {
	return ToastMsg{Text: text, Level: ToastSuccess, Until: time.Now().Add(3 * time.Second)}
}
func toastErr(text string) ToastMsg {
	return ToastMsg{Text: text, Level: ToastError, Until: time.Now().Add(5 * time.Second)}
}
func toastCmdInfo(text string) tea.Cmd {
	return func() tea.Msg { return toastInfo(text) }
}

// --- Cmd factories -------------------------------------------------------

func quitConfirmCmd() tea.Cmd {
	return func() tea.Msg { return QuitConfirmRequestMsg{} }
}

func openCmdPaletteCmd() tea.Cmd {
	return func() tea.Msg { return openCmdPaletteMsg{} }
}

func openHelpCmd() tea.Cmd {
	return func() tea.Msg { return openHelpMsg{} }
}

func screenChangeCmd(target Screen) tea.Cmd {
	return func() tea.Msg { return ScreenChangeMsg{Target: target} }
}

func toastCmdError(e ErrorMsg) tea.Cmd {
	return func() tea.Msg {
		return ToastMsg{
			Text:  e.Err.Error(),
			Level: ToastError,
			Until: time.Now().Add(3 * time.Second),
		}
	}
}

func offlineCmd(offline bool) tea.Cmd {
	return func() tea.Msg { return OfflineStateMsg{Offline: offline} }
}

func refreshActiveCmd() tea.Cmd {
	return func() tea.Msg { return RefreshActiveScreenMsg{} }
}

// Internal activation markers consumed by overlay models once wired.
type openCmdPaletteMsg struct{}
type openHelpMsg struct{}

var _ tea.Model = Model{}
