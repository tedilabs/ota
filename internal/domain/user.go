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

// UserProfile mirrors Okta's user profile. Custom fields are preserved in Extras.
type UserProfile struct {
	Login       string // primary identifier (email)
	Email       string
	FirstName   string
	LastName    string
	DisplayName string
	MobilePhone string // PII — masked by default in views
	SecondEmail string // PII — masked by default in views
	Department  string
	// Extras holds organization-specific custom profile fields (PRD §7.10).
	Extras map[string]any
}

// UserCredentials captures the minimal credential metadata read-only views use.
type UserCredentials struct {
	Provider     string // OKTA / FEDERATION / SOCIAL / ...
	ProviderType string
}
