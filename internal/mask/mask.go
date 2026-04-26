package mask

import (
	"strings"
	"unicode"
)

// Phone returns the masked representation of a phone number, preserving the
// leading country code prefix (up to the first separator group) and the
// trailing digit group. Each interior digit group is replaced with asterisks
// of the same length.
//
// E.g., "+1-555-123-4567" → "+1-***-***-4567", "+82-10-1234-5678" → "+82-**-****-5678".
//
// Inputs that don't match the "+<cc>-<group>-...-<last>" shape (missing
// separators, not enough digit groups, non-digit groups) are returned
// unchanged to avoid producing misleading partial masks.
func Phone(v string) string {
	if v == "" || !strings.HasPrefix(v, "+") {
		return v
	}
	groups := strings.Split(v, "-")
	// Require: country code + at least one middle group + trailing group.
	if len(groups) < 3 {
		return v
	}
	// All groups beyond the country code must be non-empty digit-only runs.
	for _, g := range groups[1:] {
		if g == "" {
			return v
		}
		for _, r := range g {
			if !unicode.IsDigit(r) {
				return v
			}
		}
	}
	out := make([]string, len(groups))
	out[0] = groups[0]
	last := len(groups) - 1
	for i := 1; i < last; i++ {
		out[i] = strings.Repeat("*", len(groups[i]))
	}
	out[last] = groups[last]
	return strings.Join(out, "-")
}

// Email returns the masked representation of an email: the local part's first
// character is preserved, the rest of the local part is replaced with "***",
// and the unchanged "@domain" follows. E.g., "alice@acme.com" → "a***@acme.com".
//
// Inputs without "@" after position 0 or without a domain after "@" are
// returned unchanged.
func Email(v string) string {
	at := strings.IndexByte(v, '@')
	if at <= 0 || at == len(v)-1 {
		return v
	}
	return v[:1] + "***" + v[at:]
}
