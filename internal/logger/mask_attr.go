package logger

import "log/slog"

// SensitiveKeys is the set of slog attribute keys whose values are replaced
// with "***" before write. Kept as package-level so tests can extend.
var SensitiveKeys = map[string]struct{}{
	"authorization":  {},
	"api_token":      {},
	"token":          {},
	"mobile_phone":   {},
	"second_email":   {},
	"phone_number":   {},
}

// MaskAttr is a slog.HandlerOptions.ReplaceAttr function that scrubs values
// whose keys match SensitiveKeys. See docs/CONVENTIONS.md §6.2.
func MaskAttr(_ []string, a slog.Attr) slog.Attr {
	if _, ok := SensitiveKeys[a.Key]; ok {
		return slog.String(a.Key, "***")
	}
	return a
}
