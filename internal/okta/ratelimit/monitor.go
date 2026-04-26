package ratelimit

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
)

// Monitor records last-observed rate-limit values per category.
// Safe for concurrent use. Consumed via domain.RateLimitPort.
type Monitor struct {
	mu        sync.RWMutex
	clock     clock.Clock
	snapshots map[string]domain.RateLimitSnapshot
}

// NewMonitor constructs a Monitor.
func NewMonitor(clk clock.Clock) *Monitor {
	return &Monitor{clock: clk, snapshots: map[string]domain.RateLimitSnapshot{}}
}

// Observe records the headers of an HTTP response. The category is derived
// from the request path (see CategoryFromPath). Responses without the
// X-Rate-Limit-Limit header are ignored.
func (m *Monitor) Observe(resp *http.Response) {
	if resp == nil || resp.Header == nil {
		return
	}
	limitStr := resp.Header.Get("X-Rate-Limit-Limit")
	if limitStr == "" {
		return
	}

	path := ""
	if resp.Request != nil && resp.Request.URL != nil {
		path = resp.Request.URL.Path
	}
	category := CategoryFromPath(path)

	snap := domain.RateLimitSnapshot{
		Category:  category,
		Remaining: atoi(resp.Header.Get("X-Rate-Limit-Remaining")),
		Limit:     atoi(limitStr),
		Reset:     parseReset(resp.Header.Get("X-Rate-Limit-Reset")),
		Observed:  m.clock.Now(),
	}

	m.mu.Lock()
	m.snapshots[category] = snap
	m.mu.Unlock()
}

// Snapshots returns a copy of the current category snapshots.
func (m *Monitor) Snapshots() []domain.RateLimitSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.RateLimitSnapshot, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		out = append(out, s)
	}
	return out
}

// CategoryFromPath classifies an Okta API path into a rate-limit bucket.
// Returns one of: "management", "logs", "policies", "apps", or "other".
func CategoryFromPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/logs"):
		return "logs"
	case strings.HasPrefix(path, "/api/v1/policies"):
		return "policies"
	case strings.HasPrefix(path, "/api/v1/apps"):
		return "apps"
	case strings.HasPrefix(path, "/api/v1/users"),
		strings.HasPrefix(path, "/api/v1/groups"):
		return "management"
	default:
		return "other"
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func parseReset(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC()
	}
	return time.Time{}
}

// Compile-time check: Monitor implements domain.RateLimitPort.
var _ domain.RateLimitPort = (*Monitor)(nil)
