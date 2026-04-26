package domain

import (
	"encoding/json"
	"time"
)

// PolicyType classifies an Okta policy. MVP supports 7 types (REQ-R04 AC-1).
type PolicyType string

const (
	PolicyTypeOktaSignOn          PolicyType = "OKTA_SIGN_ON"
	PolicyTypeAccessPolicy        PolicyType = "ACCESS_POLICY"
	PolicyTypePassword            PolicyType = "PASSWORD"
	PolicyTypeMFAEnroll           PolicyType = "MFA_ENROLL"
	PolicyTypeProfileEnrollment   PolicyType = "PROFILE_ENROLLMENT"
	PolicyTypePostAuthSession     PolicyType = "POST_AUTH_SESSION"
	PolicyTypeIDPDiscovery        PolicyType = "IDP_DISCOVERY"
)

// RichRenderedPolicyTypes returns the set that have custom action summary
// rendering (REQ-R04 AC-5). The remaining three use raw JSON view only.
func RichRenderedPolicyTypes() []PolicyType {
	return []PolicyType{
		PolicyTypeOktaSignOn,
		PolicyTypeAccessPolicy,
		PolicyTypePassword,
		PolicyTypeMFAEnroll,
	}
}

// PolicyStatus is enabled/disabled per policy (REQ-R04 AC-3).
type PolicyStatus string

const (
	PolicyStatusActive   PolicyStatus = "ACTIVE"
	PolicyStatusInactive PolicyStatus = "INACTIVE"
)

// Policy describes an Okta policy (id prefix 00p).
type Policy struct {
	ID          string
	Name        string
	Description string
	Type        PolicyType
	Priority    int
	Status      PolicyStatus
	System      bool // base policies cannot be deactivated/deleted (REQ-R04 AC-3 SYS badge)
	Created     time.Time
	LastUpdated time.Time
	// Raw preserves the original JSON for the `r` toggle / raw-only types.
	Raw json.RawMessage
}

// PolicyRule is a rule inside a policy (priority-ordered).
type PolicyRule struct {
	ID          string
	Name        string
	Priority    int
	Status      PolicyStatus
	System      bool
	Created     time.Time
	LastUpdated time.Time
	Raw         json.RawMessage
}
