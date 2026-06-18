package app

import (
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tedilabs/ota/internal/apilog"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/oktastatus"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/tui/apps"
	"github.com/tedilabs/ota/internal/tui/authenticators"
	"github.com/tedilabs/ota/internal/tui/groups"
	"github.com/tedilabs/ota/internal/tui/logs"
	"github.com/tedilabs/ota/internal/tui/overlay"
	"github.com/tedilabs/ota/internal/tui/policies"
	"github.com/tedilabs/ota/internal/tui/rules"
	"github.com/tedilabs/ota/internal/tui/admins"
	"github.com/tedilabs/ota/internal/tui/apitokens"
	"github.com/tedilabs/ota/internal/tui/authservers"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/users"
	"github.com/tedilabs/ota/internal/tui/zones"
	"github.com/tedilabs/ota/internal/version"
)

// Screen identifies the active resource screen (TUI_DESIGN §2.2).
type Screen int

const (
	// ScreenUsers stays at iota 0 so the zero-value InitialScreen
	// resolves to the Users list — the boot default for both the
	// real `ota` binary and the existing test scaffolding.
	ScreenUsers Screen = iota
	ScreenGroups
	ScreenRules
	ScreenPolicies
	ScreenLogs
	ScreenApps
	ScreenAuthenticators
	ScreenNetworkZones
	ScreenAuthorizationServers
	ScreenAPITokens
	ScreenAdministrators
	ScreenUserDetail
	ScreenGroupDetail
	ScreenRuleDetail
	ScreenPolicyDetail
	ScreenLogDetail
	// ScreenUserEdit hosts SCR-012 — Users Profile Edit Form
	// (REQ-W01). Pushed onto navStack by `e` from list/detail or by
	// `:edit` palette. Pops on save success / clean-Esc / discard
	// confirm.
	ScreenUserEdit
	// ScreenGroupEdit hosts the Groups profile edit form. Same
	// lifecycle as ScreenUserEdit; pushed only when the target group
	// is OKTA_GROUP (APP_GROUP / BUILT_IN are upstream-managed).
	ScreenGroupEdit
	// ScreenRuleEdit hosts the Group Rule edit form. Pushed only
	// when the target rule is INACTIVE or INVALID (Okta refuses
	// edits on ACTIVE rules with a 400).
	ScreenRuleEdit
	// ScreenPolicyEdit hosts the Policy edit form. v0.2 scope:
	// metadata only (name / description / priority / status).
	ScreenPolicyEdit
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
	case ScreenAuthenticators:
		return "authenticators"
	case ScreenNetworkZones:
		return "network-zones"
	case ScreenAuthorizationServers:
		return "authorization-servers"
	case ScreenAPITokens:
		return "api-tokens"
	case ScreenAdministrators:
		return "administrators"
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
	case ScreenUserEdit:
		return "user-edit"
	case ScreenGroupEdit:
		return "group-edit"
	case ScreenRuleEdit:
		return "rule-edit"
	case ScreenPolicyEdit:
		return "policy-edit"
	}
	return "unknown"
}

// ActiveScreenName returns the active screen's label — exported helper so
// tests can assert against the router state without reaching into internals.
func ActiveScreenName(m Model) string { return m.active.String() }

// resetNav replaces the entire stack with the supplied root and
// updates m.active to match. Used by `:` palette commands —
// "navigate to <res>" is the operator declaring a fresh root, so
// any prior history is discarded (and visiting a screen the
// operator already passed through doesn't pile up duplicates).
func (m *Model) resetNav(s Screen) {
	m.navStack = []Screen{s}
	m.active = s
}

// pushNav appends a frame onto the stack — used by every cross-
// resource drill-down so Esc walks back to the previous resource.
// Idempotent: pushing the screen the operator is already on is a
// no-op (avoids piling up Logs frames when the operator hits `l`
// twice on the same Logs screen, etc.).
func (m *Model) pushNav(s Screen) {
	if len(m.navStack) > 0 && m.navStack[len(m.navStack)-1] == s {
		m.active = s
		return
	}
	m.navStack = append(m.navStack, s)
	m.active = s
}

// popNav drops the top frame and returns the new top. Returns
// (zero, false) when the stack already holds the root frame; the
// caller should fire the quit confirm in that case (root Esc).
func (m *Model) popNav() (Screen, bool) {
	if len(m.navStack) <= 1 {
		return 0, false
	}
	m.navStack = m.navStack[:len(m.navStack)-1]
	m.active = m.navStack[len(m.navStack)-1]
	return m.active, true
}

// canPopNav reports whether popNav would do work — used by the Esc
// handler so the App Shell can pick "back" vs "quit confirm" before
// surfacing the legacy "nothing to close" toast.
func (m Model) canPopNav() bool { return len(m.navStack) > 1 }


// now returns the model's logical wall clock.
func (m Model) now() time.Time {
	if m.deps.Clock != nil {
		return m.deps.Clock.Now()
	}
	return time.Now()
}

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
	// OverlayAPIRecorder — global "Okta API timeline" overlay bound
	// to `~`. Reads from the apilog.Recorder snapshot the App Shell
	// holds in m.deps.APIRecorder.
	OverlayAPIRecorder
	// OverlayDiscardConfirm — soft (L1) confirm shown when the
	// operator presses Esc on a dirty Users Edit Form (REQ-W01
	// AC-5.2 / D-W4). Default action is "keep editing"; only an
	// explicit y/Y discards the in-flight changes. Distinct from
	// OverlayActionConfirm because the pendingAction structure
	// can't host a Form snapshot.
	OverlayDiscardConfirm
	// OverlayStatusPicker — bound to `s` from Users list / detail.
	// Renders the valid lifecycle transitions for the selected user
	// and routes the picked one through OverlayActionConfirm so the
	// "are you sure?" guardrail stays consistent with `:deactivate`,
	// `:delete`, etc.
	OverlayStatusPicker
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
	// UserActionSuspend / UserActionUnsuspend back the status picker
	// transitions ACTIVE↔SUSPENDED. Distinct from
	// Deactivate/Activate (which DEPROVISION the user) — Suspend
	// blocks sign-in while keeping every assignment intact.
	UserActionSuspend
	UserActionUnsuspend
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
	UsersPort                domain.UsersPort
	GroupsPort               domain.GroupsPort
	GroupRulesPort           domain.GroupRulesPort
	PoliciesPort             domain.PoliciesPort
	LogsPort                 domain.LogsPort
	AppsPort                 domain.AppsPort
	AuthenticatorsPort       domain.AuthenticatorsPort
	NetworkZonesPort         domain.NetworkZonesPort
	AuthorizationServersPort domain.AuthorizationServersPort
	APITokensPort            domain.APITokensPort
	AdministratorsPort       domain.AdministratorsPort

	// APIRecorder is the cross-session NDJSON capture of every Okta
	// HTTP round-trip; the `~` overlay reads its in-memory snapshot
	// to render the timeline. Nil disables the overlay.
	APIRecorder *apilog.Recorder

	// LogsRefreshInterval / DefaultRefreshInterval drive the auto-refresh
	// tickers (issue #177 v0.1.16). Logs default 5s, every other list
	// default 10s. Zero on either disables auto-refresh on that surface.
	LogsRefreshInterval    time.Duration
	DefaultRefreshInterval time.Duration

	// OktaStatusEndpoint is the status.okta.com URL the App Shell
	// probes for the title-bar status segment (issue #190 v0.2.2).
	// Empty disables the probe entirely so unit tests don't burn
	// outbound HTTP. main.go sets it to the public default.
	OktaStatusEndpoint string

	// Optional initial state for tests / direct embedding.
	InitialScreen Screen
}

