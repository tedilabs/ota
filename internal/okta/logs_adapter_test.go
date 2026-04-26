package okta_test

// REQ-R05 AC-3 / REQ-E01 AC-3 — Logs adapter hole-free tail integration.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta"
	"github.com/tedilabs/ota/internal/okta/testfx"
)

// REQ-R05 AC-3 — Tail 도중 429가 발생하면 같은 since로 재시도해야 하며
// 이벤트 누락이 없어야 한다.
func Test_LogsAdapter_Tail_HoleFreeResumeAfterRateLimit(t *testing.T) {
	t.Parallel()

	srv := testfx.NewFakeOktaServer(t, "logs_tail_hole_free")
	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL:     srv.URL,
		APIToken:   "ssws-test-token",
		HTTPClient: srv.Client(),
	}, okta.WithClock(clock.NewFake(time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC))))
	require.NoError(t, err)

	// 1차 호출: 초기 이벤트 3개.
	since := time.Date(2026, 4, 24, 12, 25, 0, 0, time.UTC)
	q := domain.LogsQuery{Since: &since, SortOrder: domain.SortAscending, Limit: 1000}

	iter, err := cli.Logs().Search(context.Background(), q)
	require.NoError(t, err)
	var firstBatch []domain.LogEvent
	for {
		e, hasMore, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !hasMore {
			break
		}
		firstBatch = append(firstBatch, e)
	}
	iter.Close()
	require.Len(t, firstBatch, 3, "tail_initial fixture의 이벤트 3개")

	// 2차 호출: 마지막 published + 1ms 이후. 시나리오상 429 → retry success.
	lastSeen := firstBatch[len(firstBatch)-1].Published
	next := lastSeen.Add(time.Millisecond)
	q2 := domain.LogsQuery{Since: &next, SortOrder: domain.SortAscending, Limit: 1000}

	iter2, err := cli.Logs().Search(context.Background(), q2)
	require.NoError(t, err, "429 후 재시도 성공해야 한다 (REQ-R05 AC-3)")
	var secondBatch []domain.LogEvent
	for {
		e, hasMore, err := iter2.Next(context.Background())
		require.NoError(t, err)
		if !hasMore {
			break
		}
		secondBatch = append(secondBatch, e)
	}
	iter2.Close()
	assert.Len(t, secondBatch, 1, "tail_poll_next fixture의 이벤트 1개 (REQ-R05 AC-3 hole-free)")
}
