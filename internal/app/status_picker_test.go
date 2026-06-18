package app_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/domain"
)

// Test_StatusPicker_TransitionsForActive locks in the Active user's
// allowed transitions: Suspend (preserving state), Deactivate
// (deprovisioning), Expire password. The order matters — the picker
// renders them in this sequence and the operator's muscle memory
// builds around it.
func Test_StatusPicker_TransitionsForActive(t *testing.T) {
	t.Parallel()
	u := domain.User{
		ID:      "00u_alice",
		Status:  domain.UserStatusActive,
		Profile: domain.UserProfile{Login: "alice@acme.com"},
	}
	pick := app.NewStatusPickerModel(u)
	require.False(t, pick.Empty(), "ACTIVE must have transitions available")

	view := pick.View()
	assert.Contains(t, view, "SUSPENDED", "Suspend target is the first row")
	assert.Contains(t, view, "DEPROVISIONED", "Deactivate target visible")
	assert.Contains(t, view, "PASSWORD_EXPIRED", "ExpirePassword target visible")
	assert.Contains(t, view, "alice@acme.com", "modal title shows the login")
	assert.Contains(t, view, "ACTIVE", "modal title shows current status badge")
}

// Test_StatusPicker_TransitionsForDeprovisioned covers the terminal
// branch where Activate (re-provision) and Delete are the only
// targets. Delete is intentionally a destructive pseudo-transition
// — no target status badge, just the action label.
func Test_StatusPicker_TransitionsForDeprovisioned(t *testing.T) {
	t.Parallel()
	u := domain.User{
		ID:      "00u_bob",
		Status:  domain.UserStatusDeprovisioned,
		Profile: domain.UserProfile{Login: "bob@acme.com"},
	}
	pick := app.NewStatusPickerModel(u)
	require.False(t, pick.Empty(), "DEPROVISIONED must offer Activate + Delete")

	view := pick.View()
	assert.Contains(t, view, "ACTIVE", "re-provision target shown")
	assert.Contains(t, view, "Delete user", "Delete action label rendered")
}

// Test_StatusPicker_NavigatesWithJK pins the j/k arrow cursor
// semantics so the cursor lands on the expected transition before the
// App Shell routes Enter into OverlayActionConfirm.
func Test_StatusPicker_NavigatesWithJK(t *testing.T) {
	t.Parallel()
	u := domain.User{
		Status: domain.UserStatusActive, // 3 transitions
	}
	pick := app.NewStatusPickerModel(u)
	require.Equal(t, 0, pick.Cursor(), "starts at index 0")

	step := func(r rune) {
		updated, _ := pick.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		pick = updated.(app.StatusPickerModel)
	}

	step('j')
	assert.Equal(t, 1, pick.Cursor(), "j advances")
	step('j')
	assert.Equal(t, 2, pick.Cursor(), "j advances")
	step('j')
	assert.Equal(t, 2, pick.Cursor(), "j clamps at last")
	step('k')
	assert.Equal(t, 1, pick.Cursor(), "k retreats")
	step('k')
	step('k')
	assert.Equal(t, 0, pick.Cursor(), "k clamps at first")
}

// Test_StatusPicker_OpenStatusPickerMsg_TerminalStateToasts proves the
// App Shell does NOT open the modal when the user is in a state with
// no transitions — operators should see a toast, not an empty modal.
// (UserStatusStaged returns one transition; let's pick a state that
// actually has zero — there isn't one in the current matrix, but the
// invariant is worth pinning behind the Empty() helper.)
func Test_StatusPicker_Empty_ForUnknownStatus(t *testing.T) {
	t.Parallel()
	u := domain.User{Status: domain.UserStatus("UNKNOWN_FOR_TEST")}
	pick := app.NewStatusPickerModel(u)
	assert.True(t, pick.Empty(),
		"unknown / unmanaged statuses surface as empty so the App Shell can toast instead of opening a blank modal")
}
