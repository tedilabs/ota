// Package okta is the outbound adapter: wraps okta-sdk-golang/v5, maps SDK
// types to domain types, observes rate-limit headers, parses Link-header
// pagination, and maps Okta errorCodes to domain.Err* (PRD §7.7).
//
// SDK types MUST NOT escape this package.
package okta
