package shared

import (
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

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

// OpenUserEditMsg is the entry-point message for REQ-W01 (SCR-012
// Users Edit Form). Emitted by the Users list / detail when the
// operator presses `e`, by the `:edit` palette command, or by any
// surface that wants to drill into the Edit screen. The App Shell
// pushes ScreenUserEdit onto the nav stack and forwards the ID to
// the new EditModel which fires `GET /api/v1/users/{id}` (AC-1.3).
// ID may be a userID or login (Okta accepts either, same as
// OpenUserDetailMsg).
type OpenUserEditMsg struct{ ID string }

// UserUpdatedMsg is broadcast by EditModel after a successful save
// (REQ-W01 AC-4.5). The User is the server-echoed snapshot — list
// and detail screens use it to patch their cache with the last
// authoritative profile so the next render reflects the change
// without an extra fetch.
type UserUpdatedMsg struct{ User domain.User }

// UserEditDiscardedMsg is emitted by EditModel when the operator
// confirms the discard option on the unsaved-changes prompt — i.e.,
// they want to throw away every pending edit and exit the form.
// The App Shell pops the ScreenUserEdit frame back to whichever
// surface pushed it (list / detail) so Esc-then-Enter feels like a
// single "back" gesture rather than two distinct decisions.
type UserEditDiscardedMsg struct{}

// OpenGroupEditMsg is the entry-point message for the Groups edit
// form (parallel to OpenUserEditMsg). The App Shell pushes
// ScreenGroupEdit onto the nav stack and forwards the group ID to
// the new EditModel. Only OKTA_GROUP types support edit — the
// list / detail key handler guards on Type before emitting this.
type OpenGroupEditMsg struct{ ID string }

// GroupUpdatedMsg broadcasts the post-save Group snapshot so the
// Groups list / detail patches its cache with the last authoritative
// profile without an extra GET.
type GroupUpdatedMsg struct{ Group domain.Group }

// GroupEditDiscardedMsg parallels UserEditDiscardedMsg — emitted on
// "Discard and exit", consumed by the App Shell to popNav the
// ScreenGroupEdit frame.
type GroupEditDiscardedMsg struct{}

// OpenRuleEditMsg / RuleUpdatedMsg / RuleEditDiscardedMsg are the
// Group Rule edit-form counterparts of the Users / Groups messages.
// The list/detail key handler emits OpenRuleEditMsg after a Status
// check (only INACTIVE / INVALID rules accept edits).
type OpenRuleEditMsg struct{ ID string }
type RuleUpdatedMsg struct{ Rule domain.GroupRule }
type RuleEditDiscardedMsg struct{}

// OpenPolicyEditMsg / PolicyUpdatedMsg / PolicyEditDiscardedMsg are
// the Policy edit-form counterparts. System policies refuse status /
// priority changes — the upstream 400 surfaces as an inline error;
// no client-side guard.
type OpenPolicyEditMsg struct{ ID string }
type PolicyUpdatedMsg struct{ Policy domain.Policy }
type PolicyEditDiscardedMsg struct{}

// OpenStatusPickerMsg is the entry-point message for the user status
// picker. Emitted by the Users list / detail when the operator
// presses `s` with a user selected. The App Shell uses the embedded
// User snapshot to compute valid lifecycle transitions and opens
// OverlayStatusPicker. Carrying the whole User (not just an ID)
// lets the picker render the current status badge in its title
// without an extra fetch.
type OpenStatusPickerMsg struct{ User domain.User }

// OpenLogsMsg switches the active screen to Logs and pre-fills the
// server-side `filter=` parameter with `Filter` (an Okta System Log
// filter expression, e.g., `target.id eq "00uABC"`) so the operator
// lands on log events involving that resource (#F2 v0.2.5; #F4
// v0.2.5 — switched from q= to filter= for precise ID-based
// matching). Emitted by every list / detail screen on the `l`
// keypress; the resource composes the expression around its ID.
type OpenLogsMsg struct{ Filter string }

// OpenScreenMsg is the screen-level navigation request a child
// screen can emit when it wants the App Shell to switch the active
// resource by canonical name (the same lookup the `:` palette uses
// via screenFromName). Used today by the home dashboard's card-
// drilldown — pressing Enter on the Users card emits
// OpenScreenMsg{Target: "users"} and the App Shell pushes the
// matching frame onto the nav stack. Shape mirrors app.SwitchScreenMsg
// so the App Shell handler can forward to the same resolver.
type OpenScreenMsg struct{ Target string }

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
