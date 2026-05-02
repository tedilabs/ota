// Package apilog records every Okta HTTP round-trip the app issues
// to a per-day NDJSON file under the user's cache directory and to
// an in-memory ring buffer. The TUI reads the ring directly for the
// API timeline overlay; the on-disk log is rotated daily and
// retained for 3 days.
//
// Sensitive fields are redacted at write time:
//   - Authorization / Cookie / Set-Cookie / Proxy-Authorization
//     headers are replaced with "***"
//   - JSON bodies pass through RedactJSONBody which masks known PII
//     keys (mobilePhone, primaryPhone, secondEmail, postalAddress,
//     streetAddress, zipCode, …) before write
//   - Bodies are hard-capped at MaxBodyBytes (64 KiB) so a large
//     /api/v1/logs response can't blow the cache up
package apilog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MaxBodyBytes caps each captured request / response body so a huge
// /api/v1/logs response can't bloat the on-disk log.
const MaxBodyBytes = 64 * 1024

// RetentionDays is how long rotated log files are kept on disk.
const RetentionDays = 3

// DefaultRingSize bounds the in-memory entry ring used by the
// timeline overlay. Older entries paginate from disk on demand.
const DefaultRingSize = 500

// Entry is one captured Okta round-trip.
type Entry struct {
	SeqID           uint64      `json:"seq"`
	Time            time.Time   `json:"ts"`
	Method          string      `json:"method"`
	URL             string      `json:"url"`
	Path            string      `json:"path"`
	Status          int         `json:"status"`
	DurationMS      int64       `json:"duration_ms"`
	RequestHeaders  http.Header `json:"request_headers,omitempty"`
	RequestBody     string      `json:"request_body,omitempty"`
	ResponseHeaders http.Header `json:"response_headers,omitempty"`
	ResponseBody    string      `json:"response_body,omitempty"`
	Err             string      `json:"error,omitempty"`
}

// Recorder is the central capture point. Safe for concurrent calls.
type Recorder struct {
	dir      string
	ringSize int

	mu      sync.Mutex
	ring    []Entry
	head    int  // next slot to write
	full    bool // ring has wrapped at least once
	seq     atomic.Uint64
	file    *os.File
	writer  *bufio.Writer
	fileDay string
	disabled bool
}

// New constructs a Recorder rooted at dir. ringSize<=0 falls back to
// DefaultRingSize. dir is created with 0700 permissions; old logs
// (older than RetentionDays days) are pruned before the recorder
// returns. If dir creation fails, a disabled Recorder is returned —
// callers can still use the round-tripper (calls are no-ops).
func New(dir string, ringSize int) (*Recorder, error) {
	return NewWithClock(dir, ringSize, time.Now())
}

// NewWithClock is the now-injecting variant — used by tests that
// pin pruning to a fixture date.
func NewWithClock(dir string, ringSize int, now time.Time) (*Recorder, error) {
	if ringSize <= 0 {
		ringSize = DefaultRingSize
	}
	r := &Recorder{
		dir:      dir,
		ringSize: ringSize,
		ring:     make([]Entry, ringSize),
	}
	if dir == "" {
		r.disabled = true
		return r, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		r.disabled = true
		return r, fmt.Errorf("apilog: mkdir %s: %w", dir, err)
	}
	if err := pruneOlderThan(dir, now, RetentionDays); err != nil {
		// Pruning failure is not fatal — log and keep going.
		_ = err
	}
	return r, nil
}

// Disabled reports whether this Recorder is a no-op (e.g., cache
// directory unavailable).
func (r *Recorder) Disabled() bool { return r == nil || r.disabled }

// Record stamps a SeqID + appends to the ring + writes the NDJSON
// line to today's log file. Sensitive fields are already redacted by
// the transport before they reach Record — the recorder treats the
// entry as final.
func (r *Recorder) Record(e Entry) {
	if r == nil || r.disabled {
		return
	}
	e.SeqID = r.seq.Add(1)
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ring[r.head] = e
	r.head++
	if r.head >= r.ringSize {
		r.head = 0
		r.full = true
	}

	if err := r.writeLocked(e); err != nil {
		// Disk write failure shouldn't take the app down — drop the
		// disk side, keep recording in-memory. A future call will
		// retry the rotation.
		if r.file != nil {
			_ = r.file.Close()
			r.file = nil
			r.writer = nil
		}
	}
}

// Snapshot returns the ring contents in chronological order (oldest
// first). The returned slice is safe to retain; entries are values.
func (r *Recorder) Snapshot() []Entry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.full {
		out := make([]Entry, r.head)
		copy(out, r.ring[:r.head])
		return out
	}
	out := make([]Entry, r.ringSize)
	copy(out, r.ring[r.head:])
	copy(out[r.ringSize-r.head:], r.ring[:r.head])
	return out
}

// Close flushes any buffered writer and closes the open log file.
func (r *Recorder) Close() error {
	if r == nil || r.disabled {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer != nil {
		_ = r.writer.Flush()
		r.writer = nil
	}
	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}

// writeLocked appends e as one NDJSON line to today's file. Caller
// holds r.mu.
func (r *Recorder) writeLocked(e Entry) error {
	day := e.Time.UTC().Format("2006-01-02")
	if r.file == nil || r.fileDay != day {
		if r.writer != nil {
			_ = r.writer.Flush()
			r.writer = nil
		}
		if r.file != nil {
			_ = r.file.Close()
			r.file = nil
		}
		path := filepath.Join(r.dir, "api-"+day+".ndjson")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		r.file = f
		r.writer = bufio.NewWriter(f)
		r.fileDay = day
	}
	buf, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := r.writer.Write(buf); err != nil {
		return err
	}
	if err := r.writer.WriteByte('\n'); err != nil {
		return err
	}
	return r.writer.Flush()
}

// pruneOlderThan deletes api-*.ndjson files in dir whose date stamp
// is more than retentionDays days before now.
func pruneOlderThan(dir string, now time.Time, retentionDays int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays)
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, "api-") || !strings.HasSuffix(name, ".ndjson") {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, "api-"), ".ndjson")
		t, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	return nil
}
