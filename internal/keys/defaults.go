package keys

// Defaults returns the built-in ID → keybinding string mapping (Vim/k9s).
// See docs/TUI_DESIGN.md §3 for the authoritative table.
func Defaults() map[ID]string {
	return map[ID]string{
		// §3.2 Navigation.
		IDNavDown:     "j",
		IDNavUp:       "k",
		IDNavLeft:     "h",
		IDNavRight:    "l",
		IDNavTop:      "g g",
		IDNavBottom:   "G",
		IDNavHalfDown: "Ctrl-d",
		IDNavHalfUp:   "Ctrl-u",
		IDNavPageUp:   "Ctrl-b",
		IDNavPageDn:   "Ctrl-f",
		IDNavSelect:   "Enter",
		IDNavTabNext:  "Tab",
		IDNavTabPrev:  "Shift-Tab",
		IDNavLineHome: "0",
		IDNavLineEnd:  "$",

		// §3.1 App / global.
		IDAppQuit:        "q",
		IDAppHelp:        "?",
		IDAppRefresh:     "R",
		IDAppBack:        "Esc",
		IDGlobalHardQuit: "Ctrl-c",
		IDGlobalRedraw:   "Ctrl-l",

		// §3.1 + §3.3 Prompts / search.
		IDCmdOpen:    ":",
		IDSearchOpen: "/",

		// §3.3 + §3.6 Observe / yank.
		IDActionYank:      "y",
		IDActionYankField: "y f",
		IDActionYankRow:   "y y",
		IDActionOpenWeb:   "o",
		IDActionExpand:    "e",
		IDActionToggleRaw: "r",
		IDActionDetail:    "d",

		// §3.5 Sort cycle. bubbletea delivers Shift+letter as the uppercase
		// rune so the binding string is the bare uppercase letter.
		IDSortStatus:    "S",
		IDSortName:      "N",
		IDSortLastLogin: "L",
		IDSortCreated:   "C",

		// §3.3 Logs. `s` = tail toggle (REQ-R05 AC-3), `f` = follow/auto-scroll.
		IDLogsTailToggle:   "s",
		IDLogsFollowToggle: "f",

		// PII prompt commands (not raw keys — included for the ID catalog).
		IDPIIUnmask: ":unmask",
		IDPIIMask:   ":mask",
	}
}

// allIDs returns the set of recognised IDs (used to detect unknown user
// override keys). Kept in sync with Defaults by construction.
func allIDs() map[ID]struct{} {
	d := Defaults()
	out := make(map[ID]struct{}, len(d))
	for id := range d {
		out[id] = struct{}{}
	}
	return out
}
