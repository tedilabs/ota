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

	// Activate transitions a STAGED / DEPROVISIONED user into ACTIVE
	// (issue #187 v0.2.2). When sendEmail is true Okta sends the
	// activation invitation; false suppresses it (typical for
	// admin-managed flows).
	Activate(ctx context.Context, userID string, sendEmail bool) error
	// Deactivate transitions any non-DEPROVISIONED user into
	// DEPROVISIONED. Reversible only via Activate. sendEmail flag
	// matches Okta's API.
	Deactivate(ctx context.Context, userID string, sendEmail bool) error
	// ExpirePassword forces the next sign-in to require a password
	// change. The user's existing password remains valid until the
	// next attempt — distinct from ResetPassword which immediately
	// invalidates the credential.
	ExpirePassword(ctx context.Context, userID string) error
	// Delete removes a DEPROVISIONED user permanently. Okta requires
	// the account already be deactivated, so callers should chain
	// Deactivate first when the operator confirms a hard delete.
	Delete(ctx context.Context, userID string) error

	// Suspend transitions an ACTIVE user into SUSPENDED — blocks
	// sign-in but keeps assignments + group membership + factors
	// intact. Reversible via Unsuspend (distinct from
	// Deactivate / Activate which destroy and re-provision state).
	// Used by the status picker to flip the lifecycle from a single
	// keypress.
	Suspend(ctx context.Context, userID string) error
	// Unsuspend transitions a SUSPENDED user back to ACTIVE. No-op
	// (Okta returns 200) on users already in another state.
	Unsuspend(ctx context.Context, userID string) error

	// UpdateProfile applies a partial-merge profile patch
	// (REQ-W01 / D-T4). Returns the updated User (server echo) so
	// the caller can patch its list/detail cache with the last-write
	// snapshot. When patch.IsEmpty() is true, MUST return
	// ErrEmptyPatch without making an HTTP call (D-T5 / D-W13).
	//
	// Error mapping (PRD §5.6 AC-6):
	//   - *BadRequestError (E0000001) — Causes []FieldError for inline display
	//   - ErrTokenInvalid (E0000004 / E0000011)
	//   - ErrForbidden (E0000006)
	//   - ErrNotFound (E0000007)
	//   - ErrFeatureDisabled (E0000038)
	//   - *RateLimitedError (E0000047 / 429)
	//   - ErrEmptyPatch (IsEmpty short-circuit)
	UpdateProfile(ctx context.Context, userID string, patch UserProfilePatch) (User, error)
}

// GroupsPort is the outbound boundary for Okta Groups.
type GroupsPort interface {
	List(ctx context.Context, q GroupsQuery) (Iterator[Group], error)
	Get(ctx context.Context, id string) (Group, error)
	Members(ctx context.Context, q GroupMembersQuery) (Iterator[User], error)
	AppCount(ctx context.Context, id string) (int, error)
	// ListApps returns the apps assigned to the group via
	// `/api/v1/groups/{id}/apps`. Powers the Group Detail Apps
	// box (issue #189 v0.2.2). Distinct from AppCount which
	// only returns the cardinality for the Groups list column.
	ListApps(ctx context.Context, groupID string) ([]App, error)
}

// GroupRulesPort is the outbound boundary for Okta Group Rules.
type GroupRulesPort interface {
	List(ctx context.Context, q GroupRulesQuery) (Iterator[GroupRule], error)
	Get(ctx context.Context, id string) (GroupRule, error)
	// Activate transitions a rule from INACTIVE / INVALID to ACTIVE
	// (issue #188 v0.2.2). Okta evaluates the rule's expression on
	// activation; an invalid expression keeps the rule at INVALID
	// regardless.
	Activate(ctx context.Context, ruleID string) error
	// Deactivate transitions a rule to INACTIVE.
	Deactivate(ctx context.Context, ruleID string) error
	// Delete removes a rule permanently. Okta requires the rule to
	// be INACTIVE — callers should chain Deactivate when needed.
	Delete(ctx context.Context, ruleID string) error
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

// AuthenticatorsPort is the outbound boundary for Okta Authenticators
// — the org-wide set of factors (password / email / phone /
// security_question / okta_verify / webauthn / etc.) operators can
// enable for end-user enrollment. Issue #F1 v0.2.5.
type AuthenticatorsPort interface {
	List(ctx context.Context) ([]Authenticator, error)
	Get(ctx context.Context, id string) (Authenticator, error)
}

// NetworkZonesPort is the outbound boundary for Okta Network Zones
// (/api/v1/zones). Read-only in MVP — operators inspect the
// configured IP / dynamic boundaries that policies reference.
type NetworkZonesPort interface {
	List(ctx context.Context) ([]NetworkZone, error)
	Get(ctx context.Context, id string) (NetworkZone, error)
}

// AuthorizationServersPort is the outbound boundary for Okta Custom
// Authorization Servers (/api/v1/authorizationServers). Read-only in
// MVP.
type AuthorizationServersPort interface {
	List(ctx context.Context) ([]AuthorizationServer, error)
	Get(ctx context.Context, id string) (AuthorizationServer, error)
}

// APITokensPort is the outbound boundary for Okta API Tokens
// (/api/v1/api-tokens). Read-only in MVP — minting / revoking is
// out of scope for the inspect-only surface.
type APITokensPort interface {
	List(ctx context.Context) ([]APIToken, error)
}

// AdministratorsPort is the outbound boundary for Okta IAM admin
// role assignments. List flattens
// /api/v1/iam/assignees/users + per-user /api/v1/users/{id}/roles
// into one (user, role) pair per row.
type AdministratorsPort interface {
	List(ctx context.Context) ([]Administrator, error)
}

// LogsPort is the outbound boundary for Okta System Logs.
// Search is the iterator-style entrypoint used by tail mode (auto-
// follows Link: rel="next" until exhausted). SearchPage is the
// one-page entrypoint used by History mode (#F3 v0.2.5) so the
// operator drives "load older" pagination explicitly via the `after`
// cursor returned in LogPage.After.
type LogsPort interface {
	Search(ctx context.Context, q LogsQuery) (Iterator[LogEvent], error)
	SearchPage(ctx context.Context, q LogsQuery) (LogPage, error)
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
