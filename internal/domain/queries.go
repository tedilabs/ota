package domain

import "time"

// SortOrder applies to endpoints that accept ordering (e.g., /logs).
type SortOrder string

const (
	SortAscending  SortOrder = "ASCENDING"
	SortDescending SortOrder = "DESCENDING"
)

// UsersQuery parameterizes UsersPort.List (REQ-U03/U04).
type UsersQuery struct {
	// Q is Okta's `q` free-text match (prefix/substring).
	Q string
	// Search is Okta's `search` SCIM-like expression. Mutually used with Q;
	// callers pass at most one typically.
	Search string
	// Filter is the strict `filter` expression (rarely used for Users).
	Filter string
	Limit  int
	After  string // opaque cursor from PageInfo
}

// GroupsQuery parameterizes GroupsPort.List.
type GroupsQuery struct {
	Q      string
	Search string
	Filter string
	Limit  int
	After  string
}

// GroupMembersQuery parameterizes GroupsPort.Members.
type GroupMembersQuery struct {
	GroupID string
	Limit   int
	After   string
}

// GroupRulesQuery parameterizes GroupRulesPort.List.
type GroupRulesQuery struct {
	Limit int
	After string
}

// PoliciesQuery parameterizes PoliciesPort.List.
type PoliciesQuery struct {
	Type  PolicyType
	Limit int
	After string
}

// AppsQuery parameterizes AppsPort.List. Type, when non-empty,
// narrows the result set to that AppType (issue #166's per-type
// palette routes feed this directly).
type AppsQuery struct {
	Type   AppType
	Q      string
	Filter string
	Limit  int
	After  string
}

// LogsQuery parameterizes LogsPort.Search (REQ-R05).
type LogsQuery struct {
	Since     *time.Time
	Until     *time.Time
	Filter    string
	Q         string
	SortOrder SortOrder
	Limit     int
	After     string
}
