package errormap

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// apiError mirrors Okta's error response body shape (PRD §7.7).
type apiError struct {
	ErrorCode    string  `json:"errorCode"`
	ErrorSummary string  `json:"errorSummary"`
	ErrorLink    string  `json:"errorLink"`
	ErrorID      string  `json:"errorId"`
	ErrorCauses  []cause `json:"errorCauses"`
}

type cause struct {
	ErrorSummary string `json:"errorSummary"`
}

// FromResponse classifies an Okta HTTP error response. For 2xx returns nil.
// For errors, returns a domain-layer error selected by the Okta errorCode
// per PRD §7.7:
//
//	E0000001 → *domain.BadRequestError (unwraps to ErrBadRequest)
//	E0000004 → ErrTokenInvalid
//	E0000006 → ErrForbidden
//	E0000007 → ErrNotFound
//	E0000011 → ErrTokenInvalid
//	E0000022 → ErrBadRequest ("Deactivate before deleting" — informational)
//	E0000038 → ErrFeatureDisabled
//	E0000047 → *domain.RateLimitedError (unwraps to ErrRateLimited)
//	5xx      → ErrOktaServer
//
// The response body is consumed. On parse failure, a sentinel appropriate to
// the HTTP status is returned.
func FromResponse(resp *http.Response) error {
	if resp == nil {
		return domain.ErrNetwork
	}
	if resp.StatusCode < 400 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var ae apiError
	if len(body) > 0 {
		_ = json.Unmarshal(body, &ae)
	}

	summary := ae.ErrorSummary
	if summary == "" {
		summary = strings.TrimSpace(http.StatusText(resp.StatusCode))
	}

	// 429 → RateLimitedError regardless of errorCode (some tenants omit the code).
	if resp.StatusCode == http.StatusTooManyRequests || ae.ErrorCode == "E0000047" {
		rle := &domain.RateLimitedError{
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			// Category is filled by the adapter via ratelimit.CategoryFromPath.
		}
		return wrap(rle, summary)
	}

	switch ae.ErrorCode {
	case "E0000001":
		causes := make([]domain.FieldError, 0, len(ae.ErrorCauses))
		for _, c := range ae.ErrorCauses {
			field, _ := splitCause(c.ErrorSummary)
			causes = append(causes, domain.FieldError{
				Field:   field,
				Summary: c.ErrorSummary, // full original (contains field label callers match on)
			})
		}
		return wrap(&domain.BadRequestError{Causes: causes, Raw: summary}, summary)
	case "E0000022":
		return wrap(domain.ErrBadRequest, summary)
	case "E0000004", "E0000011":
		return wrap(domain.ErrTokenInvalid, summary)
	case "E0000006":
		return wrap(domain.ErrForbidden, summary)
	case "E0000007":
		return wrap(domain.ErrNotFound, summary)
	case "E0000038":
		return wrap(domain.ErrFeatureDisabled, summary)
	}

	if resp.StatusCode >= 500 {
		return wrap(domain.ErrOktaServer, summary)
	}

	// Fall-through: classify by status code when errorCode is unknown.
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return wrap(domain.ErrTokenInvalid, summary)
	case http.StatusForbidden:
		return wrap(domain.ErrForbidden, summary)
	case http.StatusNotFound:
		return wrap(domain.ErrNotFound, summary)
	case http.StatusBadRequest:
		return wrap(domain.ErrBadRequest, summary)
	}
	return wrap(domain.ErrOktaServer, summary)
}

// wrap annotates err with summary for human-readable Error() while preserving
// errors.Is / errors.As semantics.
func wrap(err error, summary string) error {
	if summary == "" {
		return err
	}
	return fmt.Errorf("%s: %w", summary, err)
}

// splitCause parses Okta's typical cause "field: reason" shape. When no ":"
// is present, returns ("", whole).
func splitCause(s string) (field, summary string) {
	i := strings.Index(s, ":")
	if i < 0 {
		return "", s
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
}

// parseRetryAfter reads an HTTP Retry-After header value (delta-seconds or
// HTTP-date). Returns 0 when empty or unparseable.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d
	}
	return 0
}
