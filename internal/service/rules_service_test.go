package service_test

// REQ-R03 — GroupRulesService: id→name 해소, INVALID 경고 플래그.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
)

// REQ-R03 AC-4 — 타겟 그룹 id가 name으로 해소되어야 한다.
func Test_RulesService_List_ResolvesTargetGroupNames(t *testing.T) {
	t.Parallel()
	rules := fakes.NewGroupRulesPort(t)
	rules.ListFunc = func(_ context.Context, _ domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
		return &fakes.SliceIterator[domain.GroupRule]{
			Items: []domain.GroupRule{
				{ID: "0pr_active", Status: domain.GroupRuleStatusActive,
					TargetGroupIDs: []string{"00g_engineering"}},
			},
		}, nil
	}
	groups := fakes.NewGroupsPort(t)
	groups.GetFunc = func(_ context.Context, id string) (domain.Group, error) {
		assert.Equal(t, "00g_engineering", id)
		return domain.Group{ID: id, Profile: domain.GroupProfile{Name: "Engineering"}}, nil
	}

	svc := service.NewRulesService(rules, groups)
	items, err := svc.ListWithTargetNames(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, []string{"Engineering"}, items[0].TargetGroupNames,
		"id → name 해소되어야 한다 (REQ-R03 AC-4)")
}

// REQ-R03 AC-4 — 타겟 그룹 조회 실패 시 id 그대로 + "(name unavailable)" 스트링.
func Test_RulesService_List_FallsBackToIDWhenGroupLookupFails(t *testing.T) {
	t.Parallel()
	rules := fakes.NewGroupRulesPort(t)
	rules.ListFunc = func(_ context.Context, _ domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
		return &fakes.SliceIterator[domain.GroupRule]{
			Items: []domain.GroupRule{
				{ID: "0pr_ghost", TargetGroupIDs: []string{"00g_gone"}},
			},
		}, nil
	}
	groups := fakes.NewGroupsPort(t)
	groups.GetFunc = func(_ context.Context, _ string) (domain.Group, error) {
		return domain.Group{}, domain.ErrNotFound
	}

	svc := service.NewRulesService(rules, groups)
	items, err := svc.ListWithTargetNames(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].TargetGroupNames, 1)
	assert.Contains(t, items[0].TargetGroupNames[0], "00g_gone",
		"이름 해소 실패 시 id 노출 (REQ-R03 AC-4)")
	assert.Contains(t, items[0].TargetGroupNames[0], "unavailable")
}
