package oktastatus_test

// v0.2.2 #190 — pin the Statuspage.io status.json shape parser.
// status.okta.com is the only live endpoint we ship against; the
// indicator string ↔ Indicator enum mapping is the contract Okta
// has documented at https://status.okta.com/api.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/oktastatus"
)

// Test_Probe_Fetch_DecodesOperational pins the canonical happy
// path: indicator=none → IndicatorOperational + 🟢 emoji.
func Test_Probe_Fetch_DecodesOperational(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/status.json", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "page":   { "name": "Okta" },
		  "status": { "indicator": "none", "description": "All Systems Operational" }
		}`))
	}))
	defer srv.Close()

	probe := oktastatus.Probe{Endpoint: srv.URL + "/api/v2/status.json"}
	snap := probe.Fetch(context.Background())

	assert.Equal(t, oktastatus.IndicatorOperational, snap.Indicator)
	assert.Equal(t, "All Systems Operational", snap.Description)
	assert.Equal(t, "🟢", snap.Indicator.Emoji())
	assert.Equal(t, "ok", snap.Indicator.Label())
	assert.False(t, snap.FetchedAt.IsZero())
}

// Test_Probe_Fetch_DecodesAllIndicatorStrings rounds out the enum:
// every documented Statuspage indicator string maps to a non-Unknown
// Indicator with a distinct emoji + label.
func Test_Probe_Fetch_DecodesAllIndicatorStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw      string
		want     oktastatus.Indicator
		emoji    string
		label    string
	}{
		{"none", oktastatus.IndicatorOperational, "🟢", "ok"},
		{"minor", oktastatus.IndicatorMinor, "🟡", "minor"},
		{"major", oktastatus.IndicatorMajor, "🟠", "major"},
		{"critical", oktastatus.IndicatorCritical, "🔴", "critical"},
		{"maintenance", oktastatus.IndicatorMaintenance, "🛠", "maint"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":{"indicator":"` + tc.raw + `","description":"x"}}`))
			}))
			defer srv.Close()
			snap := (&oktastatus.Probe{Endpoint: srv.URL}).Fetch(context.Background())
			require.Equal(t, tc.want, snap.Indicator)
			require.Equal(t, tc.emoji, snap.Indicator.Emoji())
			require.Equal(t, tc.label, snap.Indicator.Label())
		})
	}
}

// Test_Probe_Fetch_HTTPErrorCollapsesToUnknown — 5xx server errors
// must NOT crash; the chrome falls back to a muted ❔ glyph so the
// operator sees "we don't know" instead of a stale "operational".
func Test_Probe_Fetch_HTTPErrorCollapsesToUnknown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	snap := (&oktastatus.Probe{Endpoint: srv.URL, Timeout: time.Second}).Fetch(context.Background())
	assert.Equal(t, oktastatus.IndicatorUnknown, snap.Indicator)
	assert.Equal(t, "❔", snap.Indicator.Emoji())
}

// Test_Probe_Fetch_DialErrorCollapsesToUnknown — bogus endpoint
// (DNS / connection failure) returns the unknown glyph + a non-zero
// FetchedAt so the chrome gating-on-FetchedAt-isZero test stays
// reliable.
func Test_Probe_Fetch_DialErrorCollapsesToUnknown(t *testing.T) {
	t.Parallel()

	snap := (&oktastatus.Probe{
		Endpoint: "http://127.0.0.1:0/dead",
		Timeout:  100 * time.Millisecond,
	}).Fetch(context.Background())
	assert.Equal(t, oktastatus.IndicatorUnknown, snap.Indicator)
	assert.False(t, snap.FetchedAt.IsZero(),
		"FetchedAt must be stamped even on dial failure so the chrome can mount the muted segment")
}
