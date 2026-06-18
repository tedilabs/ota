package domain

import "time"

// GroupRuleStatus is the Okta group rule status (REQ-R03 AC-2).
type GroupRuleStatus string

const (
	GroupRuleStatusActive   GroupRuleStatus = "ACTIVE"
	GroupRuleStatusInactive GroupRuleStatus = "INACTIVE"
	// GroupRuleStatusInvalid is rendered in red (REQ-R03 AC-2) — operators must
	// notice it immediately.
	GroupRuleStatusInvalid  GroupRuleStatus = "INVALID"
)

// GroupRule is a dynamic membership rule (id prefix 0pr).
type GroupRule struct {
	ID          string
	Name        string
	Status      GroupRuleStatus
	// Expression is the Okta Expression Language source (read-only, displayed
	// in monospace per REQ-R03 AC-3).
	Expression string
	// TargetGroupIDs are the groups this rule assigns matching users to.
	// Names are resolved at the service layer (REQ-R03 AC-4).
	TargetGroupIDs []string
	Created        time.Time
	LastUpdated    time.Time
}

// GroupRuleUpdate carries the editable rule fields. Okta requires
// the rule to be INACTIVE / INVALID before the PUT — the screen
// guards on Status before opening the form. Strict-replace
// semantics: every field must be supplied (the screen reads
// unchanged fields from the loaded snapshot).
type GroupRuleUpdate struct {
	Name           string
	Expression     string
	TargetGroupIDs []string
}
