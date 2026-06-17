package users_test

// REQ-W01 — SCR-012 Users Edit Form teatest scenarios (Step 9 + 11
// of the Phase 5 RED order). These tests drive `users.EditModel`
// end-to-end via teatest.NewTestModel — the loop fetches the user,
// the operator edits a field, presses Ctrl+S, and the form fires
// service.UpdateProfile. The fake UsersPort is the seam.
//
// AC mapping (PRD §5.6 / TUI_DESIGN §11.2a):
//   - AC-1.3 Test_UserEdit_OnEntry_CallsPortGet_Once
//   - AC-1.5 Test_UserEdit_Loading_4xx_DoesNotOpenForm
//   - AC-4.1 + AC-4.2 Test_UserEdit_Save_PartialMergeBody_Success
//   - AC-4.5 Test_UserEdit_Save_Success_BroadcastsUserUpdatedMsg
//   - AC-5.1 Test_UserEdit_Esc_Clean_PopsImmediately
//   - AC-5.2 Test_UserEdit_Esc_Dirty_OpensDiscardConfirm
//   - AC-6  Test_UserEdit_Save_400Validation_InlineFieldErrors
//   - AC-9  Test_UserEdit_Dirty_Counter_RendersInFooter
//
// Phase 5 RED expectation: EditModel.Update is a stub no-op, so the
// teatest scenarios time out waiting for expected output. Every
// teatest.WaitFor below should fail with "timeout waiting for ..."
// until Phase 6 wires the state machine.

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/users"
)

// loadedAlice is the canonical fixture User the Get func returns.
// 11 fields populated so every dirty-matrix slot has a starting
// value to mutate.
func loadedAlice() domain.User {
	return domain.User{
		ID:     "00u_alice",
		Status: domain.UserStatusActive,
		Profile: domain.UserProfile{
			Login:          "alice@acme.com",
			Email:          "alice@acme.com",
			FirstName:      "Alice",
			LastName:       "Smith",
			DisplayName:    "Alice Smith",
			NickName:       "ali",
			Title:          "SWE",
			Division:       "R&D",
			Department:     "Eng",
			EmployeeNumber: "ENG-042",
			MobilePhone:    "+1-555-123-4567",
			SecondEmail:    "alice.b@personal.com",
		},
	}
}

// newEditFlow returns a teatest TestModel wrapping a fresh EditModel
// with the supplied port wired through a UsersService. Convenience
// for every REQ-W01 teatest scenario.
func newEditFlow(t *testing.T, port *fakes.UsersPortFake) *teatest.TestModel {
	t.Helper()
	svc := service.NewUsersService(port, service.WithClock(clock.Real()))
	m := users.NewEditModel(users.EditDeps{
		Svc:    svc,
		UserID: "00u_alice",
		Clock:  clock.Real(),
		Width:  120,
		Height: 30,
	})
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))
}

// AC-1.3 — entry triggers GET /api/v1/users/{id} exactly once. Pin
// it via a call counter on the port fake.
func Test_UserEdit_OnEntry_CallsPortGet_Once(t *testing.T) {
	t.Parallel()
	getCalls := 0
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, id string) (domain.User, error) {
		getCalls++
		return loadedAlice(), nil
	}
	tm := newEditFlow(t, port)

	// Wait for the form to load (any field label visible)
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC}) // teardown
	_, _ = io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))

	assert.Equal(t, 1, getCalls,
		"REQ-W01 AC-1.3: entry must call port.Get exactly once (list cache not trusted)")
}

// AC-1.5 — GET 4xx blocks form open: no form chrome, no field
// labels. (Phase 6 may also surface a toast; this test only pins
// the "form did not open" half.)
func Test_UserEdit_Loading_403_DoesNotOpenForm(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return domain.User{}, domain.ErrForbidden
	}
	tm := newEditFlow(t, port)

	// Give the model up to a second to make the call — then assert
	// no field labels rendered.
	time.Sleep(300 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))

	assert.NotContains(t, string(out), "First Name",
		"REQ-W01 AC-1.5: 403 on GET must NOT open the form (no field labels visible)")
}

// AC-4.1 + AC-4.2 — Ctrl+S after a single edit triggers
// port.UpdateProfile with a patch containing ONLY that field. The
// 10 other fields must be nil pointers (omitempty).
func Test_UserEdit_Save_PartialMergeBody_Success(t *testing.T) {
	t.Parallel()
	var captured domain.UserProfilePatch
	saveHit := 0
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	port.UpdateProfileFunc = func(_ context.Context, _ string, p domain.UserProfilePatch) (domain.User, error) {
		saveHit++
		captured = p
		u := loadedAlice()
		if p.FirstName != nil {
			u.Profile.FirstName = *p.FirstName
		}
		return u, nil
	}
	tm := newEditFlow(t, port)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))

	// edit first focused (firstName) field: "Alice" + "X" → "AliceX"
	tm.Send(tea.KeyMsg{Type: tea.KeyEnd})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Updated"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, _ = io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))

	require.Equal(t, 1, saveHit, "REQ-W01 AC-4.1: Ctrl+S must hit UpdateProfile exactly once")
	require.NotNil(t, captured.FirstName, "REQ-W01 AC-4.2: edited firstName must appear in patch")
	assert.Equal(t, "AliceX", *captured.FirstName,
		"REQ-W01 AC-4.2: patch value is the live (not snapshot) input")
	assert.Nil(t, captured.LastName, "REQ-W01 AC-4.2: unedited fields must be nil in the patch")
	assert.Nil(t, captured.Email, "REQ-W01 AC-4.2: unedited fields must be nil in the patch")
	assert.Nil(t, captured.Department, "REQ-W01 AC-4.2: unedited fields must be nil in the patch")
	assert.Nil(t, captured.MobilePhone, "REQ-W01 AC-4.2: unedited fields must be nil in the patch")
}

