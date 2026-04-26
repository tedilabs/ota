package shared

// SortGlyph returns the coloured ↑ / ↓ glyph used to mark active list
// columns (issue #118). Direction strings:
//
//	"asc"  → tk.Success ↑ (green)
//	"desc" → tk.Danger  ↓ (red)
//	other  → empty string (no glyph rendered)
//
// Lipgloss strips automatically under NO_COLOR / Monochrome via the
// already-configured profile, so callers can paste the result straight
// into the column header.
func SortGlyph(direction string, tk Tokens) string {
	switch direction {
	case "asc":
		return tk.Success.Render("↑")
	case "desc":
		return tk.Danger.Render("↓")
	default:
		return ""
	}
}
