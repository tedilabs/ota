package errormap

import (
	"errors"

	"github.com/tedilabs/ota/internal/domain"
)

// UserMessage returns the operator-facing string for a domain error,
// per PRD §7.7 / REQ-C04 AC-4. Returns "" when err is nil. Generic errors
// (network, server) are mapped to their friendly equivalents; field-level
// causes from BadRequestError are summarized when present.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}

	// Typed errors first so we can surface extra context.
	var bre *domain.BadRequestError
	if errors.As(err, &bre) {
		if len(bre.Causes) > 0 {
			c := bre.Causes[0]
			if c.Field != "" {
				return "Validation failed: " + c.Field + " — " + c.Summary
			}
			return "Validation failed: " + c.Summary
		}
		return "Validation failed. Check the request and try again."
	}

	var rle *domain.RateLimitedError
	if errors.As(err, &rle) {
		return "Rate limit hit. ota will retry automatically — Ctrl-c to abort."
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "Resource not found. Refreshing list…"
	case errors.Is(err, domain.ErrForbidden):
		return "Insufficient permissions for this action (token may be Read-Only)."
	case errors.Is(err, domain.ErrTokenInvalid):
		return "API token invalid or expired. Rotate the token and restart ota."
	case errors.Is(err, domain.ErrFeatureDisabled):
		return "This feature is disabled for your organization. Contact your Okta administrator."
	case errors.Is(err, domain.ErrRateLimited):
		return "Rate limit hit. ota will retry automatically — Ctrl-c to abort."
	case errors.Is(err, domain.ErrBadRequest):
		return "Request rejected by Okta. See :errors for details."
	case errors.Is(err, domain.ErrOktaServer):
		return "Okta server error. Retrying…"
	case errors.Is(err, domain.ErrNetwork):
		return "Network error — ota is offline. Reconnecting…"
	}
	return "Unexpected error. See :errors for details."
}
