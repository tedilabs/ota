package okta_test

// QA-017 coverage 보강 — Groups/Rules/Policies adapter integration.
// httptest.Server + testdata/oktaapi fixture 재사용.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta"
	"github.com/tedilabs/ota/internal/okta/testfx"
)

// REQ-R02 AC-1 — Groups.List 드레인 후 BUILT_IN + OKTA_GROUP + APP_GROUP 혼재.
func Test_GroupsAdapter_List_DrainsMixedTypes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/groups", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/groups/list_mixed.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	}, okta.WithClock(clock.Real()))
	require.NoError(t, err)

	iter, err := cli.Groups().List(context.Background(), domain.GroupsQuery{Limit: 200})
	require.NoError(t, err)
	defer iter.Close()

	types := map[domain.GroupType]int{}
	for {
		g, more, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !more {
			break
		}
		types[g.Type]++
	}
	assert.Equal(t, 1, types[domain.GroupTypeBuiltIn], "BUILT_IN 1개")
	assert.Equal(t, 2, types[domain.GroupTypeOkta], "OKTA_GROUP 2개")
	assert.Equal(t, 1, types[domain.GroupTypeApp], "APP_GROUP 1개")
}

// REQ-R02 — Get 단건 조회.
func Test_GroupsAdapter_Get_DecodesSingle(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/groups/00g_engineering", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"00g_engineering","type":"OKTA_GROUP","profile":{"name":"Engineering"}}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	g, err := cli.Groups().Get(context.Background(), "00g_engineering")
	require.NoError(t, err)
	assert.Equal(t, "00g_engineering", g.ID)
	assert.Equal(t, "Engineering", g.Profile.Name)
	assert.Equal(t, domain.GroupTypeOkta, g.Type)
}

// REQ-R02 AC-3 — Members iterator 드레인.
func Test_GroupsAdapter_Members_DrainsPage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/groups/00g_everyone/users", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/users/list_page1.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	iter, err := cli.Groups().Members(context.Background(),
		domain.GroupMembersQuery{GroupID: "00g_everyone"})
	require.NoError(t, err)
	defer iter.Close()

	var count int
	for {
		_, more, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !more {
			break
		}
		count++
	}
	assert.Equal(t, 3, count, "list_page1 fixture의 user 3명")
}

