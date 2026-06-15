package dashboard_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/dashboard"
)

func Test_DeltaFor_ComputesDiffAndPct(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 12000, ObservedAt: now.AddDate(0, 0, -7)}))
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 12438, ObservedAt: now}))

	d := dashboard.DeltaFor(c, dashboard.CardUsers, now, 7)
	require.True(t, d.Compared, "7d-ago roll exists; Δ must be marked Compared")
	assert.Equal(t, 12438, d.Current)
	assert.Equal(t, 12000, d.Previous)
	assert.Equal(t, 438, d.Diff)
	assert.InDelta(t, 3.65, d.Pct, 0.01)
}

func Test_DeltaFor_NoHistory_ReportsUncompared(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 12438, ObservedAt: now}))

	d := dashboard.DeltaFor(c, dashboard.CardUsers, now, 7)
	assert.False(t, d.Compared, "no 7d-ago roll → Compared must be false")
	assert.Equal(t, 12438, d.Current,
		"Current still resolves from today's roll even when Compared is false")
}

func Test_DeltaFor_MissedDay_BackfillsFromClosestRoll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	// 8-day-ago roll exists (operator skipped exactly 7-day-ago).
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 11900, ObservedAt: now.AddDate(0, 0, -8)}))
	require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{Total: 12438, ObservedAt: now}))

	d := dashboard.DeltaFor(c, dashboard.CardUsers, now, 7)
	require.True(t, d.Compared, "must walk back to find a usable roll")
	assert.Equal(t, 11900, d.Previous)
	assert.Equal(t, 538, d.Diff)
}

func Test_DeltaFor_Sparkline_NormalizesToBlockRange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := dashboard.New(dir, "https://acme.okta.com")
	require.NoError(t, err)

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	// Seed 14 days of monotonically increasing rolls.
	for i := 13; i >= 0; i-- {
		require.NoError(t, c.Put(dashboard.CardUsers, dashboard.Counts{
			Total:      12000 + (13-i)*40,
			ObservedAt: now.AddDate(0, 0, -i),
		}))
	}

	d := dashboard.DeltaFor(c, dashboard.CardUsers, now, 7)
	require.NotEmpty(t, d.Sparkline, "Sparkline must be populated for a 14-day history")
	assert.Equal(t, 0, d.Sparkline[0], "oldest entry → bucket 0")
	assert.Equal(t, 8, d.Sparkline[len(d.Sparkline)-1],
		"newest entry → top bucket")
}

func Test_RenderSparkline_StampsBlockRamp(t *testing.T) {
	t.Parallel()
	got := dashboard.RenderSparkline([]int{0, 2, 4, 6, 8})
	// Each bucket maps to its glyph in the canonical 9-step ramp.
	for i, want := range []string{" ", "▂", "▄", "▆", "█"} {
		assert.True(t, strings.Contains(got, want),
			"sparkline must include %q at bucket %d (got %q)", want, i, got)
	}
}

func Test_NormalizeSparkline_FlatSeries_RendersMidHeight(t *testing.T) {
	t.Parallel()
	// All-equal input shouldn't produce a blank line — the operator
	// would read "no data" when really the data IS there and stable.
	got := dashboard.NormalizeSparkline([]int{5, 5, 5, 5})
	for _, b := range got {
		assert.Equal(t, 4, b,
			"flat series must collapse to mid-height bucket so the line stays visible")
	}
}
