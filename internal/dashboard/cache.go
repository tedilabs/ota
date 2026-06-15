// Package dashboard hosts the cross-session metrics cache the home
// screen uses for instant first paint. Each card writes its latest
// observed snapshot to <UserCacheDir>/ota/dashboard/<card>.json so a
// freshly-launched ota renders sane numbers in < 100ms while the
// real Okta fetch lands in the background.
//
// Why a separate package: home/home.go is render-only. The cache is
// the durable side, and the (planned) Δ-vs-7d-ago calculations will
// keep a rolling per-day index here too. Keeping disk + format
// concerns out of the TUI package means the cache is testable
// without a tea.Program.
package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Counts captures the headline metrics each list-card surfaces. The
// fields are nullable via *int so a partial fetch (some metrics
// landed, others still pending) can still be cached without
// pretending zero is "we observed zero".
type Counts struct {
	Total              int           `json:"total"`
	ByStatus           map[string]int `json:"by_status,omitempty"`
	BySubtype          map[string]int `json:"by_subtype,omitempty"`           // SAML / OIDC / BOOKMARK / SWA / OKTA_GROUP / APP_GROUP / BUILT_IN …
	ObservedAt         time.Time      `json:"observed_at"`
}

// Snapshot is the full home-screen cache document — one file per
// org, indexed by card key. Adding new cards is a JSON-additive
// change so older ota binaries reading a newer snapshot ignore
// unknown keys.
type Snapshot struct {
	OrgURL string             `json:"org_url"`
	Cards  map[string]Counts  `json:"cards,omitempty"`
	// History stores per-card daily count rolls so we can compute
	// Δ vs N days ago without re-fetching N days of System Logs.
	// Keyed by YYYY-MM-DD (UTC). Phase 3 populates + reads this;
	// Phase 2 just keeps the field for forward-compat.
	History map[string]map[string]int `json:"history,omitempty"`
}

// Cache reads and writes the per-org snapshot file under
// <UserCacheDir>/ota/dashboard/. Safe for concurrent use — the
// home screen reads from one goroutine (View) while card fetchers
// write from background tea.Cmds.
type Cache struct {
	dir     string
	orgURL  string
	disabled bool

	mu   sync.Mutex
	snap Snapshot
	dirty bool
}

// New constructs the cache. Pass the operator's OrgURL so per-tenant
// snapshots don't collide on shared workstations. dir defaults to
// <UserCacheDir>/ota/dashboard when empty; resolution failure falls
// back to a disabled cache (the home screen still works, it just
// doesn't survive a restart).
func New(dir, orgURL string) (*Cache, error) {
	c := &Cache{orgURL: orgURL}
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			c.disabled = true
			return c, fmt.Errorf("dashboard: %w", err)
		}
		dir = filepath.Join(base, "ota", "dashboard")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		c.disabled = true
		return c, fmt.Errorf("dashboard: mkdir %s: %w", dir, err)
	}
	c.dir = dir
	c.snap = Snapshot{OrgURL: orgURL, Cards: map[string]Counts{}, History: map[string]map[string]int{}}
	// Read whatever's already on disk so first-render shows
	// last-session numbers immediately.
	if err := c.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Corrupt file: log and continue with an empty snapshot
		// rather than crash. Next successful write overwrites it.
		_ = err
	}
	return c, nil
}

// Disabled reports whether the cache is a no-op (e.g. cache dir
// unavailable). Callers can still drive the home screen; reads
// just return zero values and writes silently drop.
func (c *Cache) Disabled() bool { return c == nil || c.disabled }

