package service_test

// REQ-R01 / REQ-U04 — UsersService 유스케이스 계약.
//
// 이 테스트들은 Phase 5 Red 단계에서는 UsersService 구현체가 없어
// 컴파일 실패로 Red. Phase 6에서 service.UsersService가 생기면 통과.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
)

// REQ-U04 AC-1 — `/` 키는 항상 `q` 자유 텍스트. Service.Search 계층에서 q 파라미터를
// UsersQuery.Q로 그대로 전달.
func Test_UsersService_Search_QIsPassedThroughAsFreeText(t *testing.T) {
	t.Parallel()

	var observed domain.UsersQuery
	port := fakes.NewUsersPort(t)
	port.ListFunc = func(_ context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
		observed = q
		return &fakes.SliceIterator[domain.User]{}, nil
	}

	svc := service.NewUsersService(port)
	_, err := svc.Search(context.Background(), service.UsersQuery{Q: "alice"})
	require.NoError(t, err)
	assert.Equal(t, "alice", observed.Q, "service가 Q를 그대로 Port에 전달해야 한다 (REQ-U04 AC-1)")
	assert.Empty(t, observed.Search, "Q 모드일 때 Search 파라미터는 비어야 한다")
}

// REQ-U04 AC-2 — `:search` 커맨드는 SCIM-like search 표현을 그대로 Port에 전달.
func Test_UsersService_Search_SearchExpressionIsForwarded(t *testing.T) {
	t.Parallel()
	expr := `profile.department eq "Engineering" and status eq "ACTIVE"`

	var observed domain.UsersQuery
	port := fakes.NewUsersPort(t)
	port.ListFunc = func(_ context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
		observed = q
		return &fakes.SliceIterator[domain.User]{}, nil
	}

	svc := service.NewUsersService(port)
	_, err := svc.Search(context.Background(), service.UsersQuery{Search: expr})
	require.NoError(t, err)
	assert.Equal(t, expr, observed.Search)
}

// REQ-C04 AC-4 — 어댑터가 ErrForbidden을 반환하면 service는 그것을 보존해야 한다
// (상위 TUI가 도메인 센티넬로 분기하기 위함).
func Test_UsersService_Get_PreservesForbiddenSentinel(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return domain.User{}, domain.ErrForbidden
	}
	svc := service.NewUsersService(port)

	_, err := svc.Get(context.Background(), "00u_active_alice")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden),
		"service는 adapter의 ErrForbidden 센티넬을 소실 없이 전파해야 한다 (REQ-C04 AC-4)")
}

// REQ-E01 AC-6 — 30초 TTL 캐시. 동일 쿼리 두 번이면 Port는 한 번만 호출되어야 한다.
func Test_UsersService_Search_CachesResultsByQueryKey(t *testing.T) {
	t.Parallel()
	var calls int
	port := fakes.NewUsersPort(t)
	port.ListFunc = func(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
		calls++
		return &fakes.SliceIterator[domain.User]{
			Items: []domain.User{{ID: "00u_active_alice"}},
		}, nil
	}

	svc := service.NewUsersService(port)
	q := service.UsersQuery{Q: "alice"}
	_, err := svc.Search(context.Background(), q)
	require.NoError(t, err)
	_, err = svc.Search(context.Background(), q)
	require.NoError(t, err)

	assert.Equal(t, 1, calls, "동일 쿼리 2회 호출 중 캐시 hit 1회 예상 (REQ-E01 AC-6)")
}

// REQ-R01 AC-6 — User factors 조회는 Port.ListFactors를 호출해 결과를 그대로 반환.
func Test_UsersService_Factors_DelegatesToPort(t *testing.T) {
	t.Parallel()
	want := []domain.Factor{
		{ID: "opf_push_001", Type: domain.FactorTypePush, Status: domain.FactorStatusActive},
		{ID: "opf_sms_001", Type: domain.FactorTypeSMS, Status: domain.FactorStatusActive},
	}
	port := fakes.NewUsersPort(t)
	port.ListFactorsFunc = func(_ context.Context, userID string) ([]domain.Factor, error) {
		assert.Equal(t, "00u_active_alice", userID)
		return want, nil
	}

	svc := service.NewUsersService(port)
	got, err := svc.Factors(context.Background(), "00u_active_alice")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
