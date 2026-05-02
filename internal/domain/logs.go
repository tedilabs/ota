package domain

import (
	"encoding/json"
	"time"
)

// Severity is an Okta System Log severity (REQ-R05 AC-1).
type Severity string

const (
	SeverityDebug Severity = "DEBUG"
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
)

// OutcomeResult is the event outcome category.
type OutcomeResult string

const (
	OutcomeSuccess        OutcomeResult = "SUCCESS"
	OutcomeFailure        OutcomeResult = "FAILURE"
	OutcomeChallenge      OutcomeResult = "CHALLENGE"
	OutcomeSkipped        OutcomeResult = "SKIPPED"
	OutcomeUnknown        OutcomeResult = "UNKNOWN"
)

// ActorType distinguishes human Users from SystemPrincipal (REQ-R05 AC-8).
type ActorType string

const (
	ActorTypeUser            ActorType = "User"
	ActorTypeSystemPrincipal ActorType = "SystemPrincipal"
	ActorTypeClient          ActorType = "Client"
)

// LogEvent is an Okta System Log entry.
type LogEvent struct {
	UUID        string
	Published   time.Time
	Severity    Severity
	EventType   string
	DisplayMsg  string
	Actor       Actor
	Targets     []Target
	Client      Client
	Outcome     Outcome
	Request     json.RawMessage
	Debug       json.RawMessage
	Transaction json.RawMessage
	// Raw preserves the full event JSON for the detail view.
	Raw json.RawMessage
}

// LogPage is a single paginated batch of log events. After is the
// cursor (Okta's `after` query param) the caller feeds back into the
// next LogsQuery to retrieve older events; empty when the page
// covered everything within the requested window. Issue #F3 v0.2.5
// — replaces the auto-paginating iterator for History fetches so the
// operator drives "load more" explicitly.
type LogPage struct {
	Events []LogEvent
	After  string
}

// Actor of an event (REQ-R05 AC-1).
type Actor struct {
	ID            string
	Type          ActorType
	DisplayName   string
	AlternateID   string // typically login/email; masking governed by config (TUI_DESIGN §7.3)
}

// Target of an event. Multi-valued (e.g., user + app).
type Target struct {
	ID          string
	Type        string
	DisplayName string
	AlternateID string
}

// Client holds request origin metadata.
type Client struct {
	IPAddress string
	UserAgent string
	Geo       Geo
}

// Geo is a simplified geographic context (see `client.geographicalContext`).
type Geo struct {
	Country string
	State   string
	City    string
}

// Outcome is the event outcome.
type Outcome struct {
	Result OutcomeResult
	Reason string
}
