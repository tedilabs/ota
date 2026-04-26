package service_test

// REQ-R04 — PoliciesService 계약 테스트. 타입 필수 · priority 정렬 · rich/raw 분기.

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

// REQ-R04 AC-2 — type 지정 없이 List 호출 시 명시적 에러를 반환해야 한다
// (Okta API가 type 파라미터 필수이므로 빈 요청은 차단).
func Test_PoliciesService_List_RequiresType(t *testing.T) {
	t.Parallel()
	port := fakes.NewPoliciesPort(t)
	// Port.ListFunc가 설정되지 않았어도 service가 먼저 거절해야 하므로 호출이 없을 것.
	svc := service.NewPoliciesService(port)

	_, err := svc.List(context.Background(), service.PoliciesQuery{})
	require.Error(t, err, "type 미지정 시 에러 반환해야 한다 (REQ-R04 AC-2)")
	assert.True(t, errors.Is(err, service.ErrPolicyTypeRequired),
		"명시적 센티넬 service.ErrPolicyTypeRequired 기대")
}

// REQ-R04 AC-3 — 리스트는 priority 오름차순으로 반환되어야 한다.
func Test_PoliciesService_List_OrdersByPriorityAscending(t *testing.T) {
	t.Parallel()
	port := fakes.NewPoliciesPort(t)
	port.ListFunc = func(_ context.Context, _ domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
		return &fakes.SliceIterator[domain.Policy]{
			Items: []domain.Policy{
				{ID: "00p_c", Priority: 5, Type: domain.PolicyTypeOktaSignOn},
				{ID: "00p_a", Priority: 1, Type: domain.PolicyTypeOktaSignOn},
				{ID: "00p_b", Priority: 3, Type: domain.PolicyTypeOktaSignOn},
			},
		}, nil
	}

	svc := service.NewPoliciesService(port)
	policies, err := svc.ListAll(context.Background(),
		service.PoliciesQuery{Type: domain.PolicyTypeOktaSignOn})
	require.NoError(t, err)
	require.Len(t, policies, 3)
	assert.Equal(t, "00p_a", policies[0].ID, "priority=1이 먼저")
	assert.Equal(t, "00p_b", policies[1].ID, "priority=3이 두번째")
	assert.Equal(t, "00p_c", policies[2].ID, "priority=5가 마지막")
}
