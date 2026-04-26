package domain

import "time"

// UserStatus is an Okta user lifecycle state.
type UserStatus string

// Okta user lifecycle states (see REQ-R01 AC-2).
const (
	UserStatusStaged           UserStatus = "STAGED"
	UserStatusProvisioned      UserStatus = "PROVISIONED"
	UserStatusActive           UserStatus = "ACTIVE"
	UserStatusSuspended        UserStatus = "SUSPENDED"
	UserStatusLockedOut        UserStatus = "LOCKED_OUT"
	UserStatusPasswordExpired  UserStatus = "PASSWORD_EXPIRED"
	UserStatusDeprovisioned    UserStatus = "DEPROVISIONED"
)

// User represents an Okta user with its core fields exposed to ota.
type User struct {
	ID             string
	Status         UserStatus
	Profile        UserProfile
	Credentials    UserCredentials
	Created        time.Time
	Activated      *time.Time
	StatusChanged  *time.Time
	LastLogin      *time.Time
	LastUpdated    time.Time
	PasswordChanged *time.Time
}

// UserProfile mirrors Okta's user profile. Custom fields are preserved in
// Extras. The named fields below cover Okta's standard profile schema —
// keep them in sync with the Okta API "User Profile" object so the
// adapter can map them straight in.
type UserProfile struct {
	Login          string // primary identifier (email)
	Email          string
	FirstName      string
	LastName       string
	DisplayName    string
	NickName       string // Okta standard field
	Title          string // Okta standard field
	Division       string // Okta standard field
	Department     string
	EmployeeNumber string // Okta standard field
	MobilePhone    string // PII — masked by default in views
	SecondEmail    string // PII — masked by default in views
	// Extras holds organization-specific custom profile fields (PRD §7.10).
	Extras map[string]any
}

// UserCredentials captures the minimal credential metadata read-only views use.
type UserCredentials struct {
	Provider     string // OKTA / FEDERATION / SOCIAL / ...
	ProviderType string
}
