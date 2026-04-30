package shared

import "time"

// UnmaskFieldMsg asks the active Detail screen to reveal a specific PII
// field. Sent by the App Shell when the operator types :unmask <field>
// into the command palette (issue #115). Lives here in shared so both
// app/ and tui/* can reference it without a cycle.
type UnmaskFieldMsg struct{ Field string }

// MaskAllMsg asks the active Detail screen to re-mask every previously
// unmasked field. Sent on the bare :mask command (no argument).
type MaskAllMsg struct{}

// OpenGroupDetailMsg asks the App Shell to switch to the Groups screen
// and open the detail surface for the given group ID. Emitted by any
// screen offering a drill-down — issue #171's User Detail Groups row
// Enter is the first source. Lives in shared so user/group/app
// packages can avoid an import cycle through internal/app.
type OpenGroupDetailMsg struct{ ID string }

// OpenAppDetailMsg is the Apps counterpart of OpenGroupDetailMsg —
// drill into an app's detail surface from another screen (issue #171).
type OpenAppDetailMsg struct{ ID string }

// OpenUserDetailMsg is the Users counterpart — drill into a user's
// detail surface from another screen (#G2 / U7 v0.2.4). ID may be a
// userID or a login (Okta APIs accept either). Used by Group Detail
// Members box drill-down and Log Detail actor drill-down.
type OpenUserDetailMsg struct{ ID string }

// ActionItem is a single row in the resource action menu (issue
// #175 v0.1.15). Each screen exposes Actions() []ActionItem; the
// App Shell builds the picker around them and dispatches RunAction
// with the chosen ID. ID is opaque to the overlay — the screen
// owning the action interprets it.
type ActionItem struct {
	ID    string
	Label string
	Hint  string
}

// RunUserActionMsg routes a user-lifecycle action picked from the
// `a` menu (issue #175) into the App Shell's existing confirmation
// flow. Kind is the canonical `:` palette command name —
// "reset-password" / "unlock" / "reset-factors" / "activate" /
// "deactivate" / "expire-password" / "delete". Lives in shared so
// the users list can emit it without an app→tui→app cycle.
type RunUserActionMsg struct{ Kind string }

// RunRuleActionMsg is the Group Rule counterpart to
// RunUserActionMsg (issue #188 v0.2.2). Kind ∈ {"activate",
// "deactivate", "delete"}.
type RunRuleActionMsg struct{ Kind string }

// RefreshScreenMsg asks the active screen to re-fetch its data
// without waiting for the next auto-refresh tick. Issue #192
// v0.2.3 — surfaced after destructive actions complete (Activate /
// Deactivate / Reset Password / etc) so the list/detail reflects
// the new state immediately, and after network-restored events.
// Each list / detail screen handles this by firing its main fetch
// Cmd; the auto-refresh tick chain continues unchanged.
type RefreshScreenMsg struct{}

// ToastLevel categorizes toast severity. Issue #A7 v0.2.4 — moved
// from internal/app to shared so any TUI package can emit a toast
// without an import cycle through the App Shell.
type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastSuccess
	ToastWarn
	ToastError
)

// ActionFailedMsg is broadcast by the App Shell when a destructive
// action errored against a specific resource ID, so the active list
// can flash that row red (#U11 v0.2.4). The list looks up the row by
// TargetID and stamps a `failedAt` timestamp; View renders RowDanger
// for HighlightWindow.
type ActionFailedMsg struct {
	TargetID string
}

// ToastMsg is a transient message rendered as a color-coded floating
// band (success: green ✓, error: red ✗, warn: yellow !, info: accent
// •). Until is the auto-dismiss time (zero → defaults to ~3s).
// Issue #A7 v0.2.4 — single canonical toast type that consolidates
// the prior `app.ToastMsg`, the chrome's right-anchored statusToast
// slot, and the per-screen `detailToast` strings.
type ToastMsg struct {
	Text  string
	Level ToastLevel
	Until time.Time
}
