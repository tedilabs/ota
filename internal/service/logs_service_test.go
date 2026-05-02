package service_test

// REQ-R05 — LogsService tail 동작. Since 재설정, 기본 7초 간격, adaptive polling.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
)

// REQ-R05 AC-2 — 초기 Since는 now - 5m.
func Test_LogsService_TailSeed_InitialSinceIsFiveMinutesBehindNow(t *testing.T) {
	t.Parallel()
	fixedNow := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	want := fixedNow.Add(-5 * time.Minute)

	tail := service.NewLogsTail(fakes.NewLogsPort(t),
		service.WithLogsTailNow(func() time.Time { return fixedNow }),
	)

	q := tail.InitialQuery()
	require.NotNil(t, q.Since, "Since는 지정되어야 한다")
	assert.True(t, q.Since.Equal(want),
		"Since는 now-5m이어야 한다: want %s got %s", want, q.Since)
	assert.Equal(t, domain.SortAscending, q.SortOrder, "tail 모드는 ASCENDING")
	assert.Equal(t, 1000, q.Limit, "limit=1000 (도메인 §2.1)")
}

// REQ-R05 AC-2 — 폴링 후 Since는 마지막 published + 1ms로 갱신되어야 한다.
func Test_LogsService_TailAdvanceSince_UsesLastPublishedPlusOneMillisecond(t *testing.T) {
	t.Parallel()
	lastSeen := time.Date(2026, 4, 24, 12, 30, 30, 300_000_000, time.UTC)
	want := lastSeen.Add(time.Millisecond)

	tail := service.NewLogsTail(fakes.NewLogsPort(t))
	next := tail.NextSinceAfter(lastSeen)
	assert.True(t, next.Equal(want),
		"다음 Since는 lastPublished+1ms여야 중복 없이 이어진다 (REQ-R05 AC-2): want %s got %s",
		want, next)
}

// REQ-R05 AC-2 — Adaptive: X-Rate-Limit-Limit이 60 미만이면 폴 간격 15초로 상향.
func Test_LogsService_TailInterval_AdaptiveUpgradesOnLowRateLimit(t *testing.T) {
	t.Parallel()
	tail := service.NewLogsTail(fakes.NewLogsPort(t))

	// 기본은 7s
	assert.Equal(t, 7*time.Second, tail.PollInterval(),
		"기본 poll interval은 7초 (REQ-R05 AC-2)")

	// 첫 응답에서 limit=50 관찰
	tail.ObserveRateLimit(50)
	assert.Equal(t, 15*time.Second, tail.PollInterval(),
		"limit<60이면 15초로 상향 (REQ-R05 AC-2)")
}

// REQ-R05 AC-3 — 429 관찰 시 tail은 일시정지 상태가 되어야 한다.
func Test_LogsService_TailPause_On429(t *testing.T) {
	t.Parallel()
	tail := service.NewLogsTail(fakes.NewLogsPort(t))

	assert.False(t, tail.Paused(), "시작 시 paused=false")
	tail.Pause(2 * time.Second)
	assert.True(t, tail.Paused(), "Pause 호출 후 paused=true (REQ-R05 AC-3)")
}

// REQ-R05 AC-3 — 복구 후 Since가 리셋되지 않고 마지막 관찰 시점을 보존해야 한다.
func Test_LogsService_TailPauseResume_PreservesSince(t *testing.T) {
	t.Parallel()
	tail := service.NewLogsTail(fakes.NewLogsPort(t))

	lastSeen := time.Date(2026, 4, 24, 12, 30, 30, 300_000_000, time.UTC)
	expected := tail.NextSinceAfter(lastSeen)

	tail.Pause(1 * time.Second)
	tail.Resume()

	got := tail.NextSinceAfter(lastSeen)
	assert.True(t, got.Equal(expected),
		"Pause/Resume 사이에 Since 계산이 변하지 않아야 한다 (REQ-R05 AC-3)")
}

// REQ-R05 AC-4 — bounds the default fetch to the last 30 minutes.
// #F3 v0.2.5 flipped sortOrder to DESCENDING so when the result set
// exceeds LimitPerFetch the API returns the newest events; the TUI
// re-sorts ASCENDING for display so terminal-tail layout is preserved.
func Test_LogsService_HistoryQuery_DefaultIs30mDescending(t *testing.T) {
	t.Parallel()
	svc := service.NewLogsService(fakes.NewLogsPort(t))
	q := svc.HistoryQuery()
	assert.Equal(t, domain.SortDescending, q.SortOrder,
		"히스토리 모드는 DESCENDING (#F3 v0.2.5)")
	if assert.NotNil(t, q.Since, "30m default must populate Since") {
		assert.LessOrEqual(t, time.Since(*q.Since), 31*time.Minute,
			"기본 Since는 최근 30분 이내")
	}
}

// REQ-R05 AC-5 — 프리셋 5종이 전부 등록되어야 한다.
func Test_LogsService_Presets_AllFiveRegistered(t *testing.T) {
	t.Parallel()
	presets := service.LogsPresets()
	wantNames := []string{
		"Failed Sign-ins 24h",
		"Group Rule Changes",
		"Group Rule Deactivations (may remove memberships)",
		"API Token Activity",
		"MFA Challenges",
	}
	got := map[string]bool{}
	for _, p := range presets {
		got[p.Name] = true
	}
	for _, n := range wantNames {
		assert.True(t, got[n], "프리셋 누락: %q (REQ-R05 AC-5)", n)
	}
}
