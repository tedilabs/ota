package overlay_test

// Phase 6d — Visual lock-in for SCR-900 (palette), SCR-902 (help),
// SCR-903 (confirm). Goldens come from TUI_DESIGN §16.10–§16.12.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/overlay"
)

func init() { testfx.PinTestEnvironment() }

// --- Golden snapshots --------------------------------------------------------

func Test_PaletteGolden_Default(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	m := overlay.NewCmdPaletteModel()
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/palette_default.txt")
}

func Test_HelpGolden_ScreenUsers(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	m := overlay.NewHelpModel()
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/help_screen_users.txt")
}

func Test_ConfirmGolden_UnmaskDialog(t *testing.T) {
	t.Parallel()
// Phase 6d-{3,4,5,6} unblocked — golden lock-in active.

	m := overlay.NewConfirmModel("Unmask PII field · mobilePhone")
	got := testfx.StripANSI(m.View())
	testfx.AssertGolden(t, got, "testdata/golden/confirm_unmask.txt")
}

// --- Spec lock-in (Active, regression-safe today) ---------------------------

// Test_Palette_HasCorePaletteCommands locks in REQ-U02 AC-1: every resource
// + control command appears as a hint.
func Test_Palette_HasCorePaletteCommands(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewCmdPaletteModel().View())
	for _, cmd := range []string{":users", ":groups", ":grouprules", ":policies", ":logs", ":help", ":quit"} {
		assert.Contains(t, got, cmd, "palette must surface %q hint (REQ-U02 AC-1)", cmd)
	}
}

// Test_Help_HasCoreNavigationKeys locks in TUI_DESIGN §16.11: the help
// overlay lists every core key (j/k/Enter/Tab/q/?/:/etc.).
func Test_Help_HasCoreNavigationKeys(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewHelpModel().View())
	for _, key := range []string{"j", "k", "Enter", "Esc", ":", "/", "?", "q", "Ctrl-d/u"} {
		assert.Contains(t, got, key, "help overlay must list %q key hint (TUI_DESIGN §16.11)", key)
	}
}

// Test_Confirm_HasYNHint locks in SCR-903: the confirm dialog explicitly
// shows both "[y/N]" and "y/n" so operators don't miss either form.
func Test_Confirm_HasYNHint(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewConfirmModel("Quit ota?").View())
	assert.Contains(t, got, "y", "Confirm overlay must hint at y/n (SCR-903)")
	assert.Contains(t, got, "N", "Confirm overlay default-no must be capitalized (SCR-903)")
}
