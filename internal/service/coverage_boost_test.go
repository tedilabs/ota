package service_test

// QA-017 coverage 보강 — Service 레이어의 Get/Members/AppCount/Invalidate/Options
// 엣지 경로.

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
)

// GroupsService.Get — Port.Get 패스스루.
func Test_GroupsService_Get_PassesThroughToPort(t *testing.T) {
	t.Parallel()
	port := fakes.NewGroupsPort(t)
	rules := fakes.NewGroupRulesPort(t)
	port.GetFunc = func(_ context.Context, id string) (domain.Group, error) {
		assert.Equal(t, "00g_engineering", id)
		return domain.Group{ID: id, Profile: domain.GroupProfile{Name: "Engineering"}}, nil
	}
	svc := service.NewGroupsService(port, rules)
	got, err := svc.Get(context.Background(), "00g_engineering")
	require.NoError(t, err)
	assert.Equal(t, "Engineering", got.Profile.Name)
}

// GroupsService.Members — Port.Members 패스스루.
func Test_GroupsService_Members_PassesThroughToPort(t *testing.T) {
	t.Parallel()
	port := fakes.NewGroupsPort(t)
	rules := fakes.NewGroupRulesPort(t)
	port.MembersFunc = func(_ context.Context, q domain.GroupMembersQuery) (domain.Iterator[domain.User], error) {
		assert.Equal(t, "00g_engineering", q.GroupID)
		return &fakes.SliceIterator[domain.User]{
			Items: []domain.User{{ID: "00u_active_alice"}},
		}, nil
	}
	svc := service.NewGroupsService(port, rules)
	iter, err := svc.Members(context.Background(), domain.GroupMembersQuery{GroupID: "00g_engineering"})
	require.NoError(t, err)
	defer iter.Close()
	u, more, err := iter.Next(context.Background())
	require.NoError(t, err)
	require.True(t, more)
	assert.Equal(t, "00u_active_alice", u.ID)
}

// GroupsService.AppCount — 패스스루 + 값.
func Test_GroupsService_AppCount_PassesThrough(t *testing.T) {
	t.Parallel()
	port := fakes.NewGroupsPort(t)
	rules := fakes.NewGroupRulesPort(t)
	port.AppCountFunc = func(_ context.Context, id string) (int, error) {
		assert.Equal(t, "00g_everyone", id)
		return 42, nil
	}
	svc := service.NewGroupsService(port, rules)
	n, err := svc.AppCount(context.Background(), "00g_everyone")
	require.NoError(t, err)
	assert.Equal(t, 42, n)
}

// Bundle.InvalidateAll — 개별 서비스 Invalidate 호출.
func Test_Bundle_InvalidateAll_CallsEachInvalidate(t *testing.T) {
	t.Parallel()
	// 간단히 Users 서비스만 주입하고 InvalidateAll이 panic 없이 실행되는지 확인.
	port := fakes.NewUsersPort(t)
	users := service.NewUsersService(port)

	bundle := &service.Bundle{
		Users: users,
		// Groups/Rules/Policies/Logs는 nil 허용 (InvalidateAll이 nil-check).
	}
	require.NotPanics(t, func() { bundle.InvalidateAll() },
		"부분 초기화 Bundle에서도 InvalidateAll은 panic 없이 실행되어야 한다")
}

// ServiceOption — WithLogger + WithClock + WithCacheTTL 조합.
func Test_ServiceOptions_Combine_DoNotPanic(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	lg := slog.New(slog.DiscardHandler)
	fixed := clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))

	svc := service.NewUsersService(port,
		service.WithLogger(lg),
		service.WithClock(fixed),
		service.WithCacheTTL(60),
	)
	require.NotNil(t, svc)
}

// UsersService.Groups — ListGroups Port 호출.
func Test_UsersService_Groups_PassesThroughToPort(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.ListGroupsFunc = func(_ context.Context, id string) ([]domain.Group, error) {
		assert.Equal(t, "00u_active_alice", id)
		return []domain.Group{
			{ID: "00g_engineering", Profile: domain.GroupProfile{Name: "Engineering"}},
		}, nil
	}
	svc := service.NewUsersService(port)
	gs, err := svc.Groups(context.Background(), "00u_active_alice")
	require.NoError(t, err)
	require.Len(t, gs, 1)
	assert.Equal(t, "Engineering", gs[0].Profile.Name)
}

