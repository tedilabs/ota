package domain

import "time"

// APIToken is an Okta org-scoped API token (the SSWS bearer values
// operators mint to drive Workforce Identity / management APIs).
// Listing returns metadata only; the token's secret value is shown
// exactly once at creation time and never returned afterward.
//
// Fields:
//   - ID / Name — operator-set identifier + label.
//   - UserID    — the owner principal (admin user) the token was
//                 minted for.
//   - ClientName — UA / client name the operator passed at create.
//   - LastUpdated / Created — wall-clock stamps.
//   - ExpiresAt — token expiry (zero if never).
type APIToken struct {
	ID          string
	Name        string
	UserID      string
	ClientName  string
	Created     time.Time
	LastUpdated time.Time
	ExpiresAt   time.Time
}