// Model is ota's top-level tea.Model (App Shell). Hosts the active screen,
// overlays (cmd palette, help, ...), and the global status bar.
type Model struct {
	deps    Deps
	active  Screen
	overlay Overlay

	// navStack tracks the screen-level navigation history (Android
	// activity-stack semantics, 2026-05-04). The bottom is the root
	// resource the operator opened with `:` — palette commands
	// REPLACE the stack with that root. Cross-resource drill-downs
	// (User Detail → Group Detail, Log actor → User Detail, `l`
	// jumping to Logs scoped to the current resource) PUSH onto the
	// stack instead of replacing it. Esc with nothing else to close
	// pops one level; popping the last frame fires the quit confirm.
	// The stack always carries at least one element while the App
	// Shell is alive — pushNav / popNav maintain that invariant.
	navStack []Screen

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

	// statusPicker is the user status picker overlay. Non-nil only
	// while overlay == OverlayStatusPicker.
	statusPicker *StatusPickerModel

	// apiRecorderModel renders the global Okta API timeline overlay
	// when overlay == OverlayAPIRecorder. Bound to the `~` keybinding;
	// nil when m.deps.APIRecorder is nil (recorder unavailable).
	apiRecorderModel *overlay.APIRecorderModel

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

	// toast is the most recent ToastMsg awaiting render — shown as a
	// color-coded floating band stamped on top of the body for the
	// duration set on Until (issues #195/#A7 v0.2.4). Single canonical
	// transient-message slot for action results (Activate / Reset MFA
	// / Delete), Esc no-ops, and yank/copy feedback alike.
	toast *shared.ToastMsg
	// toastGen invalidates stale clear ticks if a newer toast lands
	// before the old one's expiry fires.
	toastGen int

	// oktaStatus is the most recent status.okta.com snapshot
	// (issue #190 v0.2.2). The chrome's title bar renders the
	// emoji + label so operators see live tenant-side health
	// alongside the local rate-limit indicator. Polled every
	// 5 minutes via a tea.Tick chain, plus once on Init.
	oktaStatus oktastatus.Snapshot

	// width / height track the current terminal size, updated via
	// tea.WindowSizeMsg. The chrome renders 100% to width (TUI_DESIGN
	// §15.0a v1.2.0): widths >= 80 pass through unchanged so wide
	// terminals fill edge-to-edge; widths < 80 clamp to 80 and rely on
	// the renderer's truncation to keep the layout intact; width == 0
	// (no WindowSizeMsg yet) falls back to shared.ChromeWidth.
	width  int
	height int

	// editTargetID is the userID the next ScreenUserEdit instance must
	// fetch on Init (REQ-W01 AC-1.1 / D-W16). Set by the OpenUserEditMsg
	// handler immediately before pushNav(ScreenUserEdit) so buildScreen
	// has the right ID. Cleared after the EditModel is built.
	editTargetID string

	// groupEditTargetID parallels editTargetID for ScreenGroupEdit.
	groupEditTargetID string

	// ruleEditTargetID parallels for ScreenRuleEdit.
	ruleEditTargetID string

	// policyEditTargetID parallels for ScreenPolicyEdit (added in
	// the same multi-resource edit-form round).
	policyEditTargetID string
}

