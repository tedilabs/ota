package okta_test

// QA-017 coverage 보강 — Users adapter의 Get/ListGroups/ListFactors + Client Options.

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta"
	"github.com/tedilabs/ota/internal/okta/ratelimit"
	"github.com/tedilabs/ota/internal/okta/testfx"
)

// REQ-R01 AC-3 — Get 단건 조회가 detail 응답을 domain.User로 매핑.
func Test_UsersAdapter_Get_DecodesSingle(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/00u_active_alice", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/users/detail_active.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	u, err := cli.Users().Get(context.Background(), "00u_active_alice")
	require.NoError(t, err)
	assert.Equal(t, "00u_active_alice", u.ID)
	assert.Equal(t, domain.UserStatusActive, u.Status)
	assert.Contains(t, u.Profile.Login, "alice")
}

// REQ-R01 AC-3 — Get이 404면 domain.ErrNotFound.
func Test_UsersAdapter_Get_NotFound_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		body := testfx.LoadFixture(t, "oktaapi/errors/E0000007_not_found.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	_, err = cli.Users().Get(context.Background(), "00u_nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound,
		"404는 domain.ErrNotFound 매핑 (PRD §7.7)")
}

// REQ-R01 AC-3 — ListGroups는 User가 속한 그룹 배열 반환.
func Test_UsersAdapter_ListGroups_DecodesArray(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/00u_active_alice/groups", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/users/groups_of_user.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	gs, err := cli.Users().ListGroups(context.Background(), "00u_active_alice")
	require.NoError(t, err)
	require.Len(t, gs, 2, "groups_of_user fixture는 Everyone + Engineering 2개")
	assert.Contains(t, []string{gs[0].Profile.Name, gs[1].Profile.Name}, "Engineering")
}

// REQ-R01 AC-6 — ListFactors는 7종 factor type을 전부 매핑.
func Test_UsersAdapter_ListFactors_MapsAllTypes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/factors"))
		w.Header().Set("Content-Type", "application/json")
		body := testfx.LoadFixture(t, "oktaapi/users/factors_all_types.json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	fs, err := cli.Users().ListFactors(context.Background(), "00u_active_alice")
	require.NoError(t, err)
	require.Len(t, fs, 7, "factors_all_types fixture는 7종")

	types := map[domain.FactorType]bool{}
	for _, f := range fs {
		types[f.Type] = true
	}
	for _, want := range []domain.FactorType{
		domain.FactorTypePush,
		domain.FactorTypeSMS,
		domain.FactorTypeTOTP,
		domain.FactorTypeWebAuthn,
		domain.FactorTypeEmail,
		domain.FactorTypeQuestion,
		domain.FactorTypeHardwareToken,
	} {
		assert.True(t, types[want], "factor type %s 매핑 (REQ-R01 AC-6)", want)
	}
}

// Client Options — WithLogger, WithMonitor 호출 시 panic 없이 client 생성.
func Test_OktaClient_WithOptions_Accepts(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	lg := slog.New(slog.DiscardHandler)
	monitor := ratelimit.NewMonitor(clock.Real())

	cli, err := okta.NewClient(context.Background(),
		okta.Config{OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client()},
		okta.WithLogger(lg),
		okta.WithMonitor(monitor),
		okta.WithClock(clock.Real()),
		okta.WithMaxRetries(5),
	)
	require.NoError(t, err)
	require.NotNil(t, cli)
	// Monitor가 주입된 그대로 노출되는지 확인.
	assert.Same(t, monitor, cli.RateLimitMonitor())
}
