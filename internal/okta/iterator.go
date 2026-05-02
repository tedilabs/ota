package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tedilabs/ota/internal/okta/pagination"
)

// pagedIterator is a shared Iterator[T] implementation that walks Okta list
// endpoints via the Link: rel="next" header. Each page's body is unmarshalled
// into []T via the supplied decode function; pages are fetched lazily on
// demand as Next drains the buffer.
type pagedIterator[T any] struct {
	client   *Client
	nextURL  string
	buffer   []T
	done     bool
	decode   func(raw json.RawMessage) (T, error)
	firstErr error
}

// newPagedIterator builds an iterator from an initial URL. The iterator does
// not fetch until the caller first invokes Next.
func newPagedIterator[T any](cli *Client, initialURL string, decode func(json.RawMessage) (T, error)) *pagedIterator[T] {
	return &pagedIterator[T]{client: cli, nextURL: initialURL, decode: decode}
}

func (it *pagedIterator[T]) Next(ctx context.Context) (T, bool, error) {
	var zero T
	if it.firstErr != nil {
		err := it.firstErr
		it.firstErr = nil
		return zero, false, err
	}
	for len(it.buffer) == 0 {
		if it.done {
			return zero, false, nil
		}
		if err := it.fetch(ctx); err != nil {
			return zero, false, err
		}
	}
	item := it.buffer[0]
	it.buffer = it.buffer[1:]
	return item, true, nil
}

func (it *pagedIterator[T]) Close() error { return nil }

// fetch pulls one more page and appends to the buffer. When no next link is
// returned, sets done.
func (it *pagedIterator[T]) fetch(ctx context.Context) error {
	if it.nextURL == "" {
		it.done = true
		return nil
	}
	resp, err := it.client.doGet(ctx, it.nextURL)
	if err != nil {
		return err
	}
	defer drainAndClose(resp)

	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		return fmt.Errorf("okta: read body: %w", err)
	}

	var raws []json.RawMessage
	if err := json.Unmarshal(body.Bytes(), &raws); err != nil {
		return fmt.Errorf("okta: decode list: %w", err)
	}
	for _, r := range raws {
		v, err := it.decode(r)
		if err != nil {
			return fmt.Errorf("okta: decode element: %w", err)
		}
		it.buffer = append(it.buffer, v)
	}

	// Okta sometimes splits Link into multiple headers (one per
	// rel value) rather than one comma-joined value. http.Header.Get
	// returns only the first, which may miss rel="next" entirely.
	// Join all Link values before parsing.
	linkHdr := joinHeaderValues(resp.Header.Values("Link"))
	cursor, hasNext := pagination.NextCursor(linkHdr)
	if !hasNext {
		it.nextURL = ""
		it.done = true
		return nil
	}

	// Follow the Link header URL, rewriting host to the client's base.
	// Some fixtures encode the cursor directly; honor either form.
	it.nextURL = nextURLFromLinkHeader(it.client, linkHdr, cursor)
	return nil
}

// nextURLFromLinkHeader extracts the next URL from the Link header and
// rewrites its host. Falls back to a cursor-based URL if parsing fails.
func nextURLFromLinkHeader(c *Client, linkHeader, cursor string) string {
	// Re-parse to extract the absolute URL, not just the cursor.
	for _, part := range splitCommaRespectingAngles(linkHeader) {
		segs := splitSemicolons(part)
		if len(segs) < 2 {
			continue
		}
		urlPart := trimAngles(segs[0])
		rel := ""
		for _, s := range segs[1:] {
			if isRelNext(s) {
				rel = s
				break
			}
		}
		if rel == "" {
			continue
		}
		return c.rewriteAbsoluteURL(urlPart)
	}
	// No parseable next URL; the cursor alone isn't enough without knowing
	// the path, so we signal no-more.
	_ = cursor
	return ""
}

// Lightweight local splitters avoid a net/http dependency for parsing beyond
// what pagination.NextCursor already provides.
func splitCommaRespectingAngles(s string) []string {
	var out []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

func splitSemicolons(s string) []string {
	var out []string
	for _, p := range bytes.Split([]byte(s), []byte(";")) {
		out = append(out, string(bytes.TrimSpace(p)))
	}
	return out
}

func trimAngles(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return s
}

func isRelNext(s string) bool {
	s = strings.TrimSpace(s)
	return s == `rel="next"` || s == "rel=next"
}

