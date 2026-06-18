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

// GroupProfileUpdate carries the full replacement profile a Group
// edit submits. Okta's PUT /api/v1/groups/{id} is strict-replace —
// the wire body must include every profile field, even those the
// operator didn't change. The screen rebuilds this struct from the
// loaded snapshot + the form's dirty diff so unchanged fields aren't
// dropped. Only OKTA_GROUP types accept profile updates; the screen
// guards on Type before opening the form.
type GroupProfileUpdate struct {
	Name        string
	Description string
}
