package app_test

// v0.1.1 Red — TUI_DESIGN §15.0a (v1.2.0): responsive sizing.
//
// The chrome must fill 100% of the terminal width reported by
// tea.WindowSizeMsg. The previous v1.0/v1.1 cap of 200 columns is dropped:
// the App Shell forwards the raw width (clamped only to a minimum of 80) to
// shared.RenderChrome so wide terminals (160, 180, 220+) render edge-to-edge.
//
// These tests stay Red until v0.1.1-3 (responsive chrome) lands. Specifically:
//
//   - clampWidth in app.go currently caps at 200. A 220-cell terminal must
//     render a 220-cell chrome, not 200.
//   - The chrome's default (no WindowSizeMsg yet) is shared.ChromeWidth (85).
//     After WindowSizeMsg{160}, every body row must include 160 visible cells.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/testfx"
)

// Test_AppShell_Chrome_FillsTerminalWidth_160 — when WindowSizeMsg reports a
// 160-cell width, the chrome must render at exactly 160 cells per row. Today
// the renderer pads to clampWidth's value; for v1.2.0 we want full fill.
//
// Verifies via row-length assertion on the title bar (top-most border row).
func Test_AppShell_Chrome_FillsTerminalWidth_160(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers, Profile: "prod", OrgURL: "https://acme.okta.com"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	got := testfx.StripANSI(updated.(app.Model).View())

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	require.NotEmpty(t, lines, "View must produce at least the chrome borders")
	// The first row is the rounded top border ("╭...─╮"). Its rune count must
	// equal the requested terminal width.
	topRow := lines[0]
	got160 := runeCount(topRow)
	assert.Equal(t, 160, got160,
		"At width=160, chrome top border must be 160 cells wide (TUI_DESIGN §15.0a). got=%d row=%q", got160, topRow)
}

// Test_AppShell_Chrome_FillsTerminalWidth_220 — beyond the prior 200-col cap.
// Stays Red until clampWidth no longer caps at 200.
func Test_AppShell_Chrome_FillsTerminalWidth_220(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers, Profile: "prod", OrgURL: "https://acme.okta.com"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 220, Height: 40})
	got := testfx.StripANSI(updated.(app.Model).View())

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	require.NotEmpty(t, lines)
	topRow := lines[0]
	gotW := runeCount(topRow)
	assert.Equal(t, 220, gotW,
		"width=220 must render a 220-cell chrome (no 200 cap, TUI_DESIGN §15.0a v1.2.0). got=%d", gotW)
}

// Test_AppShell_Chrome_AllRowsSameWidth — every visible row of the chrome must
// be the same column count, otherwise the box looks ragged.
func Test_AppShell_Chrome_AllRowsSameWidth(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers, Profile: "prod", OrgURL: "https://acme.okta.com"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	got := testfx.StripANSI(updated.(app.Model).View())

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	require.NotEmpty(t, lines)
	want := runeCount(lines[0])
	for i, line := range lines {
		assert.Equal(t, want, runeCount(line),
			"row %d width mismatch — chrome rows must share width %d. got=%d row=%q",
			i, want, runeCount(line), line)
	}
}

// runeCount returns the rune count of s — a stand-in for visible cell count
// in the ASCII profile pinned by testfx.PinTestEnvironment.
func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
