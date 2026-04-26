package pagination_test

// REQ-R01 AC-4, REQ-R02 AC-3, PRD §7.3 — Link 헤더 커서 파서.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/okta/pagination"
)

func Test_LinkHeader_ParsesNextCursor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		header     string
		wantCursor string
		wantNext   bool
	}{
		{
			"next_and_self",
			`<https://dev-example.okta.com/api/v1/users?limit=200&after=ABC123>; rel="next", ` +
				`<https://dev-example.okta.com/api/v1/users?limit=200>; rel="self"`,
			"ABC123",
			true,
		},
		{
			"self_only",
			`<https://dev-example.okta.com/api/v1/users?limit=200>; rel="self"`,
			"",
			false,
		},
		{
			"empty_header",
			``,
			"",
			false,
		},
		{
			"cursor_is_opaque_not_decoded",
			`<https://dev-example.okta.com/api/v1/logs?since=2026-04-24T12Z&after=opaque%2Btoken%2Fbase64>; rel="next"`,
			"opaque%2Btoken%2Fbase64",
			true,
		},
		{
			// RFC 5988 — 따옴표 없는 rel=next도 허용.
			"rel_without_quotes",
			`<https://dev-example.okta.com/api/v1/users?after=NOQUOTE>; rel=next`,
			"NOQUOTE",
			true,
		},
		{
			// Malformed (rel 파트 없음) — 스킵되어 hasNext=false.
			"malformed_missing_rel",
			`<https://dev-example.okta.com/api/v1/users?after=X>`,
			"",
			false,
		},
		{
			// next 링크는 있으나 `after` 파라미터가 빠진 경우 — hasNext=true, cursor="".
			"next_link_without_after_param",
			`<https://dev-example.okta.com/api/v1/users?limit=200>; rel="next"`,
			"",
			true,
		},
		{
			// next가 rel="prev"·rel="self" 뒤에 오는 혼합 순서.
			"next_after_prev",
			`<https://dev-example.okta.com/api/v1/users?after=P1>; rel="prev", ` +
				`<https://dev-example.okta.com/api/v1/users?after=N1>; rel="next"`,
			"N1",
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cursor, hasNext := pagination.NextCursor(tc.header)
			assert.Equal(t, tc.wantCursor, cursor)
			assert.Equal(t, tc.wantNext, hasNext)
		})
	}
}
