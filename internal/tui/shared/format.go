package shared

import (
	"fmt"
	"time"
)

// RelativeTime returns a compact "2h ago" / "1d ago" / "—" string for a
// possibly-nil timestamp. now is the reference clock — pass clock.Now() at
// the call site so tests can freeze time.
func RelativeTime(t *time.Time, now time.Time) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	d := now.Sub(*t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d/(30*24*time.Hour)))
	default:
		return fmt.Sprintf("%dy ago", int(d/(365*24*time.Hour)))
	}
}

// Truncate clips s to width visible cells and appends "…" when truncation
// happened. Pure ASCII width assumption; safe for the runes we render.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	cut := truncateVisible(s, width-1)
	return cut + "…"
}
