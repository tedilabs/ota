package service

import "time"

// LogsPreset is a named Logs filter (REQ-R05 AC-5).
type LogsPreset struct {
	Name string
	// Filter is the Okta `filter` expression for /api/v1/logs.
	Filter string
	// SinceOffset, if non-zero, sets `since` to now - offset. Zero means "no
	// default since" (use history defaults).
	SinceOffset time.Duration
}

// LogsPresets returns the 5 built-in presets (REQ-R05 AC-5).
func LogsPresets() []LogsPreset {
	return []LogsPreset{
		{
			Name:        "Failed Sign-ins 24h",
			Filter:      `eventType eq "user.session.start" and outcome.result eq "FAILURE"`,
			SinceOffset: 24 * time.Hour,
		},
		{
			Name:   "Group Rule Changes",
			Filter: `eventType sw "group.rule"`,
		},
		{
			Name:   "Group Rule Deactivations (may remove memberships)",
			Filter: `eventType eq "group.rule.deactivate"`,
		},
		{
			Name:   "API Token Activity",
			Filter: `eventType sw "system.api_token"`,
		},
		{
			Name:   "MFA Challenges",
			Filter: `eventType sw "user.authentication.auth_via_mfa"`,
		},
	}
}
