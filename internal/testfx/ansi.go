// Package testfx provides shared test fixtures for the ota TUI: ANSI
// stripping, color-profile pinning, and golden-file assertions used by every
// internal/tui/<screen> package's golden_test.go.
//
// Usage from a test file:
//
//	func init() { testfx.PinTestEnvironment() }
//
//	func Test_FooGolden_Default(t *testing.T) {
//	    got := testfx.StripANSI(model.View())
//	    testfx.AssertGolden(t, got, "testdata/golden/list_default.txt")
//	}
//
// Update goldens with `go test -update ./internal/tui/...`.
package testfx

import (
	"github.com/charmbracelet/x/ansi"
)

// StripANSI removes every ANSI escape sequence (CSI, OSC, etc.) from s so
// golden files only diff on textual content. Backed by charmbracelet/x/ansi
// — the same parser ota's renderer uses, so we strip exactly what lipgloss
// emits.
func StripANSI(s string) string {
	return ansi.Strip(s)
}