// REQ-R02 AC-4 — AppCount 첫 페이지 집계.
func Test_GroupsAdapter_AppCount_ReturnsFirstPageLen(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/groups/00g_engineering/apps", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// 3개 앱만 간단히.
		_, _ = w.Write([]byte(`[{"id":"0oa_1"},{"id":"0oa_2"},{"id":"0oa_3"}]`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	n, err := cli.Groups().AppCount(context.Background(), "00g_engineering")
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

// REQ-R02 AC-4 — AppCount가 403 Forbidden이면 domain.ErrForbidden.
func Test_GroupsAdapter_AppCount_ForbiddenReturnsErrForbidden(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/errors/E0000006_forbidden.json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	_, err = cli.Groups().AppCount(context.Background(), "00g_any")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden,
		"Read-Only Admin 권한 부족 시 ErrForbidden (PRD §7.7)")
}

// REQ-R03 AC-1/AC-2 — Rules.List가 3 상태(ACTIVE/INACTIVE/INVALID) 드레인.
func Test_GroupRulesAdapter_List_DrainsAllThreeStates(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/groups/rules", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/grouprules/list_all_states.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	iter, err := cli.GroupRules().List(context.Background(), domain.GroupRulesQuery{})
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
	assert.Equal(t, 1, states[domain.GroupRuleStatusInvalid],
		"INVALID rule이 정확히 1개 드레인 (REQ-R03 AC-2)")
}

// REQ-R03 — Rules.Get 단건.
func Test_GroupRulesAdapter_Get_Decodes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/groups/rules/0pr_active", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"0pr_active","name":"Engineers to Eng","status":"ACTIVE",` +
			`"conditions":{"expression":{"value":"user.department == \"Engineering\"","type":"urn:okta:expression:1.0"}},` +
			`"actions":{"assignUserToGroups":{"groupIds":["00g_engineering"]}}}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	r, err := cli.GroupRules().Get(context.Background(), "0pr_active")
	require.NoError(t, err)
	assert.Equal(t, "0pr_active", r.ID)
	assert.Equal(t, domain.GroupRuleStatusActive, r.Status)
	assert.Contains(t, r.Expression, "Engineering")
	assert.Equal(t, []string{"00g_engineering"}, r.TargetGroupIDs)
}

// REQ-R04 AC-2 — Policies.List는 type 파라미터 필수.
func Test_PoliciesAdapter_List_RequiresType(t *testing.T) {
	t.Parallel()
	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: "http://unreachable", APIToken: "t", HTTPClient: &http.Client{Timeout: time.Second},
	})
	require.NoError(t, err)

	_, err = cli.Policies().List(context.Background(), domain.PoliciesQuery{})
	require.Error(t, err, "type 미지정 시 adapter가 즉시 거절해야 한다 (REQ-R04 AC-2)")
	assert.Contains(t, err.Error(), "Type")
}

// REQ-R04 AC-1 — OKTA_SIGN_ON 타입 리스트 드레인 + type 쿼리가 실제 전송됨.
func Test_PoliciesAdapter_List_SendsTypeQuery_AndDrains(t *testing.T) {
	t.Parallel()
	var observed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = r.URL.Query().Get("type")
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/policies/okta_sign_on.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	iter, err := cli.Policies().List(context.Background(),
		domain.PoliciesQuery{Type: domain.PolicyTypeOktaSignOn})
	require.NoError(t, err)
	defer iter.Close()

	var count int
	for {
		p, more, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !more {
			break
		}
		count++
		assert.Equal(t, domain.PolicyTypeOktaSignOn, p.Type)
	}
	assert.Equal(t, 2, count, "okta_sign_on fixture에 2개 policy")
	assert.Equal(t, "OKTA_SIGN_ON", observed, "type 쿼리 파라미터가 URL에 전달되어야 한다")
}

// REQ-R04 AC-6 — Get이 Raw JSON을 보존해야 한다 (raw 모드 토글용).
func Test_PoliciesAdapter_Get_PreservesRawJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"00p_raw","type":"IDP_DISCOVERY","name":"IdP","settings":{"providers":[]}}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	p, err := cli.Policies().Get(context.Background(), "00p_raw")
	require.NoError(t, err)
	assert.Equal(t, "00p_raw", p.ID)
	assert.Equal(t, domain.PolicyTypeIDPDiscovery, p.Type)
	assert.NotEmpty(t, p.Raw, "Raw JSON이 보존되어야 한다 (REQ-R04 AC-6 raw 토글)")
	// Raw에 "providers"가 포함되어야 한다.
	assert.Contains(t, string(p.Raw), "providers")

	// Raw가 실제 파싱 가능.
	var verify map[string]any
	require.NoError(t, json.Unmarshal(p.Raw, &verify))
	assert.Equal(t, "00p_raw", verify["id"])
}

// REQ-R04 AC-4 — Rules(policy)는 priority 순.
func Test_PoliciesAdapter_Rules_DecodesList(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/rules"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"pr_2","name":"Medium","priority":2,"status":"ACTIVE"},
			{"id":"pr_1","name":"Top","priority":1,"status":"ACTIVE","system":true}
		]`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	rules, err := cli.Policies().Rules(context.Background(), "00p_x")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "pr_2", rules[0].ID, "API 응답 순서 그대로 — service 레이어가 priority 정렬")
}

// Client builder 메서드 smoke — 0% 커버 함수 (Groups/GroupRules/Policies/RateLimitMonitor).
func Test_Client_BuilderMethods_ReturnAdapters(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	require.NotNil(t, cli.Users())
	require.NotNil(t, cli.Groups())
	require.NotNil(t, cli.GroupRules())
	require.NotNil(t, cli.Policies())
	require.NotNil(t, cli.Logs())
	require.NotNil(t, cli.RateLimitMonitor(), "RateLimitMonitor는 non-nil이어야 한다")
}
