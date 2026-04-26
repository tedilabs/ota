package testfx

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// PinTestEnvironment fixes the rendering profile so golden output is
// deterministic across machines:
//
//   - NO_COLOR=1 propagates to any code that branches on shared.MonochromeEnabled.
//   - lipgloss.SetColorProfile(termenv.Ascii) forces every Style.Render to skip
//     color sequences entirely (defense-in-depth even when StripANSI runs).
//
// Call once from an init() at the top of each *_golden_test.go.
func PinTestEnvironment() {
	_ = os.Setenv("NO_COLOR", "1")
	lipgloss.SetColorProfile(termenv.Ascii)
}