// Get returns the cached counts for `card`. Zero value + false when
// the card hasn't been observed yet.
func (c *Cache) Get(card string) (Counts, bool) {
	if c == nil || c.disabled {
		return Counts{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.snap.Cards[card]
	return v, ok
}

// Snapshot returns a defensive copy of the full snapshot — used by
// Phase 3 to compute Δ from the history field.
func (c *Cache) Snapshot() Snapshot {
	if c == nil || c.disabled {
		return Snapshot{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cards := make(map[string]Counts, len(c.snap.Cards))
	for k, v := range c.snap.Cards {
		cards[k] = v
	}
	hist := make(map[string]map[string]int, len(c.snap.History))
	for day, perCard := range c.snap.History {
		cp := make(map[string]int, len(perCard))
		for k, v := range perCard {
			cp[k] = v
		}
		hist[day] = cp
	}
	return Snapshot{OrgURL: c.snap.OrgURL, Cards: cards, History: hist}
}

// Put writes the latest observed counts for `card` and persists the
// snapshot to disk. Also records the day's total in History so
// Phase 3 has the data needed to compute "+47 vs 7d ago" without
// extra API calls.
func (c *Cache) Put(card string, counts Counts) error {
	if c == nil || c.disabled {
		return nil
	}
	c.mu.Lock()
	if c.snap.Cards == nil {
		c.snap.Cards = map[string]Counts{}
	}
	c.snap.Cards[card] = counts
	if c.snap.History == nil {
		c.snap.History = map[string]map[string]int{}
	}
	day := counts.ObservedAt.UTC().Format("2006-01-02")
	if day == "0001-01-01" {
		day = time.Now().UTC().Format("2006-01-02")
	}
	if _, ok := c.snap.History[day]; !ok {
		c.snap.History[day] = map[string]int{}
	}
	c.snap.History[day][card] = counts.Total
	c.dirty = true
	err := c.flushLocked()
	c.mu.Unlock()
	return err
}

// HistoricalTotal returns the total observed for `card` exactly
// `daysAgo` days before `now`, plus a found flag. Used by Phase 3
// to render "+47 ↑ (7d)" cells.
func (c *Cache) HistoricalTotal(card string, now time.Time, daysAgo int) (int, bool) {
	if c == nil || c.disabled {
		return 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	day := now.UTC().AddDate(0, 0, -daysAgo).Format("2006-01-02")
	per, ok := c.snap.History[day]
	if !ok {
		return 0, false
	}
	v, ok := per[card]
	return v, ok
}

// PruneHistoryOlderThan drops day-rollups older than `cutoff` so
// the snapshot file doesn't grow unbounded. 60 days is plenty for
// Δ-vs-7d + a 30d trend later.
func (c *Cache) PruneHistoryOlderThan(now time.Time, retainDays int) {
	if c == nil || c.disabled || retainDays <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := now.UTC().AddDate(0, 0, -retainDays)
	for day := range c.snap.History {
		t, err := time.Parse("2006-01-02", day)
		if err != nil || t.Before(cutoff) {
			delete(c.snap.History, day)
			c.dirty = true
		}
	}
	if c.dirty {
		_ = c.flushLocked()
	}
}

func (c *Cache) load() error {
	path := filepath.Join(c.dir, snapshotFileName(c.orgURL))
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var s Snapshot
	if err := json.Unmarshal(buf, &s); err != nil {
		return err
	}
	if s.Cards == nil {
		s.Cards = map[string]Counts{}
	}
	if s.History == nil {
		s.History = map[string]map[string]int{}
	}
	c.snap = s
	return nil
}

// flushLocked persists the current snapshot. Caller must hold c.mu.
func (c *Cache) flushLocked() error {
	if !c.dirty {
		return nil
	}
	path := filepath.Join(c.dir, snapshotFileName(c.orgURL))
	buf, err := json.MarshalIndent(c.snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		return err
	}
	c.dirty = false
	return nil
}

// snapshotFileName derives a filesystem-safe basename from the
// OrgURL so multi-tenant operators can have independent caches.
// "https://acme.okta.com" → "acme.okta.com.json"; empty falls back
// to "snapshot.json".
func snapshotFileName(orgURL string) string {
	s := orgURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '/' || s[i] == '\\' || s[i] == ':' {
			s = s[:i]
			break
		}
	}
	if s == "" {
		return "snapshot.json"
	}
	return s + ".json"
}

// Card identifiers — keep these stable; they're the cache keys
// persisted to disk. Adding a new card is JSON-additive (old
// binaries ignore unknown keys), but renaming one orphans the
// history.
const (
	CardUsers          = "users"
	CardGroups         = "groups"
	CardApps           = "apps"
	CardGroupRules     = "group_rules"
	CardPolicies       = "policies"
	CardAuthenticators = "authenticators"
	CardAdmins         = "admins"
	CardAPITokens      = "api_tokens"
)
