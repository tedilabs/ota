package testfx

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// Test_StripANSI_RemovesColorCodes — sanity check that lipgloss-rendered
// strings come out as plain text after StripANSI. Pinning the test profile
// is defense-in-depth; even if a future change re-enables color emission,
// StripANSI must remove it.
func Test_StripANSI_RemovesColorCodes(t *testing.T) {
	t.Parallel()

	// Force a real ANSI emission for this assertion regardless of NO_COLOR.
	r := lipgloss.NewRenderer(nil)
	style := r.NewStyle().Bold(true)
	rendered := style.Render("HELLO")

	stripped := StripANSI(rendered)
	assert.NotContains(t, stripped, "\x1b[", "stripped output must contain no ESC sequences")
}

func Test_StripANSI_PreservesPlainText(t *testing.T) {
	t.Parallel()
	plain := "STATUS  LOGIN          LAST LOGIN\n"
	assert.Equal(t, plain, StripANSI(plain))
}
