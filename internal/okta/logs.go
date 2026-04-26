package okta

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/tedilabs/ota/internal/domain"
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
