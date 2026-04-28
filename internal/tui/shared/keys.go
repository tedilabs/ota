package shared

import tea "github.com/charmbracelet/bubbletea"

// NormalizeArrowKey rewrites tea.KeyUp / KeyDown / KeyLeft / KeyRight
// as their Vim-rune equivalents so list models with switch-on-runes
// keypaths handle arrows for free (issue #159). Non-arrow messages
// pass through unchanged.
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
