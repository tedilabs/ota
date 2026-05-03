package domain

// Administrator is one admin role assignment surfaced from Okta's
// IAM v2 endpoint (/api/v1/iam/assignees/users — returns every user
// who carries at least one admin role plus the role identifiers
// that resolved them). MVP renders the assignment as a flat row so
// operators can answer "who has admin access today?" without
// drilling per-user.
//
// Each Administrator captures one (user, role) pair — a principal
// with multiple admin roles produces multiple rows, mirroring how
// the API exposes the data.
type Administrator struct {
	UserID    string
	Login     string
	FirstName string
	LastName  string
	RoleID    string
	RoleType  string // SUPER_ADMIN / ORG_ADMIN / GROUP_MEMBERSHIP_ADMIN / READ_ONLY_ADMIN / etc.
	RoleLabel string // human-readable role description from Okta
	Status    string // STAFF | ASSIGNED — Okta's lifecycle for the assignment
}
