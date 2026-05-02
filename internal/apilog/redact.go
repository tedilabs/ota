package apilog

import (
	"encoding/json"
	"net/http"
	"strings"
)

// sensitiveHeaders are stripped from captured headers and replaced
// with `***` before write. Lower-cased for case-insensitive match.
var sensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
	"x-api-key":           {},
	"x-auth-token":        {},
	"x-okta-api-token":    {},
}

// piiKeys are JSON keys whose values should be masked when scrubbing
// captured request / response bodies. Match is case-sensitive
// against the JSON object key — operators looking at debug data
// don't need to see the actual phone number / address / DOB.
var piiKeys = map[string]struct{}{
	"mobilePhone":      {},
	"primaryPhone":     {},
	"secondEmail":      {},
	"streetAddress":    {},
	"streetAddress2":   {},
	"streetAddress3":   {},
	"city":             {},
	"state":            {},
	"zipCode":          {},
	"postalCode":       {},
	"postalAddress":    {},
	"countryCode":      {},
	"countryName":      {},
	"address":          {},
	"firstName":        {},
	"lastName":         {},
	"middleName":       {},
	"honorificPrefix":  {},
	"honorificSuffix":  {},
	"birthdate":        {},
	"birthDate":        {},
	"dateOfBirth":      {},
	"ssn":              {},
	"taxId":            {},
	"nationalId":       {},
	"passportNumber":   {},
	"driversLicense":   {},
	// Free-text profile fields that often carry PII spillover.
	"manager":          {},
	"managerId":        {},
	"organization":     {},
	"department":       {},
	"division":         {},
	"costCenter":       {},
	"employeeNumber":   {},
	"profileUrl":       {},
	// Auth secrets — distinct from PII but equally sensitive.
	"password":         {},
	"clientSecret":     {},
	"client_secret":    {},
	"refresh_token":    {},
	"refreshToken":     {},
	"access_token":     {},
	"accessToken":      {},
	"id_token":         {},
	"idToken":          {},
	"sessionToken":     {},
	"recoveryToken":    {},
	"oneTimePassword":  {},
	"otp":              {},
}

// RedactedToken is the placeholder substituted for redacted values.
const RedactedToken = "***"

// RedactHeaders returns a defensive copy of h with sensitive header
// values replaced by `***`. Original headers are never mutated.
func RedactHeaders(h http.Header) http.Header {
	if len(h) == 0 {
		return nil
	}
	out := make(http.Header, len(h))
	for k, vs := range h {
		if _, hit := sensitiveHeaders[strings.ToLower(k)]; hit {
			masked := make([]string, len(vs))
			for i := range vs {
				masked[i] = RedactedToken
			}
			out[k] = masked
			continue
		}
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

// RedactJSONBody masks PII / secret keys inside body and returns the
// re-marshaled JSON. Falls back to the original (capped) input when
// the body isn't valid JSON — the cap-and-string behavior is
// applied by the caller.
func RedactJSONBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return body
	}
	scrubbed := scrubValue(v)
	out, err := json.Marshal(scrubbed)
	if err != nil {
		return body
	}
	return out
}

// scrubValue walks a decoded JSON value, replacing any value whose
// JSON key matches piiKeys with the redaction token. Nested objects
// and arrays are scrubbed recursively.
func scrubValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, child := range t {
			if _, hit := piiKeys[k]; hit {
				out[k] = RedactedToken
				continue
			}
			out[k] = scrubValue(child)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			out[i] = scrubValue(child)
		}
		return out
	default:
		return v
	}
}

// CapBody truncates body to MaxBodyBytes when it would otherwise
// blow the cache file up. Truncated bodies get a `…[truncated]`
// suffix so the operator can tell.
func CapBody(body []byte) []byte {
	if len(body) <= MaxBodyBytes {
		return body
	}
	out := make([]byte, 0, MaxBodyBytes+16)
	out = append(out, body[:MaxBodyBytes]...)
	out = append(out, "…[truncated]"...)
	return out
}
