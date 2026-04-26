package pagination

import "strings"

// NextCursor extracts the value of the `after` query parameter from a
// Link-header URL with rel="next". Returns "" when no next link is present.
//
// Okta's Link header format:
//
//	Link: <https://example.okta.com/api/v1/users?limit=200&after=CURSOR>; rel="next",
//	      <https://example.okta.com/api/v1/users?limit=200>; rel="self"
//
// The `after` cursor is opaque (PRD §7.3). This parser preserves it verbatim
// — no URL decoding — so opaque tokens such as base64 values round-trip.
func NextCursor(linkHeader string) (cursor string, hasNext bool) {
	if linkHeader == "" {
		return "", false
	}
	for _, part := range strings.Split(linkHeader, ",") {
		segments := strings.Split(strings.TrimSpace(part), ";")
		if len(segments) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(segments[0])
		relPart := ""
		for _, s := range segments[1:] {
			s = strings.TrimSpace(s)
			if strings.HasPrefix(s, "rel=") {
				relPart = s
				break
			}
		}
		if !isNextRel(relPart) {
			continue
		}
		raw := stripAngleBrackets(urlPart)
		return extractAfter(raw), true
	}
	return "", false
}

func isNextRel(rel string) bool {
	return rel == `rel="next"` || rel == "rel=next"
}

func stripAngleBrackets(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return s
}

// extractAfter finds "after=<cursor>" anywhere in the URL's query. Values are
// returned verbatim (no URL decoding) — the cursor is opaque per Okta.
func extractAfter(rawURL string) string {
	q := rawURL
	if i := strings.IndexByte(q, '?'); i >= 0 {
		q = q[i+1:]
	}
	for _, kv := range strings.Split(q, "&") {
		const key = "after="
		if strings.HasPrefix(kv, key) {
			return kv[len(key):]
		}
	}
	return ""
}
