package testfx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Scenario mirrors testdata/scenarios/<name>.json.
type Scenario struct {
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	RequestCoverage []string         `json:"req_id_coverage"`
	Steps           []ScenarioStep   `json:"steps"`
}

// ScenarioStep is one request/response pair.
type ScenarioStep struct {
	Request  ScenarioRequest  `json:"request"`
	Response ScenarioResponse `json:"response"`
}

// ScenarioRequest is what the scenario expects the caller to send.
type ScenarioRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
}

// ScenarioResponse is the scripted reply.
type ScenarioResponse struct {
	Status      int               `json:"status"`
	Headers     map[string]string `json:"headers"`
	BodyFixture string            `json:"body_fixture"` // relative to testdata/
	BodyInline  json.RawMessage   `json:"body_inline"`  // optional, overrides BodyFixture if present
}

// LoadScenario reads and parses a scenario bundle by name (without .json).
func LoadScenario(t *testing.T, name string) *Scenario {
	t.Helper()
	p := filepath.Join(TestdataRoot(t), "scenarios", name+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("testfx: load scenario %q: %v", name, err)
	}
	var s Scenario
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("testfx: parse scenario %q: %v", name, err)
	}
	return &s
}

// FakeServer is a minimal scripted server.
//
// It advances through scenario.Steps in order. Each request consumes the next
// step. Unexpected extra requests fail the test via t.Errorf.
type FakeServer struct {
	*httptest.Server

	t   *testing.T
	mu  sync.Mutex
	pos int
	s   *Scenario
}

// NewFakeOktaServer spins up an httptest.Server scripted from the named
// scenario. Callers should defer srv.Close().
func NewFakeOktaServer(t *testing.T, scenarioName string) *FakeServer {
	t.Helper()
	sc := LoadScenario(t, scenarioName)
	fs := &FakeServer{t: t, s: sc}
	fs.Server = httptest.NewServer(http.HandlerFunc(fs.handle))
	t.Cleanup(func() { fs.Close() })
	return fs
}

func (f *FakeServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.pos >= len(f.s.Steps) {
		f.t.Errorf("testfx: unexpected extra request to %s %s (scenario %s already drained)",
			r.Method, r.URL.Path, f.s.Name)
		http.Error(w, "scenario drained", http.StatusInternalServerError)
		return
	}
	step := f.s.Steps[f.pos]
	f.pos++

	if strings.ToUpper(step.Request.Method) != strings.ToUpper(r.Method) {
		f.t.Errorf("testfx: scenario %s step %d: expected method %s, got %s",
			f.s.Name, f.pos-1, step.Request.Method, r.Method)
	}
	if step.Request.Path != "" && step.Request.Path != r.URL.Path {
		f.t.Errorf("testfx: scenario %s step %d: expected path %s, got %s",
			f.s.Name, f.pos-1, step.Request.Path, r.URL.Path)
	}
	// Query is advisory — we don't enforce equality because ordering of params
	// can vary; tests that care can use RequestsSeen().

	resp := step.Response
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	if resp.Status == 0 {
		resp.Status = http.StatusOK
	}
	w.WriteHeader(resp.Status)

	body := resp.BodyInline
	if len(body) == 0 && resp.BodyFixture != "" {
		body = LoadFixture(f.t, resp.BodyFixture)
	}
	if len(body) > 0 {
		if _, err := io.Copy(w, bytes.NewReader(body)); err != nil {
			f.t.Errorf("testfx: write body: %v", err)
		}
	}
}

// StepsServed reports how many scenario steps the server has consumed. Useful
// for retry-count assertions (REQ-E01 AC-2 max 3 retries, etc.).
func (f *FakeServer) StepsServed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pos
}
