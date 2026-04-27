package domain

import (
	"encoding/json"
	"time"
)

// AppType is the Okta application sign-on / integration kind.
// Surfaces in the apps list and powers the per-type palette routes
// (issue #166): `:saml-app`, `:oidc-app`, `:bookmark-app`, etc.
type AppType string

const (
	AppTypeSAML     AppType = "SAML_2_0"
	AppTypeOIDC     AppType = "OPENID_CONNECT"
	AppTypeBookmark AppType = "BOOKMARK"
	AppTypeSWA      AppType = "AUTO_LOGIN"
	AppTypeSCIM     AppType = "SCIM"
	AppTypeOther    AppType = "OTHER"
)

// AppStatus is the application lifecycle state.
type AppStatus string

const (
	AppStatusActive   AppStatus = "ACTIVE"
	AppStatusInactive AppStatus = "INACTIVE"
)

// App represents an Okta application instance.
type App struct {
	ID          string
	Name        string // canonical app name (e.g., "okta_org2org")
	Label       string // operator-facing display label
	Status      AppStatus
	SignOnMode  string  // SAML_2_0 / OPENID_CONNECT / BOOKMARK / AUTO_LOGIN / ...
	Type        AppType // derived from SignOnMode for grouping/filtering
	Created     time.Time
	LastUpdated time.Time
	// Raw preserves the full app JSON for the detail view's Raw tab.
	Raw json.RawMessage
}
