package home

import (
	"context"
	"time"

	"github.com/tedilabs/ota/internal/dashboard"
	"github.com/tedilabs/ota/internal/domain"
)

// sampleSize caps every list-card fetch to a single Okta page —
// the dashboard pays one API call per card instead of N pages of a
// full enumeration. Tenants with > sampleSize entries see a "≈"
// prefix + the sample total as a lower bound on the card.
//
// Why 200: Okta's default page size for users/groups/apps is 200
// (the upper bound varies by endpoint but 200 is universally
// accepted). One read = one API call.
const sampleSize = 200

// logsSampleSize is the per-fetch cap for log walks. Okta returns
// up to 1000 events per page on /api/v1/logs; bounding the
// Activity card to one page keeps the logs category rate-limit
// budget intact at the cost of an "≈" caveat on busy tenants.
const logsSampleSize = 1000

// countUsers reads up to sampleSize users from the list iterator
// and buckets by status. Returns (counts, sampled) — sampled = true
// when the iterator might have more data we deliberately didn't
// drain.
func countUsers(ctx context.Context, port domain.UsersPort, now time.Time) (dashboard.Counts, bool, error) {
	if port == nil {
		return dashboard.Counts{}, false, nil
	}
	it, err := port.List(ctx, domain.UsersQuery{})
	if err != nil {
		return dashboard.Counts{}, false, err
	}
	defer it.Close()

	out := dashboard.Counts{ByStatus: map[string]int{}, ObservedAt: now}
	for i := 0; i < sampleSize; i++ {
		u, hasMore, err := it.Next(ctx)
		if err != nil {
			return dashboard.Counts{}, false, err
		}
		if !hasMore {
			return out, false, nil
		}
		out.Total++
		key := string(u.Status)
		if key == "" {
			key = "UNKNOWN"
		}
		out.ByStatus[key]++
	}
	// We hit the sample cap — assume more pages exist. False
	// positive on tenants with exactly sampleSize entries is
	// acceptable (the "≈" prefix just reads as approximate).
	return out, true, nil
}

func countGroups(ctx context.Context, port domain.GroupsPort, now time.Time) (dashboard.Counts, bool, error) {
	if port == nil {
		return dashboard.Counts{}, false, nil
	}
	it, err := port.List(ctx, domain.GroupsQuery{})
	if err != nil {
		return dashboard.Counts{}, false, err
	}
	defer it.Close()

	out := dashboard.Counts{BySubtype: map[string]int{}, ObservedAt: now}
	for i := 0; i < sampleSize; i++ {
		g, hasMore, err := it.Next(ctx)
		if err != nil {
			return dashboard.Counts{}, false, err
		}
		if !hasMore {
			return out, false, nil
		}
		out.Total++
		key := string(g.Type)
		if key == "" {
			key = "UNKNOWN"
		}
		out.BySubtype[key]++
	}
	return out, true, nil
}

func countApps(ctx context.Context, port domain.AppsPort, now time.Time) (dashboard.Counts, bool, error) {
	if port == nil {
		return dashboard.Counts{}, false, nil
	}
	it, err := port.List(ctx, domain.AppsQuery{})
	if err != nil {
		return dashboard.Counts{}, false, err
	}
	defer it.Close()

	out := dashboard.Counts{
		ByStatus:   map[string]int{},
		BySubtype:  map[string]int{},
		ObservedAt: now,
	}
	for i := 0; i < sampleSize; i++ {
		a, hasMore, err := it.Next(ctx)
		if err != nil {
			return dashboard.Counts{}, false, err
		}
		if !hasMore {
			return out, false, nil
		}
		out.Total++
		statusKey := string(a.Status)
		if statusKey == "" {
			statusKey = "UNKNOWN"
		}
		out.ByStatus[statusKey]++
		typeKey := string(a.Type)
		if typeKey == "" {
			typeKey = "UNKNOWN"
		}
		out.BySubtype[typeKey]++
	}
	return out, true, nil
}
