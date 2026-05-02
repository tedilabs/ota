package okta

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta/pagination"
)

// LogsAdapter implements domain.LogsPort.
type LogsAdapter struct{ client *Client }

// Search issues a /api/v1/logs query. Returns an iterator that pages through
// results via Link-header (REQ-R05). Hole-free tail resume is handled by the
// service layer (LogsTail.NextSinceAfter).
func (a *LogsAdapter) Search(ctx context.Context, q domain.LogsQuery) (domain.Iterator[domain.LogEvent], error) {
	initial := a.client.buildURL("/api/v1/logs" + buildLogsQuery(q))
	decode := func(raw json.RawMessage) (domain.LogEvent, error) {
		var we wireLogEvent
		if err := json.Unmarshal(raw, &we); err != nil {
			return domain.LogEvent{}, err
		}
		return mapLogEvent(&we, raw), nil
	}
	return newPagedIterator(a.client, initial, decode), nil
}

// SearchPage issues one /api/v1/logs query and returns the parsed
// events plus the next-page cursor extracted from the Link: rel="next"
// header (#F3 v0.2.5). Used by History mode so operators advance
// pagination explicitly via Enter on the "load older" sentinel
// instead of the iterator silently fanning out every page.
func (a *LogsAdapter) SearchPage(ctx context.Context, q domain.LogsQuery) (domain.LogPage, error) {
	u := a.client.buildURL("/api/v1/logs" + buildLogsQuery(q))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.LogPage{}, err
	}
	defer drainAndClose(resp)
	var raws []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raws); err != nil {
		return domain.LogPage{}, err
	}
	out := make([]domain.LogEvent, 0, len(raws))
	for _, r := range raws {
		var we wireLogEvent
		if err := json.Unmarshal(r, &we); err != nil {
			return domain.LogPage{}, err
		}
		out = append(out, mapLogEvent(&we, r))
	}
	// Okta can split the Link header into multiple lines (rel=self
	// and rel=next as separate `Link:` entries) instead of one
	// comma-joined header. http.Header.Get returns only the first,
	// which may be "self" — missing the "next" cursor entirely.
	// Join all Link values so NextCursor sees both rels.
	cursor, _ := pagination.NextCursor(joinHeaderValues(resp.Header.Values("Link")))
	return domain.LogPage{Events: out, After: cursor}, nil
}

// joinHeaderValues concatenates every value of a multi-valued HTTP
// header into the comma-separated form RFC 7230 allows so a parser
// expecting one combined string sees every entry. Returns "" when
// the header is absent.
func joinHeaderValues(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	if len(vs) == 1 {
		return vs[0]
	}
	return strings.Join(vs, ", ")
}

func buildLogsQuery(q domain.LogsQuery) string {
	v := url.Values{}
	if q.Since != nil {
		v.Set("since", formatLogsTime(*q.Since))
	}
	if q.Until != nil {
		v.Set("until", formatLogsTime(*q.Until))
	}
	if q.Filter != "" {
		v.Set("filter", q.Filter)
	}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.SortOrder != "" {
		v.Set("sortOrder", string(q.SortOrder))
	}
	limit := q.Limit
	if limit == 0 {
		limit = 1000
	}
	v.Set("limit", strconv.Itoa(limit))
	if q.After != "" {
		v.Set("after", q.After)
	}
	return "?" + v.Encode()
}

// formatLogsTime matches Okta's accepted input: seconds-resolution
// (RFC3339) when the timestamp has no sub-second component; millisecond
// precision ("YYYY-MM-DDTHH:MM:SS.sssZ") otherwise.
func formatLogsTime(t time.Time) string {
	t = t.UTC()
	if t.Nanosecond() == 0 {
		return t.Format("2006-01-02T15:04:05Z")
	}
	return t.Format("2006-01-02T15:04:05.000Z")
}

var _ domain.LogsPort = (*LogsAdapter)(nil)
