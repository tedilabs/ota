package app_test

// REQ-W01 Phase 7 QA — regression tests for findings discovered during
// the qa-inspector cross-read pass (qa-findings #QA-W01-1..#QA-W01-4).
//
// These FAIL-FIRST tests reproduce production-only bugs that the Phase 5
// teatest scenarios miss because they exercise EditModel in isolation
// rather than through the App Shell. Each test asserts the contract a
// fix must satisfy.
//
// Status at time of writing (2026-06-17, Phase 7 QA Cycle 1):
//   - QA-W01-1 Test_AppShell_Esc_OnDirtyEditForm_OpensDiscardConfirm   → FAIL (Critical, data-loss)
//   - QA-W01-2 Test_AppShell_Esc_DuringSaving_DoesNotPopNav            → FAIL (High, race)
//   - QA-W01-3 Test_AppShell_PaletteEdit_ResolvesScreenUserEdit        → FAIL (High, missing palette alias)
//   - QA-W01-4 Test_AppShell_PaletteE_ResolvesScreenUserEdit           → FAIL (High, missing :e palette alias)

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// qaEditPort is a minimal UsersPort that returns a populated user on
// Get so the EditModel finishes Loading → Editing. UpdateProfile blocks
// (never returns) so we can pin the saving-state behaviour without
// racing the success branch.
type qaEditPort struct {
	user     domain.User
	saveHold chan struct{}
}

func newQAEditPort() *qaEditPort {
	return &qaEditPort{
		user: domain.User{
			ID:     "00u_alice",
			Status: domain.UserStatusActive,
			Profile: domain.UserProfile{
				Login: "alice@acme.com", Email: "alice@acme.com",
				FirstName: "Alice", LastName: "Smith",
			},
		},
		saveHold: make(chan struct{}),
	}
}

func (p *qaEditPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &qaEditPortIter{u: p.user, more: true}, nil
}
func (p *qaEditPort) Get(_ context.Context, _ string) (domain.User, error) { return p.user, nil }
func (p *qaEditPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *qaEditPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *qaEditPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *qaEditPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *qaEditPort) Unlock(_ context.Context, _ string) error             { return nil }
func (p *qaEditPort) ResetFactors(_ context.Context, _ string) error       { return nil }
func (p *qaEditPort) Activate(_ context.Context, _ string, _ bool) error   { return nil }
func (p *qaEditPort) Deactivate(_ context.Context, _ string, _ bool) error { return nil }
func (p *qaEditPort) ExpirePassword(_ context.Context, _ string) error     { return nil }
func (p *qaEditPort) Suspend(_ context.Context, _ string) error   { return nil }
func (p *qaEditPort) Unsuspend(_ context.Context, _ string) error { return nil }
func (p *qaEditPort) Delete(_ context.Context, _ string) error             { return nil }
func (p *qaEditPort) UpdateProfile(ctx context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
	// Block until the test signals via close(saveHold), unless ctx
	// cancels first. Mirrors a slow Okta save.
	select {
	case <-p.saveHold:
		return p.user, nil
	case <-ctx.Done():
		return domain.User{}, ctx.Err()
	}
}

type qaEditPortIter struct {
	u    domain.User
	more bool
}

func (it *qaEditPortIter) Next(_ context.Context) (domain.User, bool, error) {
	if !it.more {
		return domain.User{}, false, nil
	}
	it.more = false
	return it.u, true, nil
}
func (it *qaEditPortIter) Close() error { return nil }

