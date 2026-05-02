package okta_test

// #F3 v0.2.5 follow-up — when Okta splits the Link response into
// multiple `Link:` headers (one per rel value) instead of a single
// comma-joined header, http.Header.Get returns only the first.
// joinHeaderValues collapses the slice so NextCursor sees every rel
// — without it, rel="next" can be hidden behind rel="self" and the
// load-older sentinel stays dark.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta"
)

func Test_LogsAdapter_SearchPage_MultiLinkHeader_PicksNext(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Two separate Link headers — the order Okta sometimes uses.
		w.Header().Add("Link", `<https://acme.okta.com/api/v1/logs?limit=100>; rel="self"`)
		w.Header().Add("Link", `<https://acme.okta.com/api/v1/logs?limit=100&after=NEXT_CURSOR>; rel="next"`)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{})
	}))
	t.Cleanup(srv.Close)

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL:     srv.URL,
		APIToken:   "ssws-test-token",
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	page, err := cli.Logs().SearchPage(context.Background(), domain.LogsQuery{Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, "NEXT_CURSOR", page.After,
		"SearchPage must extract rel=next from a multi-Link-header response (#F3 follow-up)")
}
