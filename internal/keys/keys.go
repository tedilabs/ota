package keys

// ID is a canonical key-binding identifier. See TUI_DESIGN §3 for the
// authoritative list; ota uses these strings as the YAML key names so users
// can override via `keybindings.<id>: "<keybind-string>"`.
type ID string

// Global navigation (TUI_DESIGN §3.2).
const (
	IDNavDown     ID = "nav.down"
	IDNavUp       ID = "nav.up"
	IDNavLeft     ID = "nav.left"
	IDNavRight    ID = "nav.right"
	IDNavTop      ID = "nav.top"       // gg
	IDNavBottom   ID = "nav.bottom"    // G
	IDNavHalfDown ID = "nav.half_down" // Ctrl-d
	IDNavHalfUp   ID = "nav.half_up"   // Ctrl-u
	IDNavPageUp   ID = "nav.page_up"   // Ctrl-b
	IDNavPageDn   ID = "nav.page_down" // Ctrl-f
	IDNavSelect   ID = "nav.select"    // Enter
	IDNavTabNext  ID = "nav.tab_next"  // Tab
	IDNavTabPrev  ID = "nav.tab_prev"  // Shift-Tab
	IDNavLineHome ID = "nav.line_home" // 0 / Home
	IDNavLineEnd  ID = "nav.line_end"  // $ / End
)

// App / global lifecycle (TUI_DESIGN §3.1).
const (
	IDAppQuit        ID = "app.quit"          // q — close current; List→quit confirm
	IDAppHelp        ID = "app.help"          // ?
	IDAppRefresh     ID = "app.refresh"       // R
	IDAppBack        ID = "app.back"          // Esc
	IDGlobalHardQuit ID = "global.hard_quit"  // Ctrl-c
	IDGlobalRedraw   ID = "global.redraw"     // Ctrl-l
)

// Prompts and search (TUI_DESIGN §3.1 + §3.3).
const (
	IDCmdOpen    ID = "cmd.open"    // :
	IDSearchOpen ID = "search.open" // /
	IDSearchNext ID = "search.next" // n
	IDSearchPrev ID = "search.prev" // N
)

// Observe / yank actions (TUI_DESIGN §3.3).
const (
	IDActionYank      ID = "action.yank"       // y
	IDActionYankField ID = "action.yank_field" // yf
	IDActionYankRow   ID = "action.yank_row"   // yy
	IDActionOpenWeb   ID = "action.open_web"   // o
	IDActionExpand    ID = "action.expand"     // e
	IDActionToggleRaw ID = "action.toggle_raw" // r
)

// Logs-specific (TUI_DESIGN §3.3). `s` toggles tail mode (REQ-R05 AC-3);
// `f` toggles auto-scroll/follow. These differ from earlier stub drafts
// that used `t` — TUI_DESIGN §3.3 is authoritative.
const (
	IDLogsTailToggle   ID = "logs.tail_toggle" // s
	IDLogsFollowToggle ID = "logs.follow"      // f
)

// PII mask toggle (TUI_DESIGN §7.2). These are prompt command names rather
// than raw keystrokes; included here so they participate in the ID catalog.
const (
	IDPIIUnmask ID = "pii.unmask"
	IDPIIMask   ID = "pii.mask"
)
