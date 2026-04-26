package service_test

// REQ-R02 — GroupsService: 타입 필터, 멤버 iterator, RULE 배지 판별.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
)

// REQ-R02 AC-2 — :filter type eq "OKTA_GROUP" 같은 표현이 GroupsQuery.Filter로 전달되어야 한다.
func Test_GroupsService_Search_FilterIsPassedThrough(t *testing.T) {
	t.Parallel()
	filter := `type eq "OKTA_GROUP"`
	var observed domain.GroupsQuery
	port := fakes.NewGroupsPort(t)
	port.ListFunc = func(_ context.Context, q domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
		observed = q
		return &fakes.SliceIterator[domain.Group]{}, nil
	}

	svc := service.NewGroupsService(port, fakes.NewGroupRulesPort(t))
	_, err := svc.Search(context.Background(), service.GroupsQuery{Filter: filter})
	require.NoError(t, err)
	assert.Equal(t, filter, observed.Filter)
}

// REQ-R02 AC-1 — Group rule이 타겟팅하는 그룹은 DynamicTargeted=true로 마킹된다.
func Test_GroupsService_Search_FlagsDynamicTargetedGroupsViaRules(t *testing.T) {
	t.Parallel()
	// Rules → 00g_engineering 타겟.
	rules := fakes.NewGroupRulesPort(t)
	rules.ListFunc = func(_ context.Context, _ domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
		return &fakes.SliceIterator[domain.GroupRule]{
			Items: []domain.GroupRule{
				{ID: "0pr_active", Status: domain.GroupRuleStatusActive, TargetGroupIDs: []string{"00g_engineering"}},
			},
		}, nil
	}

	groups := fakes.NewGroupsPort(t)
	groups.ListFunc = func(_ context.Context, _ domain.GroupsQuery) (domain.Iterator[domain.Group], error) {
		return &fakes.SliceIterator[domain.Group]{
			Items: []domain.Group{
				{ID: "00g_engineering", Type: domain.GroupTypeOkta, Profile: domain.GroupProfile{Name: "Engineering"}},
				{ID: "00g_sales", Type: domain.GroupTypeOkta, Profile: domain.GroupProfile{Name: "Sales"}},
			},
		}, nil
	}

	svc := service.NewGroupsService(groups, rules)
	iter, err := svc.Search(context.Background(), service.GroupsQuery{})
	require.NoError(t, err)
	defer iter.Close()

	got := map[string]bool{}
	for {
		g, hasMore, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !hasMore {
			break
		}
		got[g.ID] = g.DynamicTargeted
	}
	assert.True(t, got["00g_engineering"], "engineering은 rule 타겟이므로 DynamicTargeted=true (REQ-R02 AC-1)")
	assert.False(t, got["00g_sales"], "sales는 rule 타겟 아님 → DynamicTargeted=false")
}
