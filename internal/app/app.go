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
	"github.com/tedilabs/ota/internal/tui/apps"
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
	ScreenApps
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
	case ScreenApps:
		return "apps"
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
	// OverlayActionMenu — issue #175 v0.1.15. Resource-specific
	// action picker bound to `a` from any list / detail screen.
	// Reads the active screen's Actions() and dispatches the
	// selected ID back into Update on Enter.
	OverlayActionMenu
)

// Actioner is implemented by screens that publish a list of
// resource-specific actions for the `a` action menu (issue #175).
// The App Shell calls Actions() when the operator presses `a`,
// builds an ActionMenuModel, and dispatches RunAction(id) on Enter.
type Actioner interface {
	Actions() []overlay.ActionMenuItem
	RunAction(id string) (tea.Model, tea.Cmd)
}

// UserActionKind classifies the Users lifecycle action a confirmation
// modal is gating. v0.2.2 #187 extends issue #125's original 3 with
// 4 more lifecycle ops the Okta API exposes:
//   - Activate    (POST /lifecycle/activate)
//   - Deactivate  (POST /lifecycle/deactivate)
//   - ExpirePassword (POST /lifecycle/expire_password)
//   - Delete      (DELETE /api/v1/users/{id})
type UserActionKind int

const (
	UserActionNone UserActionKind = iota
	UserActionResetPassword
	UserActionUnlock
	UserActionResetFactors
	UserActionActivate
	UserActionDeactivate
	UserActionExpirePassword
	UserActionDelete
)

// pendingUserAction is the (kind, target) pair the App Shell keeps in
// flight while OverlayActionConfirm is open. Reset back to its zero
// value when the operator confirms or cancels.
type pendingUserAction struct {
	Kind UserActionKind
	User domain.User
}

// RuleActionKind classifies the Group Rule lifecycle action a
// confirmation modal is gating (issue #188 v0.2.2). Mirrors
// UserActionKind for the Group Rules screen — Activate /
// Deactivate / Delete are the three lifecycle ops Okta exposes.
type RuleActionKind int

const (
	RuleActionNone RuleActionKind = iota
	RuleActionActivate
	RuleActionDeactivate
	RuleActionDelete
)

// pendingRuleAction is the (kind, target) pair the App Shell keeps
// in flight while OverlayActionConfirm is open with a rule action.
// Either pendingAction (UserActionKind) OR pendingRule may be set
// — the modal renders one at a time, mutually exclusive.
type pendingRuleAction struct {
	Kind RuleActionKind
	Rule domain.GroupRule
}

