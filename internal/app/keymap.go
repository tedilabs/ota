package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/keys"
)

// KeyContext narrows the interpretation of a tea.KeyMsg. The classifier
// refuses to dispatch a key to its bound ID when a context captures input
// (e.g., the search / cmd prompt is focused) — REQ-U03 AC-1.
type KeyContext int

const (
	// KeyContextDefault means list/detail screens — full key resolution applies.
	KeyContextDefault KeyContext = iota
	// KeyContextInputActive means a textinput is focused (search `/`, cmd `:`,
	// filter). Non-control keys must pass through as raw input.
	KeyContextInputActive
	// KeyContextOverlayModal means a modal (help, confirm) captures keys.
	KeyContextOverlayModal
)

// ClassifyKey maps a tea.KeyMsg to a key ID using the resolved bindings.
// Arrows are normalized to their Vim equivalents (REQ-U01 AC-1). Returns the
// empty ID when no binding matches.
//
// Equivalent to ClassifyKeyInContext with KeyContextDefault.
func ClassifyKey(msg tea.KeyMsg, resolved keys.ResolvedMap) keys.ID {
	return ClassifyKeyInContext(msg, resolved, KeyContextDefault)
}

// ClassifyKeyInContext applies context-sensitive classification: in
// input-active contexts, runes are passed through to the textinput (empty
// ID returned) while arrows / Esc / Enter still resolve normally.
func ClassifyKeyInContext(msg tea.KeyMsg, resolved keys.ResolvedMap, ctx KeyContext) keys.ID {
	// Arrow keys map to Vim equivalents regardless of context (REQ-U01 AC-1).
	switch msg.Type {
	case tea.KeyDown:
		return keys.IDNavDown
	case tea.KeyUp:
		return keys.IDNavUp
	case tea.KeyLeft:
		return keys.IDNavLeft
	case tea.KeyRight:
		return keys.IDNavRight
	}

	// In input-active context, runes are consumed by the textinput — refuse
	// to dispatch a binding (REQ-U03 AC-1).
	if ctx == KeyContextInputActive && msg.Type == tea.KeyRunes {
		return ""
	}

	// Default: match the rune (for KeyRunes) against the reverse binding map.
	if msg.Type == tea.KeyRunes {
		return resolved.Reverse()[string(msg.Runes)]
	}
	// Named keys (Esc/Enter/Tab/...) — match by String().
	return resolved.Reverse()[msg.String()]
}
