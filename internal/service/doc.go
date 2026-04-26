// Package service holds use-case level orchestration: query normalization,
// TTL caching (REQ-E01 AC-6), tail polling (REQ-R05), and profile reset.
//
// Services depend only on domain.*Port interfaces; concrete adapters
// (internal/okta.*) are wired in cmd/ota/wire.go. SDK types MUST NOT reach
// this package (enforced by depguard).
package service
