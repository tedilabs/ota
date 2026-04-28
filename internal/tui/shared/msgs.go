package shared

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
// "reset-password" / "unlock" / "reset-factors". Lives in shared so
// the users list can emit it without an app→tui→app cycle.
type RunUserActionMsg struct{ Kind string }
