package okta_test

// REQ-R01 AC-4, PRD §7.3 — Users adapter integration 테스트.
// testfx.FakeOktaServer + scenarios/pagination_multi_page.json 로 Link 헤더 순회
// 검증.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta"
	"github.com/tedilabs/ota/internal/okta/testfx"
)

// REQ-R01 AC-4 + PRD §7.3 — Link 헤더가 유효할 때 순차 페이지 fetch.
func Test_UsersAdapter_List_IteratesAllPagesViaLinkHeader(t *testing.T) {
	t.Parallel()

	srv := testfx.NewFakeOktaServer(t, "pagination_multi_page")
	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL:     srv.URL,
		APIToken:   "ssws-test-token",
		HTTPClient: srv.Client(),
	}, okta.WithClock(clock.Real()))
	require.NoError(t, err)

	iter, err := cli.Users().List(context.Background(), domain.UsersQuery{Limit: 200})
	require.NoError(t, err)
	defer iter.Close()

	var ids []string
	for {
		u, hasMore, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !hasMore {
			break
		}
		ids = append(ids, u.ID)
	}
	assert.Len(t, ids, 6, "page1(3) + page2(3) + empty page = 6 users 반환")
	// 순서도 보존되어야 한다 (fetch 순차 보장, PRD §7.3 병렬 금지).
	assert.Equal(t, "00u_active_alice", ids[0])
}

// REQ-E01 AC-2 — 429 → Retry-After 후 재시도 성공.
func Test_UsersAdapter_List_RetriesOn429AndRecovers(t *testing.T) {
	t.Parallel()

	srv := testfx.NewFakeOktaServer(t, "rate_limit_429_recovery")
	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL:     srv.URL,
		APIToken:   "ssws-test-token",
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	iter, err := cli.Users().List(context.Background(), domain.UsersQuery{Limit: 200})
	require.NoError(t, err)
	defer iter.Close()

	// 두 번째 시도에서 성공. 429 자동 재시도 (REQ-E01 AC-2).
	u, hasMore, err := iter.Next(context.Background())
	require.NoError(t, err)
	require.True(t, hasMore)
	assert.Equal(t, "00u_active_alice", u.ID,
		"429 후 재시도로 실제 데이터를 받아야 한다")
	// 두 단계 (429 + 200) 서빙 확인.
	assert.Equal(t, 2, srv.StepsServed())
}

// REQ-C04 AC-4 — 404 응답은 domain.ErrNotFound로 매핑되어야 한다.
func Test_UsersAdapter_Get_NotFoundReturnsSentinel(t *testing.T) {
	t.Parallel()
	// 인라인 시나리오 작성 대신, fixture로 404 시나리오 하나 더 만들 수도 있으나
	// 여기서는 users_eventually_consistent 시나리오의 첫 단계를 재사용:
	// Get 성공 → but 이 테스트는 not_found 의미가 없음. 대신 별도 체크.
	// Phase 5 Red 단계에서는 이 테스트가 구현 부재로 실패하므로 단순 호출만으로도 panic.
	t.Skip("404 전용 시나리오 fixture 추가 후 복원 (Phase 6)")
}
