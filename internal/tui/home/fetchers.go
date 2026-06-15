package home

import (
	"context"
	"time"

	"github.com/tedilabs/ota/internal/dashboard"
	"github.com/tedilabs/ota/internal/domain"
)

// Iterator-walking fetchers — each lists every row from the matching
// port, buckets by the field the home card cares about, and stamps
// the ObservedAt time.
//
// Why drain the full iterator instead of `?limit=0`: Okta returns
// the actual list, not a count. For typical Workforce Identity
// tenants (< 50k users) this is one or two pages and finishes in
// well under the 30s context budget. Enterprise tenants with 100k+
// users will want a smarter approach (a metrics endpoint, or
// sampling) — flag that as follow-up work when an operator with a
// big tenant complains.

// countUsers walks the Users iterator and buckets by Status.
func countUsers(ctx context.Context, port domain.UsersPort, now time.Time) (dashboard.Counts, error) {
	if port == nil {
		return dashboard.Counts{}, nil
	}
	it, err := port.List(ctx, domain.UsersQuery{})
	if err != nil {
		return dashboard.Counts{}, err
	}
	defer it.Close()

	out := dashboard.Counts{
		ByStatus:   map[string]int{},
		ObservedAt: now,
	}
	for {
		u, hasMore, err := it.Next(ctx)
		if err != nil {
			return dashboard.Counts{}, err
		}
		if !hasMore {
			break
		}
		out.Total++
		key := string(u.Status)
		if key == "" {
			key = "UNKNOWN"
		}
		out.ByStatus[key]++
	}
	return out, nil
}

// countGroups walks the Groups iterator and buckets by Type
// (OKTA_GROUP / APP_GROUP / BUILT_IN).
func countGroups(ctx context.Context, port domain.GroupsPort, now time.Time) (dashboard.Counts, error) {
	if port == nil {
		return dashboard.Counts{}, nil
	}
	it, err := port.List(ctx, domain.GroupsQuery{})
	if err != nil {
		return dashboard.Counts{}, err
	}
	defer it.Close()

	out := dashboard.Counts{
		BySubtype:  map[string]int{},
		ObservedAt: now,
	}
	for {
		g, hasMore, err := it.Next(ctx)
		if err != nil {
			return dashboard.Counts{}, err
		}
		if !hasMore {
			break
		}
		out.Total++
		key := string(g.Type)
		if key == "" {
			key = "UNKNOWN"
		}
		out.BySubtype[key]++
	}
	return out, nil
}

// countApps walks every app type the Apps screen lists, buckets by
// Status (ACTIVE / INACTIVE) for the headline status row, and by
// AppType (SAML_2_0 / OPENID_CONNECT / BOOKMARK / AUTO_LOGIN / …)
// for the subtype breakdown so the card surfaces both axes.
func countApps(ctx context.Context, port domain.AppsPort, now time.Time) (dashboard.Counts, error) {
	if port == nil {
		return dashboard.Counts{}, nil
	}
	it, err := port.List(ctx, domain.AppsQuery{})
	if err != nil {
		return dashboard.Counts{}, err
	}
	defer it.Close()

	out := dashboard.Counts{
		ByStatus:   map[string]int{},
		BySubtype:  map[string]int{},
		ObservedAt: now,
	}
	for {
		a, hasMore, err := it.Next(ctx)
		if err != nil {
			return dashboard.Counts{}, err
		}
		if !hasMore {
			break
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
	return out, nil
}
