package shared

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// HighlightWindow is the duration each refresh-detected row change
// keeps its RowChanged flash on a list. 1.5s reads as a deliberate
// flash — long enough to notice, short enough to not feel sticky.
// Issue #193 v0.2.3.
const HighlightWindow = 1500 * time.Millisecond

// HighlightTick is the cadence the per-list highlight tick fires
// at while at least one row is still inside HighlightWindow. ~250ms
// gives the flash time to feel like a fade without burning frames.
const HighlightTick = 250 * time.Millisecond

// DiffChanges returns a refreshed changedAt map: rows in `next` whose
// tracked fields differ from `prev` (or are brand-new) get stamped at
// `now`; entries for rows that vanished from `next` are dropped;
// unchanged rows keep any in-flight timestamp so a flash that started
// mid-fetch keeps fading. Generic over the row type so each list can
// pass its own ID extractor + tracked-field comparator.
func DiffChanges[T any](
	prev, next []T,
	existing map[string]time.Time,
	now time.Time,
	idOf func(T) string,
	equal func(T, T) bool,
) map[string]time.Time {
	out := map[string]time.Time{}
	prevByID := make(map[string]T, len(prev))
	for _, x := range prev {
		if id := idOf(x); id != "" {
			prevByID[id] = x
		}
	}
	for _, x := range next {
		id := idOf(x)
		if id == "" {
			continue
		}
		old, hadOld := prevByID[id]
		switch {
		case !hadOld:
			out[id] = now
		case !equal(old, x):
			out[id] = now
		default:
			if t, ok := existing[id]; ok {
				out[id] = t
			}
		}
	}
	return out
}

// HasFreshHighlights reports whether any entry in changedAt is still
// inside HighlightWindow — the View needs to keep flashing those rows
// and the model owes another tick.
func HasFreshHighlights(changedAt map[string]time.Time, now time.Time) bool {
	for _, t := range changedAt {
		if now.Sub(t) < HighlightWindow {
			return true
		}
	}
	return false
}

// IsRowChanged reports whether the row identified by id is currently
// inside HighlightWindow. Lists call this from their View to decide
// whether to apply the RowChanged token.
func IsRowChanged(changedAt map[string]time.Time, id string, now time.Time) bool {
	t, ok := changedAt[id]
	if !ok {
		return false
	}
	return now.Sub(t) < HighlightWindow
}

// ScheduleHighlightTickCmd returns a tea.Tick that fires the given
// msg ~HighlightTick later. Each list passes its own zero-value tick
// msg type so the Update switch can route precisely.
func ScheduleHighlightTickCmd(msg tea.Msg) tea.Cmd {
	return tea.Tick(HighlightTick, func(time.Time) tea.Msg {
		return msg
	})
}

// TimePtrsEqual returns true when two *time.Time values represent the
// same instant, treating nil == nil as equal. Useful when implementing
// a tracked-field comparator for DiffChanges over domain types whose
// timestamps are pointer-typed (LastLogin, StatusChanged, …).
func TimePtrsEqual(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	}
	return a.Equal(*b)
}
