package okta_test

// Tests for the Apps adapter (issue #167). The user reported a live
// "Failed to load Apps · Request rejected by Okta" — root cause was
// the adapter sending `filter=signOnMode eq "SAML_2_0"` to
// /api/v1/apps, which Okta doesn't support on that endpoint
// (only user.id / group.id / name predicates). The fix moved type
// scoping to a client-side iterator. These tests pin both halves:
// the wire request must NOT carry signOnMode, and the result iter
// must filter by Type even though the server returned everything.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta"
)

const appsMixedBody = `[
  {"id":"0oa_saml1","name":"salesforce","label":"Salesforce","status":"ACTIVE","signOnMode":"SAML_2_0","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-06-01T00:00:00.000Z"},
  {"id":"0oa_oidc1","name":"okta_org2org","label":"Org2Org","status":"ACTIVE","signOnMode":"OPENID_CONNECT","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-06-01T00:00:00.000Z"},
  {"id":"0oa_bk1","name":"intranet","label":"Intranet","status":"INACTIVE","signOnMode":"BOOKMARK","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-06-01T00:00:00.000Z"}
]`

func Test_AppsAdapter_List_DoesNotSendSignOnModeFilter(t *testing.T) {
	t.Parallel()

	var lastFilter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/apps", r.URL.Path)
		lastFilter = r.URL.Query().Get("filter")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(appsMixedBody))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	}, okta.WithClock(clock.Real()))
	require.NoError(t, err)

	iter, err := cli.Apps().List(context.Background(), domain.AppsQuery{Type: domain.AppTypeSAML, Limit: 200})
	require.NoError(t, err)
	defer iter.Close()

	// Drain — must succeed even though the adapter doesn't pass a
	// type filter on the wire.
	out := []domain.App{}
	for {
		a, hasMore, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !hasMore {
			break
		}
		out = append(out, a)
	}

	assert.NotContains(t, lastFilter, "signOnMode",
		"Apps API doesn't accept `signOnMode` predicate — adapter must not send it (issue #167)")
	assert.Len(t, out, 1, "client-side filter must narrow to the SAML row")
	assert.Equal(t, "Salesforce", out[0].Label)
}

// Test_AppsAdapter_List_NoTypeReturnsEverything verifies the
// type-less query still drains the full app list (no client-side
// filter applied).
func Test_AppsAdapter_List_NoTypeReturnsEverything(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(appsMixedBody))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "t", HTTPClient: srv.Client(),
	}, okta.WithClock(clock.Real()))
	require.NoError(t, err)

	iter, err := cli.Apps().List(context.Background(), domain.AppsQuery{Limit: 200})
	require.NoError(t, err)
	defer iter.Close()

	out := []domain.App{}
	for {
		a, hasMore, err := iter.Next(context.Background())
		require.NoError(t, err)
		if !hasMore {
			break
		}
		out = append(out, a)
	}
	assert.Len(t, out, 3, "no Type → return every app")
}
