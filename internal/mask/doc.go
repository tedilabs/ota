// Package mask provides pure PII masking utilities consumed by TUI views
// (TUI_DESIGN §7, PRD §6.2, REQ-R01 AC-6).
//
// Services and domain pass raw values through unchanged; only the render
// boundary calls these helpers.
package mask
