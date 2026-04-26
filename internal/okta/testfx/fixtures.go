package testfx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// FixtureMeta mirrors the <file>.meta.json sidecars that accompany each
// testdata/oktaapi/ JSON body.
type FixtureMeta struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

// TestdataRoot returns an absolute path to the repository's testdata/
// directory, independent of the working directory of the invoking _test.go
// (which differs per package).
func TestdataRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("testfx: runtime.Caller failed")
	}
	// this file: <repo>/internal/okta/testfx/fixtures.go → repo root is ../../..
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	td := filepath.Join(root, "testdata")
	if _, err := os.Stat(td); err != nil {
		t.Fatalf("testfx: testdata not found at %s: %v", td, err)
	}
	return td
}

// LoadFixture reads a JSON body under testdata/.
// Path is slash-separated relative to testdata/, e.g. "oktaapi/users/list_page1.json".
func LoadFixture(t *testing.T, relPath string) []byte {
	t.Helper()
	p := filepath.Join(TestdataRoot(t), filepath.FromSlash(relPath))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("testfx: load fixture %q: %v", relPath, err)
	}
	return b
}

// LoadFixtureMeta reads the .meta.json sidecar for a given body fixture.
// bodyRelPath should end in ".json"; the sidecar is "<base>.meta.json".
// If the sidecar is missing, defaults to {Status: 200, Headers: {"Content-Type": "application/json"}}.
func LoadFixtureMeta(t *testing.T, bodyRelPath string) FixtureMeta {
	t.Helper()
	if !strings.HasSuffix(bodyRelPath, ".json") {
		t.Fatalf("testfx: meta path requires .json suffix, got %q", bodyRelPath)
	}
	metaRel := strings.TrimSuffix(bodyRelPath, ".json") + ".meta.json"
	full := filepath.Join(TestdataRoot(t), filepath.FromSlash(metaRel))
	b, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return FixtureMeta{Status: 200, Headers: map[string]string{"Content-Type": "application/json"}}
		}
		t.Fatalf("testfx: load meta %q: %v", metaRel, err)
	}
	var m FixtureMeta
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("testfx: parse meta %q: %v", metaRel, err)
	}
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	if _, ok := m.Headers["Content-Type"]; !ok {
		m.Headers["Content-Type"] = "application/json"
	}
	return m
}

// LoadHTTPResponse reconstructs an *http.Response from a body fixture and its
// meta sidecar. Useful for unit-testing errormap without a server.
func LoadHTTPResponse(t *testing.T, bodyRelPath string) *http.Response {
	t.Helper()
	body := LoadFixture(t, bodyRelPath)
	meta := LoadFixtureMeta(t, bodyRelPath)

	hdr := http.Header{}
	for k, v := range meta.Headers {
		hdr.Set(k, v)
	}
	if hdr.Get("Content-Length") == "" {
		hdr.Set("Content-Length", strconv.Itoa(len(body)))
	}

	return &http.Response{
		Status:     http.StatusText(meta.Status),
		StatusCode: meta.Status,
		Header:     hdr,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    nil,
	}
}
