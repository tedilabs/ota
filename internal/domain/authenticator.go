package domain

import "time"

// Authenticator is a configured Okta authenticator (#F1 v0.2.5).
// The Authenticators API surfaces the org's enrolled methods —
// password, email, phone (SMS/Voice), security_question, webauthn,
// okta_verify, etc. Each carries a type + key + status with optional
// per-method settings.
type Authenticator struct {
	ID          string
	Type        AuthenticatorType
	Key         string // method-specific key, e.g. "okta_verify", "phone_number"
	Name        string // human-readable name
	Status      AuthenticatorStatus
	Provider    AuthenticatorProvider // OKTA / DUO / GOOGLE / RSA / SYMANTEC / YUBICO
	Created     time.Time
	LastUpdated time.Time
}

// AuthenticatorType is the Okta classification of the authenticator
// method (password / email / phone / security_question / app /
// security_key / federated). Maps directly to the API response's
// `type` field.
type AuthenticatorType string

const (
	AuthenticatorTypePassword         AuthenticatorType = "password"
	AuthenticatorTypeEmail            AuthenticatorType = "email"
	AuthenticatorTypePhone            AuthenticatorType = "phone"
	AuthenticatorTypeSecurityQuestion AuthenticatorType = "security_question"
	AuthenticatorTypeApp              AuthenticatorType = "app"
	AuthenticatorTypeSecurityKey      AuthenticatorType = "security_key"
	AuthenticatorTypeFederated        AuthenticatorType = "federated"
)

// AuthenticatorStatus mirrors the Okta lifecycle. ACTIVE methods can
// be enrolled by users; INACTIVE ones are configured but disabled.
type AuthenticatorStatus string

const (
	AuthenticatorStatusActive   AuthenticatorStatus = "ACTIVE"
	AuthenticatorStatusInactive AuthenticatorStatus = "INACTIVE"
)

// AuthenticatorProvider is the issuer / vendor of the authenticator.
// Most factors are OKTA; DUO / RSA / SYMANTEC etc. plug in via the
// Vendor adapter pattern.
type AuthenticatorProvider string