// SelectedRuleStater is the Group Rule counterpart of
// SelectedUserStater (issue #188 v0.2.2). Lets the App Shell read
// the rule the action menu is targeting.
type SelectedRuleStater interface {
	SelectedRule() (domain.GroupRule, bool)
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
	AppsPort       domain.AppsPort

	// LogsRefreshInterval / DefaultRefreshInterval drive the auto-refresh
	// tickers (issue #177 v0.1.16). Logs default 5s, every other list
	// default 10s. Zero on either disables auto-refresh on that surface.
	LogsRefreshInterval    time.Duration
	DefaultRefreshInterval time.Duration

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

	// actionMenu is the resource-specific action picker (issue #175).
	// Non-nil only while overlay == OverlayActionMenu.
	actionMenu *overlay.ActionMenuModel

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

	// pendingRule is the Group Rule counterpart of pendingAction
	// (issue #188 v0.2.2). Only one of pendingAction / pendingRule
	// is set at any time; the confirmation modal renders whichever
	// the operator triggered.
	pendingRule pendingRuleAction

	// statusToast is the chrome's right-anchored one-shot message on
	// the status row (v0.2.0): "yanked 5 lines", "nothing to close",
	// "refreshed". Clears on the next key press so the slot stays
	// available for whichever action the operator just triggered.
	statusToast string

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
	case openActionMenuMsg:
		// Issue #175: build the action picker from the active
		// screen's Actioner. The `a` key handler already gated on
		// activeChildHasActions(), so by the time we arrive here
		// the assertion is reliable.
		acts, ok := m.activeChildActions()
		if !ok {
			return m, nil
		}
		picker := overlay.NewActionMenuModel(m.resourceLabel(), acts)
		m.actionMenu = &picker
		m.overlay = OverlayActionMenu
		return m, nil
	case shared.RunUserActionMsg:
		// Issue #175: the action menu's RunAction emits this msg
		// for each picked Users lifecycle operation. Map the Kind
		// string back to the existing UserActionKind enum so the
		// y/N confirmation modal fires (issue #125 flow stays the
		// single source of truth for destructive ops).
		switch msg.Kind {
		case "reset-password":
			return m.openActionConfirm(UserActionResetPassword)
		case "unlock":
			return m.openActionConfirm(UserActionUnlock)
		case "reset-factors":
			return m.openActionConfirm(UserActionResetFactors)
		case "activate":
			return m.openActionConfirm(UserActionActivate)
		case "deactivate":
			return m.openActionConfirm(UserActionDeactivate)
		case "expire-password":
			return m.openActionConfirm(UserActionExpirePassword)
		case "delete":
			return m.openActionConfirm(UserActionDelete)
		}
		return m, nil
	case shared.RunRuleActionMsg:
		// Group Rule lifecycle dispatcher (issue #188 v0.2.2).
		// Same gate-via-confirm pattern as RunUserActionMsg.
		switch msg.Kind {
		case "activate":
			return m.openRuleActionConfirm(RuleActionActivate)
		case "deactivate":
			return m.openRuleActionConfirm(RuleActionDeactivate)
		case "delete":
			return m.openRuleActionConfirm(RuleActionDelete)
		}
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
	case OpenGroupDetailMsg:
		// Issue #171: cross-screen drill-down from User Detail's
		// Groups row Enter. Switch to the Groups list (building it
		// if needed), then forward an internal groups msg so the
		// list fetches the target group and surfaces detail mode.
		m.active = ScreenGroups
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenGroups)
		child := m.screens[ScreenGroups]
		updated, fwd := child.Update(groups.OpenDetailByIDMsg{ID: msg.ID})
		m.screens[ScreenGroups] = updated
		return m, tea.Batch(cmd, fwd)
	case OpenAppDetailMsg:
		// Issue #171: same flow for the Apps Wrapper.
		m.active = ScreenApps
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenApps)
		child := m.screens[ScreenApps]
		updated, fwd := child.Update(apps.OpenDetailByIDMsg{ID: msg.ID})
		m.screens[ScreenApps] = updated
		return m, tea.Batch(cmd, fwd)
	case OpenPolicyTypeMsg:
		// Issue #165 — replace the Policies wrapper with one scoped
		// to the requested type so the picker doesn't render
		// underneath the typed list.
		m.active = ScreenPolicies
		m.overlay = OverlayNone
		mdl := policies.NewWrapperForType(policies.Deps{
			Port:   m.deps.PoliciesPort,
			Clock:  m.deps.Clock,
			Logger: m.deps.Logger,
			Keys:   m.deps.Keys,
			Width:  m.width,
			Height: m.height,
		}, domain.PolicyType(msg.Type))
		m.screens[ScreenPolicies] = mdl
		return m, mdl.Init()
	case OpenAppTypeMsg:
		// Issue #166 — same pattern for Apps.
		m.active = ScreenApps
		m.overlay = OverlayNone
		mdl := apps.NewWrapperForType(apps.Deps{
			Port:            m.deps.AppsPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		}, domain.AppType(msg.Type))
		m.screens[ScreenApps] = mdl
		return m, mdl.Init()
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
	case "app", "apps", "a":
		return ScreenApps, true
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

	width := clampWidth(m.width)
	bodyLines := clampBodyLines(m.height)
	body := m.composeBody()
	// Floating palette / filter: render the input box as an overlay
	// stamped onto the TOP of the body region — replaces the first
	// few body lines instead of prepending and pushing the chrome
	// off-screen (issue #129). The list stays visible behind the
	// overlay, k9s-style, and the chrome's BodyLines budget is
	// honored regardless of suggestion-list length.
	switch {
	case m.overlay == OverlayPalette:
		body = stampOverlayOnTop(m.renderPaletteBox(tokens, width-3), body)
	case m.activeChildIsQueryEditing():
		body = stampOverlayOnTop(m.renderQueryBox(tokens, width-3), body)
	case m.activeChildIsFiltering():
		body = stampOverlayOnTop(m.renderFilterBox(tokens, width-3), body)
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
	visible, total, hasCount := m.activeChildCount()
	chrome := shared.ChromeInput{
		Tokens:       tokens,
		Width:        width,
		Brand:        "ota",
		Tenant:       tenantFromOrgURL(m.deps.OrgURL),
		Profile:      m.profileLabel(),
		Principal:    m.principalLogin,
		Version:      version.Tag,
		Timezone:     "UTC",
		RateLimit:    m.rateLimitState(),
		Resource:     m.resourceLabel(),
		Filter:       m.activeChildFilter(),
		CountVisible: visible,
		CountTotal:   total,
		HasCount:     hasCount,
		DividerRight: m.activeChildDividerRight(),
		StatusBadges: m.composeChromeBadges(),
		StatusToast:  m.statusToast,
		Body:         body,
		BodyLines:    bodyLines,
		KeyHints:     m.keyHints(tokens),
		Offline:      m.offline,
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
//
// v0.1.15: the previous upper cap of 60 rows clipped tall terminals —
// once the list emitted budget = h-9 rows that exceeded the chrome's
// 60-row cap, pressing `G` advanced the cursor onto a row the chrome
// silently dropped. The cap is gone; tall terminals now use their
// full vertical budget.
func clampBodyLines(h int) int {
	// v0.2.0 chrome reserves 7 rows: top border, title, upper
	// divider, body, status row (NEW), lower divider, key hints,
	// bottom border. Body trades 1 row for permanent state
	// visibility (sort/filter/follow/tail/visual badges).
	const reserved = 7
	if h <= 0 {
		return 16
	}
	rows := h - reserved
	if rows < 5 {
		return 5
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
	if m.overlay == OverlayActionMenu && m.actionMenu != nil {
		// Issue #175: render the action picker as the body — same
		// "modal owns the screen" pattern Help uses, so the picker
		// reads as the focal element and key hints don't compete.
		contentWidth := clampWidth(m.width) - 3
		return centerInBody(m.actionMenu.View(), contentWidth)
	}
	if m.overlay == OverlayHelp && m.helpModel != nil {
		// Issue #147: hand the modal as much width as the chrome
		// can spare — content area minus a small breathing-room
		// gutter — so it fills the screen instead of clinging to
		// the top-left corner.
		contentWidth := clampWidth(m.width) - 3
		modalWidth := contentWidth - 4
		if modalWidth < 60 {
			modalWidth = 60
		}
		sized := m.helpModel.WithWidth(modalWidth)
		return centerInBody(sized.View(), contentWidth)
	}
	if child, ok := m.screens[m.active]; ok {
		return child.View()
	}
	return "(loading…)"
}

// centerInBody horizontally centers a multi-line block inside the
// chrome's content width. Each line gets enough leading spaces to push
// the block to the visual center; lines wider than contentWidth are
// returned unchanged so the chrome can decide what to truncate.
func centerInBody(block string, contentWidth int) string {
	if contentWidth <= 0 {
		return block
	}
	lines := strings.Split(block, "\n")
	maxW := 0
	for _, l := range lines {
		if w := shared.VisibleWidth(l); w > maxW {
			maxW = w
		}
	}
	if maxW >= contentWidth {
		return block
	}
	pad := strings.Repeat(" ", (contentWidth-maxW)/2)
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = pad + l
	}
	return strings.Join(out, "\n")
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
		// v0.2.2 #188: render either pendingAction (user lifecycle)
		// or pendingRule (group-rule lifecycle) — mutually exclusive.
		if m.pendingRule.Kind != RuleActionNone {
			label := ruleActionLabel(m.pendingRule.Kind)
			name := m.pendingRule.Rule.Name
			if name == "" {
				name = m.pendingRule.Rule.ID
			}
			return tk.Danger.Render(label+" for ") + tk.Accent.Render(name) +
				tk.Danger.Render("?") + tk.Muted.Render("  (y/N)")
		}
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

// QueryStater is implemented by screens that own a server-side
// query prompt (today: Logs `Q` mirroring Okta's dashboard Search).
// Distinct from FilterStater so the chrome can render a `Q`-prefixed
// floating box vs `/`-prefixed filter box without conflating the
// two — the local filter narrows already-loaded data, the query
// re-fetches from the API. Issue #185 v0.2.1.
type QueryStater interface {
	QueryEditing() bool
	QueryInput() string
}

// FilterStater is implemented by list screens that own a `/` incremental
// filter so the App Shell can render the same floating input box used
// for the palette (issue #123).
type FilterStater interface {
	Filtering() bool
	Filter() string
}

// Counter is implemented by list screens that publish a "N of M"
// count to the chrome so it can stamp the count next to the resource
// label in the upper divider (issue #136). Returning ("", "") signals
// the screen has no count to show — the divider falls back to just
// the resource label.
type Counter interface {
	Count() (visible int, total int)
}

// ChromeBadgeStater is implemented by screens that contribute mode
// badges to the chrome's status row (v0.2.0). The slot replaces the
// inline status surfaces each screen used to maintain (Logs tail
// /follow lines, detail `-- VISUAL --`, hscroll silence). Order in
// the returned slice is preserved; chrome handles overflow truncation
// from the right.
type ChromeBadgeStater interface {
	StatusBadges() []shared.ChromeBadge
}

// EscapeOpStater is implemented by screens that report whether `Esc`
// would do something (close detail, clear filter, exit Visual). The
// App Shell uses it to surface a `nothing to close` toast in the
// status row when Esc is a no-op — replaces the silent Esc that
// operators repeatedly reported.
type EscapeOpStater interface {
	EscapeWillAct() bool
}

// escWillAct reports whether the active screen would do something
// in response to Esc (close detail, clear filter, exit Visual,
// abort Type-Picker, etc.). When false, the App Shell surfaces a
// `nothing to close` toast instead of forwarding the keystroke.
func (m Model) escWillAct() bool {
	if m.overlay != OverlayNone {
		return true
	}
	child, ok := m.screens[m.active]
	if !ok {
		return false
	}
	if st, ok := child.(EscapeOpStater); ok {
		return st.EscapeWillAct()
	}
	// Conservative default: forward Esc when the screen doesn't
	// publish a hint. Otherwise silent-Esc lists never reach the
	// child, which is worse than forwarding a no-op.
	return true
}

// composeChromeBadges assembles the chrome status row contents
// (v0.2.0). Order: App Shell-owned action / offline first, then
// the active screen's screen-specific badges (sort, filter, follow,
// tail, range, focus, hscroll). Truncation under width pressure
// drops trailing entries first.
func (m Model) composeChromeBadges() []shared.ChromeBadge {
	var out []shared.ChromeBadge
	if m.overlay == OverlayActionConfirm {
		switch {
		case m.pendingRule.Kind != RuleActionNone:
			out = append(out, shared.ChromeBadge{
				Key:   "ACTION",
				Value: ruleActionLabel(m.pendingRule.Kind),
				Tone:  shared.BadgeDanger,
			})
		case m.pendingAction.Kind != UserActionNone:
			out = append(out, shared.ChromeBadge{
				Key:   "ACTION",
				Value: userActionLabel(m.pendingAction.Kind),
				Tone:  shared.BadgeDanger,
			})
		}
	}
	child, ok := m.screens[m.active]
	if !ok {
		return out
	}
	stater, ok := child.(ChromeBadgeStater)
	if !ok {
		return out
	}
	out = append(out, stater.StatusBadges()...)
	return out
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

// activeChildIsQueryEditing reports whether the active screen has
// its server-side query prompt open (issue #185).
func (m Model) activeChildIsQueryEditing() bool {
	child, ok := m.screens[m.active]
	if !ok {
		return false
	}
	qs, ok := child.(QueryStater)
	return ok && qs.QueryEditing()
}

// activeChildQueryInput returns the active screen's in-progress
// server-query buffer (empty when no QueryStater or not editing).
func (m Model) activeChildQueryInput() string {
	child, ok := m.screens[m.active]
	if !ok {
		return ""
	}
	qs, ok := child.(QueryStater)
	if !ok {
		return ""
	}
	return qs.QueryInput()
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

// LastUpdatedStater is implemented by screens that publish a
// last-refresh timestamp for the chrome's upper-divider right slot
// (issue #177 v0.1.16). Returning the zero time disables the segment.
type LastUpdatedStater interface {
	LastUpdated() time.Time
}

// activeChildDividerRight assembles the right-side label of the
// upper divider — currently a single "updated 12:34:56 UTC" segment
// for screens that implement LastUpdatedStater. Empty when the
// active screen hasn't refreshed yet (zero time) or doesn't track
// the timestamp at all.
func (m Model) activeChildDividerRight() string {
	child, ok := m.screens[m.active]
	if !ok {
		return ""
	}
	st, ok := child.(LastUpdatedStater)
	if !ok {
		return ""
	}
	t := st.LastUpdated()
	if t.IsZero() {
		return ""
	}
	return "updated " + t.UTC().Format("15:04:05") + " UTC"
}

// activeChildCount returns the (visible, total, ok) triple for the
// active child screen — drives the "N of M" segment the chrome stamps
// next to the resource label in the upper divider (issue #136).
// ok is false for screens that don't implement Counter (detail
// surfaces) so the chrome leaves the segment off entirely.
func (m Model) activeChildCount() (visible, total int, ok bool) {
	child, found := m.screens[m.active]
	if !found {
		return 0, 0, false
	}
	c, isCounter := child.(Counter)
	if !isCounter {
		return 0, 0, false
	}
	v, t := c.Count()
	return v, t, true
}

// renderFilterBox builds the floating input box for `/` filter mode.
// Sibling of renderPaletteBox — same chrome, different prompt — sized
// to the data box width so the boxes line up vertically (issue #129).
func (m Model) renderFilterBox(tk shared.Tokens, innerWidth int) string {
	prompt := tk.Accent.Render("/")
	input := ""
	if child, ok := m.screens[m.active]; ok {
		if fs, ok := child.(FilterStater); ok {
			input = fs.Filter()
		}
	}
	cursor := tk.RowCursor.Render(" ")
	return modalBox(prompt+input+cursor, innerWidth, tk)
}

// renderQueryBox builds the floating input box for the `Q`
// server-side query mode (issue #185 v0.2.1). Same chrome as the
// `/` filter box; the `Q` prefix and the muted hint distinguish
// it from the local-filter prompt. Body explains the difference
// inline so the operator doesn't have to remember which is which.
func (m Model) renderQueryBox(tk shared.Tokens, innerWidth int) string {
	prompt := tk.Accent.Render("Q ")
	input := m.activeChildQueryInput()
	cursor := tk.RowCursor.Render(" ")
	hint := "\n" + tk.Muted.Render("server-side search · Enter applies · Esc cancels")
	return modalBox(prompt+input+cursor+hint, innerWidth, tk)
}

// renderPaletteBox builds the floating input box for the `:` palette.
// The key-hint footer was dropped (issue #129) — discoverability lives
// in the `?` modal, not on every palette press. Suggestions render
// downward inside the same box, capped so the modal never grows
// taller than maxPaletteHeight rows.
func (m Model) renderPaletteBox(tk shared.Tokens, innerWidth int) string {
	prompt := tk.Accent.Render(":")
	input := m.paletteInput
	cursor := tk.RowCursor.Render(" ")

	var b strings.Builder
	b.WriteString(prompt + input + cursor)
	if sugs := m.paletteSuggestions(); len(sugs) > 0 {
		b.WriteByte('\n')
		const maxSugs = 6
		max := maxSugs
		if max > len(sugs) {
			max = len(sugs)
		}
		for i := 0; i < max; i++ {
			line := sugs[i]
			if i == m.paletteSuggestionIdx {
				line = tk.RowCursor.Render("› " + line)
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
	return modalBox(b.String(), innerWidth, tk)
}

// stampOverlayOnTop replaces the first N lines of body with overlay's
// lines (where N == overlay's line count). Used so the floating
// palette / filter input boxes overlap the data body instead of
// pushing it down — keeps the chrome's row budget honoured even when
// the suggestion list is long (issue #134).
func stampOverlayOnTop(overlay, body string) string {
	overlayLines := strings.Split(overlay, "\n")
	bodyLines := strings.Split(body, "\n")
	out := make([]string, 0, len(bodyLines))
	out = append(out, overlayLines...)
	if len(overlayLines) < len(bodyLines) {
		out = append(out, bodyLines[len(overlayLines):]...)
	}
	return strings.Join(out, "\n")
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

// modalBox renders a single rounded-border box whose OUTER width
// equals contentWidth — i.e. the cells the chrome reserves for the
// body. lipgloss `.Width(N)` sizes the inside-of-border area
// (padding included), so the outer width works out to N + 2 (one
// border cell on each side). To make the rendered box land at
// exactly contentWidth so its `╮` corner butts up against the
// chrome's right `│` border, set `.Width(contentWidth - 2)`.
//
// Issue #151: the previous form passed `.Width(contentWidth)`
// which over-shot by 2 cells, and an even earlier form under-shot
// by 4 cells leaving a visible gap between the modal's right edge
// and the chrome's right border.
func modalBox(body string, contentWidth int, tk shared.Tokens) string {
	if contentWidth < 10 {
		contentWidth = 10
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5e81ac")).
		Padding(0, 1).
		Width(contentWidth - 2)
	return border.Render(body)
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
			Port:            m.deps.UsersPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenGroups:
		mdl := groups.NewListModel(groups.Deps{
			Port:            m.deps.GroupsPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenRules:
		mdl := rules.NewListModel(rules.Deps{
			Port:            m.deps.GroupRulesPort,
			Groups:          m.deps.GroupsPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenPolicies:
		// Issue #154: the policies screen is now a Wrapper that
		// routes between the type-select picker and the type-aware
		// list/detail. Esc on the list returns to the picker.
		mdl := policies.NewWrapper(policies.Deps{
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
			Service:         svc,
			Tail:            tail,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.LogsRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenApps:
		mdl := apps.NewWrapper(apps.Deps{
			Port:            m.deps.AppsPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	}
	// Detail views are populated by drill-down handlers; not auto-built.
	return nil, nil
}

// --- Key handling --------------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// v0.2.0: any key dismisses the previous one-shot status toast
	// so the slot is clear for whatever the operator just triggered.
	m.statusToast = ""

	// Palette has input focus while open.
	if m.overlay == OverlayPalette {
		return m.handlePaletteKey(msg)
	}
	if m.overlay == OverlayHelp || m.overlay == OverlayQuitConfirm || m.overlay == OverlayActionConfirm {
		return m.handleOverlayKey(msg)
	}
	if m.overlay == OverlayActionMenu {
		return m.handleActionMenuKey(msg)
	}

	// v0.2.0: Esc on a list with no overlay / filter / Visual / detail
	// open used to be silent — operators repeatedly reported pressing
	// Esc and getting no feedback. Surface a one-shot toast in the
	// status row so the no-op is acknowledged without side effects.
	if msg.Type == tea.KeyEsc && !m.escWillAct() {
		m.statusToast = "nothing to close"
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, quitConfirmCmd()
	// NOTE: Esc is intentionally NOT handled here. Each child Screen owns
	// what Esc means in its current context — closing detail mode, ending
	// `/` filtering, exiting visual selection, etc. The bottom delegation
	// block forwards it to the active child.
	case tea.KeyRunes:
		// While the child is in `/` filter mode every printable
		// rune is content; the App Shell's command shortcuts
		// (`:`, `?`, `q`, `a`) must NOT intercept them or the
		// operator can't type those letters into the filter.
		// Issue #176 v0.1.16 — operators reported `q` triggering
		// the quit modal mid-search.
		if m.activeChildIsFiltering() {
			break
		}
		switch string(msg.Runes) {
		case ":":
			return m, openCmdPaletteCmd()
		case "?":
			return m, openHelpCmd()
		case "q":
			return m, quitConfirmCmd()
		case "a":
			// Issue #175: open the resource action menu when the
			// active screen exposes one. Falls through to the
			// child screen's own `a` binding when no actions are
			// published, so existing semantics on screens that
			// don't implement Actioner stay intact.
			if m.activeChildHasActions() {
				return m, openActionMenuCmd()
			}
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
		case paletteCmdPolicyType:
			// Issue #165: jump straight to the typed list, replacing
			// any existing Policies wrapper so the picker doesn't
			// reappear underneath.
			return m, openPolicyTypeCmd(domain.PolicyType(arg))
		case paletteCmdAppType:
			// Issue #166: same pattern for Apps.
			return m, openAppTypeCmd(domain.AppType(arg))
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

// paletteCommandPool returns the singular canonical command names
// the palette autocomplete surfaces. Plural / hyphenated / k9s-style
// short aliases (users / grouprules / gr / etc.) are still routed by
// screenFromName when the operator types them — issue #150 just
// drops them from the suggestion list to keep the autocomplete
// readable.
func paletteCommandPool() []string {
	return []string{
		"user",
		"group",
		"group-rule",
		"policy",
		// Direct policy-type routes (issue #165). Operators that
		// know which kind of policy they're after can skip the
		// picker; the autocomplete surfaces every supported type.
		"okta-sign-on",
		"access-policy",
		"password-policy",
		"mfa-enroll",
		"profile-enrollment",
		"post-auth-session",
		"idp-discovery",
		"log",
		// Apps + per-app-type routes (issue #166).
		"app",
		"saml-app",
		"oidc-app",
		"bookmark-app",
		"swa-app",
		"scim-app",
		"unmask", "mask",
		"reset-password", "unlock", "reset-mfa",
		"quit",
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
			// v0.2.2 #188 — pendingRule and pendingAction are
			// mutually exclusive (openActionConfirm /
			// openRuleActionConfirm clear the other). Fire the
			// correct dispatcher based on which is set.
			if m.pendingRule.Kind != RuleActionNone {
				ra := m.pendingRule
				m.pendingRule = pendingRuleAction{}
				m.overlay = OverlayNone
				return m, runRuleActionCmd(m.deps.GroupRulesPort, ra)
			}
			action := m.pendingAction
			m.pendingAction = pendingUserAction{}
			m.overlay = OverlayNone
			return m, runUserActionCmd(m.deps.UsersPort, action)
		case "n", "N":
			m.pendingAction = pendingUserAction{}
			m.pendingRule = pendingRuleAction{}
			m.overlay = OverlayNone
		}
	}
	return m, nil
}

// handleActionMenuKey routes key input while OverlayActionMenu is
// open. Esc / `a` cancel; Enter dispatches the highlighted item via
// the active screen's Actioner.RunAction(); arrow / j / k advance.
func (m Model) handleActionMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = OverlayNone
		m.actionMenu = nil
		return m, nil
	case tea.KeyEnter:
		if m.actionMenu == nil {
			m.overlay = OverlayNone
			return m, nil
		}
		picked, ok := m.actionMenu.Selected()
		m.overlay = OverlayNone
		m.actionMenu = nil
		if !ok || picked.ID == "" {
			return m, nil
		}
		child, found := m.screens[m.active]
		if !found {
			return m, nil
		}
		actioner, ok := child.(Actioner)
		if !ok {
			return m, nil
		}
		updatedChild, cmd := actioner.RunAction(picked.ID)
		// RunAction returns the (possibly-updated) child Model AND a
		// Cmd. The Model is meant for the screen's own state — the
		// App Shell may need to react too (e.g., open the lifecycle
		// confirmation modal), so route the Cmd through the
		// dispatcher chain instead of dropping it.
		if updatedChild != nil {
			m.screens[m.active] = updatedChild
		}
		return m, cmd
	case tea.KeyRunes:
		if string(msg.Runes) == "a" {
			// Toggle off — `a` re-press closes the menu, matching
			// the symmetric open/close pattern `?` uses for help.
			m.overlay = OverlayNone
			m.actionMenu = nil
			return m, nil
		}
	}
	if m.actionMenu != nil {
		updated, cmd := m.actionMenu.Update(msg)
		if mm, ok := updated.(overlay.ActionMenuModel); ok {
			m.actionMenu = &mm
		}
		return m, cmd
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
	// paletteCmdPolicyType opens ScreenPolicies straight on a list
	// for the given PolicyType — issue #165 (`:okta-sign-on`,
	// `:password-policy`, etc.).
	paletteCmdPolicyType
	// paletteCmdAppType opens ScreenApps straight on a list for the
	// given AppType — issue #166 (`:saml-app`, `:oidc-app`, ...).
	paletteCmdAppType
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
	// Direct policy-type routes (issue #165). The verb arg field
	// carries the canonical PolicyType so the App Shell can build a
	// type-scoped Wrapper without forcing the operator through the
	// type-select picker.
	if pt, ok := policyTypeFromName(strings.ToLower(cmd)); ok {
		return paletteCmdPolicyType, 0, string(pt), true
	}
	// Direct app-type routes (issue #166).
	if at, ok := appTypeFromName(strings.ToLower(cmd)); ok {
		return paletteCmdAppType, 0, string(at), true
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

// appTypeFromName maps a palette verb to a domain.AppType — the
// per-app-type palette routes (issue #166). Plurals + canonical
// Okta signOnMode names both resolve so muscle memory routes.
func appTypeFromName(name string) (domain.AppType, bool) {
	switch name {
	case "saml-app", "saml_app", "samlapp", "saml":
		return domain.AppTypeSAML, true
	case "oidc-app", "oidc_app", "oidcapp", "oidc",
		"openid-connect", "openidconnect", "openid_connect":
		return domain.AppTypeOIDC, true
	case "bookmark-app", "bookmark_app", "bookmarkapp", "bookmark":
		return domain.AppTypeBookmark, true
	case "swa-app", "swa_app", "swaapp", "swa", "auto-login", "auto_login":
		return domain.AppTypeSWA, true
	case "scim-app", "scim_app", "scimapp", "scim":
		return domain.AppTypeSCIM, true
	case "other-app", "other_app", "otherapp":
		return domain.AppTypeOther, true
	}
	return "", false
}

// policyTypeFromName maps a palette verb to a domain.PolicyType.
// Recognises every alias an operator might naturally type — both
// the canonical Okta SDK names (OKTA_SIGN_ON) and the friendlier
// hyphenated forms (`okta-sign-on`).
func policyTypeFromName(name string) (domain.PolicyType, bool) {
	switch name {
	case "okta-sign-on", "okta_sign_on", "oktasignon", "sign-on", "signon":
		return domain.PolicyTypeOktaSignOn, true
	case "access", "access-policy", "access_policy", "accesspolicy":
		return domain.PolicyTypeAccessPolicy, true
	case "password", "password-policy", "password_policy":
		return domain.PolicyTypePassword, true
	case "mfa", "mfa-enroll", "mfa_enroll", "mfaenroll":
		return domain.PolicyTypeMFAEnroll, true
	case "profile-enrollment", "profile_enrollment", "profileenrollment":
		return domain.PolicyTypeProfileEnrollment, true
	case "post-auth-session", "post_auth_session", "postauthsession":
		return domain.PolicyTypePostAuthSession, true
	case "idp-discovery", "idp_discovery", "idpdiscovery":
		return domain.PolicyTypeIDPDiscovery, true
	}
	return "", false
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
	m.pendingRule = pendingRuleAction{}
	m.overlay = OverlayActionConfirm
	return m, nil
}

// openRuleActionConfirm is the Group Rule sibling (issue #188).
func (m Model) openRuleActionConfirm(kind RuleActionKind) (tea.Model, tea.Cmd) {
	child, ok := m.screens[m.active]
	if !ok {
		return m, toastCmdInfo("no active screen")
	}
	stater, ok := child.(SelectedRuleStater)
	if !ok {
		return m, toastCmdInfo("action not available on this screen")
	}
	rule, ok := stater.SelectedRule()
	if !ok {
		return m, toastCmdInfo("no rule selected")
	}
	m.pendingRule = pendingRuleAction{Kind: kind, Rule: rule}
	m.pendingAction = pendingUserAction{}
	m.overlay = OverlayActionConfirm
	return m, nil
}

// ruleActionLabel returns a human-readable label for the action.
func ruleActionLabel(k RuleActionKind) string {
	switch k {
	case RuleActionActivate:
		return "Activate rule"
	case RuleActionDeactivate:
		return "Deactivate rule"
	case RuleActionDelete:
		return "Delete rule"
	}
	return ""
}

// runRuleActionCmd dispatches the active pendingRule against the
// GroupRulesPort and emits a toast with the result.
func runRuleActionCmd(port domain.GroupRulesPort, action pendingRuleAction) tea.Cmd {
	if port == nil {
		return toastCmdInfo("GroupRulesPort not wired — action skipped")
	}
	return func() tea.Msg {
		ctx := context.Background()
		name := action.Rule.Name
		if name == "" {
			name = action.Rule.ID
		}
		switch action.Kind {
		case RuleActionActivate:
			if err := port.Activate(ctx, action.Rule.ID); err != nil {
				return toastErr("activate rule failed: " + err.Error())
			}
			return toastInfo("activated rule " + name)
		case RuleActionDeactivate:
			if err := port.Deactivate(ctx, action.Rule.ID); err != nil {
				return toastErr("deactivate rule failed: " + err.Error())
			}
			return toastInfo("deactivated rule " + name)
		case RuleActionDelete:
			if err := port.Delete(ctx, action.Rule.ID); err != nil {
				return toastErr("delete rule failed: " + err.Error())
			}
			return toastInfo("deleted rule " + name)
		}
		return nil
	}
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
	case UserActionActivate:
		return "Activate user"
	case UserActionDeactivate:
		return "Deactivate user"
	case UserActionExpirePassword:
		return "Expire password"
	case UserActionDelete:
		return "Delete user"
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
		case UserActionActivate:
			if err := port.Activate(ctx, action.User.ID, true); err != nil {
				return toastErr("activate failed: " + err.Error())
			}
			return toastInfo("activated " + login)
		case UserActionDeactivate:
			if err := port.Deactivate(ctx, action.User.ID, false); err != nil {
				return toastErr("deactivate failed: " + err.Error())
			}
			return toastInfo("deactivated " + login)
		case UserActionExpirePassword:
			if err := port.ExpirePassword(ctx, action.User.ID); err != nil {
				return toastErr("expire password failed: " + err.Error())
			}
			return toastInfo("password expired for " + login)
		case UserActionDelete:
			if err := port.Delete(ctx, action.User.ID); err != nil {
				return toastErr("delete failed: " + err.Error())
			}
			return toastInfo("deleted " + login)
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

// openPolicyTypeCmd returns a Cmd that wraps the requested policy
// type into an OpenPolicyTypeMsg the App Shell handles by rebuilding
// the Policies wrapper directly on the typed list (issue #165).
func openPolicyTypeCmd(t domain.PolicyType) tea.Cmd {
	return func() tea.Msg { return OpenPolicyTypeMsg{Type: string(t)} }
}

// openAppTypeCmd returns a Cmd that wraps the requested app type
// into an OpenAppTypeMsg the App Shell handles by rebuilding the
// Apps wrapper directly on the typed list (issue #166).
func openAppTypeCmd(t domain.AppType) tea.Cmd {
	return func() tea.Msg { return OpenAppTypeMsg{Type: string(t)} }
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
type openActionMenuMsg struct{}

// openActionMenuCmd opens the resource-specific action picker
// (issue #175). Routed through a Cmd so `a` flows the same way as
// `:` / `?` — the App Shell sees an openActionMenuMsg and instantiates
// the overlay with the active screen's Actioner output.
func openActionMenuCmd() tea.Cmd {
	return func() tea.Msg { return openActionMenuMsg{} }
}

// activeChildHasActions reports whether the active screen exposes
// any resource-specific actions for `a` to surface. Returns false
// when the screen doesn't implement Actioner OR returns an empty
// slice — keeps `a` from opening an empty menu.
func (m Model) activeChildHasActions() bool {
	acts, ok := m.activeChildActions()
	return ok && len(acts) > 0
}

// activeChildActions returns the active screen's published action
// list and an `ok` flag. The flag is false when the screen doesn't
// implement Actioner; callers fall back to whatever stock handling
// the original key had.
func (m Model) activeChildActions() ([]overlay.ActionMenuItem, bool) {
	child, found := m.screens[m.active]
	if !found {
		return nil, false
	}
	a, ok := child.(Actioner)
	if !ok {
		return nil, false
	}
	return a.Actions(), true
}

var _ tea.Model = Model{}
