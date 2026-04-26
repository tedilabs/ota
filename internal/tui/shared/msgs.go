package shared

// UnmaskFieldMsg asks the active Detail screen to reveal a specific PII
// field. Sent by the App Shell when the operator types :unmask <field>
// into the command palette (issue #115). Lives here in shared so both
// app/ and tui/* can reference it without a cycle.
type UnmaskFieldMsg struct{ Field string }

// MaskAllMsg asks the active Detail screen to re-mask every previously
// unmasked field. Sent on the bare :mask command (no argument).
type MaskAllMsg struct{}
