package shared

import tea "github.com/charmbracelet/bubbletea"

// NormalizeArrowKey rewrites tea.KeyUp / KeyDown / KeyLeft / KeyRight
// as their Vim-rune equivalents so list models with switch-on-runes
// keypaths handle arrows for free (issue #159). Non-arrow messages
// pass through unchanged.
//
// #F2 v0.2.5 — `l` rune now means "open Logs for the current
// resource", so screens that bind Right→hScroll-right should call
// NormalizeArrowKeyVerticalOnly instead and handle Left/Right
// explicitly. Otherwise the operator's Right press would open Logs
// every time.
func NormalizeArrowKey(msg tea.KeyMsg) tea.KeyMsg {
	switch msg.Type {
	case tea.KeyDown:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	case tea.KeyUp:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	case tea.KeyLeft:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	case tea.KeyRight:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	}
	return msg
}

// NormalizeArrowKeyVerticalOnly rewrites only Up/Down arrows to j/k
// runes; Left/Right pass through so callers can wire them to
// hScroll-left/right without colliding with the `l`-for-Logs rune
// (#F2 v0.2.5).
func NormalizeArrowKeyVerticalOnly(msg tea.KeyMsg) tea.KeyMsg {
	switch msg.Type {
	case tea.KeyDown:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	case tea.KeyUp:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	}
	return msg
}
