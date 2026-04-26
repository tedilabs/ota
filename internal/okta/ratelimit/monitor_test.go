package ratelimit_test

// REQ-E01 — Rate Limit 모니터.

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/okta/ratelimit"
)

// REQ-E01 AC-4 — 카테고리별 last-observed 저장.
func Test_Monitor_Observe_StoresPerCategory(t *testing.T) {
	t.Parallel()
	fixedNow := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(fixedNow)
	m := ratelimit.NewMonitor(clk)

	// management 카테고리 관찰
	m.Observe(mustRespHeader("/api/v1/users", 598, 600, fixedNow.Add(time.Minute)))
	// logs 카테고리 관찰
	m.Observe(mustRespHeader("/api/v1/logs", 118, 120, fixedNow.Add(time.Minute)))

	snaps := m.Snapshots()
	byCategory := map[string]int{}
	for _, s := range snaps {
		byCategory[s.Category] = s.Remaining
	}
	assert.Equal(t, 598, byCategory["management"], "management 카테고리 remaining 보존")
	assert.Equal(t, 118, byCategory["logs"], "logs 카테고리 remaining 보존")
}

// REQ-E01 AC-5 — /logs 경로는 logs 카테고리.
func Test_CategoryFromPath_ClassifiesCorrectly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want string
	}{
		{"/api/v1/users", "management"},
		{"/api/v1/users/00u123", "management"},
		{"/api/v1/groups", "management"},
		{"/api/v1/groups/rules", "management"},
		{"/api/v1/logs", "logs"},
		{"/api/v1/policies", "policies"},
		{"/api/v1/apps", "apps"},
		{"/api/v1/unknown", "other"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := ratelimit.CategoryFromPath(tc.path)
			assert.Equal(t, tc.want, got, "path %s → %s", tc.path, tc.want)
		})
	}
}

// Observe는 category별 가장 최근 관찰을 유지한다.
func Test_Monitor_Observe_KeepsLastObservedPerCategory(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))
	m := ratelimit.NewMonitor(clk)

	m.Observe(mustRespHeader("/api/v1/users", 600, 600, time.Now()))
	m.Observe(mustRespHeader("/api/v1/users", 598, 600, time.Now()))
	m.Observe(mustRespHeader("/api/v1/users", 595, 600, time.Now()))

	snaps := m.Snapshots()
	var mgmt int
	for _, s := range snaps {
		if s.Category == "management" {
			mgmt = s.Remaining
		}
	}
	assert.Equal(t, 595, mgmt, "가장 최근 관찰이 유지되어야 한다")
}

func mustRespHeader(path string, remaining, limit int, reset time.Time) *http.Response {
	hdr := http.Header{}
	hdr.Set("X-Rate-Limit-Remaining", strconv.Itoa(remaining))
	hdr.Set("X-Rate-Limit-Limit", strconv.Itoa(limit))
	hdr.Set("X-Rate-Limit-Reset", strconv.FormatInt(reset.Unix(), 10))
	req, err := http.NewRequest(http.MethodGet, "https://dev-example.okta.com"+path, nil)
	if err != nil {
		panic(err)
	}
	return &http.Response{
		StatusCode: 200,
		Header:     hdr,
		Request:    req,
	}
}

