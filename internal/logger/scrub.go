package logger

import "regexp"

// tokenPatterns recognises secrets that may appear in panic stack traces or
// stringified errors. They are replaced by `[REDACTED]` before any output is
// written. See REQ-C05 AC-2/AC-3.
var tokenPatterns = []*regexp.Regexp{
	// SSWS token (Okta API key): "SSWS <key>"; key is opaque hex/base64.
	regexp.MustCompile(`SSWS\s+[A-Za-z0-9._\-+=/]+`),
	// "Authorization: ..." header line as it would appear in a wrapped error.
	regexp.MustCompile(`(?i)Authorization:\s*[^\s\\"]+`),
	// `api_token=<value>` query/form fragment.
	regexp.MustCompile(`(?i)api[_-]?token\s*[:=]\s*[A-Za-z0-9._\-+=/]+`),
	// Raw bearer-style tokens.
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-+=/]+`),
}

// ScrubText replaces all token patterns in s with `[REDACTED]`. Safe for
// arbitrary input (panic.Stack, error messages, etc.).
func ScrubText(s string) string {
	for _, re := range tokenPatterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
