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
// overlay lists every core key. v0.1.5-3 grouped the rows into
// k9s-style sections (Resource / General / Navigation) and the
// half-page row was renamed "Ctrl-d / Ctrl-u" to match the README
// notation, so the substring expectation is updated to match.
func Test_Help_HasCoreNavigationKeys(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewHelpModel().View())
	for _, key := range []string{"j", "k", "Esc", ":", "/", "?", "q", "Ctrl-d / Ctrl-u"} {
		assert.Contains(t, got, key, "help overlay must list %q key hint (TUI_DESIGN §16.11)", key)
	}
	for _, section := range []string{"── Resource ──", "── General ──", "── Navigation ──"} {
		assert.Contains(t, got, section, "help overlay must group entries under %q (issue #120)", section)
	}
}

// Test_HelpForUsers_ShowsSortKeys verifies the Users-screen help advertises
// the Shift+S/N/L/C sort cycle bindings introduced in v0.1.1.
func Test_HelpForUsers_ShowsSortKeys(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewHelpModelFor("users").View())
	assert.Contains(t, got, "Help · Users List")
	for _, key := range []string{"Shift+S", "Shift+N", "Shift+L", "Shift+C"} {
		assert.Contains(t, got, key, "Users help must advertise %q sort key", key)
	}
	assert.Contains(t, got, "Enter / d")
}

// Test_HelpForLogs_ShowsTailKeys verifies the Logs help advertises tail (s)
// and follow (f) — keys that would be no-ops on Users / Groups / Rules.
func Test_HelpForLogs_ShowsTailKeys(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewHelpModelFor("logs").View())
	assert.Contains(t, got, "Help · System Logs")
	for _, line := range []string{"toggle tail badge", "toggle auto-follow"} {
		assert.Contains(t, got, line, "Logs help must mention %q", line)
	}
}

// Test_HelpForGroups_OmitsUsersOnlySortKeys ensures we don't leak
// Users-only sort keys (Shift+L / Shift+C) into the Groups help — they
// would do nothing on Groups and confuse operators.
func Test_HelpForGroups_OmitsUsersOnlySortKeys(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewHelpModelFor("groups").View())
	assert.Contains(t, got, "Help · Groups List")
	assert.Contains(t, got, "Shift+N", "Groups help must advertise Shift+N (sort by NAME)")
	assert.NotContains(t, got, "Shift+L",
		"Groups help must NOT mention Shift+L — Last Login is a Users-only column")
	assert.NotContains(t, got, "Shift+C",
		"Groups help must NOT mention Shift+C — CREATED is a Users-only column")
}

// Test_HelpForRules_HighlightsInvalidStatus locks in the rule-specific
// language so operators understand the operational rank ordering.
func Test_HelpForRules_HighlightsInvalidStatus(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewHelpModelFor("rules").View())
	assert.Contains(t, got, "Help · Group Rules List")
	assert.Contains(t, got, "INVALID first",
		"Rules help must call out INVALID-first sort rank (TUI_DESIGN §3.5a)")
}

// Test_Confirm_HasYNHint locks in SCR-903: the confirm dialog explicitly
// shows both "[y/N]" and "y/n" so operators don't miss either form.
func Test_Confirm_HasYNHint(t *testing.T) {
	t.Parallel()
	got := testfx.StripANSI(overlay.NewConfirmModel("Quit ota?").View())
	assert.Contains(t, got, "y", "Confirm overlay must hint at y/n (SCR-903)")
	assert.Contains(t, got, "N", "Confirm overlay default-no must be capitalized (SCR-903)")
}
