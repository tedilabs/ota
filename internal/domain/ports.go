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
	// ListAppLinks returns the app dashboard links for the user —
	// what /api/v1/users/{id}/appLinks reports. Powers the "assigned
	// apps" section on User Detail (issue #168).
	ListAppLinks(ctx context.Context, userID string) ([]AppLink, error)

	// Lifecycle operations (issue #125). These are the first WRITE ops
	// in ota — they alter the user's auth state in the upstream Okta
	// tenant, so the TUI surfaces them behind a confirmation modal and
	// always logs an audit line via the toast bar.
	//
	// ResetPassword sends the standard reset-password email when
	// sendEmail is true; when false it returns a one-time
	// resetPasswordUrl the operator can hand out manually.
	ResetPassword(ctx context.Context, userID string, sendEmail bool) (string, error)
	// Unlock clears the LOCKED_OUT state. No-op (returns nil) on users
	// already in another state — Okta accepts the call and returns 200.
	Unlock(ctx context.Context, userID string) error
	// ResetFactors removes every enrolled MFA factor on the user, so
	// next sign-in forces re-enrollment. Destructive.
	ResetFactors(ctx context.Context, userID string) error
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

// AppsPort is the outbound boundary for Okta Applications. Powers
// the Apps screen's type-select → list → detail flow (issue #166).
type AppsPort interface {
	List(ctx context.Context, q AppsQuery) (Iterator[App], error)
	Get(ctx context.Context, id string) (App, error)
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
