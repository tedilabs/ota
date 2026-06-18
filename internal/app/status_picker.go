package app

// Status picker — modal that lists the valid target statuses for the
// currently-selected user, mapped to a UserActionKind. Operator picks
// one (j/k + Enter) and the App Shell hands off to the existing
// destructive-action confirm flow (OverlayActionConfirm) so the
// "are you sure?" guardrail stays consistent with `:deactivate`,
// `:delete`, etc.
//
// Transitions sourced from the Okta lifecycle state machine
// (see _workspace/edit-form-users/02_okta_domain_input.md §4 and
// docs/PRD.md §5.6 — REQ-W01 covered profile mutation, the status
// picker is the parallel surface for lifecycle mutation).

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// StatusTransition is one entry in the status picker menu — a target
// status the operator can flip the user into, the UserActionKind that
// performs the flip, and a one-line hint explaining the side effect
// so the operator doesn't pick blind.
type StatusTransition struct {
	TargetStatus domain.UserStatus
	Action       UserActionKind
	Hint         string
}

// statusTransitionsFor returns the valid transitions for `current`.
// Empty result means the user is in a terminal / unmanaged state and
// the picker should render "(no transitions available)".
func statusTransitionsFor(current domain.UserStatus) []StatusTransition {
	switch current {
	case domain.UserStatusStaged, domain.UserStatusProvisioned:
		return []StatusTransition{
			{domain.UserStatusActive, UserActionActivate, "complete activation; sends invite email"},
		}
	case domain.UserStatusActive:
		return []StatusTransition{
			{domain.UserStatusSuspended, UserActionSuspend, "block sign-in; keep groups / apps / factors"},
			{domain.UserStatusDeprovisioned, UserActionDeactivate, "deprovision; revokes every session"},
			{domain.UserStatusPasswordExpired, UserActionExpirePassword, "force password change at next sign-in"},
		}
	case domain.UserStatusSuspended:
		return []StatusTransition{
			{domain.UserStatusActive, UserActionUnsuspend, "lift suspend; restore sign-in"},
			{domain.UserStatusDeprovisioned, UserActionDeactivate, "deprovision; revokes every session"},
		}
	case domain.UserStatusLockedOut:
		return []StatusTransition{
			{domain.UserStatusActive, UserActionUnlock, "clear lockout; restore sign-in"},
			{domain.UserStatusDeprovisioned, UserActionDeactivate, "deprovision; revokes every session"},
		}
	case domain.UserStatusPasswordExpired:
		return []StatusTransition{
			{domain.UserStatusDeprovisioned, UserActionDeactivate, "deprovision; revokes every session"},
		}
	case domain.UserStatusDeprovisioned:
		return []StatusTransition{
			{domain.UserStatusActive, UserActionActivate, "re-provision; sends activation email"},
			// Delete uses a synthetic "(deleted)" pseudo-status — the
			// picker treats it as a terminal removal rather than a
			// status flip. Status field uses empty string so the
			// renderer falls back to the action label.
			{domain.UserStatus(""), UserActionDelete, "permanent removal — irreversible"},
		}
	}
	return nil
}

// StatusPickerModel renders the picker as a centered modal — the
// same MountModal surface the Quit / Action menu / API recorder
// overlays use. State is a cursor index; navigation is j/k/↑/↓
// (Enter and Esc are handled by the App Shell so the chosen
// transition routes into openActionConfirm).
type StatusPickerModel struct {
	user         domain.User
	transitions  []StatusTransition
	cursor       int
}

// NewStatusPickerModel constructs a picker around the user's current
// status. The transitions are filled in eagerly so the App Shell can
// short-circuit the open path when the status is terminal.
func NewStatusPickerModel(user domain.User) StatusPickerModel {
	return StatusPickerModel{
		user:        user,
		transitions: statusTransitionsFor(user.Status),
	}
}

// Init implements tea.Model.
func (m StatusPickerModel) Init() tea.Cmd { return nil }

// Update advances the cursor on j/k/↑/↓. Enter and Esc bubble up to
// the App Shell — this model stays focused on selection state.
func (m StatusPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.Type {
	case tea.KeyDown:
		if m.cursor < len(m.transitions)-1 {
			m.cursor++
		}
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			if m.cursor < len(m.transitions)-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

// View renders the centered modal — title shows the user identifier
// + current status; body lists one row per valid transition with the
// target status badge + hint trail.
func (m StatusPickerModel) View() string {
	tk := shared.Dark()
	login := m.user.Profile.Login
	if login == "" {
		login = m.user.ID
	}
	currentBadge := string(m.user.Status)
	if currentBadge == "" {
		currentBadge = "—"
	}
	title := "Change status · " + login + "  (current: " + currentBadge + ")"

	var b strings.Builder
	if len(m.transitions) == 0 {
		b.WriteString(tk.Muted.Render("(no transitions available for " + currentBadge + ")"))
	} else {
		for i, tr := range m.transitions {
			prefix := "  "
			label := statusActionLabel(tr)
			if i == m.cursor {
				prefix = "▸ "
				label = tk.Accent.Render(label)
			}
			b.WriteString(prefix + label)
			if tr.Hint != "" {
				b.WriteString("  " + tk.Muted.Render(tr.Hint))
			}
			if i < len(m.transitions)-1 {
				b.WriteByte('\n')
			}
		}
	}
	footer := "<j/k> nav · <Enter> pick · <Esc> cancel"
	if len(m.transitions) == 0 {
		footer = "<Esc> close"
	}
	return shared.MountModal(shared.ModalIn{
		Title:  title,
		Body:   b.String(),
		Footer: footer,
		Tone:   shared.ModalToneAccent,
		Width:  72,
		Tokens: tk,
	})
}

// Cursor exposes the picker cursor so the App Shell can resolve the
// chosen transition on Enter.
func (m StatusPickerModel) Cursor() int { return m.cursor }

// Selected returns the highlighted transition, or zero value + false
// when the picker is empty.
func (m StatusPickerModel) Selected() (StatusTransition, bool) {
	if m.cursor < 0 || m.cursor >= len(m.transitions) {
		return StatusTransition{}, false
	}
	return m.transitions[m.cursor], true
}

// Empty reports whether there are no transitions for this status —
// used by the App Shell to short-circuit the open with a toast.
func (m StatusPickerModel) Empty() bool { return len(m.transitions) == 0 }

// statusActionLabel renders a row label like "→ SUSPENDED  (Suspend user)".
// Target-status badge is the primary signal; the action verb is the
// muted parenthetical so the operator picks the desired *state* and
// only secondarily reads the *verb*.
func statusActionLabel(tr StatusTransition) string {
	target := string(tr.TargetStatus)
	if target == "" {
		// Delete pseudo-transition — no target status.
		return "✗ " + userActionLabel(tr.Action)
	}
	return "→ " + target + "  (" + userActionLabel(tr.Action) + ")"
}
