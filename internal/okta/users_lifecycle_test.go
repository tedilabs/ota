package okta_test

// Tests for the Users lifecycle endpoints (issue #125): the WRITE ops
// behind the Users list / detail screen actions. We exercise:
//   - POST /api/v1/users/{id}/lifecycle/reset_password (sendEmail flag)
//   - POST /api/v1/users/{id}/lifecycle/unlock
//   - POST /api/v1/users/{id}/lifecycle/reset_factors
//
// Each test pins the request shape (method, path, query) and the
// response decode path so a regression in the adapter surfaces as a
// targeted assertion failure rather than a generic e2e flake.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/okta"
)

func Test_UsersAdapter_ResetPassword_SendEmailTrue_ReturnsEmptyURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/users/00u_alice/lifecycle/reset_password", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("sendEmail"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	url, err := cli.Users().ResetPassword(context.Background(), "00u_alice", true)
	require.NoError(t, err)
	assert.Empty(t, url, "sendEmail=true must return an empty URL — Okta sends the email itself")
}

func Test_UsersAdapter_ResetPassword_SendEmailFalse_ReturnsResetURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "false", r.URL.Query().Get("sendEmail"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resetPasswordUrl":"https://acme.okta.com/reset_password/abc"}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	url, err := cli.Users().ResetPassword(context.Background(), "00u_alice", false)
	require.NoError(t, err)
	assert.Equal(t, "https://acme.okta.com/reset_password/abc", url)
}

func Test_UsersAdapter_Unlock_PostsAndReturnsNil(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/users/00u_alice/lifecycle/unlock", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	require.NoError(t, cli.Users().Unlock(context.Background(), "00u_alice"))
	assert.True(t, called, "Unlock must hit /lifecycle/unlock")
}

func Test_UsersAdapter_ResetFactors_PostsAndReturnsNil(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/users/00u_alice/lifecycle/reset_factors", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	require.NoError(t, cli.Users().ResetFactors(context.Background(), "00u_alice"))
	assert.True(t, called, "ResetFactors must hit /lifecycle/reset_factors")
}
