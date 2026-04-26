package service

import (
	"context"
	"strconv"

	"github.com/tedilabs/ota/internal/domain"
)

// drainIterator materializes an Iterator into a slice, closing it on return.
func drainIterator[T any](ctx context.Context, iter domain.Iterator[T]) ([]T, error) {
	defer func() { _ = iter.Close() }()
	var out []T
	for {
		v, hasMore, err := iter.Next(ctx)
		if err != nil {
			return nil, err
		}
		if !hasMore {
			return out, nil
		}
		out = append(out, v)
	}
}

// sliceIterator is a re-playable iterator over an in-memory slice. Used to
// return cached service results as Iterator[T] without re-hitting the port.
type sliceIterator[T any] struct {
	items []T
	pos   int
}

func newSliceIterator[T any](items []T) domain.Iterator[T] {
	return &sliceIterator[T]{items: items}
}

func (s *sliceIterator[T]) Next(ctx context.Context) (T, bool, error) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, false, ctx.Err()
	default:
	}
	if s.pos >= len(s.items) {
		var zero T
		return zero, false, nil
	}
	v := s.items[s.pos]
	s.pos++
	return v, true, nil
}

func (s *sliceIterator[T]) Close() error { return nil }

func itoa(n int) string { return strconv.Itoa(n) }
