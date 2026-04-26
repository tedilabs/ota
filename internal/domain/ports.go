package domain

import (
	"context"
	"time"
)

// UsersPort is the outbound boundary for Okta Users. Implemented by
// internal/okta.UsersAdapter. Consumed by internal/service.UsersService and
// TUI Screen Models.
//
// Contract:
//   - Every method respects ctx cancellation and propagates it to HTTP.
//   - Returned errors are domain sentinels (see errors.go) or errors.Is-compatible.
//   - List returns an Iterator; callers must Close when done.
type UsersPort interface {
	List(ctx context.Context, q UsersQuery) (Iterator[User], error)
	Get(ctx context.Context, idOrLogin string) (User, error)
	ListGroups(ctx context.Context, userID string) ([]Group, error)
	ListFactors(ctx context.Context, userID string) ([]Factor, error)
}

// GroupsPort is the outbound boundary for Okta Groups.
type GroupsPort interface {
	List(ctx context.Context, q GroupsQuery) (Iterator[Group], error)
	Get(ctx context.Context, id string) (Group, error)
	Members(ctx context.Context, q GroupMembersQuery) (Iterator[User], error)
	AppCount(ctx context.Context, id string) (int, error)
}

// GroupRulesPort is the outbound boundary for Okta Group Rules.
type GroupRulesPort interface {
	List(ctx context.Context, q GroupRulesQuery) (Iterator[GroupRule], error)
	Get(ctx context.Context, id string) (GroupRule, error)
}

// PoliciesPort is the outbound boundary for Okta Policies.
type PoliciesPort interface {
	List(ctx context.Context, q PoliciesQuery) (Iterator[Policy], error)
	Get(ctx context.Context, id string) (Policy, error)
	Rules(ctx context.Context, policyID string) ([]PolicyRule, error)
}

// LogsPort is the outbound boundary for Okta System Logs.
// Search is the single entrypoint for both history and tail modes — tail is
// implemented at the service layer by repeatedly calling Search with an
// advancing `since` cursor (REQ-R05 AC-2).
type LogsPort interface {
	Search(ctx context.Context, q LogsQuery) (Iterator[LogEvent], error)
}

// RateLimitSnapshot is a last-observed reading for a single API category
// (REQ-E01 AC-4). Category examples: "management", "logs", "policies", "apps".
type RateLimitSnapshot struct {
	Category  string
	Remaining int
	Limit     int
	Reset     time.Time
	Observed  time.Time // wall-clock time of observation (for staleness display)
}

// RateLimitPort exposes the adapter-level monitor to the UI layer so the
// statusbar and :ratelimit screen can render category snapshots.
type RateLimitPort interface {
	Snapshots() []RateLimitSnapshot
}

// HealthPort answers `:healthcheck` — a lightweight liveness call to the
// configured tenant (ARCHITECTURE §12.2).
type HealthPort interface {
	Check(ctx context.Context) error
}
