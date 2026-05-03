package domain

import "time"

// AuthorizationServer is an Okta OAuth 2.0 / OIDC Custom Authorization
// Server — the per-org or per-app issuer operators configure to
// mint access tokens with custom claims, scopes, and policies. The
// list endpoint returns these without their nested policies/claims;
// those are reachable via per-server endpoints when needed.
type AuthorizationServer struct {
	ID          string
	Name        string
	Description string
	Audiences   []string
	Issuer      string
	Status      AuthorizationServerStatus
	Created     time.Time
	LastUpdated time.Time
}

// AuthorizationServerStatus mirrors the Okta lifecycle.
type AuthorizationServerStatus string

const (
	AuthorizationServerStatusActive   AuthorizationServerStatus = "ACTIVE"
	AuthorizationServerStatusInactive AuthorizationServerStatus = "INACTIVE"
)
