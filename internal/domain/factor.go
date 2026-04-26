package domain

import "time"

// FactorType is the Okta MFA factor type (REQ-R01 AC-6).
type FactorType string

const (
	FactorTypePush         FactorType = "push"                 // Okta Verify Push
	FactorTypeTOTP         FactorType = "token:software:totp"
	FactorTypeSMS          FactorType = "sms"
	FactorTypeCall         FactorType = "call"
	FactorTypeEmail        FactorType = "email"
	FactorTypeWebAuthn     FactorType = "webauthn"
	FactorTypeHardwareToken FactorType = "token:hardware"
	FactorTypeQuestion     FactorType = "question"
)

// FactorStatus mirrors Okta's enrollment lifecycle for a factor.
type FactorStatus string

const (
	FactorStatusNotSetup           FactorStatus = "NOT_SETUP"
	FactorStatusPendingActivation  FactorStatus = "PENDING_ACTIVATION"
	FactorStatusActive             FactorStatus = "ACTIVE"
	FactorStatusExpired            FactorStatus = "EXPIRED"
	FactorStatusDisabled           FactorStatus = "DISABLED"
)

// Factor is a registered MFA factor for a user.
type Factor struct {
	ID          string
	Type        FactorType
	Provider    string // OKTA / FIDO / DUO / GOOGLE / ...
	VendorName  string
	Status      FactorStatus
	Profile     FactorProfile
	Created     time.Time
	LastUpdated time.Time
}

// FactorProfile holds type-specific details (PII-bearing).
type FactorProfile struct {
	PhoneNumber  string // SMS/Voice — PII, masked by default
	Email        string // Email factor — PII, masked by default
	CredentialID string // WebAuthn
	DeviceType   string // Okta Verify
	Name         string // Okta Verify device model
}