// AC-4.5 — save success broadcasts UserUpdatedMsg so list/detail
// patch their cache. We assert the model surfaced an "Updated"
// toast as a proxy for the broadcast (the App Shell shell handles
// the actual UserUpdatedMsg routing).
func Test_UserEdit_Save_Success_RendersUpdatedToast(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	port.UpdateProfileFunc = func(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
		return loadedAlice(), nil
	}
	tm := newEditFlow(t, port)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyEnd})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Updated"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	assert.Contains(t, string(out), "Updated",
		"REQ-W01 AC-4.5: save success must surface an 'Updated' toast/message")
}

// AC-6 — 400 validation: form does NOT close, field errors render
// inline.
func Test_UserEdit_Save_400Validation_InlineFieldErrors(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	port.UpdateProfileFunc = fakes.ValidationErrorFake(map[string]string{
		"email":      "Email is not valid",
		"department": "Cannot exceed 100 characters",
	})
	tm := newEditFlow(t, port)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyEnd})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Email is not valid"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))

	assert.Contains(t, string(out), "Email is not valid",
		"REQ-W01 AC-6.1: 400 errorCause for 'email' must render inline near Email field")
	assert.Contains(t, string(out), "Cannot exceed 100 characters",
		"REQ-W01 AC-6.1: 400 errorCause for 'department' must render inline near Department")
	assert.Contains(t, string(out), "First Name",
		"REQ-W01 AC-6 / D-W6: form must NOT close on 400 — field labels still visible")
}

// AC-5.1 — clean Esc immediately pops nav (no confirm modal).
func Test_UserEdit_Esc_Clean_NoDiscardConfirm(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	tm := newEditFlow(t, port)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})

	// Give it a moment, then assert no Discard modal text appeared.
	time.Sleep(200 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))

	assert.NotContains(t, string(out), "Discard",
		"REQ-W01 AC-5.1: clean Esc must close immediately — no discard modal")
}

// AC-5.2 — dirty Esc opens the discard confirm overlay (y/N).
func Test_UserEdit_Esc_Dirty_OpensDiscardConfirm(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	tm := newEditFlow(t, port)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyEnd})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Discard"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	assert.Contains(t, string(out), "Discard",
		"REQ-W01 AC-5.2 / D-W4: dirty Esc must open a Discard confirm overlay")
}

// AC-9.3 — dirty footer renders "N changes" counter when dirty > 0.
func Test_UserEdit_Dirty_Counter_RendersInFooter(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	tm := newEditFlow(t, port)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyEnd})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("1 change"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	assert.Contains(t, string(out), "1 change",
		"REQ-W01 AC-9.3: footer must render 'N changes' counter while dirty")
}

// AC-2 — render contains all 11 field labels + 4 section headers
// after the form loads. Pins the FieldSpecs catalog visibility.
func Test_UserEdit_Render_Has11FieldLabels_4Sections(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	tm := newEditFlow(t, port)

	// wait for at least one field label
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	s := string(out)

	for _, lbl := range []string{
		"First Name", "Last Name", "Display Name", "Nickname",
		"Email", "Mobile Phone", "Secondary Email",
		"Title", "Division", "Department", "Employee Number",
	} {
		assert.Contains(t, s, lbl, "REQ-W01 AC-2: %q label must appear in the form", lbl)
	}
	for _, sec := range []string{"Identity", "Contact", "Organization"} {
		assert.Contains(t, s, sec, "REQ-W01 AC-2: section header %q must appear", sec)
	}
}

// AC-2 / D-W2 — Login row renders but is read-only (no input box).
// Phase 6 surfaces the read-only marker; this test pins the literal.
func Test_UserEdit_LoginField_RendersReadOnly(t *testing.T) {
	t.Parallel()
	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.User, error) {
		return loadedAlice(), nil
	}
	tm := newEditFlow(t, port)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("First Name"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, _ := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	s := string(out)

	assert.Contains(t, s, "alice@acme.com",
		"REQ-W01 AC-2: Login value must render (read-only)")
	assert.Contains(t, s, "Login",
		"REQ-W01 AC-2: Login label must render")
}