func newQAAppModel(t *testing.T) (app.Model, *qaEditPort) {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	port := newQAEditPort()
	m := app.New(app.Deps{
		UsersPort: port,
		Clock:     clock.Real(),
		Keys:      keymap,
		Profile:   "test",
		OrgURL:    "https://acme.okta.com",
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	return updated.(app.Model), port
}

// QA-W01-1 (Critical, data-loss) — When the operator presses Esc on a
// DIRTY edit form, the App Shell MUST forward Esc to EditModel so it can
// raise the discard confirm modal (REQ-W01 AC-5.2 / D-W4). Today the
// shell pops the nav stack first regardless of EscapeWillAct (commit
// a68426b hardened Esc precedence), silently discarding all unsaved
// edits.
//
// Repro:
//   1. Open Users list → press `e` → ScreenUserEdit pushes.
//   2. Wait for loading→editing transition.
//   3. Type a rune in the focused (firstName) field → dirty=1.
//   4. Press Esc.
// Expected: discard confirm modal visible ("Discard" text); active
// screen remains user-edit.
// Actual: active screen is "users" (popped silently). PRD AC-5.2
// + D-W4 violated.
func Test_AppShell_Esc_OnDirtyEditForm_OpensDiscardConfirm(t *testing.T) {
	t.Parallel()
	m, _ := newQAAppModel(t)

	// Trigger edit form entry via OpenUserEditMsg (Phase 5 lock-in test
	// proves this path).
	updated, cmd := m.Update(shared.OpenUserEditMsg{ID: "00u_alice"})
	m = updated.(app.Model)
	require.Equal(t, "user-edit", app.ActiveScreenName(m),
		"precondition: edit screen active")

	// Drain the Init Cmd so the loading→editing transition completes.
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		updated, cmd = m.Update(msg)
		m = updated.(app.Model)
	}

	// Type into the focused first editable field (firstName) → dirty=1.
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(app.Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	m = updated.(app.Model)

	// Press Esc — must open Discard confirm, NOT pop nav.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(app.Model)

	assert.Equal(t, "user-edit", app.ActiveScreenName(m),
		"REQ-W01 AC-5.2 / D-W4: dirty Esc must NOT pop the nav stack — discard confirm comes first")
	view := m.View()
	assert.True(t, strings.Contains(view, "Discard") || strings.Contains(view, "discard"),
		"REQ-W01 AC-5.2 / D-W4: dirty Esc must surface a Discard prompt in the active view (got: %q)", trim(view, 200))
}

// QA-W01-2 (High, race) — Esc during EditStateSaving MUST NOT pop the
// nav stack while a save POST is in flight (REQ-W01 AC-4.3 / AC-5.3).
// Today the shell pops nav unconditionally; the save Cmd completes
// later and emits UserUpdatedMsg into a screen the operator has
// already left. At minimum, the saving form should stay visible so
// Ctrl+C can abort.
//
// Repro:
//   1. Open edit form → fill → Ctrl+S → state = EditStateSaving.
//   2. Press Esc.
// Expected: still on user-edit (saving cannot be Esc-cancelled).
// Actual: nav pops to users; save races to completion in background.
func Test_AppShell_Esc_DuringSaving_DoesNotPopNav(t *testing.T) {
	t.Parallel()
	m, port := newQAAppModel(t)
	defer close(port.saveHold) // unblock save before test ends

	updated, cmd := m.Update(shared.OpenUserEditMsg{ID: "00u_alice"})
	m = updated.(app.Model)
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		updated, cmd = m.Update(msg)
		m = updated.(app.Model)
	}

	// Edit + Ctrl+S → enters saving state. The save Cmd is in flight
	// (blocked on port.saveHold) so we get a clean snapshot of the
	// EditStateSaving moment.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(app.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	m = updated.(app.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(app.Model)

	require.Equal(t, "user-edit", app.ActiveScreenName(m),
		"precondition: saving state still on user-edit")

	// Esc during saving must NOT pop nav (AC-4.3 / AC-5.3).
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(app.Model)

	assert.Equal(t, "user-edit", app.ActiveScreenName(m),
		"REQ-W01 AC-4.3 / AC-5.3: Esc must be disabled while saving — pressing it cannot drop the form (Ctrl+C only aborts)")
}

// QA-W01-3 (High, missing palette alias) — `:edit` palette command MUST
// resolve to ScreenUserEdit (TUI_DESIGN §3.4, PRD §5.6 AC-1 via :edit).
// Today screenFromName only recognises "user-edit" / "useredit" — the
// canonical PRD-promised `:edit` typo is silently rejected.
//
// Repro: SwitchScreenMsg{Target:"edit"} → active screen unchanged.
// Expected: active screen "user-edit".
func Test_AppShell_PaletteEdit_ResolvesScreenUserEdit(t *testing.T) {
	t.Parallel()
	m, _ := newQAAppModel(t)

	updated, _ := m.Update(app.SwitchScreenMsg{Target: "edit"})
	m = updated.(app.Model)

	assert.Equal(t, "user-edit", app.ActiveScreenName(m),
		"REQ-W01 AC-1.2 / TUI_DESIGN §3.4: :edit palette command must resolve to ScreenUserEdit")
}

// QA-W01-4 (High, missing palette alias) — `:e` short alias must also
// resolve (TUI_DESIGN §3.4 — `:edit` / `:e` both listed).
func Test_AppShell_PaletteE_ResolvesScreenUserEdit(t *testing.T) {
	t.Parallel()
	m, _ := newQAAppModel(t)

	updated, _ := m.Update(app.SwitchScreenMsg{Target: "e"})
	m = updated.(app.Model)

	assert.Equal(t, "user-edit", app.ActiveScreenName(m),
		"REQ-W01 AC-1.2 / TUI_DESIGN §3.4: :e palette short alias must resolve to ScreenUserEdit")
}

// Operator follow-up (2026-06-18) — pressing Enter on the highlighted
// "Discard and exit" option must actually leave the edit screen. The
// initial wiring emitted form.DiscardRequestedMsg which nothing
// handled, so the form stayed open with the prompt visible. The
// shared.UserEditDiscardedMsg + App Shell popNav wiring is the fix —
// this test pins the contract.
func Test_AppShell_DiscardAndExit_PopsNav(t *testing.T) {
	t.Parallel()
	m, _ := newQAAppModel(t)

	updated, cmd := m.Update(shared.OpenUserEditMsg{ID: "00u_alice"})
	m = updated.(app.Model)
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		updated, cmd = m.Update(msg)
		m = updated.(app.Model)
	}
	require.Equal(t, "user-edit", app.ActiveScreenName(m), "precondition: edit screen active")

	// Dirty the form, then Esc to open the discard picker.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(app.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Z")})
	m = updated.(app.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(app.Model)
	require.Contains(t, m.View(), "Discard", "precondition: discard prompt visible")

	// Left arrow → highlight "Discard and exit", Enter → confirm.
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(app.Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(app.Model)
	// The Cmd returns shared.UserEditDiscardedMsg — drain it so the
	// App Shell consumes the pop.
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		updated, cmd = m.Update(msg)
		m = updated.(app.Model)
	}

	assert.NotEqual(t, "user-edit", app.ActiveScreenName(m),
		"Enter on 'Discard and exit' must pop the edit frame back to the previous screen")
}

// trim is a tiny helper for assertion messages on long Views.
func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
