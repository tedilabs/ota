package app_test

// E2E smoke: verify that App.Init -> child Init Cmd -> Okta HTTP fetch ->
// usersLoadedMsg -> View() actually surfaces user data when run end-to-end
// against an httptest-backed Okta server. Reproduces the user's reported
// scenario without needing a live tenant.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/okta"
	"github.com/tedilabs/ota/internal/okta/testfx"
	"github.com/tedilabs/ota/internal/service"
)

func Test_E2E_UsersList_FetchesViaWireAndRendersInView(t *testing.T) {
	t.Parallel()

	srv := testfx.NewFakeOktaServer(t, "pagination_multi_page")
	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL:     srv.URL,
		APIToken:   "ssws-test-token",
		HTTPClient: srv.Client(),
	}, okta.WithClock(clock.Real()))
	require.NoError(t, err)

	bundle := &service.Bundle{
		Users:    service.NewUsersService(cli.Users()),
		Groups:   service.NewGroupsService(cli.Groups(), cli.GroupRules()),
		Rules:    service.NewGroupRulesService(cli.GroupRules(), cli.Groups()),
		Policies: service.NewPoliciesService(cli.Policies()),
		Logs:     service.NewLogsService(cli.Logs()),
		LogsTail: service.NewLogsTail(cli.Logs()),
	}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	mdl := app.New(app.Deps{
		Services:       bundle,
		RateLimit:      cli.RateLimitMonitor(),
		Keys:           keymap,
		Clock:          clock.Real(),
		Profile:        "test",
		OrgURL:         srv.URL,
		UsersPort:      cli.Users(),
		GroupsPort:     cli.Groups(),
		GroupRulesPort: cli.GroupRules(),
		PoliciesPort:   cli.Policies(),
		LogsPort:       cli.Logs(),
	})

	// Step 1 — initial fetch Cmd from App.Init() (delegates to ScreenUsers).
	initCmd := mdl.Init()
	require.NotNil(t, initCmd, "App.Init must return a fetch Cmd for the initial Users screen")

	// Step 2 — execute the fetch (synchronously) to obtain the resulting Msg.
	msg := initCmd()
	require.NotNil(t, msg, "fetch Cmd must produce a Msg (loaded or err)")

	// Step 3 — feed Msg back through Update so the child Screen records data.
	updated, _ := mdl.Update(msg)
	mdl = updated.(app.Model)

	// Step 4 — Render and assert user logins appear.
	view := mdl.View()
	t.Logf("rendered view (full):\n%s", view)

	// pagination_multi_page fixture seeds alice + 5 more users.
	assert.Contains(t, view, "alice", "View() must contain seeded user 'alice' once fetch completes")
	assert.NotContains(t, view, "Failed to load",
		"View() must not show the ErrorPanel — fetch should succeed")
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Sanity helper: ensure tea.Cmd is non-nil at compile time.
var _ = func() tea.Cmd { return nil }

func Test_E2E_UsersList_401InvalidToken_RendersErrorPanel(t *testing.T) {
	t.Parallel()

	// Inline httptest that always returns 401 with Okta-shaped body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorCode":"E0000011","errorSummary":"Invalid token provided","errorLink":"E0000011","errorId":"oae","errorCauses":[]}`))
	}))
	t.Cleanup(srv.Close)

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL:     srv.URL,
		APIToken:   "ssws-bad-token",
		HTTPClient: srv.Client(),
	}, okta.WithClock(clock.Real()), okta.WithMaxRetries(0))
	require.NoError(t, err)

	bundle := &service.Bundle{
		Users:    service.NewUsersService(cli.Users()),
		Groups:   service.NewGroupsService(cli.Groups(), cli.GroupRules()),
		Rules:    service.NewGroupRulesService(cli.GroupRules(), cli.Groups()),
		Policies: service.NewPoliciesService(cli.Policies()),
		Logs:     service.NewLogsService(cli.Logs()),
		LogsTail: service.NewLogsTail(cli.Logs()),
	}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)

	mdl := app.New(app.Deps{
		Services:       bundle,
		RateLimit:      cli.RateLimitMonitor(),
		Keys:           keymap,
		Clock:          clock.Real(),
		Profile:        "test",
		OrgURL:         srv.URL,
		UsersPort:      cli.Users(),
		GroupsPort:     cli.Groups(),
		GroupRulesPort: cli.GroupRules(),
		PoliciesPort:   cli.Policies(),
		LogsPort:       cli.Logs(),
	})

	initCmd := mdl.Init()
	require.NotNil(t, initCmd)
	msg := initCmd()
	require.NotNil(t, msg)
	updated, _ := mdl.Update(msg)
	mdl = updated.(app.Model)

	view := mdl.View()
	t.Logf("401 view (full):\n%s", view)

	assert.Contains(t, view, "Failed to load",
		"View should show the ErrorPanel header on auth failure")
	// errormap.UserMessage(E0000011) 메시지 일부가 표시되어야 한다.
	assert.True(t,
		strings.Contains(strings.ToLower(view), "token") ||
			strings.Contains(strings.ToLower(view), "invalid") ||
			strings.Contains(strings.ToLower(view), "auth"),
		"View must reveal the actual reason (token/invalid/auth) not just a generic banner")
}
