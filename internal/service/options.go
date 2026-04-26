package service

import (
	"log/slog"

	"github.com/tedilabs/ota/internal/clock"
)

// ServiceOption tunes optional service dependencies. See docs/CONVENTIONS.md §8.
type ServiceOption func(*serviceOptions)

type serviceOptions struct {
	Logger *slog.Logger
	Clock  clock.Clock
	// CacheTTLSeconds overrides the default 30s TTL (REQ-E01 AC-6).
	CacheTTLSeconds int
}

// WithLogger injects a structured logger.
func WithLogger(l *slog.Logger) ServiceOption {
	return func(o *serviceOptions) { o.Logger = l }
}

// WithClock injects a Clock (use clock.NewFake in tests).
func WithClock(c clock.Clock) ServiceOption {
	return func(o *serviceOptions) { o.Clock = c }
}

// WithCacheTTL overrides the cache TTL in seconds.
func WithCacheTTL(sec int) ServiceOption {
	return func(o *serviceOptions) { o.CacheTTLSeconds = sec }
}

func defaultOptions() serviceOptions {
	return serviceOptions{
		Logger:          slog.Default(),
		Clock:           clock.Real(),
		CacheTTLSeconds: 30,
	}
}

func applyOptions(opts []ServiceOption) serviceOptions {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return o
}
