// Package domain contains ota's pure core types, invariants, and Port interfaces.
//
// External dependencies (Okta SDK, net/http, filesystem, time) MUST NOT be
// imported from this package. The adapter layer (internal/okta) converts SDK
// types to these domain types; TUI and service layers consume them directly.
//
// Port interfaces (UsersPort, GroupsPort, ...) are declared here as the
// neutral consumer-side contract (see docs/ARCHITECTURE.md §6.2).
package domain
