package domain

import "time"

// GroupType is the Okta group classification (REQ-R02 AC-1).
type GroupType string

const (
	GroupTypeOkta    GroupType = "OKTA_GROUP"
	GroupTypeApp     GroupType = "APP_GROUP"
	GroupTypeBuiltIn GroupType = "BUILT_IN"
)

// Group represents an Okta group.
type Group struct {
	ID                      string
	Type                    GroupType
	Profile                 GroupProfile
	Created                 time.Time
	LastUpdated             time.Time
	LastMembershipUpdated   *time.Time
	// DynamicTargeted marks groups that are targeted by at least one Group Rule
	// (the RULE badge in REQ-R02 AC-1). Derived at runtime, not returned by the
	// Okta API directly.
	DynamicTargeted bool
	// MemberCount carries _embedded.stats.usersCount when the list
	// query enables expand=stats (issue #161). nil means "unknown"
	// — render "—" in the list rather than "0".
	MemberCount *int
}

// GroupProfile carries the human-facing fields of a group.
type GroupProfile struct {
	Name        string
	Description string
}
