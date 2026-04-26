package domain

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors returned by adapters after Okta errorCode mapping
// (PRD §7.7 / ARCHITECTURE §9.2). Callers match with errors.Is.
var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrRateLimited     = errors.New("rate limited")
	ErrTokenInvalid    = errors.New("token invalid or expired")
	ErrBadRequest      = errors.New("bad request")
	ErrOktaServer      = errors.New("okta server error")
	ErrFeatureDisabled = errors.New("feature disabled")
	ErrNetwork         = errors.New("network error")
)

// RateLimitedError wraps ErrRateLimited with the Retry-After hint so callers
// can schedule the resume tick (REQ-E01 AC-2). Use errors.As to unwrap.
type RateLimitedError struct {
	RetryAfter time.Duration
	Category   string // "management" / "logs" / "policies" / ...
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("rate limited (retry after %s, category=%s)", e.RetryAfter, e.Category)
}

func (e *RateLimitedError) Unwrap() error { return ErrRateLimited }

// BadRequestError wraps ErrBadRequest with Okta errorCauses for field-level
// display (REQ-U04 AC-3 / PRD §7.7 E0000001).
type BadRequestError struct {
	Causes []FieldError
	Raw    string
}

// FieldError is a single errorCauses entry.
type FieldError struct {
	Field   string
	Summary string
}

func (e *BadRequestError) Error() string {
	if len(e.Causes) == 0 {
		return "bad request"
	}
	return fmt.Sprintf("bad request: %d cause(s)", len(e.Causes))
}

func (e *BadRequestError) Unwrap() error { return ErrBadRequest }
