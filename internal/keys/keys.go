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

// Prompts and search (TUI_DESIGN §3.1 + §3.3). Search is incremental — the
// `/` filter updates the visible set live as the user types, so the older
// `n` / `N` next/previous step IDs were dead and were dropped in v0.1.1.
const (
	IDCmdOpen    ID = "cmd.open"    // :
	IDSearchOpen ID = "search.open" // /
)

// Observe / yank actions (TUI_DESIGN §3.3 + §3.6).
const (
	IDActionYank      ID = "action.yank"       // y
	IDActionYankField ID = "action.yank_field" // yf
	IDActionYankRow   ID = "action.yank_row"   // yy
	IDActionOpenWeb   ID = "action.open_web"   // o
	IDActionExpand    ID = "action.expand"     // e
	IDActionToggleRaw ID = "action.toggle_raw" // r
	IDActionDetail    ID = "action.detail"     // d — open full attribute detail (TUI_DESIGN §3.6)
)

// Column sort cycle (TUI_DESIGN §3.5). bubbletea delivers Shift+letter as
// the uppercase rune so the binding strings here are bare uppercase letters;
// the §3.5 table renders them as `Shift+S` etc. for human readability.
// Pressing the bound key cycles dirOff → ↑ → ↓ → off; activating a different
// column resets the previous to dirOff (single active sort column).
const (
	IDSortStatus    ID = "sort.status"     // Shift+S — STATUS column
	IDSortName      ID = "sort.name"       // Shift+N — NAME / LOGIN column
	IDSortLastLogin ID = "sort.last_login" // Shift+L — LAST LOGIN / UPDATED column
	IDSortCreated   ID = "sort.created"    // Shift+C — CREATED / CHANGED column
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
