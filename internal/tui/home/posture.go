package home

import (
	"context"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// PostureMetrics is the Risk & Governance card payload.
//
// After the Option A pivot (2026-06) every signal here is derived
// from a single 7-day /api/v1/logs walk — the dashboard no longer
// fans out across Administrators / APITokens / GroupRules /
// Authenticators ports, which would burn the management
// rate-limit category every time the card refreshed.
//
// Each field captures a governance signal an Okta admin would
// otherwise have to derive by clicking through the System Log UI
// with three or four filter facets.
type PostureMetrics struct {
	WindowSince time.Time // 7d back from ObservedAt
	ObservedAt  time.Time

	// Identity surface — 7d sign-in posture.
	SignIns7d        int
	FailedSignIns7d  int
	AccountLocks7d   int
	MFAResets7d      int

	// Admin surface — sprawl + sensitive-write pressure.
	SensitiveWrites7d     int // API token + role + policy mutations
	DistinctAdminActors7d int // unique actors behind those writes

	// Lifecycle surface — destructive moves worth a glance.
	UserDeletes7d   int
	AppRemoves7d    int // application.lifecycle.delete + lifecycle.deactivate
	UserSuspends7d  int

	Sampled bool
	Err     string
}

// countPosture reads a single page of System Log events from the
// last 7 days and buckets them into the Risk & Governance signals.
// Single-page bounded (logsSampleSize) — busy tenants see a
// `Sampled` flag flip and the card renders an "≈" prefix on the
// affected rows.
func countPosture(ctx context.Context, port domain.LogsPort, now time.Time) PostureMetrics {
	out := PostureMetrics{
		ObservedAt:  now,
		WindowSince: now.AddDate(0, 0, -7),
	}
	if port == nil {
		return out
	}
	since := out.WindowSince
	q := domain.LogsQuery{
		Since:     &since,
		SortOrder: domain.SortDescending,
		Limit:     logsSampleSize,
	}
	it, err := port.Search(ctx, q)
	if err != nil {
		out.Err = truncate(err.Error(), 64)
		return out
	}
	defer it.Close()

	adminActors := map[string]struct{}{}
	noteAdmin := func(actorID string) {
		if actorID == "" {
			return
		}
		adminActors[actorID] = struct{}{}
	}

	for i := 0; i < logsSampleSize; i++ {
		ev, hasMore, err := it.Next(ctx)
		if err != nil {
			out.Err = truncate(err.Error(), 64)
			return out
		}
		if !hasMore {
			out.DistinctAdminActors7d = len(adminActors)
			return out
		}
		switch ev.EventType {
		case "user.session.start":
			out.SignIns7d++
			if ev.Outcome.Result == domain.OutcomeFailure {
				out.FailedSignIns7d++
			}
		case "user.account.lock":
			out.AccountLocks7d++
		case "user.mfa.factor.reset_all":
			out.MFAResets7d++

		// Lifecycle.
		case "user.lifecycle.delete.initiated":
			out.UserDeletes7d++
		case "user.lifecycle.suspend":
			out.UserSuspends7d++
		case "application.lifecycle.delete",
			"application.lifecycle.deactivate":
			out.AppRemoves7d++

		// Sensitive admin writes — every entry here also notes the
		// actor for the distinct-admin-actor count.
		case "system.api_token.create",
			"system.api_token.delete",
			"system.api_token.revoke",
			"user.role.add",
			"user.role.remove",
			"group.role.add",
			"group.role.remove",
			"policy.lifecycle.create",
			"policy.lifecycle.update",
			"policy.lifecycle.delete",
			"policy.rule.update",
			"policy.rule.delete":
			out.SensitiveWrites7d++
			noteAdmin(ev.Actor.ID)
		}
	}
	// Hit the sample cap — there are likely more events we
	// deliberately didn't drain.
	out.Sampled = true
	out.DistinctAdminActors7d = len(adminActors)
	return out
}
