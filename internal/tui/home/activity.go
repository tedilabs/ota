package home

import (
	"context"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// ActivityMetrics is the per-window aggregation the Activity card
// renders. Captured for both the 24h and 7d windows in parallel so
// the card surfaces "spike now vs typical week" at a glance.
type ActivityMetrics struct {
	WindowLabel    string // "24h" or "7d"
	WindowSince    time.Time
	SignIns        int
	FailedSignIns  int
	AccountLocks   int
	MFAResets      int
	AdminActions   int
	UserCreates    int
	UserDeletes    int
	// HourlyBuckets is the 24-bucket sign-in series for sparkline
	// rendering (newest bucket = the hour up to now). Populated only
	// for the 24h window; 7d aggregations leave it nil since the
	// sparkline already lives on the count cards.
	HourlyBuckets []int
}

// activityWindow drives countActivity — kept separate so the Activity
// card can fan out two concurrent fetches (24h, 7d) without sharing
// state through closure capture.
type activityWindow struct {
	label    string
	since    time.Duration
	withSpark bool
}

// countActivity reads up to logsSampleSize log events from the
// requested window, buckets every event we recognize, and (for the
// short window) populates the hourly histogram driving the
// Activity card's sparkline. Single-page bounded — busy tenants
// see "≈" prefixed totals rather than a multi-page Okta /logs
// walk that would chew through the logs rate-limit budget.
func countActivity(ctx context.Context, port domain.LogsPort, now time.Time, w activityWindow) (ActivityMetrics, bool, error) {
	out := ActivityMetrics{
		WindowLabel: w.label,
		WindowSince: now.Add(-w.since),
	}
	if port == nil {
		return out, false, nil
	}
	if w.withSpark {
		out.HourlyBuckets = make([]int, 24)
	}
	since := now.Add(-w.since)
	q := domain.LogsQuery{
		Since:     &since,
		SortOrder: domain.SortAscending,
		Limit:     logsSampleSize,
	}
	it, err := port.Search(ctx, q)
	if err != nil {
		return out, false, err
	}
	defer it.Close()
	for i := 0; i < logsSampleSize; i++ {
		ev, hasMore, err := it.Next(ctx)
		if err != nil {
			return out, false, err
		}
		if !hasMore {
			return out, false, nil
		}
		switch ev.EventType {
		case "user.session.start":
			out.SignIns++
			if w.withSpark {
				bucket := hourlyBucket(now, ev.Published)
				if bucket >= 0 && bucket < 24 {
					out.HourlyBuckets[bucket]++
				}
			}
			// Failed sign-ins surface via the outcome on the same
			// event class — Okta records both success and FAILURE
			// under user.session.start with different outcomes.
			if ev.Outcome.Result == domain.OutcomeFailure {
				out.FailedSignIns++
			}
		case "user.account.lock":
			out.AccountLocks++
		case "user.mfa.factor.reset_all":
			out.MFAResets++
		case "user.lifecycle.create":
			out.UserCreates++
		case "user.lifecycle.delete.initiated":
			out.UserDeletes++
		}
		// Admin actions: anything where actor.type is User and the
		// eventType is a system / lifecycle write op. We approximate
		// by any eventType starting with "system." or
		// "policy.lifecycle." — Okta doesn't expose a "is_admin"
		// flag so this is a heuristic with reasonable signal/noise.
		if ev.Actor.Type == domain.ActorTypeUser &&
			(startsWith(ev.EventType, "system.") || startsWith(ev.EventType, "policy.lifecycle.")) {
			out.AdminActions++
		}
	}
	// Hit the sample cap — there are likely more events we
	// deliberately didn't drain. Caller's "sampled" flag flips.
	return out, true, nil
}

// hourlyBucket returns the 24-bucket index for `t` relative to
// `now`. Bucket 0 = oldest hour (24h ago); bucket 23 = the hour
// up to `now`. Events outside the window return -1.
func hourlyBucket(now, t time.Time) int {
	delta := now.Sub(t)
	if delta < 0 || delta > 24*time.Hour {
		return -1
	}
	hour := int(delta / time.Hour)
	return 23 - hour
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
