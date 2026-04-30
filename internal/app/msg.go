package app

import (
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// Global message types broadcast via the app shell.

// RateLimitObservedMsg is fanned out when the adapter records a new snapshot
// (REQ-E01 AC-1). The statusbar subscribes; categories are shown in :ratelimit.
type RateLimitObservedMsg struct {
	Snapshot domain.RateLimitSnapshot
}

// ErrorMsg wraps a non-fatal error for toast + :errors history (REQ-E02).
type ErrorMsg struct {
	Err    error
	Source string // e.g., "users.list", "logs.tail.poll"
}

// ToastMsg is the canonical toast type now living in shared so any
// TUI package can emit one without an import cycle (#A7 v0.2.4).
// Re-exported here as type aliases so existing app.ToastMsg /
// app.ToastError callers keep working.
type ToastMsg = shared.ToastMsg
type ToastLevel = shared.ToastLevel

const (
	ToastInfo    = shared.ToastInfo
	ToastSuccess = shared.ToastSuccess
	ToastWarn    = shared.ToastWarn
	ToastError   = shared.ToastError
)

// ProfileSwitchStartedMsg triggers "Switching to <name>…" toast and cache
// invalidation (REQ-C02 AC-3).
type ProfileSwitchStartedMsg struct{ Target string }

// ProfileSwitchedMsg fires when re-init completes.
type ProfileSwitchedMsg struct{ Active string }

// QuitConfirmRequestMsg asks the overlay to show quit confirmation (REQ-U07).
type QuitConfirmRequestMsg struct{}

// NetworkErrorMsg signals a connectivity failure observed by the adapter or
// service layer (REQ-E03). The App Shell transitions to offline state.
type NetworkErrorMsg struct {
	Err    error
	Source string
}

// NetworkRestoredMsg fires when connectivity is observed to resume
// (REQ-E03 AC-3). The App Shell triggers a refresh of the active screen.
type NetworkRestoredMsg struct{}

// OfflineStateMsg is the effect of a NetworkErrorMsg: flips the statusbar
// indicator. Offline=false clears it.
type OfflineStateMsg struct {
	Offline bool
}

// ScreenChangeMsg requests a switch of the active resource screen (REQ-U02 AC-1).
// Used by the palette after a `:users`/`:groups`/… command resolves.
type ScreenChangeMsg struct {
	Target Screen
}

// SwitchScreenMsg requests a screen switch by name (REQ-U02 AC-1). Accepts
// strings like "users", "groups", "grouprules", "policies", "logs", or a
// detail form like "user" for drill-down. Consumed by App Shell, which
// translates to an internal Screen enum.
type SwitchScreenMsg struct {
	Target string
}

// OpenPolicyTypeMsg jumps directly to the Policies list scoped to a
// specific PolicyType — issue #165's `:okta-sign-on` /
// `:password-policy` / etc. palette routes. The App Shell rebuilds
// the Policies Wrapper with NewWrapperForType so the type picker
// doesn't reappear.
type OpenPolicyTypeMsg struct {
	Type string // domain.PolicyType as string for cross-package compat
}

// OpenAppTypeMsg jumps directly to the Apps list scoped to a
// specific AppType — issue #166's `:saml-app` / `:oidc-app` /
// `:bookmark-app` etc. palette routes. The App Shell rebuilds the
// Apps Wrapper with NewWrapperForType so the picker doesn't render.
type OpenAppTypeMsg struct {
	Type string // domain.AppType as string for cross-package compat
}

// OpenResourceMsg requests a drill-down to a resource's detail view
// (REQ-U05 AC-1). Kind is one of "user", "group", "rule", "policy",
// "log"; ID is the Okta resource identifier.
type OpenResourceMsg struct {
	Kind string
	ID   string
}

// OpenGroupDetailMsg / OpenAppDetailMsg are re-exported from
// internal/tui/shared so callers that only depend on internal/app can
// keep referencing them by their App Shell name (issue #171). The
// concrete types live in shared to avoid a tui→app import cycle when
// child screens (e.g. Users detail) emit drill-down requests.
type (
	OpenGroupDetailMsg = shared.OpenGroupDetailMsg
	OpenAppDetailMsg   = shared.OpenAppDetailMsg
	OpenUserDetailMsg  = shared.OpenUserDetailMsg
)
