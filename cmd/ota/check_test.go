package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_RunCheck_Success_HittsHttptest probes a fake Okta server that returns
// one user and asserts the diagnostic prints "OK" plus the user count.
func Test_RunCheck_Success_HittsHttptest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"00u1","status":"ACTIVE","profile":{"login":"alice@acme.com","firstName":"Alice","lastName":"Smith"}}]`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OKTA_ORG_URL", srv.URL)
	t.Setenv("OKTA_API_TOKEN", "test-token-fake")

	var stdout, stderr bytes.Buffer
	code := runCheck(context.Background(), WireInput{}, &stdout, &stderr)

	require.Equalf(t, 0, code, "exit code must be 0 for healthy probe — stdout was:\n%s", stdout.String())
	out := stdout.String()
	assert.Contains(t, out, "result:     OK", "diagnostic must show OK on success")
	assert.Contains(t, out, "alice@acme.com", "first user login must surface as evidence")
	assert.NotContains(t, out, "test-token-fake",
		"diagnostic must NOT leak the raw token (REQ-C05 AC-3)")
}

// Test_RunCheck_401_UnauthorizedReportsHint asserts the diagnostic spells out
// the auth failure (status, message, actionable hint) when the tenant returns
// 401 — the most common operator question after "왜 안 보이지?".
func Test_RunCheck_401_UnauthorizedReportsHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorCode":"E0000011","errorSummary":"Invalid token provided"}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OKTA_ORG_URL", srv.URL)
	t.Setenv("OKTA_API_TOKEN", "bad-token")

	var stdout, stderr bytes.Buffer
	code := runCheck(context.Background(), WireInput{}, &stdout, &stderr)

	require.Equal(t, 1, code, "exit code must be 1 on auth failure")
	out := stdout.String()
	assert.Contains(t, out, "401 Unauthorized")
	assert.Contains(t, strings.ToLower(out), "token",
		"hint must reference the token so the operator knows what to fix")
	assert.Contains(t, strings.ToLower(out), "rotate",
		"401 hint must tell the operator to rotate the token")
	assert.NotContains(t, out, "bad-token",
		"diagnostic must NOT leak the raw token (REQ-C05 AC-3)")
}

// Test_RunCheck_NoProfile_ShortCircuits asserts the early failure path (no
// OKTA_ORG_URL, no config) prints a clear remediation without attempting the
// HTTP probe.
func Test_RunCheck_NoProfile_ShortCircuits(t *testing.T) {
	t.Setenv("OKTA_ORG_URL", "")
	t.Setenv("OKTA_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	code := runCheck(context.Background(), WireInput{}, &stdout, &stderr)

	require.Equal(t, 1, code)
	out := stdout.String()
	assert.Contains(t, out, "FAIL — profile resolve failed")
	assert.Contains(t, out, "OKTA_ORG_URL")
	assert.NotContains(t, out, "GET /api/v1/users",
		"early failure must not pretend to issue an HTTP request")
}
