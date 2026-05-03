package domain

import "time"

// NetworkZone is an Okta-managed IP / dynamic / location boundary
// referenced by sign-on policies, MFA enrollment policies, and the
// Threat Insight blocklist. The Zones API surfaces both kinds:
//
//   - IP zones list explicit gateway / proxy CIDR ranges (the org's
//     egress IPs, partner networks, …).
//   - Dynamic zones reference geo + ASN + proxy attributes Okta
//     resolves at request time (e.g. "all of Russia + known TOR
//     exit nodes").
//
// MVP carries the fields ota's list/detail surfaces actually
// render — heavier fields (proxies / locations) are reachable via
// the Raw JSON tab.
type NetworkZone struct {
	ID          string
	Name        string
	Type        NetworkZoneType   // IP / DYNAMIC
	Status      NetworkZoneStatus // ACTIVE / INACTIVE
	Usage       NetworkZoneUsage  // POLICY / BLOCKLIST
	System      bool              // org-managed (e.g. LegacyIpZone)
	Created     time.Time
	LastUpdated time.Time
}

// NetworkZoneType is the API's `type` discriminator.
type NetworkZoneType string

const (
	NetworkZoneTypeIP      NetworkZoneType = "IP"
	NetworkZoneTypeDynamic NetworkZoneType = "DYNAMIC"
)

// NetworkZoneStatus mirrors the Okta lifecycle.
type NetworkZoneStatus string

const (
	NetworkZoneStatusActive   NetworkZoneStatus = "ACTIVE"
	NetworkZoneStatusInactive NetworkZoneStatus = "INACTIVE"
)

// NetworkZoneUsage is the zone's intended role: POLICY zones feed
// sign-on rules; BLOCKLIST zones feed Threat Insight denies.
type NetworkZoneUsage string

const (
	NetworkZoneUsagePolicy    NetworkZoneUsage = "POLICY"
	NetworkZoneUsageBlocklist NetworkZoneUsage = "BLOCKLIST"
)
