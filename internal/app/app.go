package app

import (
	"log/slog"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
)

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

	// offline flags the transient statusbar state after a NetworkErrorMsg.
	offline bool

	// helpModel is the screen-aware Help overlay. Non-nil only while
	// overlay == OverlayHelp; instantiated by openHelpMsg using the active
	// screen so `?` shows the keys that actually do something here.
	helpModel *overlay.HelpModel

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
// first fetch (e.g., fetchUsersCmd) starts immediately.
func (m Model) Init() tea.Cmd {
	if child, ok := m.screens[m.active]; ok {
		return child.Init()
	}
	return nil
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
		return m, nil
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
	case openCmdPaletteMsg:
		m.overlay = OverlayPalette
		m.paletteInput = ""
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
	if m.overlay != OverlayNone {
		if overlay := m.renderOverlayPanel(tokens); overlay != "" {
			if body != "" {
				body = body + "\n" + overlay
			} else {
				body = overlay
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
		Version:   version.Tag,
		Timezone:  "UTC",
		RateLimit: m.rateLimitState(),
		Resource:  m.resourceLabel(),
		Counter:   m.counterLabel(),
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

// clampBodyLines maps a terminal height to a reasonable body-row count. The
// chrome itself reserves 7 rows (TUI_DESIGN §15.0a.5): top border, TitleBar,
// ContextBar, body divider, status divider, KeyHints, bottom border.
func clampBodyLines(h int) int {
	const reserved = 7
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

// counterLabel returns the right side of the ContextBar — "profile=<name>"
// for now, until child screens publish their own counts via a future port.
func (m Model) counterLabel() string {
	if m.deps.Profile != "" {
		return "profile=" + m.deps.Profile
	}
	return ""
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
		return tk.Accent.Render(":") + m.paletteInput + tk.Muted.Render("  <Esc> cancel")
	case OverlayHelp:
		// Body composition has already replaced the screen with the modal.
		return ""
	case OverlayQuitConfirm:
		return tk.Danger.Render("Quit ota?") + tk.Muted.Render("  (y/N)")
	}
	return ""
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
	if m.overlay == OverlayHelp || m.overlay == OverlayQuitConfirm {
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
		return m, nil
	case tea.KeyEnter:
		cmd, target, arg, ok := resolvePaletteCommand(m.paletteInput)
		m.overlay = OverlayNone
		m.paletteInput = ""
		if !ok {
			return m, nil
		}
		switch cmd {
		case paletteCmdScreen:
			return m, screenChangeCmd(target)
		case paletteCmdQuit:
			return m, quitConfirmCmd()
		case paletteCmdUnmask:
			// Forward to the active screen so the detail model can flip
			// the per-field unmask flag (issue #115).
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
		}
		return m, nil
	case tea.KeyBackspace:
		if n := len(m.paletteInput); n > 0 {
			m.paletteInput = m.paletteInput[:n-1]
		}
		return m, nil
	case tea.KeyRunes:
		m.paletteInput += string(msg.Runes)
		return m, nil
	}
	return m, nil
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
