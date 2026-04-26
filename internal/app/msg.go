package app

import (
	"time"

	"github.com/tedilabs/ota/internal/domain"
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

// ToastMsg is a short status-bar message.
type ToastMsg struct {
	Text  string
	Level ToastLevel
	Until time.Time // auto-dismiss
}

// ToastLevel categorizes toast severity.
type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastSuccess
	ToastWarn
	ToastError
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

// RefreshActiveScreenMsg asks the active screen to re-fetch (REQ-E03 AC-3).
type RefreshActiveScreenMsg struct{}

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

// OpenResourceMsg requests a drill-down to a resource's detail view
// (REQ-U05 AC-1). Kind is one of "user", "group", "rule", "policy",
// "log"; ID is the Okta resource identifier.
type OpenResourceMsg struct {
	Kind string
	ID   string
}
