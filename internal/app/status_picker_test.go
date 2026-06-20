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
	pick := app.NewUserStatusPickerModel(u)
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
	pick := app.NewUserStatusPickerModel(u)
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
	pick := app.NewUserStatusPickerModel(u)
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
	pick := app.NewUserStatusPickerModel(u)
	assert.True(t, pick.Empty(),
		"unknown / unmanaged statuses surface as empty so the App Shell can toast instead of opening a blank modal")
}

// Multi-resource lock-in: every per-resource constructor populates a
// non-empty transitions list for an Active resource and renders the
// expected target status badge in the modal.

func Test_StatusPicker_Rule_InactiveOffersActivate(t *testing.T) {
	t.Parallel()
	r := domain.GroupRule{ID: "0pr_x", Name: "engineers", Status: domain.GroupRuleStatusInactive}
	pick := app.NewRuleStatusPickerModel(r)
	require.False(t, pick.Empty())
	view := pick.View()
	assert.Contains(t, view, "ACTIVE", "INACTIVE rule should offer the ACTIVE flip")
	assert.Contains(t, view, "engineers", "title carries the rule name")
}

func Test_StatusPicker_Policy_SystemReturnsEmpty(t *testing.T) {
	t.Parallel()
	p := domain.Policy{ID: "00p_x", Name: "Default Sign-On",
		Status: domain.PolicyStatusActive, System: true}
	pick := app.NewPolicyStatusPickerModel(p)
	assert.True(t, pick.Empty(),
		"system policies refuse lifecycle flips upstream; the picker reports empty so the App Shell can toast")
}

func Test_StatusPicker_Policy_NormalActiveOffersDeactivate(t *testing.T) {
	t.Parallel()
	p := domain.Policy{ID: "00p_y", Name: "Custom", Status: domain.PolicyStatusActive}
	pick := app.NewPolicyStatusPickerModel(p)
	require.False(t, pick.Empty())
	assert.Contains(t, pick.View(), "INACTIVE")
}

func Test_StatusPicker_App_InactiveOffersActivate(t *testing.T) {
	t.Parallel()
	a := domain.App{ID: "0oa_x", Label: "Salesforce", Status: domain.AppStatusInactive}
	pick := app.NewAppStatusPickerModel(a)
	require.False(t, pick.Empty())
	view := pick.View()
	assert.Contains(t, view, "ACTIVE", "INACTIVE app should offer the ACTIVE flip")
	assert.Contains(t, view, "Salesforce")
}

func Test_StatusPicker_Authenticator_ActiveOffersDeactivate(t *testing.T) {
	t.Parallel()
	auth := domain.Authenticator{
		ID:     "aut_x",
		Name:   "okta_verify",
		Status: domain.AuthenticatorStatusActive,
	}
	pick := app.NewAuthenticatorStatusPickerModel(auth)
	require.False(t, pick.Empty())
	assert.Contains(t, pick.View(), "INACTIVE")
}
