package home

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
)

// Tests for countActivity live in the internal package because the
// function is unexported and the bucketing semantics are the contract
// the Activity card depends on. Every new EventType case added to
// countActivity should grow a row here.

func Test_countActivity_BucketsExpandedEventTypes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	events := []domain.LogEvent{
		// Identity
		{EventType: "user.session.start", Published: now.Add(-30 * time.Minute)},
		{EventType: "user.session.start", Published: now.Add(-20 * time.Minute), Outcome: domain.Outcome{Result: domain.OutcomeFailure}},
		{EventType: "user.account.lock", Published: now.Add(-15 * time.Minute)},
		{EventType: "user.mfa.factor.reset_all", Published: now.Add(-10 * time.Minute)},

		// Lifecycle — new buckets added in the Option A pivot.
		{EventType: "user.lifecycle.create", Published: now.Add(-50 * time.Minute)},
		{EventType: "user.lifecycle.delete.initiated", Published: now.Add(-50 * time.Minute)},
		{EventType: "user.lifecycle.suspend", Published: now.Add(-40 * time.Minute)},
		{EventType: "user.lifecycle.reactivate", Published: now.Add(-40 * time.Minute)},

		// App churn — new buckets.
		{EventType: "application.user_membership.add", Published: now.Add(-35 * time.Minute)},
		{EventType: "application.user_membership.add", Published: now.Add(-35 * time.Minute)},
		{EventType: "application.user_membership.remove", Published: now.Add(-35 * time.Minute)},
		{EventType: "application.lifecycle.deactivate", Published: now.Add(-35 * time.Minute)},

		// Admin surface — token / role / policy writes (also flips
		// AdminActions since the actor.type is User).
		{EventType: "system.api_token.create", Published: now.Add(-30 * time.Minute), Actor: domain.Actor{ID: "00u_alice", Type: domain.ActorTypeUser}},
		{EventType: "system.api_token.delete", Published: now.Add(-25 * time.Minute), Actor: domain.Actor{ID: "00u_alice", Type: domain.ActorTypeUser}},
		{EventType: "user.role.add", Published: now.Add(-25 * time.Minute)},
		{EventType: "group.role.remove", Published: now.Add(-25 * time.Minute)},
		{EventType: "policy.lifecycle.update", Published: now.Add(-20 * time.Minute), Actor: domain.Actor{ID: "00u_bob", Type: domain.ActorTypeUser}},
		{EventType: "policy.rule.delete", Published: now.Add(-20 * time.Minute)},
	}
	logs := fakes.NewLogsPort(t)
	logs.SearchFunc = func(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		return &fakes.SliceIterator[domain.LogEvent]{Items: events}, nil
	}

	got, sampled, err := countActivity(context.Background(), logs, now, activityWindow{
		label: "1h", since: time.Hour, withSpark: false,
	})
	require.NoError(t, err)
	assert.False(t, sampled, "iterator drained cleanly, sampled flag must stay false")

	// Identity.
	assert.Equal(t, 2, got.SignIns, "both session.start events counted as sign-ins")
	assert.Equal(t, 1, got.FailedSignIns, "FAILURE outcome flips FailedSignIns")
	assert.Equal(t, 1, got.AccountLocks)
	assert.Equal(t, 1, got.MFAResets)

	// Lifecycle.
	assert.Equal(t, 1, got.UserCreates)
	assert.Equal(t, 1, got.UserDeletes)
	assert.Equal(t, 1, got.UserSuspends)
	assert.Equal(t, 1, got.UserReactivates)

	// App churn.
	assert.Equal(t, 2, got.AppAssignAdds)
	assert.Equal(t, 1, got.AppAssignRemoves)
	assert.Equal(t, 1, got.AppConfigChanges)

	// Admin surface.
	assert.Equal(t, 2, got.APITokenWrites, "create + delete fall into APITokenWrites")
	assert.Equal(t, 2, got.RoleChanges, "user.role.add + group.role.remove")
	assert.Equal(t, 2, got.PolicyMutations, "policy.lifecycle.update + policy.rule.delete")
	// AdminActions fires for system.* / policy.lifecycle.* when actor.type=User.
	// system.api_token.create + delete (alice) + policy.lifecycle.update (bob) = 3.
	assert.Equal(t, 3, got.AdminActions,
		"AdminActions counts system.* + policy.lifecycle.* events with User actor")
}

