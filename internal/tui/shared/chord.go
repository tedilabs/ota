package shared

import "time"

// GChord captures the single bit of state needed for the Vim `gg` chord
// (jump to top): whether the previous keypress was a `g` and when it was
// observed. A second `g` within the timeout window fires the action.
//
// The struct is value-type so list models can embed it without owning a
// pointer (matches the rest of our model state) and zero-value works
// (no chord pending).
type GChord struct {
	armedAt time.Time
}

// chordWindow is the maximum gap between the two g presses. Vim is
// time-unbounded but we cap to 600ms so a stray `g` doesn't lurk forever.
const chordWindow = 600 * time.Millisecond

// Press records a `g` keypress at now. Returns true if this completes a
// gg chord (i.e. the previous press was within chordWindow), and resets
// the chord state. Returns false otherwise (the chord is now armed for
// the next press).
func (c *GChord) Press(now time.Time) bool {
	if !c.armedAt.IsZero() && now.Sub(c.armedAt) <= chordWindow {
		c.armedAt = time.Time{}
		return true
	}
	c.armedAt = now
	return false
}

// Reset clears the chord state — call when any non-g key arrives so the
// chord doesn't fire across an interrupting keypress.
func (c *GChord) Reset() { c.armedAt = time.Time{} }
