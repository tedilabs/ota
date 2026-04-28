package logs

// v0.1.17 (#179): the follow-mode auto-refresh dedupes appended
// events by UUID. Without this guard, a tick that races with a
// history reload (range key, `r`, or initial Init batch) re-emits
// rows the user already saw — the dupe complaint operators reported
// after v0.1.16 shipped.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/domain"
)

// Test_Logs_Follow_DedupesByUUID seeds the model with three events,
// then dispatches a followFetchedMsg whose batch overlaps two of the
// existing UUIDs and adds one new one. After Update the event slice
// must contain exactly four rows — no UUID duplicates.
func Test_Logs_Follow_DedupesByUUID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	existing := []domain.LogEvent{
		{UUID: "uuid-1", Published: now.Add(-3 * time.Second)},
		{UUID: "uuid-2", Published: now.Add(-2 * time.Second)},
		{UUID: "uuid-3", Published: now.Add(-1 * time.Second)},
	}
	m := SearchModel{
		events:    existing,
		follow:    true,
		followGen: 0,
	}

	tickResult := followFetchedMsg{
		gen: 0,
		events: []domain.LogEvent{
			{UUID: "uuid-2", Published: now.Add(-2 * time.Second)}, // dupe
			{UUID: "uuid-3", Published: now.Add(-1 * time.Second)}, // dupe
			{UUID: "uuid-4", Published: now.Add(1 * time.Second)},  // new
		},
		nextSince: now.Add(2 * time.Second),
		at:        now,
	}

	upd, _ := m.Update(tickResult)
	got, ok := upd.(SearchModel)
	if !ok {
		t.Fatalf("Update must return a SearchModel")
	}

	assert.Len(t, got.events, 4,
		"events must be exactly seed (3) + new (1); dupes filtered")
	assert.Equal(t, "uuid-4", got.events[3].UUID,
		"only the strictly-new uuid-4 must land in the appended slice")
	assert.Equal(t, tickResult.nextSince, got.followSince,
		"followSince must advance to the tick's nextSince")
	assert.Equal(t, tickResult.at, got.lastUpdated,
		"lastUpdated must reflect the tick's wall-clock at field")
}

// Test_Logs_Follow_AppendsAllWhenNoOverlap confirms the dedupe path
// doesn't accidentally drop events when none of the UUIDs overlap.
func Test_Logs_Follow_AppendsAllWhenNoOverlap(t *testing.T) {
	t.Parallel()

	now := time.Now()
	m := SearchModel{
		events: []domain.LogEvent{
			{UUID: "uuid-a", Published: now.Add(-1 * time.Second)},
		},
		follow:    true,
		followGen: 0,
	}

	tick := followFetchedMsg{
		gen: 0,
		events: []domain.LogEvent{
			{UUID: "uuid-b", Published: now.Add(1 * time.Second)},
			{UUID: "uuid-c", Published: now.Add(2 * time.Second)},
		},
		nextSince: now.Add(3 * time.Second),
		at:        now,
	}

	upd, _ := m.Update(tick)
	got := upd.(SearchModel)
	assert.Len(t, got.events, 3,
		"non-overlapping batch must be appended in full")
}