// New constructs the App Shell. The initial screen is materialized eagerly
// so Init() can return its first Cmd directly.
func New(deps Deps) Model {
	m := Model{
		deps:     deps,
		active:   deps.InitialScreen,
		overlay:  OverlayNone,
		navStack: []Screen{deps.InitialScreen},
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


// Update implements tea.Model. Handles global shortcuts (`:`, `?`, Ctrl-c),
// palette input, broadcasts toast/offline/refresh messages, and delegates
// non-routing messages to the active child screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to child Screen Models so they can adapt their column
		// widths / drop rules to the new terminal size. Capture each
		// child's returned Cmd so the spinner tick chain (scheduled in
		// the child's WindowSizeMsg handler) actually reaches the
		// Bubbletea runtime — without this the loading spinner sits on
		// its first frame forever.
		var cmds []tea.Cmd
		for s, child := range m.screens {
			updated, c := child.Update(msg)
			m.screens[s] = updated
			if c != nil {
				cmds = append(cmds, c)
			}
		}
		// Bubbletea always emits an initial WindowSizeMsg at startup so
		// this is the natural place to kick off the one-shot /me probe
		// (issue #124) AND the first status.okta.com poll (issue
		// #190 v0.2.2). Both fire at most once per session — the
		// status probe self-reschedules every 5min via tick.
		if c := m.kickPrincipalFetch(); c != nil {
			cmds = append(cmds, c)
		}
		if c := m.kickOktaStatusFetch(); c != nil {
			cmds = append(cmds, c)
		}
		switch len(cmds) {
		case 0:
			return m, nil
		case 1:
			return m, cmds[0]
		}
		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case ErrorMsg:
		return m, toastCmdError(msg)
	case ToastMsg:
		// Issue #195 v0.2.4 — store the floating toast band, schedule
		// a clear tick at Until, and bump the generation so an older
		// pending clear doesn't stomp this one.
		t := msg
		if t.Until.IsZero() {
			t.Until = time.Now().Add(3 * time.Second)
		}
		m.toast = &t
		m.toastGen++
		return m, scheduleToastClearCmd(m.toastGen, time.Until(t.Until))
	case toastClearMsg:
		if m.toast != nil && msg.gen == m.toastGen {
			m.toast = nil
		}
		return m, nil
	case NetworkErrorMsg:
		m.offline = true
		return m, offlineCmd(true)
	case NetworkRestoredMsg:
		m.offline = false
		// REQ-E03 AC-3 — fan out shared.RefreshScreenMsg so the
		// active list / detail re-fetches as soon as the network
		// is back. v0.2.4 #A6: dropped the legacy
		// RefreshActiveScreenMsg shim — shared.RefreshScreenMsg is
		// the single canonical refresh signal now.
		return m, refreshScreenCmd()
	case OfflineStateMsg:
		m.offline = msg.Offline
		return m, nil
	case principalLoadedMsg:
		m.principalLogin = msg.Login
		return m, nil
	case oktaStatusFetchedMsg:
		// Issue #190 v0.2.2 — store the latest snapshot so the
		// chrome's title bar can stamp the emoji + label, then
		// schedule the next 5-min tick. Failures (Indicator =
		// Unknown) still update the snapshot so the chrome
		// surfaces the unreachable state rather than a stale
		// "operational" reading.
		m.oktaStatus = msg.snap
		return m, scheduleOktaStatusTickCmd(m.deps.OktaStatusEndpoint, msg.snap.Indicator)
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
	case openAPIRecorderMsg:
		// Open / refresh the global Okta API timeline overlay (`~`).
		if m.deps.APIRecorder == nil {
			return m, nil
		}
		ar := overlay.NewAPIRecorderModel(m.deps.APIRecorder)
		m.apiRecorderModel = &ar
		m.overlay = OverlayAPIRecorder
		return m, nil
	case openActionMenuMsg:
		// Issue #175: build the action picker from the active
		// screen's Actioner. The `a` key handler already gated on
		// activeChildHasActions(), so by the time we arrive here
		// the assertion is reliable.
		// #U8 v0.2.4 — extra defense: if the screen's Actions slice
		// happens to be empty for the current row (e.g., already-
		// deleted user, no row selected), fall through to a toast
		// instead of opening an empty picker.
		acts, ok := m.activeChildActions()
		if !ok {
			return m, nil
		}
		if len(acts) == 0 {
			return m, toastCmdInfo("no actions available for this row")
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
	case actionCompletedMsg:
		// Issue #192 v0.2.3 — destructive op finished. Surface the
		// toast AND fan out a screen refresh so the list / detail
		// shows the post-action state without waiting for the
		// next auto-tick. tea.Batch keeps both signals in flight;
		// the toast renders immediately while the refresh fetch
		// runs in the background.
		toast := msg.toast
		return m, tea.Batch(
			func() tea.Msg { return toast },
			refreshScreenCmd(),
		)
	case actionFailedMsg:
		// #U11 v0.2.4 — destructive op errored. Surface the red toast
		// AND broadcast shared.ActionFailedMsg{TargetID} so the active
		// list can flash that row red. Failed actions don't change
		// state, so no refresh is fired.
		toast := msg.toast
		failed := shared.ActionFailedMsg{TargetID: msg.targetID}
		return m, tea.Batch(
			func() tea.Msg { return toast },
			func() tea.Msg { return failed },
		)
	case QuitConfirmRequestMsg:
		m.overlay = OverlayQuitConfirm
		return m, nil
	case ScreenChangeMsg:
		// `:` palette commands declare a fresh root — wipe the nav
		// stack so the operator's mental model of "Esc walks back
		// through the chain I drilled into" stays clean.
		m.resetNav(msg.Target)
		m.overlay = OverlayNone
		m.paletteInput = ""
		updated, cmd := m.ensureScreen(m.active)
		return updated, cmd
	case SwitchScreenMsg:
		if s, ok := screenFromName(msg.Target); ok {
			m.resetNav(s)
			m.overlay = OverlayNone
			m.paletteInput = ""
			updated, cmd := m.ensureScreen(m.active)
			return updated, cmd
		}
		return m, nil
	case shared.OpenScreenMsg:
		// Cross-screen drill-down (e.g., logs → users). Push onto
		// the nav stack so Esc on the destination walks back to
		// wherever the operator came from rather than silently
		// dropping the trail.
		if s, ok := screenFromName(msg.Target); ok {
			m.pushNav(s)
			m.overlay = OverlayNone
			updated, cmd := m.ensureScreen(m.active)
			return updated, cmd
		}
		return m, nil
	case OpenResourceMsg:
		// Cross-resource drill-down (e.g., Log actor → User Detail).
		// Push instead of replace so Esc returns to the previous
		// resource rather than silently switching it out.
		if s, ok := detailScreenFor(msg.Kind); ok {
			m.pushNav(s)
			m.overlay = OverlayNone
			updated, cmd := m.ensureScreen(m.active)
			return updated, cmd
		}
		return m, nil
	case OpenGroupDetailMsg:
		// Issue #171: cross-screen drill-down from User Detail's
		// Groups row Enter. Push the Groups frame so Esc returns
		// to the User Detail the operator came from.
		m.pushNav(ScreenGroups)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenGroups)
		child := m.screens[ScreenGroups]
		updated, fwd := child.Update(groups.OpenDetailByIDMsg{ID: msg.ID})
		m.screens[ScreenGroups] = updated
		return m, tea.Batch(cmd, fwd)
	case OpenAppDetailMsg:
		// Issue #171: same flow for the Apps Wrapper.
		m.pushNav(ScreenApps)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenApps)
		child := m.screens[ScreenApps]
		updated, fwd := child.Update(apps.OpenDetailByIDMsg{ID: msg.ID})
		m.screens[ScreenApps] = updated
		return m, tea.Batch(cmd, fwd)
	case OpenUserDetailMsg:
		// #G2 / U7 v0.2.4 — Users counterpart. The Users list owns
		// the OpenDetailByIDMsg handler directly (no Wrapper).
		m.pushNav(ScreenUsers)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenUsers)
		child := m.screens[ScreenUsers]
		updated, fwd := child.Update(users.OpenDetailByIDMsg{ID: msg.ID})
		m.screens[ScreenUsers] = updated
		return m, tea.Batch(cmd, fwd)
	case shared.OpenUserEditMsg:
		// REQ-W01 AC-1.1 / D-W16 — push SCR-012 onto the nav stack,
		// rebuilding the EditModel so each entry fires a fresh GET
		// (AC-1.3 — cache distrust). Existing entries are discarded so
		// the operator never lands on a stale form when re-entering.
		delete(m.screens, ScreenUserEdit)
		m.editTargetID = msg.ID
		m.pushNav(ScreenUserEdit)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenUserEdit)
		return m, cmd
	case shared.OpenStatusPickerMsg:
		// Status picker — open the modal overlay with the user
		// already attached. Short-circuit to a toast when the user
		// is in a terminal state (no valid transitions) so the
		// operator gets actionable feedback instead of an empty
		// modal.
		picker := NewStatusPickerModel(msg.User)
		if picker.Empty() {
			return m, toastCmdInfo("no status transitions for " + string(msg.User.Status))
		}
		m.statusPicker = &picker
		m.overlay = OverlayStatusPicker
		return m, nil
	case shared.UserEditDiscardedMsg:
		// Operator picked "Discard and exit" on the unsaved-changes
		// prompt. Drop the cached EditModel so a re-entry rebuilds
		// from a fresh GET, then pop the nav frame back to whoever
		// pushed us (list / detail).
		delete(m.screens, ScreenUserEdit)
		m.editTargetID = ""
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, cmd
		}
		return m, nil
	case shared.UserUpdatedMsg:
		// REQ-W01 AC-4.5 — broadcast the post-save snapshot so the
		// Users list / detail patches its cache with the server-echoed
		// User without an extra GET. v0.2.5+ surfaces also opt in by
		// implementing a UserUpdatedMsg handler. EditModel emits this
		// from its save-success branch.
		if child, ok := m.screens[ScreenUsers]; ok {
			updated, _ := child.Update(msg)
			m.screens[ScreenUsers] = updated
		}
		// Pop the edit frame so the operator lands back on the
		// previous surface (list or detail) with the patched row
		// rendered. canPopNav() guards the root case.
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, tea.Batch(cmd, refreshScreenCmd())
		}
		return m, refreshScreenCmd()
	case shared.OpenGroupEditMsg:
		// Groups edit form entry — push ScreenGroupEdit and let
		// buildScreen build a fresh EditModel that fires the initial
		// GET. Existing edit frame discarded so a re-entry never
		// lands on a stale form.
		delete(m.screens, ScreenGroupEdit)
		m.groupEditTargetID = msg.ID
		m.pushNav(ScreenGroupEdit)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenGroupEdit)
		return m, cmd
	case shared.GroupEditDiscardedMsg:
		// Discard-and-exit on the Groups edit form — drop the cached
		// model and pop back to the previous surface.
		delete(m.screens, ScreenGroupEdit)
		m.groupEditTargetID = ""
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, cmd
		}
		return m, nil
	case shared.GroupUpdatedMsg:
		// Broadcast the post-save Group snapshot to the list / detail
		// so its cache stays in sync, then pop the edit frame.
		if child, ok := m.screens[ScreenGroups]; ok {
			updated, _ := child.Update(msg)
			m.screens[ScreenGroups] = updated
		}
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, tea.Batch(cmd, refreshScreenCmd())
		}
		return m, refreshScreenCmd()
	case shared.OpenRuleEditMsg:
		delete(m.screens, ScreenRuleEdit)
		m.ruleEditTargetID = msg.ID
		m.pushNav(ScreenRuleEdit)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenRuleEdit)
		return m, cmd
	case shared.RuleEditDiscardedMsg:
		delete(m.screens, ScreenRuleEdit)
		m.ruleEditTargetID = ""
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, cmd
		}
		return m, nil
	case shared.RuleUpdatedMsg:
		if child, ok := m.screens[ScreenRules]; ok {
			updated, _ := child.Update(msg)
			m.screens[ScreenRules] = updated
		}
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, tea.Batch(cmd, refreshScreenCmd())
		}
		return m, refreshScreenCmd()
	case shared.OpenPolicyEditMsg:
		delete(m.screens, ScreenPolicyEdit)
		m.policyEditTargetID = msg.ID
		m.pushNav(ScreenPolicyEdit)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenPolicyEdit)
		return m, cmd
	case shared.PolicyEditDiscardedMsg:
		delete(m.screens, ScreenPolicyEdit)
		m.policyEditTargetID = ""
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, cmd
		}
		return m, nil
	case shared.PolicyUpdatedMsg:
		if child, ok := m.screens[ScreenPolicies]; ok {
			updated, _ := child.Update(msg)
			m.screens[ScreenPolicies] = updated
		}
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, tea.Batch(cmd, refreshScreenCmd())
		}
		return m, refreshScreenCmd()
	case shared.OpenLogsMsg:
		// #F2 / #F4 v0.2.5 — `l` shortcut from any resource. Switch
		// to Logs and forward an OpenForFilterMsg so the screen
		// pre-fills the server-side `filter=` expression keyed by
		// the resource's ID. Push the frame so Esc returns to the
		// resource the operator was inspecting.
		m.pushNav(ScreenLogs)
		m.overlay = OverlayNone
		var cmd tea.Cmd
		m, cmd = m.ensureScreen(ScreenLogs)
		child := m.screens[ScreenLogs]
		updated, fwd := child.Update(logs.OpenForFilterMsg{Filter: msg.Filter})
		m.screens[ScreenLogs] = updated
		return m, tea.Batch(cmd, fwd)
	case OpenPolicyTypeMsg:
		// Issue #165 — replace the Policies wrapper with one scoped
		// to the requested type so the picker doesn't render
		// underneath the typed list. Palette-driven so we reset the
		// nav stack — operator declared a fresh root.
		m.resetNav(ScreenPolicies)
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
		m.resetNav(ScreenApps)
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
	case "authenticator", "authenticators",
		"auth", "auths",
		"factor", "factors":
		return ScreenAuthenticators, true
	case "zone", "zones",
		"network-zone", "network-zones",
		"network_zone", "network_zones",
		"netzone", "netzones":
		return ScreenNetworkZones, true
	case "authorization-server", "authorization-servers",
		"authorization_server", "authorization_servers",
		"authserver", "authservers",
		"as":
		return ScreenAuthorizationServers, true
	case "api-token", "api-tokens",
		"api_token", "api_tokens",
		"apitoken", "apitokens",
		"token", "tokens":
		return ScreenAPITokens, true
	case "administrator", "administrators",
		"admin", "admins":
		return ScreenAdministrators, true
	case "user-edit", "user_edit", "useredit",
		"edit-user", "edit_user", "edituser",
		// REQ-W01 / TUI_DESIGN §3.4 row 335: `:edit` and `:e` are the
		// canonical palette aliases for SCR-012 (QA-W01-03). The App
		// Shell resolves them to ScreenUserEdit; the target user is
		// inferred from the active screen by the palette dispatcher
		// (Users list cursor row or Detail's user; otherwise the
		// "no user selected" toast fires).
		"edit", "e":
		return ScreenUserEdit, true
	case "group-edit", "group_edit", "groupedit",
		"edit-group", "edit_group", "editgroup":
		return ScreenGroupEdit, true
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
	// Issue #201 v0.2.4 — dim the body underneath the floating input
	// boxes (palette / query / filter) so the operator's focus lands
	// on the input. Dim happens BEFORE stamping so the input box
	// itself stays at full color.
	switch {
	case m.overlay == OverlayPalette:
		body = stampOverlayOnTop(m.renderPaletteBox(tokens, width-3), dimBody(body, tokens))
	case m.activeChildIsQueryEditing():
		body = stampOverlayOnTop(m.renderQueryBox(tokens, width-3), dimBody(body, tokens))
	case m.activeChildIsServerFilterEditing():
		body = stampOverlayOnTop(m.renderServerFilterBox(tokens, width-3), dimBody(body, tokens))
	case m.activeChildIsFiltering():
		body = stampOverlayOnTop(m.renderFilterBox(tokens, width-3), dimBody(body, tokens))
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
	// Issue #195 v0.2.4 — floating toast band stamped on top of the
	// body, suppressed when a confirmation modal owns the body so the
	// band doesn't compete with the popup. #U4 v0.2.4 — band lands on
	// the LAST body row (replacing it for the toast's lifetime) instead
	// of the first, so it doesn't cover the column header / first
	// data rows where the operator's reading focus is.
	if m.toast != nil && m.overlay != OverlayActionConfirm && m.overlay != OverlayQuitConfirm {
		band := renderToastBand(*m.toast, width-3, tokens)
		if band != "" {
			body = stampOverlayAtBottom(band, body, bodyLines)
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
		Filter:          m.activeChildFilter(),
		CountVisible:    visible,
		CountTotal:      total,
		HasCount:        hasCount,
		DividerRight:    m.activeChildDividerRight(),
		StatusBadges: m.composeChromeBadges(),
		// Issue #190 v0.2.2 — suppress the title-bar status segment
		// until the first probe completes so the chrome doesn't
		// flash a misleading "❔" on every cold start.
		OktaStatusEmoji: oktaStatusEmojiOrEmpty(m.oktaStatus, m.deps.OktaStatusEndpoint != "", m.principalLogin != ""),
		OktaStatusLabel: oktaStatusLabelOrEmpty(m.oktaStatus, m.deps.OktaStatusEndpoint != "", m.principalLogin != ""),
		Body:            body,
		BodyLines:       bodyLines,
		KeyHints:        m.keyHints(tokens),
		Offline:         m.offline,
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
		// Issue #175: action picker as a centered modal so it reads as
		// the focal element and key hints don't compete. v0.2.4 #201:
		// dim the body behind it so the operator's list context stays
		// visible (just darkened).
		return m.composeModalOverDimmedBody(m.actionMenu.View())
	}
	if m.overlay == OverlayStatusPicker && m.statusPicker != nil {
		// Status picker — same dim+stamp treatment as the action menu.
		// Enter routes the chosen transition into OverlayActionConfirm.
		return m.composeModalOverDimmedBody(m.statusPicker.View())
	}
	if m.overlay == OverlayHelp && m.helpModel != nil {
		// Issue #147: hand the help modal as much width as the chrome
		// can spare so it fills the screen. v0.2.4 #201: dim the body
		// behind it for consistency with the other popups.
		contentWidth := clampWidth(m.width) - 3
		modalWidth := contentWidth - 4
		if modalWidth < 60 {
			modalWidth = 60
		}
		sized := m.helpModel.WithWidth(modalWidth)
		return m.composeModalOverDimmedBody(sized.View())
	}
	if m.overlay == OverlayAPIRecorder && m.apiRecorderModel != nil {
		// Global Okta API timeline overlay. Sized to the chrome's
		// content rectangle so the 2-pane layout has room to breathe.
		contentWidth := clampWidth(m.width) - 3
		bodyHeight := clampBodyLines(m.height)
		sized := m.apiRecorderModel.WithSize(contentWidth, bodyHeight+2)
		return m.composeModalOverDimmedBody(sized.View())
	}
	if m.overlay == OverlayActionConfirm {
		// Issue #199 v0.2.4 — destructive-op confirmation popup floats
		// over the dimmed body so the operator still sees their list /
		// detail context (just darkened) behind the modal.
		return m.composeModalOverDimmedBody(m.renderActionConfirmModal(activeTokens()))
	}
	if m.overlay == OverlayQuitConfirm {
		// Issue #199 v0.2.4 — same dim-and-stamp treatment as the
		// action confirm; visual language stays consistent for both
		// "are you sure?" prompts.
		return m.composeModalOverDimmedBody(m.renderQuitConfirmModal(activeTokens()))
	}
	// v2 modal pattern — any active screen that implements
	// ModalRenderer renders as a centered modal over a dimmed
	// backdrop of the previous screen. Currently consumed by every
	// edit form (Users, Groups, Rules, Policies); future write
	// surfaces opt in by implementing the same interface so the
	// modal routing scales without per-resource composeBody
	// branches.
	if child, ok := m.screens[m.active]; ok {
		if mr, ok := child.(ModalRenderer); ok {
			width := 74
			if capW := clampWidth(m.width) - 8; capW > 0 && capW < width {
				width = capW
			}
			if width < 60 {
				width = 60
			}
			bodyBudget := clampBodyLines(m.height) - 4
			modal := mr.RenderModal(activeTokens(), width, bodyBudget)
			return m.composeModalOverScreenDimmed(modal, m.previousScreenForBackdrop())
		}
	}
	if child, ok := m.screens[m.active]; ok {
		return child.View()
	}
	return "(loading…)"
}

// previousScreenForBackdrop returns the screen the v2 SCR-012 modal
// should dim under itself — the frame the operator was on before
// pressing `e` (D-W17 commentary). When the operator landed on the
// edit form via a fresh `:edit` palette command (root push), there's
// no previous frame; we fall back to ScreenUsers so the backdrop is
// at least the canonical list view.
func (m Model) previousScreenForBackdrop() Screen {
	if len(m.navStack) >= 2 {
		return m.navStack[len(m.navStack)-2]
	}
	return ScreenUsers
}

// composeModalOverScreenDimmed mirrors composeModalOverDimmedBody but
// dims the supplied bgScreen instead of m.active — used by the v2
// SCR-012 modal where active == ScreenUserEdit (dimming the form
// itself would be a no-op). The body lines come from the backdrop
// screen's View() (when present); the rest of the splice + center
// logic is identical so visual language stays uniform.
func (m Model) composeModalOverScreenDimmed(modal string, bgScreen Screen) string {
	tk := activeTokens()
	contentWidth := clampWidth(m.width) - 3
	bodyHeight := clampBodyLines(m.height)

	body := ""
	if child, ok := m.screens[bgScreen]; ok && child != nil {
		body = child.View()
	}
	bodyLines := strings.Split(body, "\n")
	for len(bodyLines) < bodyHeight {
		bodyLines = append(bodyLines, "")
	}
	bodyLines = bodyLines[:bodyHeight]

	modalRows := strings.Split(strings.TrimRight(modal, "\n"), "\n")
	modalWidth := 0
	for _, ml := range modalRows {
		if w := shared.VisibleWidth(ml); w > modalWidth {
			modalWidth = w
		}
	}
	leftCol := (contentWidth - modalWidth) / 2
	if leftCol < 0 {
		leftCol = 0
	}
	rightStart := leftCol + modalWidth
	topRow := (bodyHeight - len(modalRows)) / 2
	if topRow < 0 {
		topRow = 0
	}

	out := make([]string, len(bodyLines))
	for r, line := range bodyLines {
		plain := shared.StripCSI(line)
		modalRowIdx := r - topRow
		if modalRowIdx < 0 || modalRowIdx >= len(modalRows) {
			if plain == "" {
				out[r] = ""
			} else {
				out[r] = tk.Muted.Faint(true).Render(plain)
			}
			continue
		}
		left := shared.SliceVisiblePrefix(plain, leftCol)
		right := shared.SliceVisibleSuffix(plain, rightStart)
		var sb strings.Builder
		if leftCol > 0 {
			sb.WriteString(tk.Muted.Faint(true).Render(left))
		}
		sb.WriteString(modalRows[modalRowIdx])
		if right != "" {
			sb.WriteString(tk.Muted.Faint(true).Render(right))
		}
		out[r] = sb.String()
	}
	return strings.Join(out, "\n")
}

// composeModalOverDimmedBody renders the active child's body, dims
// every line, then centers the supplied modal over the result so
// rows above + below the popup remain visible (just darkened). The
// chrome's BodyLines budget is honoured: the dimmed body is padded /
// truncated to fit, and the modal stamps at vertical center within
// that window. Issue #199 v0.2.4 — replaces the body-replacing
// MountModal layout where confirmation popups blanked out the entire
// underlying screen.
func (m Model) composeModalOverDimmedBody(modal string) string {
	tk := activeTokens()
	contentWidth := clampWidth(m.width) - 3
	bodyHeight := clampBodyLines(m.height)

	// Pull the active child's body, then pad it out to the chrome's
	// BodyLines budget so dimming covers the whole popup window
	// (otherwise lists shorter than the budget show empty rows under
	// the modal that aren't dimmed).
	body := "(loading…)"
	if child, ok := m.screens[m.active]; ok {
		body = child.View()
	}
	bodyLines := strings.Split(body, "\n")
	for len(bodyLines) < bodyHeight {
		bodyLines = append(bodyLines, "")
	}
	bodyLines = bodyLines[:bodyHeight]

	// Compute the modal's footprint so we can splice it into body
	// rows and keep body content visible on either side.
	modalRows := strings.Split(strings.TrimRight(modal, "\n"), "\n")
	modalWidth := 0
	for _, ml := range modalRows {
		if w := shared.VisibleWidth(ml); w > modalWidth {
			modalWidth = w
		}
	}
	leftCol := (contentWidth - modalWidth) / 2
	if leftCol < 0 {
		leftCol = 0
	}
	rightStart := leftCol + modalWidth
	topRow := (bodyHeight - len(modalRows)) / 2
	if topRow < 0 {
		topRow = 0
	}

	// #U16 v0.2.5 — splice the modal into body rows column-wise so
	// content peeks through to the LEFT and RIGHT of the popup, with
	// the surrounding cells dimmed via Muted.Faint(true). Was
	// previously stamping over the entire row, which erased body
	// content on the modal's row range.
	out := make([]string, len(bodyLines))
	for r, line := range bodyLines {
		plain := shared.StripCSI(line)
		modalRowIdx := r - topRow
		if modalRowIdx < 0 || modalRowIdx >= len(modalRows) {
			// Outside the modal's row range: dim the whole row.
			if plain == "" {
				out[r] = ""
			} else {
				out[r] = tk.Muted.Faint(true).Render(plain)
			}
			continue
		}
		left := shared.SliceVisiblePrefix(plain, leftCol)
		right := shared.SliceVisibleSuffix(plain, rightStart)
		var sb strings.Builder
		if leftCol > 0 {
			sb.WriteString(tk.Muted.Faint(true).Render(left))
		}
		sb.WriteString(modalRows[modalRowIdx])
		if right != "" {
			sb.WriteString(tk.Muted.Faint(true).Render(right))
		}
		out[r] = sb.String()
	}
	return strings.Join(out, "\n")
}

// renderQuitConfirmModal builds the centered yellow-title modal for
// the `q` quit prompt (#U1 v0.2.4). Tone=Warning, distinct from the
// red Danger tone reserved for irreversible ops (Reset MFA, Delete);
// quitting ota is reversible — operators just relaunch.
func (m Model) renderQuitConfirmModal(tk shared.Tokens) string {
	body := tk.Warning.Render("Quit ota?")
	width := 60
	if w := shared.VisibleWidth(body) + 6; w > width {
		width = w
	}
	if cap := clampWidth(m.width) - 8; cap > 0 && width > cap {
		width = cap
	}
	return shared.MountModal(shared.ModalIn{
		Title:  "Quit",
		Body:   body,
		Footer: "y / Enter to quit — n / Esc to cancel",
		Tone:   shared.ModalToneWarning,
		Width:  width,
		Tokens: tk,
	})
}

// renderActionConfirmModal builds the centered red-title modal that
// gates destructive Users / Group Rule lifecycle ops (issue #195
// v0.2.4). Mirrors the format the palette help and quit-confirm use,
// so the visual language stays consistent.
func (m Model) renderActionConfirmModal(tk shared.Tokens) string {
	var label, target string
	switch {
	case m.pendingRule.Kind != RuleActionNone:
		label = ruleActionLabel(m.pendingRule.Kind)
		target = m.pendingRule.Rule.Name
		if target == "" {
			target = m.pendingRule.Rule.ID
		}
	case m.pendingAction.Kind != UserActionNone:
		label = userActionLabel(m.pendingAction.Kind)
		target = m.pendingAction.User.Profile.Login
		if target == "" {
			target = m.pendingAction.User.ID
		}
	default:
		label = "Confirm action"
		target = ""
	}
	body := tk.Danger.Render(label) + " for " + tk.Accent.Render(target) + "?"
	width := 60
	if w := shared.VisibleWidth(body) + 6; w > width {
		width = w
	}
	if cap := clampWidth(m.width) - 8; cap > 0 && width > cap {
		width = cap
	}
	return shared.MountModal(shared.ModalIn{
		Title:  "Confirm action",
		Body:   body,
		Footer: "y / Enter to confirm — n / Esc to cancel",
		Tone:   shared.ModalToneDanger,
		Width:  width,
		Tokens: tk,
	})
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
	case ScreenApps:
		return "Apps"
	case ScreenAuthenticators:
		return "Authenticators"
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
		// v0.2.4 #197 — centered modal popup replaces the inline
		// footer band; composeBody renders the modal directly.
		return ""
	case OverlayActionConfirm:
		// v0.2.4 #195 — centered modal popup replaces the inline
		// footer band; composeBody renders the modal directly.
		return ""
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

// ServerFilterStater is implemented by screens that own a `F`
// server-filter prompt (today: Logs, mirroring Okta's filter= API
// param). Distinct from QueryStater so the chrome can render an
// `F`-prefixed floating box separate from the `Q`-prefixed query
// box. (#F4 v0.2.5)
type ServerFilterStater interface {
	ServerFilterEditing() bool
	ServerFilterInput() string
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
// App Shell uses it to choose between forwarding Esc to the screen
// (escWillAct → true) and firing the quit confirm (escWillAct →
// false at the navigation root). Pre-2026-05-04 it gated a
// `nothing to close` toast; the nav-stack rewrite replaced that
// toast with the back-pop / quit-confirm precedence.
type EscapeOpStater interface {
	EscapeWillAct() bool
}

// escWillAct reports whether the active screen would do something
// in response to Esc (close detail, clear filter, exit Visual,
// abort Type-Picker, etc.). When false at the root frame, the App
// Shell fires the quit confirm.
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

// escIsCritical reports whether Esc has a transient, local-only
// meaning that must run BEFORE the nav-stack pop fires. Critical
// states are operator inputs the back-nav shouldn't preempt:
//
//   - any open overlay (palette / help / action menu / quit
//     confirm / API recorder) — those have their own Esc handlers
//   - filter / query / server-filter input modes (operator is
//     mid-typing; Esc cancels the input box)
//   - visual-line selection in a detail body
//
// Settled / applied filters on a list are NOT critical — they're
// part of the screen's persistent state, and the back-nav contract
// preserves that state when the frame is later revisited. Letting
// the pop fire on a filtered list keeps the operator's mental
// model "Esc walks back" intact regardless of in-screen filters.
func (m Model) escIsCritical() bool {
	if m.overlay != OverlayNone {
		return true
	}
	child, ok := m.screens[m.active]
	if !ok {
		return false
	}
	if st, ok := child.(filterInputStater); ok && st.Filtering() {
		return true
	}
	if st, ok := child.(QueryStater); ok && st.QueryEditing() {
		return true
	}
	if st, ok := child.(ServerFilterStater); ok && st.ServerFilterEditing() {
		return true
	}
	if st, ok := child.(visualActiveStater); ok && st.DetailVisualActive() {
		return true
	}
	// Write-surface screens (REQ-W01) can block nav-pop on Esc while
	// they hold unsaved state — either a dirty form (need to surface
	// the discard confirm before destroying edits) or an in-flight
	// save (AC-4.3 / AC-5.3 — Esc must be inert until the POST
	// resolves; Ctrl+C is the only abort path).
	if st, ok := child.(escapeBlocksPopStater); ok && st.EscapeBlocksPop() {
		return true
	}
	return false
}

// escapeBlocksPopStater is implemented by write-surface screens that
// own state Esc must not destroy by popping the nav frame. Distinct
// from EscapeOpStater (which gates the post-pop quit confirm) —
// EscapeBlocksPop runs BEFORE pop and short-circuits it entirely.
type escapeBlocksPopStater interface {
	EscapeBlocksPop() bool
}

// ModalRenderer is implemented by screens that render as a centered
// modal over a dimmed backdrop instead of as a full-screen body. The
// App Shell's composeBody routes through this interface — any screen
// that implements RenderModal automatically opts into the v2 popup
// pattern, no per-resource composeBody branches required.
//
// width is the modal's outer cell count (already clamped to the
// chrome's content rectangle); bodyBudget is the max body line count
// before the chrome footer takes over. The implementation returns
// the full MountModal string (rounded border + title + body +
// footer).
type ModalRenderer interface {
	RenderModal(tk shared.Tokens, width, bodyBudget int) string
}

// visualActiveStater is implemented by detail surfaces that own a
// visual-line selection (Users today; future detail screens via
// shared.BodyCursor). Distinct from EscapeOpStater — only "is
// visual mode currently active?" matters for the Esc precedence.
type visualActiveStater interface {
	DetailVisualActive() bool
}

// filterInputStater narrows FilterStater to the boolean check the
// nav-stack-aware Esc routing actually needs (so the precedence
// decision doesn't have to allocate / format the filter string).
type filterInputStater interface {
	Filtering() bool
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

// activeChildIsServerFilterEditing reports whether the active screen
// has its `F` server-filter prompt open (#F4 v0.2.5).
func (m Model) activeChildIsServerFilterEditing() bool {
	child, ok := m.screens[m.active]
	if !ok {
		return false
	}
	fs, ok := child.(ServerFilterStater)
	return ok && fs.ServerFilterEditing()
}

// activeChildServerFilterInput returns the in-progress filter buffer.
func (m Model) activeChildServerFilterInput() string {
	child, ok := m.screens[m.active]
	if !ok {
		return ""
	}
	fs, ok := child.(ServerFilterStater)
	if !ok {
		return ""
	}
	return fs.ServerFilterInput()
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

// FetchingStater is implemented by list screens that publish whether
// an auto-refresh / on-demand fetch is currently in flight (#U10
// v0.2.4). The App Shell stamps a `↻` glyph next to the upper-divider
// timestamp while Fetching=true so operators see refresh activity on
// slow networks.
type FetchingStater interface {
	Fetching() bool
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
	stamp := "updated " + t.UTC().Format("15:04:05") + " UTC"
	// #U10 v0.2.4 — append a refresh glyph while a fetch is in
	// flight so operators see auto-refresh activity on slow nets.
	if fs, ok := child.(FetchingStater); ok && fs.Fetching() {
		stamp = "↻ " + stamp
	}
	return stamp
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
// #U13 v0.2.4 — explicit "client filter" hint distinguishes it from
// the server-side `Q` query box; the prompt stays Accent (cyan).
func (m Model) renderFilterBox(tk shared.Tokens, innerWidth int) string {
	prompt := tk.Accent.Render("/ ")
	input := ""
	if child, ok := m.screens[m.active]; ok {
		if fs, ok := child.(FilterStater); ok {
			input = fs.Filter()
		}
	}
	cursor := tk.RowCursor.Render(" ")
	hint := "\n" + tk.Muted.Render("client filter · narrows loaded rows · Esc cancels")
	return modalBox(prompt+input+cursor+hint, innerWidth, tk)
}

// renderServerFilterBox builds the floating input box for the `F`
// server-side filter prompt (#F4 v0.2.5). Yellow tone matches the
// Q box (server-side, hits the API) but the prefix glyph + hint
// distinguish them.
func (m Model) renderServerFilterBox(tk shared.Tokens, innerWidth int) string {
	prompt := tk.Warning.Render("F ")
	input := m.activeChildServerFilterInput()
	cursor := tk.RowCursor.Render(" ")
	hint := "\n" + tk.Warning.Render("server filter") +
		tk.Muted.Render(" · Okta filter expression · Enter applies · Esc cancels")
	return modalBox(prompt+input+cursor+hint, innerWidth, tk)
}

// renderQueryBox builds the floating input box for the `Q`
// server-side query mode (issue #185 v0.2.1). #U13 v0.2.4 — prompt
// + hint use the Warning tone (yellow) so operators see at a glance
// that this box hits the API, distinct from the cyan client-side
// `/` filter.
func (m Model) renderQueryBox(tk shared.Tokens, innerWidth int) string {
	prompt := tk.Warning.Render("Q ")
	input := m.activeChildQueryInput()
	cursor := tk.RowCursor.Render(" ")
	hint := "\n" + tk.Warning.Render("server search") + tk.Muted.Render(" · re-fetches from Okta · Enter applies · Esc cancels")
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

// stampOverlayAtBottom replaces the last N rows of body with overlay
// (where N == overlay's line count). Pads the body up to bodyLines
// rows first so the band always lands at the visible bottom of the
// chrome's body region, not at the bottom of a short list. Used by
// the floating toast band (#U4 v0.2.4).
func stampOverlayAtBottom(overlay, body string, bodyLines int) string {
	overlayRows := strings.Split(overlay, "\n")
	bodyRows := strings.Split(body, "\n")
	for len(bodyRows) < bodyLines {
		bodyRows = append(bodyRows, "")
	}
	if len(bodyRows) > bodyLines {
		bodyRows = bodyRows[:bodyLines]
	}
	start := len(bodyRows) - len(overlayRows)
	if start < 0 {
		start = 0
	}
	for i, ol := range overlayRows {
		idx := start + i
		if idx < 0 || idx >= len(bodyRows) {
			continue
		}
		bodyRows[idx] = ol
	}
	return strings.Join(bodyRows, "\n")
}

// stampOverlayAtRow replaces body lines starting at topRow with
// overlayLines, leaving rows above and below intact. Used by the
// confirmation modals (issue #199 v0.2.4) so the dimmed body shows
// around the popup instead of getting blanked out.
func stampOverlayAtRow(overlay, body string, topRow int) string {
	overlayLines := strings.Split(overlay, "\n")
	bodyLines := strings.Split(body, "\n")
	out := make([]string, len(bodyLines))
	copy(out, bodyLines)
	for i, ol := range overlayLines {
		idx := topRow + i
		if idx < 0 || idx >= len(out) {
			continue
		}
		out[idx] = ol
	}
	return strings.Join(out, "\n")
}

// dimBody renders each body line in the muted style so the popup
// modal floats over a darkened backdrop (issue #199 v0.2.4). Existing
// ANSI styling is stripped first so the muted color applies uniformly
// — otherwise the body's status-row tints would override Faint.
func dimBody(body string, tk shared.Tokens) string {
	if tk.Muted.GetForeground() == nil {
		return body
	}
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		stripped := shared.StripCSI(l)
		if stripped == "" {
			continue
		}
		lines[i] = tk.Muted.Faint(true).Render(stripped)
	}
	return strings.Join(lines, "\n")
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

// activeTokens picks the token set. NO_COLOR forces Monochrome.
// Otherwise an OTA_THEME env var override (when set to "dark" /
// "light" / "high-contrast" / "monochrome") wins; absent that,
// COLORFGBG-based detection picks Light on light terminals and
// falls back to Dark. Called per View() so a runtime toggle takes
// effect immediately. Issue #U12 v0.2.5.
func activeTokens() shared.Tokens {
	return shared.PickTheme(shared.ResolveTheme(os.Getenv("OTA_THEME")))
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
	case ScreenAuthenticators:
		mdl := authenticators.NewListModel(authenticators.Deps{
			Port:            m.deps.AuthenticatorsPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenNetworkZones:
		mdl := zones.New(zones.Deps{
			Port:            m.deps.NetworkZonesPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenAuthorizationServers:
		mdl := authservers.New(authservers.Deps{
			Port:            m.deps.AuthorizationServersPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenAPITokens:
		mdl := apitokens.New(apitokens.Deps{
			Port:            m.deps.APITokensPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenAdministrators:
		mdl := admins.New(admins.Deps{
			Port:            m.deps.AdministratorsPort,
			Clock:           m.deps.Clock,
			Logger:          m.deps.Logger,
			Keys:            m.deps.Keys,
			Width:           m.width,
			Height:          m.height,
			RefreshInterval: m.deps.DefaultRefreshInterval,
		})
		return mdl, mdl.Init()
	case ScreenUserEdit:
		// REQ-W01 SCR-012 — the Users Edit Form is built once per
		// entry so each open fires its own GET (AC-1.3). The
		// OpenUserEditMsg handler clears any cached instance before
		// calling ensureScreen.
		svc := m.userEditService()
		if svc == nil {
			return nil, nil
		}
		mdl := users.NewEditModel(users.EditDeps{
			Svc:    svc,
			UserID: m.editTargetID,
			Clock:  m.deps.Clock,
			Logger: m.deps.Logger,
			Width:  m.width,
			Height: m.height,
		})
		return mdl, mdl.Init()
	case ScreenGroupEdit:
		// Groups profile edit — same lifecycle contract as
		// ScreenUserEdit. Type guard happens upstream (list / detail
		// only emits OpenGroupEditMsg for OKTA_GROUP).
		svc := m.groupEditService()
		if svc == nil {
			return nil, nil
		}
		mdl := groups.NewEditModel(groups.EditDeps{
			Svc:     svc,
			GroupID: m.groupEditTargetID,
			Clock:   m.deps.Clock,
			Logger:  m.deps.Logger,
			Width:   m.width,
			Height:  m.height,
		})
		return mdl, mdl.Init()
	case ScreenRuleEdit:
		// Group rule edit — Status guard (INACTIVE / INVALID) happens
		// upstream at the list / detail key handler.
		svc := m.ruleEditService()
		if svc == nil {
			return nil, nil
		}
		mdl := rules.NewEditModel(rules.EditDeps{
			Svc:    svc,
			RuleID: m.ruleEditTargetID,
			Clock:  m.deps.Clock,
			Logger: m.deps.Logger,
			Width:  m.width,
			Height: m.height,
		})
		return mdl, mdl.Init()
	case ScreenPolicyEdit:
		// Policy edit — metadata only (no per-rule editing in v0.2).
		svc := m.policyEditService()
		if svc == nil {
			return nil, nil
		}
		mdl := policies.NewEditModel(policies.EditDeps{
			Svc:      svc,
			PolicyID: m.policyEditTargetID,
			Clock:    m.deps.Clock,
			Logger:   m.deps.Logger,
			Width:    m.width,
			Height:   m.height,
		})
		return mdl, mdl.Init()
	}
	// Detail views are populated by drill-down handlers; not auto-built.
	return nil, nil
}

// groupEditService mirrors userEditService — prefer the pre-built
// Bundle, fall back to a port-only constructor for tests.
func (m Model) groupEditService() *service.GroupsService {
	if m.deps.Services != nil && m.deps.Services.Groups != nil {
		return m.deps.Services.Groups
	}
	if m.deps.GroupsPort == nil {
		return nil
	}
	return service.NewGroupsService(m.deps.GroupsPort, m.deps.GroupRulesPort,
		service.WithClock(m.deps.Clock), service.WithLogger(m.deps.Logger))
}

// ruleEditService mirrors groupEditService for the rule edit form.
func (m Model) ruleEditService() *service.GroupRulesService {
	if m.deps.Services != nil && m.deps.Services.Rules != nil {
		return m.deps.Services.Rules
	}
	if m.deps.GroupRulesPort == nil {
		return nil
	}
	return service.NewGroupRulesService(m.deps.GroupRulesPort, m.deps.GroupsPort,
		service.WithClock(m.deps.Clock), service.WithLogger(m.deps.Logger))
}

// policyEditService mirrors ruleEditService for the Policy edit form.
func (m Model) policyEditService() *service.PoliciesService {
	if m.deps.Services != nil && m.deps.Services.Policies != nil {
		return m.deps.Services.Policies
	}
	if m.deps.PoliciesPort == nil {
		return nil
	}
	return service.NewPoliciesService(m.deps.PoliciesPort,
		service.WithClock(m.deps.Clock), service.WithLogger(m.deps.Logger))
}

// userEditService returns the UsersService the EditModel saves through.
// Prefers the pre-built service.Bundle (production wiring) but falls
// back to a port-only constructor so tests that only inject UsersPort
// still get a working save path.
func (m Model) userEditService() *service.UsersService {
	if m.deps.Services != nil && m.deps.Services.Users != nil {
		return m.deps.Services.Users
	}
	if m.deps.UsersPort == nil {
		return nil
	}
	var opts []service.ServiceOption
	if m.deps.Clock != nil {
		opts = append(opts, service.WithClock(m.deps.Clock))
	}
	return service.NewUsersService(m.deps.UsersPort, opts...)
}

// --- Key handling --------------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// #U5 v0.2.4 — any keypress dismisses the floating toast band so
	// operators can clear it manually without waiting for Until. Bump
	// the toast generation so the in-flight clear tick doesn't stomp
	// a future toast set in the same Update pass.
	if m.toast != nil {
		m.toast = nil
		m.toastGen++
	}

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
	if m.overlay == OverlayStatusPicker {
		return m.handleStatusPickerKey(msg)
	}
	if m.overlay == OverlayAPIRecorder {
		return m.handleAPIRecorderKey(msg)
	}

	// 2026-05-04 nav stack: Esc walks the navigation history.
	//
	// Precedence:
	//   1. Critical local modes (operator typing into a `/` filter
	//      prompt, `Q` query box, `F` server-filter prompt, or with
	//      visual-line selection active) — Esc cancels the input
	//      first and is forwarded to the active screen as normal.
	//   2. When the stack has more than one frame: Esc pops back
	//      to the previous frame. Applied filters and open details
	//      on the popped frame are preserved so re-visiting feels
	//      stable.
	//   3. At the root frame, Esc walks the active screen's local
	//      state first (close open detail, clear applied filter)
	//      via the existing in-screen Esc handler. Once the screen
	//      reports nothing to close, the next Esc fires the quit
	//      confirm so the operator's "back-back-back" gesture
	//      always has somewhere to land.
	if msg.Type == tea.KeyEsc && !m.escIsCritical() {
		if m.canPopNav() {
			prev, _ := m.popNav()
			updated, cmd := m.ensureScreen(prev)
			return updated, cmd
		}
		if !m.escWillAct() {
			return m, quitConfirmCmd()
		}
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
		case "R":
			// Global "refresh active screen" — emits the shared
			// RefreshScreenMsg the active list / detail consumes as
			// an out-of-band re-fetch trigger. Help advertised this
			// since v0.2.0; the wiring was simply missing until now.
			return m, refreshScreenCmd()
		case "~":
			// Global Okta API timeline overlay. Disabled when no
			// recorder was wired (e.g. tests).
			if m.deps.APIRecorder != nil {
				return m, openAPIRecorderCmd()
			}
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
		case paletteCmdHelp:
			// #U14 v0.2.4 — `:help` opens the help overlay. Same
			// path the `?` shortcut takes; lets operators discover
			// the help surface via palette autocomplete.
			return m, openHelpCmd()
		case paletteCmdAPILog:
			// `:apilog` opens the API timeline overlay; same path
			// `~` takes. Disabled when no recorder was wired.
			if m.deps.APIRecorder == nil {
				return m, toastCmdInfo("API recorder unavailable")
			}
			return m, openAPIRecorderCmd()
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
		"authenticator",
		// Read-only resource surfaces added in v0.2.5+. Each has a
		// matching screenFromName branch + Spec under
		// internal/tui/<pkg>.
		"network-zone",
		"authorization-server",
		"api-token",
		"administrator",
		"unmask", "mask",
		"reset-password", "unlock", "reset-mfa",
		// REQ-W01: `:edit` is the canonical SCR-012 palette entry
		// (TUI_DESIGN §3.4 / §11.2a). Surfaced in autocomplete so
		// operators discover it via Tab.
		"edit",
		"apilog",
		"help", "quit",
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
	if m.overlay == OverlayQuitConfirm {
		// Issue #200 v0.2.4 — Enter is the canonical "OK" key on
		// confirmation popups, alongside y/Y.
		if msg.Type == tea.KeyEnter {
			return m, tea.Quit
		}
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "y", "Y":
				return m, tea.Quit
			case "n", "N":
				m.overlay = OverlayNone
			}
		}
	}
	if m.overlay == OverlayActionConfirm {
		confirmed := msg.Type == tea.KeyEnter
		cancelled := false
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "y", "Y":
				confirmed = true
			case "n", "N":
				cancelled = true
			}
		}
		switch {
		case confirmed:
			// v0.2.2 #188 — pendingRule and pendingAction are
			// mutually exclusive (openActionConfirm /
			// openRuleActionConfirm clear the other). Fire the
			// correct dispatcher based on which is set.
			// Issue #192 v0.2.3 — chain a screen refresh after the
			// action completes so the operator sees the new state
			// without waiting for the next auto-tick.
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
		case cancelled:
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

// handleStatusPickerKey routes key input while OverlayStatusPicker is
// open. Esc / `s` cancel; Enter routes the highlighted transition
// into the destructive-action confirm gate (OverlayActionConfirm) so
// the user re-confirms the lifecycle flip — every transition here
// has an irreversible side effect (suspend, deprovision, delete).
// j/k/↑/↓ advance the cursor inside the model.
func (m Model) handleStatusPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = OverlayNone
		m.statusPicker = nil
		return m, nil
	case tea.KeyEnter:
		if m.statusPicker == nil {
			m.overlay = OverlayNone
			return m, nil
		}
		picked, ok := m.statusPicker.Selected()
		user := m.statusPicker.user
		m.overlay = OverlayNone
		m.statusPicker = nil
		if !ok {
			return m, nil
		}
		// Hand off to the destructive-action confirm path so the
		// operator gets the same "are you sure?" guardrail they'd
		// see from :deactivate / :delete / `a` menu.
		m.pendingAction = pendingUserAction{Kind: picked.Action, User: user}
		m.pendingRule = pendingRuleAction{}
		m.overlay = OverlayActionConfirm
		return m, nil
	case tea.KeyRunes:
		if string(msg.Runes) == "s" {
			// Toggle off — symmetric with `a` for the action menu.
			m.overlay = OverlayNone
			m.statusPicker = nil
			return m, nil
		}
	}
	if m.statusPicker != nil {
		updated, cmd := m.statusPicker.Update(msg)
		if sm, ok := updated.(StatusPickerModel); ok {
			m.statusPicker = &sm
		}
		return m, cmd
	}
	return m, nil
}

// handleAPIRecorderKey routes keys for the global Okta API timeline
// overlay. Esc and `~` close it (toggle); everything else flows
// through the model's own Update so j/k/Tab/g/G/R behaviors live in
// one place.
func (m Model) handleAPIRecorderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = OverlayNone
		m.apiRecorderModel = nil
		return m, nil
	case tea.KeyRunes:
		if string(msg.Runes) == "~" {
			m.overlay = OverlayNone
			m.apiRecorderModel = nil
			return m, nil
		}
	}
	if m.apiRecorderModel != nil {
		updated, cmd := m.apiRecorderModel.Update(msg)
		if ar, ok := updated.(overlay.APIRecorderModel); ok {
			m.apiRecorderModel = &ar
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
	// paletteCmdHelp opens the help overlay (#U14 v0.2.4). Same Cmd
	// the `?` shortcut fires; exposed as a palette command so the
	// route is discoverable via Tab-autocomplete.
	paletteCmdHelp
	// paletteCmdAPILog opens the global Okta API timeline overlay —
	// same Cmd the `~` shortcut fires.
	paletteCmdAPILog
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
	case "help", "h", "?":
		return paletteCmdHelp, 0, "", true
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
	case "apilog", "api-log", "api_log", "apitimeline":
		return paletteCmdAPILog, 0, "", true
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
// (Action types + open*Confirm + run*ActionCmd + actionCompletedMsg
// + label helpers moved to actions.go in #A2 v0.2.4.)

// (toastInfo / toastErr / toastCmdInfo / toastCmdError /
// scheduleToastClearCmd / renderToastBand / toastClearMsg moved to
// toast.go in #A2 v0.2.4.)

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

func offlineCmd(offline bool) tea.Cmd {
	return func() tea.Msg { return OfflineStateMsg{Offline: offline} }
}

// refreshScreenCmd returns a Cmd that fires shared.RefreshScreenMsg
// — handled by every list / detail screen as an out-of-band fetch
// trigger. Used by the action confirm flow (issue #192 v0.2.3) to
// reflect destructive ops in the active surface immediately.
func refreshScreenCmd() tea.Cmd {
	return func() tea.Msg { return shared.RefreshScreenMsg{} }
}

// Internal activation markers consumed by overlay models once wired.
type openCmdPaletteMsg struct{}
type openHelpMsg struct{}
type openActionMenuMsg struct{}
type openAPIRecorderMsg struct{}

func openAPIRecorderCmd() tea.Cmd {
	return func() tea.Msg { return openAPIRecorderMsg{} }
}

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
