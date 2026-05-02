package apilog_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/apilog"
)

func Test_New_CreatesDirAndReturnsRecorder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	r, err := apilog.New(filepath.Join(dir, "ota", "api"), 0)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.False(t, r.Disabled(), "fresh recorder must not be disabled")
}

func Test_Record_AppendsToRingAndDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := apilog.New(dir, 4)
	require.NoError(t, err)
	defer r.Close()

	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	r.Record(apilog.Entry{
		Time:   now,
		Method: "GET",
		URL:    "https://example.okta.com/api/v1/users",
		Path:   "/api/v1/users",
		Status: 200,
	})

	snap := r.Snapshot()
	require.Len(t, snap, 1)
	assert.Equal(t, "GET", snap[0].Method)
	assert.NotZero(t, snap[0].SeqID, "Record must stamp a non-zero SeqID")

	want := filepath.Join(dir, "api-2026-05-03.ndjson")
	data, err := os.ReadFile(want)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"method":"GET"`)
	assert.Contains(t, string(data), `"status":200`)
}

func Test_Snapshot_ReturnsRingInChronologicalOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := apilog.New(dir, 3)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		r.Record(apilog.Entry{
			Time:   time.Date(2026, 5, 3, 0, 0, i, 0, time.UTC),
			Method: "GET",
			Path:   "/p" + string(rune('0'+i)),
		})
	}

	snap := r.Snapshot()
	require.Len(t, snap, 3, "ring caps at size; oldest entries fall out")
	assert.Equal(t, "/p2", snap[0].Path)
	assert.Equal(t, "/p3", snap[1].Path)
	assert.Equal(t, "/p4", snap[2].Path)
}

func Test_RedactHeaders_StripsAuthCookieAndApiKey(t *testing.T) {
	t.Parallel()
	in := http.Header{
		"Authorization":   []string{"SSWS my-real-token"},
		"Cookie":          []string{"sid=abc"},
		"Set-Cookie":      []string{"sid=def"},
		"X-Okta-Api-Token": []string{"raw"},
		"User-Agent":      []string{"ota/0.2"},
		"Accept":          []string{"application/json"},
	}
	out := apilog.RedactHeaders(in)

	assert.Equal(t, []string{"***"}, out["Authorization"])
	assert.Equal(t, []string{"***"}, out["Cookie"])
	assert.Equal(t, []string{"***"}, out["Set-Cookie"])
	assert.Equal(t, []string{"***"}, out["X-Okta-Api-Token"])
	assert.Equal(t, []string{"ota/0.2"}, out["User-Agent"], "non-sensitive headers untouched")
	assert.Equal(t, []string{"application/json"}, out["Accept"])

	// Defensive copy — mutating the result must not leak back.
	out["User-Agent"][0] = "changed"
	assert.Equal(t, "ota/0.2", in["User-Agent"][0])
}

func Test_RedactJSONBody_MasksPIIAndSecretKeys(t *testing.T) {
	t.Parallel()
	in := []byte(`{
		"id":"00ualice",
		"profile":{
			"login":"alice@acme.com",
			"firstName":"Alice",
			"lastName":"Anderson",
			"mobilePhone":"+1-415-555-1234",
			"streetAddress":"100 Main St",
			"zipCode":"94107"
		},
		"credentials":{"password":{"value":"hunter2"}}
	}`)
	out := apilog.RedactJSONBody(in)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))

	profile := got["profile"].(map[string]any)
	assert.Equal(t, "alice@acme.com", profile["login"], "login is not in piiKeys → preserved")
	assert.Equal(t, "***", profile["firstName"])
	assert.Equal(t, "***", profile["lastName"])
	assert.Equal(t, "***", profile["mobilePhone"])
	assert.Equal(t, "***", profile["streetAddress"])
	assert.Equal(t, "***", profile["zipCode"])

	creds := got["credentials"].(map[string]any)
	pw := creds["password"]
	assert.Equal(t, "***", pw, "password key fully replaced regardless of nested shape")
}

func Test_RedactJSONBody_NonJSONFallsThrough(t *testing.T) {
	t.Parallel()
	in := []byte("not json at all")
	assert.Equal(t, in, apilog.RedactJSONBody(in))
}

func Test_CapBody_TruncatesAtLimit(t *testing.T) {
	t.Parallel()
	huge := make([]byte, apilog.MaxBodyBytes*2)
	for i := range huge {
		huge[i] = 'x'
	}
	out := apilog.CapBody(huge)
	assert.True(t, len(out) > apilog.MaxBodyBytes && len(out) < len(huge))
	assert.True(t, strings.HasSuffix(string(out), "…[truncated]"))
}

func Test_Transport_CapturesAndRedactsRoundTrip(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"profile":{"login":"alice@acme.com","mobilePhone":"+1-555-867-5309"}}`))
	}))
	defer srv.Close()

	r, err := apilog.New(t.TempDir(), 8)
	require.NoError(t, err)
	cli := &http.Client{Transport: r.Transport(nil)}

	req, err := http.NewRequest("GET", srv.URL+"/api/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "SSWS realtoken")

	resp, err := cli.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Contains(t, string(body), "alice@acme.com",
		"downstream consumers must see the original (un-redacted) bytes")

	snap := r.Snapshot()
	require.Len(t, snap, 1)
	got := snap[0]
	assert.Equal(t, "GET", got.Method)
	assert.Equal(t, "/api/v1/users/me", got.Path)
	assert.Equal(t, 200, got.Status)
	assert.Equal(t, []string{"***"}, got.RequestHeaders["Authorization"],
		"transport must redact Authorization before write")
	assert.Contains(t, got.ResponseBody, "alice@acme.com",
		"login is preserved (not in piiKeys)")
	assert.Contains(t, got.ResponseBody, `"***"`,
		"mobilePhone in response body must be redacted")
	assert.NotContains(t, got.ResponseBody, "+1-555-867-5309")
}

func Test_PruneOlderThan_DeletesFilesOlderThanRetention(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)

	// Produce: 1 file from today, 1 from 2 days ago (kept), 1 from
	// 5 days ago (pruned).
	fresh := filepath.Join(dir, "api-2026-05-03.ndjson")
	mid := filepath.Join(dir, "api-2026-05-01.ndjson")
	stale := filepath.Join(dir, "api-2026-04-28.ndjson")
	for _, p := range []string{fresh, mid, stale} {
		require.NoError(t, os.WriteFile(p, []byte("{}\n"), 0o600))
	}

	// Re-create recorder targeting the same directory; New runs
	// pruneOlderThan as a side-effect.
	_, err := apilog.NewWithClock(dir, 4, now)
	require.NoError(t, err)

	for _, p := range []string{fresh, mid} {
		_, err := os.Stat(p)
		assert.NoError(t, err, "%s must survive pruning", filepath.Base(p))
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("%s must be pruned (older than retention); got err = %v", filepath.Base(stale), err)
	}
}
