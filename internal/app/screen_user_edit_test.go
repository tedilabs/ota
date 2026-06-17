package app_test

// Lock-in test (not Fail-First derived): the ScreenUserEdit and
// OverlayDiscardConfirm constants are added by Phase 5 STUB patches
// (so Phase 6 has identifiers to wire). These tests lock the contract
// shape — String() canonical form, distinct enum values — so Phase 6
// can't accidentally collapse them.
//
// REQ-W01 — App Shell identity tests for the SCR-012 (Users Edit
// Form) navigation surface (Step 8 of Phase 5 RED order). Behavioural
// tests for routing (`e` → ScreenUserEdit, OpenUserEditMsg →
// push) live in user_edit_entry_test.go and are FAIL-FIRST RED.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/app"
)

// TUI_DESIGN §8.1 — ScreenUserEdit.String() must be `user-edit`.
// Used by the palette autocomplete (`:user-edit`) and the resolver.
func Test_ScreenUserEdit_String_IsUserEdit(t *testing.T) {
	t.Parallel()
	got := app.ScreenUserEdit.String()
	assert.Equal(t, "user-edit", got,
		"REQ-W01 TUI_DESIGN §8.1: ScreenUserEdit.String() must be %q so :user-edit palette resolves", "user-edit")
}

// TUI_DESIGN §8.1 — String() is the inverse of screenFromName for the
// canonical form. Since screenFromName is unexported, we exercise it
// indirectly via SwitchScreenMsg in a follow-up test (see
// Test_AppModel_SwitchScreen_UserEdit_PushesUserEditScreen below).
// Here we just pin the canonical literal alongside aliases.
func Test_ScreenUserEdit_KindIsDistinct(t *testing.T) {
	t.Parallel()
	// Distinct from ScreenUserDetail to avoid an iota collapse on
	// next constant addition.
	assert.NotEqual(t, app.ScreenUserDetail, app.ScreenUserEdit,
		"REQ-W01: ScreenUserEdit and ScreenUserDetail must be different Screen values")
}

// REQ-W01 / D-W4 — OverlayDiscardConfirm exists as a distinct overlay
// kind separate from OverlayActionConfirm (lifecycle confirm).
func Test_OverlayDiscardConfirm_IsDistinctFromActionConfirm(t *testing.T) {
	t.Parallel()
	assert.NotEqual(t, app.OverlayActionConfirm, app.OverlayDiscardConfirm,
		"REQ-W01 D-W4: OverlayDiscardConfirm must be its own overlay value (pendingAction shape can't host a Form snapshot)")
	assert.NotEqual(t, app.OverlayNone, app.OverlayDiscardConfirm,
		"OverlayDiscardConfirm must be non-zero so the overlay router can branch")
}
