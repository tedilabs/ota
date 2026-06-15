package home

import (
	"time"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/oktastatus"
)

// HealthSnapshot is the live-system signal payload the App Shell
// publishes into the home screen — Okta tenant status, rate-limit
// headroom, and the last successful fetch stamp.
//
// Why pushed from the App Shell rather than fetched here: the
// chrome already tracks all of these for its title bar / status
// row, and re-fetching from the home screen would double up
// network calls (Okta charges rate-limit budget for the status
// probe). The App Shell broadcasts an UpdateHealthMsg whenever a
// signal changes; the home Model folds it into m.health and
// re-renders the Health card.
type HealthSnapshot struct {
	OktaStatus       oktastatus.Snapshot
	RateLimits       []domain.RateLimitSnapshot
	LastFetchAt      time.Time
	APIRecorderCount int
	ObservedAt       time.Time
}

// UpdateHealthMsg is the broadcast the App Shell sends home when
// any of the live-system signals shift.
type UpdateHealthMsg struct {
	Snapshot HealthSnapshot
}

// worstRateLimit returns the rate-limit category with the lowest
// remaining-headroom percentage, so the Health card can highlight
// the category most likely to throttle next. Returns (snapshot, 0)
// when no signals are wired.
func worstRateLimit(snapshots []domain.RateLimitSnapshot) (domain.RateLimitSnapshot, float64) {
	if len(snapshots) == 0 {
		return domain.RateLimitSnapshot{}, 0
	}
	var (
		worst   domain.RateLimitSnapshot
		worstPct = 101.0
	)
	for _, s := range snapshots {
		if s.Limit <= 0 {
			continue
		}
		pct := float64(s.Remaining) / float64(s.Limit) * 100.0
		if pct < worstPct {
			worstPct = pct
			worst = s
		}
	}
	if worstPct > 100 {
		return snapshots[0], 100
	}
	return worst, worstPct
}
