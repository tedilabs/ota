package shared

import "github.com/charmbracelet/lipgloss"

// RowStyleForStatus returns the lipgloss style that should tint the
// ENTIRE row when the given resource status is abnormal — issue #155.
// Returns the zero Style + false when status is "normal" so callers
// can short-circuit and skip the row-bg pass.
//
// Mapping:
//
//	LOCKED_OUT, INVALID                          → RowDanger (red)
//	SUSPENDED, PASSWORD_EXPIRED                  → RowWarning (amber)
//	DEPROVISIONED, INACTIVE                      → RowMuted (gray)
//	everything else (ACTIVE, STAGED, PROVISIONED) → no tint
func RowStyleForStatus(status string, tk Tokens) (lipgloss.Style, bool) {
	switch status {
	case "LOCKED_OUT", "INVALID":
		return tk.RowDanger, true
	case "SUSPENDED", "PASSWORD_EXPIRED":
		return tk.RowWarning, true
	case "DEPROVISIONED", "INACTIVE":
		return tk.RowMuted, true
	}
	return lipgloss.Style{}, false
}

// StatusBadge represents one of the canonical "[icon] LABEL" cells used in
// list views. Mono is the NO_COLOR fallback (TUI_DESIGN §15.2 status table).
type StatusBadge struct {
	Label string
	Icon  string
	Mono  string
	Style lipgloss.Style
}

// Render returns the bare LABEL — issue #156 dropped the mono icon
// prefix because the row's bg tint (issue #155) already conveys
// abnormal status, and the icon doubled the column width without
// adding signal. Color comes from the Style applied by the caller's
// table cell.
func (b StatusBadge) Render(_ Tokens) string {
	return b.Label
}

// UserStatusBadge maps a domain.UserStatus value (string) to its visual
// representation per TUI_DESIGN §15.2.
func UserStatusBadge(status string, tk Tokens) StatusBadge {
	switch status {
	case "ACTIVE":
		return StatusBadge{Label: "ACTIVE", Icon: "●", Mono: "[+]", Style: tk.Success}
	case "STAGED", "PROVISIONED":
		return StatusBadge{Label: status, Icon: "○", Mono: "[-]", Style: tk.Info}
	case "SUSPENDED":
		return StatusBadge{Label: "SUSPENDED", Icon: "✗", Mono: "[X]", Style: tk.Warning}
	case "LOCKED_OUT":
		return StatusBadge{Label: "LOCKED_OUT", Icon: "⚠", Mono: "[!]", Style: tk.Danger}
	case "PASSWORD_EXPIRED":
		return StatusBadge{Label: "PASSWORD_EXPIRED", Icon: "◒", Mono: "[~]", Style: tk.Magenta}
	case "DEPROVISIONED":
		return StatusBadge{Label: "DEPROVISIONED", Icon: "⊘", Mono: "[/]", Style: tk.Muted}
	default:
		return StatusBadge{Label: status, Icon: "?", Mono: "[?]", Style: tk.Muted}
	}
}

// RuleStatusBadge maps a domain.RuleStatus value (string) per TUI_DESIGN §15.4.
func RuleStatusBadge(status string, tk Tokens) StatusBadge {
	switch status {
	case "ACTIVE":
		return StatusBadge{Label: "ACTIVE", Icon: "●", Mono: "[+]", Style: tk.Success}
	case "INACTIVE":
		return StatusBadge{Label: "INACTIVE", Icon: "○", Mono: "[-]", Style: tk.Muted}
	case "INVALID":
		return StatusBadge{Label: "INVALID", Icon: "⚠", Mono: "[!]", Style: tk.Danger}
	default:
		return StatusBadge{Label: status, Icon: "?", Mono: "[?]", Style: tk.Muted}
	}
}

// PolicyStatusBadge maps a policy status per TUI_DESIGN §15.5.
func PolicyStatusBadge(status string, tk Tokens) StatusBadge {
	switch status {
	case "ACTIVE":
		return StatusBadge{Label: "ACTIVE", Icon: "●", Mono: "[+]", Style: tk.Success}
	case "INACTIVE":
		return StatusBadge{Label: "INACTIVE", Icon: "○", Mono: "[-]", Style: tk.Muted}
	default:
		return StatusBadge{Label: status, Icon: "?", Mono: "[?]", Style: tk.Muted}
	}
}

// SeverityBadge maps log severities per TUI_DESIGN §15.6.
func SeverityBadge(sev string, tk Tokens) StatusBadge {
	switch sev {
	case "DEBUG":
		return StatusBadge{Label: "DEBUG", Icon: "·", Mono: "[.]", Style: tk.Muted}
	case "INFO":
		return StatusBadge{Label: "INFO", Icon: "ℹ", Mono: "[i]", Style: tk.Info}
	case "WARN":
		return StatusBadge{Label: "WARN", Icon: "!", Mono: "[!]", Style: tk.Warning}
	case "ERROR":
		return StatusBadge{Label: "ERR ", Icon: "✗", Mono: "[X]", Style: tk.Danger}
	default:
		return StatusBadge{Label: sev, Icon: "?", Mono: "[?]", Style: tk.Muted}
	}
}

// AppStatusBadge maps a domain.AppStatus per issue #166.
func AppStatusBadge(status string, tk Tokens) StatusBadge {
	switch status {
	case "ACTIVE":
		return StatusBadge{Label: "ACTIVE", Icon: "●", Mono: "[+]", Style: tk.Success}
	case "INACTIVE":
		return StatusBadge{Label: "INACTIVE", Icon: "○", Mono: "[-]", Style: tk.Muted}
	default:
		return StatusBadge{Label: status, Icon: "?", Mono: "[?]", Style: tk.Muted}
	}
}

// GroupTypeBadge maps a domain.GroupType per TUI_DESIGN §15.3.
func GroupTypeBadge(t string, tk Tokens) StatusBadge {
	switch t {
	case "OKTA_GROUP":
		return StatusBadge{Label: "OKTA", Icon: "◆", Mono: "[O]", Style: tk.FG}
	case "APP_GROUP":
		return StatusBadge{Label: "APP", Icon: "▣", Mono: "[A]", Style: tk.Info}
	case "BUILT_IN":
		return StatusBadge{Label: "BUILT_IN", Icon: "◈", Mono: "[B]", Style: tk.Magenta}
	default:
		return StatusBadge{Label: t, Icon: "?", Mono: "[?]", Style: tk.Muted}
	}
}
