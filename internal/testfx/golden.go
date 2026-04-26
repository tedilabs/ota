package testfx

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGoldens controls whether AssertGolden rewrites the golden file
// instead of asserting against it. Activated by `go test -update ./...`.
//
// Defined as a package-level var with flag.Bool so any test binary that
// imports testfx exposes the same -update flag. Only one definition is
// allowed per binary; we guard against duplicate registration in tests that
// also import another package using the same flag name by checking
// flag.Lookup first.
var updateGoldens = registerUpdateFlag()

// updateFlagAlias proxies flag.Lookup("update") so the value is re-read on
// every check rather than captured at registration time. This matters when
// charmbracelet/x/exp/golden registers the flag first and we cannot own its
// *bool — we need to observe runtime state, not the init-time snapshot.
type updateFlagAlias struct{ name string }

func (u *updateFlagAlias) value() bool {
	if u == nil {
		return false
	}
	if f := flag.Lookup(u.name); f != nil {
		return f.Value.String() == "true"
	}
	return false
}

func registerUpdateFlag() *updateFlagAlias {
	if flag.Lookup("update") == nil {
		flag.Bool("update", false, "rewrite golden files instead of asserting")
	}
	return &updateFlagAlias{name: "update"}
}

// AssertGolden compares got to the file at relPath (relative to the test
// package directory). When -update is set, the file is rewritten with got's
// contents and the directories are created on demand.
//
// got is normalized: trailing whitespace on each line is preserved (column
// alignment matters), but the comparison strips a single optional trailing
// newline from both sides so files committed with editors that auto-append
// don't trigger spurious diffs.
func AssertGolden(t *testing.T, got, relPath string) {
	t.Helper()

	if updateGoldens.value() {
		if err := os.MkdirAll(filepath.Dir(relPath), 0o755); err != nil {
			t.Fatalf("AssertGolden: mkdir %s: %v", filepath.Dir(relPath), err)
		}
		if err := os.WriteFile(relPath, []byte(got), 0o644); err != nil {
			t.Fatalf("AssertGolden: write %s: %v", relPath, err)
		}
		return
	}

	want, err := os.ReadFile(relPath)
	if err != nil {
		t.Fatalf("AssertGolden: read %s: %v\n\n(re-run with `go test -update` to create)\n\n--- got ---\n%s",
			relPath, err, got)
	}

	if normalize(string(want)) != normalize(got) {
		t.Fatalf("AssertGolden: %s mismatch.\n\n--- want ---\n%s\n--- got ---\n%s\n--- diff hint ---\n%s",
			relPath, string(want), got, firstDiff(string(want), got))
	}
}

func normalize(s string) string {
	return strings.TrimRight(s, "\n")
}

// firstDiff returns a short single-line snippet pointing at the first
// differing line — enough for a developer to see what changed without a
// full diff library.
func firstDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	n := len(wantLines)
	if len(gotLines) < n {
		n = len(gotLines)
	}
	for i := 0; i < n; i++ {
		if wantLines[i] != gotLines[i] {
			return "line " + itoa(i+1) + ":\n  want: " + quote(wantLines[i]) + "\n  got:  " + quote(gotLines[i])
		}
	}
	if len(wantLines) != len(gotLines) {
		return "line counts differ: want=" + itoa(len(wantLines)) + " got=" + itoa(len(gotLines))
	}
	return "(content identical after normalize — check trailing whitespace)"
}

func quote(s string) string { return "\"" + s + "\"" }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
