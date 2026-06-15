package home

import (
	"context"
	"sort"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// CriticalEvent is one row on the "Recent Critical Events" card —
// a high-signal System Log entry the operator probably wants to
// see at a glance without opening the full Logs view.
type CriticalEvent struct {
	When       time.Time
	EventType  string
	DisplayMsg string
	Severity   domain.Severity
	ActorLogin string
	ActorID    string
	TargetID   string
	UUID       string
}

// criticalEventTypes is the curated set of System Log eventTypes the
// home card surfaces. Each entry is "security-flavored" — an
// admin / operator would want immediate visibility when one fires.
// Add new entries here; the card de-dupes by UUID so no risk of
// double-counting if Okta retries a delivery.
var criticalEventTypes = map[string]struct{}{
	// User lifecycle — destructive ops are always interesting.
	"user.account.lock":                  {},
	"user.account.lock.limit":            {},
	"user.lifecycle.delete.initiated":    {},
	"user.lifecycle.suspend":             {},
	"user.lifecycle.deactivate":          {},
	"user.lifecycle.reactivate":          {},

	// MFA / auth tampering.
	"user.mfa.factor.reset_all":          {},
	"user.mfa.factor.update":             {},
	"user.mfa.factor.deactivate":         {},
	"user.session.access_admin_app":      {},

	// API / token surface.
	"system.api_token.create":            {},
	"system.api_token.delete":            {},
	"system.api_token.revoke":            {},

	// Admin role assignment changes — usually a quarterly review
	// item; spike here is worth investigating.
	"user.role.add":                      {},
	"user.role.remove":                   {},
	"group.role.add":                     {},
	"group.role.remove":                  {},

	// Policy / Rule mutations.
	"policy.lifecycle.create":            {},
	"policy.lifecycle.delete":            {},
	"policy.lifecycle.update":            {},
	"policy.rule.delete":                 {},
	"policy.rule.update":                 {},

	// Application config drift.
	"application.lifecycle.delete":       {},
	"application.lifecycle.deactivate":   {},
	"application.user_membership.add":    {},
	"application.user_membership.remove": {},
}

// fetchCriticalEvents pulls the last 6h of System Log entries,
// filters down to criticalEventTypes, and returns the most recent
// `limit` (newest first). 6h is long enough to capture the
// "overnight pager" window without burning an excessive log fetch.
func fetchCriticalEvents(ctx context.Context, port domain.LogsPort, now time.Time, limit int) ([]CriticalEvent, error) {
	if port == nil {
		return nil, nil
	}
	since := now.Add(-6 * time.Hour)
	q := domain.LogsQuery{
		Since:     &since,
		SortOrder: domain.SortDescending,
		Limit:     1000,
	}
	it, err := port.Search(ctx, q)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	out := make([]CriticalEvent, 0, limit*2)
	for {
		ev, hasMore, err := it.Next(ctx)
		if err != nil {
			return nil, err
		}
		if !hasMore {
			break
		}
		if _, hit := criticalEventTypes[ev.EventType]; !hit {
			continue
		}
		out = append(out, CriticalEvent{
			When:       ev.Published,
			EventType:  ev.EventType,
			DisplayMsg: ev.DisplayMsg,
			Severity:   ev.Severity,
			ActorLogin: ev.Actor.AlternateID,
			ActorID:    ev.Actor.ID,
			UUID:       ev.UUID,
		})
		// First target if any.
		if len(ev.Targets) > 0 {
			out[len(out)-1].TargetID = ev.Targets[0].ID
		}
	}

	// Newest first, then trim to `limit`.
	sort.Slice(out, func(i, j int) bool {
		return out[i].When.After(out[j].When)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
