package domain

import "context"

// Iterator abstracts paginated traversal over a sequence of T. Implementations
// are expected to fetch pages lazily; callers drive them one item at a time.
//
// Contract:
//   - Next returns (item, true, nil) for each item until exhaustion.
//   - At exhaustion Next returns (zero, false, nil). Subsequent calls remain
//     (zero, false, nil).
//   - A non-nil err implies hasMore=false. Callers must stop.
//   - Close releases resources (HTTP bodies, goroutines). Callers must always
//     invoke Close, even on error.
//   - Next must honor ctx cancellation. A canceled ctx yields
//     (zero, false, ctx.Err()).
type Iterator[T any] interface {
	Next(ctx context.Context) (item T, hasMore bool, err error)
	Close() error
}

// PageInfo carries opaque cursor metadata returned alongside a fetched page.
// The `after` cursor is opaque per Okta conventions (PRD §7.3) — do not parse
// or construct it in ota code.
type PageInfo struct {
	Cursor   string
	Limit    int
	HasMore  bool
}