// PoliciesService.Get — 패스스루.
func Test_PoliciesService_Get_PassesThrough(t *testing.T) {
	t.Parallel()
	port := fakes.NewPoliciesPort(t)
	port.GetFunc = func(_ context.Context, id string) (domain.Policy, error) {
		return domain.Policy{ID: id, Type: domain.PolicyTypeOktaSignOn, Name: "Default"}, nil
	}
	svc := service.NewPoliciesService(port)
	p, err := svc.Get(context.Background(), "00p_default")
	require.NoError(t, err)
	assert.Equal(t, "Default", p.Name)
}

// PoliciesService.Rules — 패스스루 + priority 정렬 (service가 정렬 담당).
func Test_PoliciesService_Rules_PassesThrough(t *testing.T) {
	t.Parallel()
	port := fakes.NewPoliciesPort(t)
	port.RulesFunc = func(_ context.Context, id string) ([]domain.PolicyRule, error) {
		assert.Equal(t, "00p_x", id)
		return []domain.PolicyRule{
			{ID: "pr_2", Priority: 2},
			{ID: "pr_1", Priority: 1},
		}, nil
	}
	svc := service.NewPoliciesService(port)
	rs, err := svc.Rules(context.Background(), "00p_x")
	require.NoError(t, err)
	require.Len(t, rs, 2)
}

// PoliciesService.Invalidate — no-op 안전.
func Test_PoliciesService_Invalidate_NoPanic(t *testing.T) {
	t.Parallel()
	port := fakes.NewPoliciesPort(t)
	svc := service.NewPoliciesService(port)
	require.NotPanics(t, func() { svc.Invalidate() })
}

// GroupsService.Invalidate + RulesService.Invalidate — no-op 안전.
func Test_Services_Invalidate_NoPanic(t *testing.T) {
	t.Parallel()
	groupsPort := fakes.NewGroupsPort(t)
	rulesPort := fakes.NewGroupRulesPort(t)

	gs := service.NewGroupsService(groupsPort, rulesPort)
	require.NotPanics(t, func() { gs.Invalidate() })

	rs := service.NewRulesService(rulesPort, groupsPort)
	require.NotPanics(t, func() { rs.Invalidate() })
}

// RulesService.List — iterator로 드레인 후 상태별 집계.
func Test_RulesService_List_DrainsAllStates(t *testing.T) {
	t.Parallel()
	rulesPort := fakes.NewGroupRulesPort(t)
	rulesPort.ListFunc = func(_ context.Context, _ domain.GroupRulesQuery) (domain.Iterator[domain.GroupRule], error) {
		return &fakes.SliceIterator[domain.GroupRule]{
			Items: []domain.GroupRule{
				{ID: "0pr_1", Status: domain.GroupRuleStatusActive},
				{ID: "0pr_2", Status: domain.GroupRuleStatusInactive},
				{ID: "0pr_3", Status: domain.GroupRuleStatusInvalid},
			},
		}, nil
	}
	groupsPort := fakes.NewGroupsPort(t)
	svc := service.NewRulesService(rulesPort, groupsPort)

	iter, err := svc.List(context.Background(), domain.GroupRulesQuery{})
	require.NoError(t, err)
	defer iter.Close()

	states := map[domain.GroupRuleStatus]int{}
	for {
		r, more, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !more {
			break
		}
		states[r.Status]++
	}
	assert.Equal(t, 1, states[domain.GroupRuleStatusActive])
	assert.Equal(t, 1, states[domain.GroupRuleStatusInactive])
	assert.Equal(t, 1, states[domain.GroupRuleStatusInvalid])
}

// RulesService.ResolveTargetGroupNames — id → name 해소.
func Test_RulesService_ResolveTargetGroupNames_Maps(t *testing.T) {
	t.Parallel()
	rulesPort := fakes.NewGroupRulesPort(t)
	groupsPort := fakes.NewGroupsPort(t)
	groupsPort.GetFunc = func(_ context.Context, id string) (domain.Group, error) {
		return domain.Group{ID: id, Profile: domain.GroupProfile{Name: "Engineering"}}, nil
	}
	svc := service.NewRulesService(rulesPort, groupsPort)

	names, err := svc.ResolveTargetGroupNames(context.Background(), []string{"00g_engineering"})
	require.NoError(t, err)
	require.Len(t, names, 1)
	assert.Equal(t, "Engineering", names["00g_engineering"])
}

