package clock

import "time"

// PercentJitter applies up to ±pct jitter to a base duration. The random
// source is injected so production and tests can differ.
type PercentJitter struct {
	Pct int            // e.g., 20 for ±20%
	Rnd func() float64 // returns value in [0.0, 1.0); production uses math/rand/v2
}

// Apply returns a duration in [base*(1-pct/100), base*(1+pct/100)]. Negative
// or zero base is returned unchanged. When Pct is <=0 or Rnd is nil, base is
// returned unchanged so tests that don't care about jitter stay predictable.
func (p *PercentJitter) Apply(base time.Duration) time.Duration {
	if base <= 0 || p.Pct <= 0 || p.Rnd == nil {
		return base
	}
	// map r ∈ [0,1) to offset ∈ [-pct, +pct]
	r := p.Rnd()
	offset := (2*r - 1) * float64(p.Pct) / 100.0
	return base + time.Duration(offset*float64(base))
}

var _ Jitter = (*PercentJitter)(nil)
