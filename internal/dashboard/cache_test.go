package dashboard_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/dashboard"
)

func Test_Cache_PutGet_RoundTripsCountsViaDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	c1, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	require.NoError(t, c1.Put(dashboard.CardUsers, dashboard.Counts{
		Total:      12438,
		ByStatus:   map[string]int{"ACTIVE": 11802, "SUSPENDED": 412, "LOCKED_OUT": 94},
		ObservedAt: now,
	}))

	// Drop the in-memory cache by re-opening — proves the snapshot
	// survives the process restart.
	c2, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)

	got, ok := c2.Get(dashboard.CardUsers)
	require.True(t, ok, "Get must hit the persisted snapshot after re-open")
	assert.Equal(t, 12438, got.Total)
	assert.Equal(t, 11802, got.ByStatus["ACTIVE"])
}

func Test_Cache_PerOrgFiles_DontCollide(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	acme, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)
	require.NoError(t, acme.Put(dashboard.CardUsers, dashboard.Counts{Total: 100, ObservedAt: time.Now()}))

	other, err := dashboard.New(dir, "https://other.okta.com")
	require.NoError(t, err)
	got, ok := other.Get(dashboard.CardUsers)
	assert.False(t, ok, "second org must NOT see the first org's snapshot")
	assert.Zero(t, got.Total)
}

func Test_Cache_HistoricalTotal_ResolvesFromHistoryMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)

	day0 := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	day7 := day0.AddDate(0, 0, -7)

	// Seed two daily rolls 7 days apart.
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 12000, ObservedAt: day7}))
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 12438, ObservedAt: day0}))

	got, ok := c.HistoricalTotal(dashboard.CardUsers, day0, 7)
	require.True(t, ok)
	assert.Equal(t, 12000, got, "HistoricalTotal must read the day-7 roll, not the latest")
}

func Test_Cache_PruneHistoryOlderThan_DropsStaleDays(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)

	now := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	// Seed two old days (90 + 100 days ago) and one recent.
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 1, ObservedAt: now.AddDate(0, 0, -100)}))
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 2, ObservedAt: now.AddDate(0, 0, -90)}))
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 12438, ObservedAt: now}))

	c.PruneHistoryOlderThan(now, 60)
	snap := c.Snapshot()

	_, has100 := snap.History[now.AddDate(0, 0, -100).Format("2006-01-02")]
	_, has90 := snap.History[now.AddDate(0, 0, -90).Format("2006-01-02")]
	_, hasToday := snap.History[now.Format("2006-01-02")]
	assert.False(t, has100, "100-days-ago history must be pruned")
	assert.False(t, has90, "90-days-ago history must be pruned")
	assert.True(t, hasToday, "today's history must survive pruning")
}

func Test_Cache_DisabledOnUnwritableDir_DoesntPanic(t *testing.T) {
	t.Parallel()
	// /dev/null/something is a write that always fails on POSIX —
	// proves the cache degrades gracefully instead of panicking.
	c, err := dashboard.New(filepath.Join("/dev/null", "ota-dashboard"), "https://acme.okta.com")
	assert.True(t, c.Disabled() || err != nil,
		"unwritable cache dir must produce a disabled cache (no panic)")
	// Disabled cache still answers Get/Put with zero-values + nil err.
	_, ok := c.Get(dashboard.CardUsers)
	assert.False(t, ok)
	assert.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 1, ObservedAt: time.Now()}),
		"Put on disabled cache must NOT error")
}