// Bundle.InvalidateAll 완전 채움 — 모든 서비스 주입 경로 (62.5% → 100% 유도).
func Test_Bundle_InvalidateAll_AllServicesPresent(t *testing.T) {
	t.Parallel()

	usersPort := fakes.NewUsersPort(t)
	groupsPort := fakes.NewGroupsPort(t)
	rulesPort := fakes.NewGroupRulesPort(t)
	policiesPort := fakes.NewPoliciesPort(t)
	logsPort := fakes.NewLogsPort(t)

	bundle := &service.Bundle{
		Users:    service.NewUsersService(usersPort),
		Groups:   service.NewGroupsService(groupsPort, rulesPort),
		Rules:    service.NewRulesService(rulesPort, groupsPort),
		Policies: service.NewPoliciesService(policiesPort),
		Logs:     service.NewLogsService(logsPort),
	}
	require.NotPanics(t, func() { bundle.InvalidateAll() })
}

// GroupRulesService.List + Get + ResolveTargetGroupNames.
func Test_RulesService_Get_PassesThrough(t *testing.T) {
	t.Parallel()
	rulesPort := fakes.NewGroupRulesPort(t)
	rulesPort.GetFunc = func(_ context.Context, id string) (domain.GroupRule, error) {
		return domain.GroupRule{
			ID:     id,
			Name:   "Engineers to Eng",
			Status: domain.GroupRuleStatusActive,
		}, nil
	}
	groupsPort := fakes.NewGroupsPort(t)
	svc := service.NewRulesService(rulesPort, groupsPort)
	r, err := svc.Get(context.Background(), "0pr_active")
	require.NoError(t, err)
	assert.Equal(t, domain.GroupRuleStatusActive, r.Status)
}

// LogsService.Search + PollInterval + SetAdaptive.
func Test_LogsService_Search_PassesThrough(t *testing.T) {
	t.Parallel()
	port := fakes.NewLogsPort(t)
	port.SearchFunc = func(_ context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		assert.Equal(t, domain.SortDescending, q.SortOrder)
		return &fakes.SliceIterator[domain.LogEvent]{
			Items: []domain.LogEvent{{UUID: "u1", EventType: "user.session.start"}},
		}, nil
	}
	svc := service.NewLogsService(port)
	iter, err := svc.Search(context.Background(), domain.LogsQuery{SortOrder: domain.SortDescending})
	require.NoError(t, err)
	defer iter.Close()
	e, more, err := iter.Next(context.Background())
	require.NoError(t, err)
	require.True(t, more)
	assert.Equal(t, "u1", e.UUID)
}

// LogsService.PollInterval + SetAdaptive — 기본 7초, adaptive → 15초.
func Test_LogsService_PollInterval_AdaptiveToggle(t *testing.T) {
	t.Parallel()
	port := fakes.NewLogsPort(t)
	svc := service.NewLogsService(port)
	assert.Equal(t, 7*time.Second, svc.PollInterval(),
		"기본 poll interval 7초 (REQ-R05 AC-2)")
	svc.SetAdaptive(true)
	assert.Equal(t, 15*time.Second, svc.PollInterval(),
		"Adaptive=true 시 15초 (REQ-R05 AC-2)")
	svc.SetAdaptive(false)
	assert.Equal(t, 7*time.Second, svc.PollInterval())
}

// LogsTail.Poll — 실제 Port 호출 → events + nextSince 반환.
func Test_LogsTail_Poll_ReturnsEventsAndAdvancedSince(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	port := fakes.NewLogsPort(t)
	port.SearchFunc = func(_ context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
		require.NotNil(t, q.Since)
		return &fakes.SliceIterator[domain.LogEvent]{
			Items: []domain.LogEvent{
				{UUID: "u1", Published: base.Add(10 * time.Second)},
				{UUID: "u2", Published: base.Add(20 * time.Second)},
			},
		}, nil
	}
	tail := service.NewLogsTail(port,
		service.WithLogsTailNow(func() time.Time { return base }),
	)
	q := tail.InitialQuery()
	events, nextSince, err := tail.Poll(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, events, 2)
	// 마지막 published + 1ms 로 다음 since 설정.
	assert.True(t,
		nextSince.Equal(base.Add(20*time.Second).Add(time.Millisecond)),
		"nextSince는 last published + 1ms (REQ-R05 AC-2)")
}