func Test_countActivity_SampledFlag_FlipsWhenPageFull(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	// Build logsSampleSize events — the iterator never returns
	// hasMore=false, so countActivity should bail out via the
	// sample-cap branch and flip sampled.
	events := make([]domain.LogEvent, logsSampleSize+5)
	for i := range events {
		events[i] = domain.LogEvent{
			EventType: "user.session.start",
			Published: now.Add(-time.Duration(i) * time.Second),
		}
	}
	logs := fakes.NewLogsPort(t)
	logs.SearchFunc = func(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		return &fakes.SliceIterator[domain.LogEvent]{Items: events}, nil
	}

	_, sampled, err := countActivity(context.Background(), logs, now, activityWindow{
		label: "24h", since: 24 * time.Hour, withSpark: true,
	})
	require.NoError(t, err)
	assert.True(t, sampled, "hitting logsSampleSize must flip the sampled flag")
}

func Test_countPosture_BucketsAndDistinctActors(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	events := []domain.LogEvent{
		// Identity / auth posture.
		{EventType: "user.session.start", Published: now.Add(-3 * 24 * time.Hour)},
		{EventType: "user.session.start", Published: now.Add(-3 * 24 * time.Hour)},
		{EventType: "user.session.start", Published: now.Add(-3 * 24 * time.Hour), Outcome: domain.Outcome{Result: domain.OutcomeFailure}},
		{EventType: "user.account.lock", Published: now.Add(-3 * 24 * time.Hour)},
		{EventType: "user.mfa.factor.reset_all", Published: now.Add(-2 * 24 * time.Hour)},

		// Lifecycle destructive.
		{EventType: "user.lifecycle.delete.initiated", Published: now.Add(-2 * 24 * time.Hour)},
		{EventType: "user.lifecycle.suspend", Published: now.Add(-2 * 24 * time.Hour)},
		{EventType: "application.lifecycle.delete", Published: now.Add(-1 * 24 * time.Hour)},

		// Sensitive admin writes — three distinct actors (alice / bob /
		// alice repeated) → distinct count must dedupe to 2.
		{EventType: "system.api_token.create", Published: now.Add(-24 * time.Hour), Actor: domain.Actor{ID: "00u_alice"}},
		{EventType: "user.role.add", Published: now.Add(-12 * time.Hour), Actor: domain.Actor{ID: "00u_bob"}},
		{EventType: "policy.lifecycle.update", Published: now.Add(-6 * time.Hour), Actor: domain.Actor{ID: "00u_alice"}},
	}
	logs := fakes.NewLogsPort(t)
	logs.SearchFunc = func(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		return &fakes.SliceIterator[domain.LogEvent]{Items: events}, nil
	}

	got := countPosture(context.Background(), logs, now)

	assert.Empty(t, got.Err)
	assert.False(t, got.Sampled, "iterator drained cleanly, sampled flag stays false")

	assert.Equal(t, 3, got.SignIns7d)
	assert.Equal(t, 1, got.FailedSignIns7d)
	assert.Equal(t, 1, got.AccountLocks7d)
	assert.Equal(t, 1, got.MFAResets7d)
	assert.Equal(t, 1, got.UserDeletes7d)
	assert.Equal(t, 1, got.UserSuspends7d)
	assert.Equal(t, 1, got.AppRemoves7d)
	assert.Equal(t, 3, got.SensitiveWrites7d,
		"all three admin-write events sum into SensitiveWrites7d")
	assert.Equal(t, 2, got.DistinctAdminActors7d,
		"alice repeated must dedupe → 2 distinct actors")
}
