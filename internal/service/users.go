package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
)

// UsersQuery is the service-level Users query alias.
type UsersQuery = domain.UsersQuery

// UsersService orchestrates Users use cases (REQ-R01, REQ-U04). Caches results
// with a 30s TTL and delegates I/O to domain.UsersPort.
type UsersService struct {
	port  domain.UsersPort
	log   *slog.Logger
	clock clock.Clock
	ttl   time.Duration

	mu    sync.Mutex
	cache map[string]usersCacheEntry
}

type usersCacheEntry struct {
	items    []domain.User
	storedAt time.Time
}

// NewUsersService constructs a UsersService.
func NewUsersService(port domain.UsersPort, opts ...ServiceOption) *UsersService {
	o := applyOptions(opts)
	return &UsersService{
		port:  port,
		log:   o.Logger,
		clock: o.Clock,
		ttl:   time.Duration(o.CacheTTLSeconds) * time.Second,
		cache: map[string]usersCacheEntry{},
	}
}

// Search returns a user iterator matching q. Results are cached by query key
// with a TTL (REQ-E01 AC-6). Subsequent calls within TTL skip port.List and
// replay the cached slice via a fresh iterator.
func (s *UsersService) Search(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
	key := usersCacheKey(q)

	s.mu.Lock()
	if entry, ok := s.cache[key]; ok {
		if s.ttl > 0 && s.clock.Now().Sub(entry.storedAt) < s.ttl {
			items := entry.items
			s.mu.Unlock()
			return newSliceIterator(items), nil
		}
		delete(s.cache, key)
	}
	s.mu.Unlock()

	iter, err := s.port.List(ctx, q)
	if err != nil {
		return nil, err
	}
	items, err := drainIterator(ctx, iter)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[key] = usersCacheEntry{items: items, storedAt: s.clock.Now()}
	s.mu.Unlock()

	return newSliceIterator(items), nil
}

// Get fetches a single user by id or login.
func (s *UsersService) Get(ctx context.Context, idOrLogin string) (domain.User, error) {
	return s.port.Get(ctx, idOrLogin)
}

// Groups returns the groups a user belongs to.
func (s *UsersService) Groups(ctx context.Context, userID string) ([]domain.Group, error) {
	return s.port.ListGroups(ctx, userID)
}

// Factors returns the user's registered MFA factors (REQ-R01 AC-6).
func (s *UsersService) Factors(ctx context.Context, userID string) ([]domain.Factor, error) {
	return s.port.ListFactors(ctx, userID)
}

// Invalidate clears the cache (used by `:refresh` and `:profile`).
func (s *UsersService) Invalidate() {
	s.mu.Lock()
	s.cache = map[string]usersCacheEntry{}
	s.mu.Unlock()
}

// usersCacheKey builds a deterministic key from the query shape.
func usersCacheKey(q domain.UsersQuery) string {
	return "q=" + q.Q + "&s=" + q.Search + "&f=" + q.Filter + "&l=" + itoa(q.Limit) + "&a=" + q.After
}
