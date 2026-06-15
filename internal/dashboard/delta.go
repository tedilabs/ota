package dashboard

import (
	"sort"
	"time"
)

// Delta is the per-card change report the home screen renders.
// Compared is true only when the cache has a historical roll for
// the prior period; first-week / fresh-cache callers must accept
// "no comparison yet" gracefully.
//
// The Sparkline field carries normalized 0–8 bucket indices (one
// per day, oldest → newest). Pass to RenderSparkline to stamp the
// `▁▂▃▄▅▆▇` glyph row.
type Delta struct {
	Current    int
	Previous   int
	Diff       int
	Pct        float64 // 0.0 when Previous == 0 (avoid div-by-zero)
	Compared   bool
	Sparkline  []int   // 0..8 inclusive; nil when no history
	WindowDays int     // distance back the comparison reaches
}

// DeltaFor returns the change report for `card` comparing `now`
// against `windowDays` days ago. Walks the cache history forward
// looking for the closest day-roll at-or-before the target date so
// the report still renders even when the operator skipped a day.
func DeltaFor(c *Cache, card string, now time.Time, windowDays int) Delta {
	if c == nil || c.Disabled() {
		return Delta{WindowDays: windowDays}
	}
	current, hasCurrent := latestRollFor(c, card, now)
	if !hasCurrent {
		return Delta{WindowDays: windowDays}
	}
	previous, hasPrev := closestRollBefore(c, card, now.AddDate(0, 0, -windowDays))
	d := Delta{
		Current:    current,
		Previous:   previous,
		Compared:   hasPrev,
		WindowDays: windowDays,
		Sparkline:  sparklineFor(c, card, now, sparklineDays(windowDays)),
	}
	if hasPrev {
		d.Diff = current - previous
		if previous != 0 {
			d.Pct = float64(d.Diff) / float64(previous) * 100.0
		}
	}
	return d
}

// sparklineDays maps the comparison window to the sparkline span —
// the trend chart shows ~3x the window so the operator can see
// whether the diff is part of a trend or a single-day spike.
func sparklineDays(window int) int {
	if window <= 7 {
		return 14
	}
	if window <= 30 {
		return 30
	}
	return window * 2
}

// latestRollFor returns the most-recent day-roll for `card` whose
// date is at-or-before `now`. Falls back to the latest snapshot if
// no rolls exist (e.g. only the Counts.Total has been recorded).
func latestRollFor(c *Cache, card string, now time.Time) (int, bool) {
	snap := c.Snapshot()
	target := now.UTC().Format("2006-01-02")
	// Direct hit.
	if per, ok := snap.History[target]; ok {
		if v, ok := per[card]; ok {
			return v, true
		}
	}
	// Walk back day-by-day for up to a week (cache write cadence is
	// typically daily; missing entries beyond that are unusual).
	for i := 1; i <= 7; i++ {
		day := now.UTC().AddDate(0, 0, -i).Format("2006-01-02")
		if per, ok := snap.History[day]; ok {
			if v, ok := per[card]; ok {
				return v, true
			}
		}
	}
	// Fallback: the Cards counter itself.
	if v, ok := snap.Cards[card]; ok {
		return v.Total, true
	}
	return 0, false
}

// closestRollBefore returns the day-roll for `card` closest to
// `target` (and not after it). Walks back up to 14 days so a
// single missed snapshot doesn't blank the Δ.
func closestRollBefore(c *Cache, card string, target time.Time) (int, bool) {
	snap := c.Snapshot()
	for i := 0; i <= 14; i++ {
		day := target.UTC().AddDate(0, 0, -i).Format("2006-01-02")
		if per, ok := snap.History[day]; ok {
			if v, ok := per[card]; ok {
				return v, true
			}
		}
	}
	return 0, false
}

// sparklineFor reads the last `days` daily rolls for `card`, in
// chronological order (oldest first), and normalizes them to 0..8
// bucket indices. Days with no recorded roll are forward-filled
// from the previous observation so the spark line doesn't drop to
// zero on a missed day.
func sparklineFor(c *Cache, card string, now time.Time, days int) []int {
	snap := c.Snapshot()
	if len(snap.History) == 0 {
		return nil
	}
	rolls := make([]int, days)
	lastSeen := -1
	for i := 0; i < days; i++ {
		day := now.UTC().AddDate(0, 0, -(days - 1 - i)).Format("2006-01-02")
		if per, ok := snap.History[day]; ok {
			if v, ok := per[card]; ok {
				rolls[i] = v
				lastSeen = v
				continue
			}
		}
		if lastSeen >= 0 {
			rolls[i] = lastSeen
		} else {
			rolls[i] = -1 // sentinel for "not observed yet"
		}
	}
	// Drop leading sentinels so the chart starts from the first
	// observation.
	first := 0
	for first < len(rolls) && rolls[first] < 0 {
		first++
	}
	if first >= len(rolls) {
		return nil
	}
	rolls = rolls[first:]
	if len(rolls) < 2 {
		return nil
	}
	min, max := rolls[0], rolls[0]
	for _, v := range rolls {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	out := make([]int, len(rolls))
	if max == min {
		// Flat line — render at mid-height so the operator sees
		// "there's data here" rather than "blank".
		for i := range out {
			out[i] = 4
		}
		return out
	}
	span := float64(max - min)
	for i, v := range rolls {
		out[i] = int(float64(v-min) / span * 8.0)
		if out[i] > 8 {
			out[i] = 8
		}
		if out[i] < 0 {
			out[i] = 0
		}
	}
	return out
}

// HistoryDays returns the chronological days that have data for
// `card`. Used by tests + introspection.
func HistoryDays(c *Cache, card string) []string {
	if c == nil || c.Disabled() {
		return nil
	}
	snap := c.Snapshot()
	out := make([]string, 0, len(snap.History))
	for day, per := range snap.History {
		if _, ok := per[card]; ok {
			out = append(out, day)
		}
	}
	sort.Strings(out)
	return out
}
