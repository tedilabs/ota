package app

// Status picker — modal that lists the valid target statuses for the
// currently-selected resource and routes the operator's pick through
// the existing destructive-action confirm flow (OverlayActionConfirm).
// One picker model covers every resource with an Active/Inactive
// lifecycle: Users, Group Rules, Policies, Apps, Authenticators.
//
// Per-resource specifics:
//   - the resource snapshot (`target` field) — used to populate the
//     pendingXxxAction struct on Enter
//   - StatusPickerSurface enum — drives the Enter dispatcher branch
//   - title subject + current status badge — rendered in the modal
//   - transitions list — per-resource lifecycle matrix
//
// The picker itself stays tiny — it owns a cursor + transitions
// list + the render. Resource-specific logic lives in the
// per-resource constructors and the App Shell's status picker
// handlers.

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// StatusPickerSurface classifies which resource a picker is targeting.
// The App Shell's Enter handler reads this to pick the right pending*
// action slot + dispatcher.
type StatusPickerSurface int

const (
	StatusPickerUser StatusPickerSurface = iota
	StatusPickerRule
	StatusPickerPolicy
	StatusPickerApp
	StatusPickerAuthenticator
)

// StatusTransition is one entry in the picker menu — a target status
// the operator can flip the resource into, the resource-specific
// action kind (cast to int — the dispatcher casts back per surface),
// and a one-line hint.
type StatusTransition struct {
	TargetStatus string
	// Action holds a resource-specific action kind cast to int.
	// Surface (on the model) tells the dispatcher how to interpret it.
	Action int
	Hint   string
	// ActionLabel is the human-readable verb ("Suspend user",
	// "Activate policy"). The picker uses it for the row trailer so
	// the operator picks the state first and reads the verb second.
	ActionLabel string
}

// StatusPickerModel is the generic picker. The same model serves
// every resource — only the constructor populates a different
// transitions list + surface tag.
type StatusPickerModel struct {
	surface      StatusPickerSurface
	target       any    // the resource snapshot (domain.User / Rule / Policy / App / Authenticator)
	titleSubject string // login / name / label — what the picker is about
	currentBadge string // current status string for the title trailer

	transitions []StatusTransition
	cursor      int
}

// Surface exposes the picker's resource tag.
func (m StatusPickerModel) Surface() StatusPickerSurface { return m.surface }

// Target exposes the resource snapshot. Callers cast based on
// Surface().
func (m StatusPickerModel) Target() any { return m.target }

// --- Per-resource constructors --------------------------------------

// NewUserStatusPickerModel builds a picker for the user lifecycle.
func NewUserStatusPickerModel(user domain.User) StatusPickerModel {
	subj := user.Profile.Login
	if subj == "" {
		subj = user.ID
	}
	curr := string(user.Status)
	if curr == "" {
		curr = "—"
	}
	return StatusPickerModel{
		surface:      StatusPickerUser,
		target:       user,
		titleSubject: subj,
		currentBadge: curr,
		transitions:  userTransitionsFor(user.Status),
	}
}

// NewRuleStatusPickerModel builds a picker for a Group Rule.
func NewRuleStatusPickerModel(rule domain.GroupRule) StatusPickerModel {
	subj := rule.Name
	if subj == "" {
		subj = rule.ID
	}
	curr := string(rule.Status)
	if curr == "" {
		curr = "—"
	}
	return StatusPickerModel{
		surface:      StatusPickerRule,
		target:       rule,
		titleSubject: subj,
		currentBadge: curr,
		transitions:  ruleTransitionsFor(rule.Status),
	}
}

// NewPolicyStatusPickerModel builds a picker for a Policy. System
// policies (marked SYS) refuse status flips upstream — the
// transitions list returns empty so the App Shell surfaces a toast
// instead of opening a useless modal.
func NewPolicyStatusPickerModel(policy domain.Policy) StatusPickerModel {
	subj := policy.Name
	if subj == "" {
		subj = policy.ID
	}
	curr := string(policy.Status)
	if curr == "" {
		curr = "—"
	}
	return StatusPickerModel{
		surface:      StatusPickerPolicy,
		target:       policy,
		titleSubject: subj,
		currentBadge: curr,
		transitions:  policyTransitionsFor(policy.Status, policy.System),
	}
}

// NewAppStatusPickerModel builds a picker for an App.
func NewAppStatusPickerModel(app domain.App) StatusPickerModel {
	subj := app.Label
	if subj == "" {
		subj = app.ID
	}
	curr := string(app.Status)
	if curr == "" {
		curr = "—"
	}
	return StatusPickerModel{
		surface:      StatusPickerApp,
		target:       app,
		titleSubject: subj,
		currentBadge: curr,
		transitions:  appTransitionsFor(app.Status),
	}
}

// NewAuthenticatorStatusPickerModel builds a picker for an
// Authenticator. Org-wide change — the hint warns operators.
func NewAuthenticatorStatusPickerModel(auth domain.Authenticator) StatusPickerModel {
	subj := auth.Name
	if subj == "" {
		subj = auth.ID
	}
	curr := string(auth.Status)
	if curr == "" {
		curr = "—"
	}
	return StatusPickerModel{
		surface:      StatusPickerAuthenticator,
		target:       auth,
		titleSubject: subj,
		currentBadge: curr,
		transitions:  authenticatorTransitionsFor(auth.Status),
	}
}

// --- Per-resource transition matrices -------------------------------

func userTransitionsFor(current domain.UserStatus) []StatusTransition {
	switch current {
	case domain.UserStatusStaged, domain.UserStatusProvisioned:
		return []StatusTransition{
			{string(domain.UserStatusActive), int(UserActionActivate),
				"complete activation; sends invite email", userActionLabel(UserActionActivate)},
		}
	case domain.UserStatusActive:
		return []StatusTransition{
			{string(domain.UserStatusSuspended), int(UserActionSuspend),
				"block sign-in; keep groups / apps / factors", userActionLabel(UserActionSuspend)},
			{string(domain.UserStatusDeprovisioned), int(UserActionDeactivate),
				"deprovision; revokes every session", userActionLabel(UserActionDeactivate)},
			{string(domain.UserStatusPasswordExpired), int(UserActionExpirePassword),
				"force password change at next sign-in", userActionLabel(UserActionExpirePassword)},
		}
	case domain.UserStatusSuspended:
		return []StatusTransition{
			{string(domain.UserStatusActive), int(UserActionUnsuspend),
				"lift suspend; restore sign-in", userActionLabel(UserActionUnsuspend)},
			{string(domain.UserStatusDeprovisioned), int(UserActionDeactivate),
				"deprovision; revokes every session", userActionLabel(UserActionDeactivate)},
		}
	case domain.UserStatusLockedOut:
		return []StatusTransition{
			{string(domain.UserStatusActive), int(UserActionUnlock),
				"clear lockout; restore sign-in", userActionLabel(UserActionUnlock)},
			{string(domain.UserStatusDeprovisioned), int(UserActionDeactivate),
				"deprovision; revokes every session", userActionLabel(UserActionDeactivate)},
		}
	case domain.UserStatusPasswordExpired:
		return []StatusTransition{
			{string(domain.UserStatusDeprovisioned), int(UserActionDeactivate),
				"deprovision; revokes every session", userActionLabel(UserActionDeactivate)},
		}
	case domain.UserStatusDeprovisioned:
		return []StatusTransition{
			{string(domain.UserStatusActive), int(UserActionActivate),
				"re-provision; sends activation email", userActionLabel(UserActionActivate)},
			// Delete pseudo-transition — no target status badge.
			{"", int(UserActionDelete),
				"permanent removal — irreversible", userActionLabel(UserActionDelete)},
		}
	}
	return nil
}

func ruleTransitionsFor(current domain.GroupRuleStatus) []StatusTransition {
	switch current {
	case domain.GroupRuleStatusInactive, domain.GroupRuleStatusInvalid:
		return []StatusTransition{
			{string(domain.GroupRuleStatusActive), int(RuleActionActivate),
				"evaluate expression and enable the rule", ruleActionLabel(RuleActionActivate)},
		}
	case domain.GroupRuleStatusActive:
		return []StatusTransition{
			{string(domain.GroupRuleStatusInactive), int(RuleActionDeactivate),
				"stop evaluating; preserve definition", ruleActionLabel(RuleActionDeactivate)},
		}
	}
	return nil
}

func policyTransitionsFor(current domain.PolicyStatus, system bool) []StatusTransition {
	if system {
		// System policies refuse lifecycle flips — Okta returns 403.
		// Return an empty list so the App Shell can short-circuit to
		// a toast.
		return nil
	}
	switch current {
	case domain.PolicyStatusInactive:
		return []StatusTransition{
			{string(domain.PolicyStatusActive), int(PolicyActionActivate),
				"resume policy evaluation", policyActionLabel(PolicyActionActivate)},
		}
	case domain.PolicyStatusActive:
		return []StatusTransition{
			{string(domain.PolicyStatusInactive), int(PolicyActionDeactivate),
				"stop evaluating; preserve rules", policyActionLabel(PolicyActionDeactivate)},
		}
	}
	return nil
}

func appTransitionsFor(current domain.AppStatus) []StatusTransition {
	switch current {
	case domain.AppStatusInactive:
		return []StatusTransition{
			{string(domain.AppStatusActive), int(AppActionActivate),
				"enable assignments + sign-in", appActionLabel(AppActionActivate)},
		}
	case domain.AppStatusActive:
		return []StatusTransition{
			{string(domain.AppStatusInactive), int(AppActionDeactivate),
				"block sign-in; preserve assignments", appActionLabel(AppActionDeactivate)},
		}
	}
	return nil
}

func authenticatorTransitionsFor(current domain.AuthenticatorStatus) []StatusTransition {
	switch current {
	case domain.AuthenticatorStatusInactive:
		return []StatusTransition{
			{string(domain.AuthenticatorStatusActive), int(AuthenticatorActionActivate),
				"enable factor org-wide for new enrollments", authenticatorActionLabel(AuthenticatorActionActivate)},
		}
	case domain.AuthenticatorStatusActive:
		return []StatusTransition{
			{string(domain.AuthenticatorStatusInactive), int(AuthenticatorActionDeactivate),
				"disable factor; existing enrollments preserved", authenticatorActionLabel(AuthenticatorActionDeactivate)},
		}
	}
	return nil
}

// --- Model lifecycle ------------------------------------------------

// Init implements tea.Model.
func (m StatusPickerModel) Init() tea.Cmd { return nil }

// Update advances the cursor on j/k/↑/↓. Enter and Esc bubble up
// to the App Shell so the dispatcher reads Selected() and routes
// per Surface().
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

// View renders the centered modal — title shows the subject + current
// badge; body lists one row per valid transition.
func (m StatusPickerModel) View() string {
	tk := shared.Dark()
	title := "Change status · " + m.titleSubject + "  (current: " + m.currentBadge + ")"

	var b strings.Builder
	if len(m.transitions) == 0 {
		b.WriteString(tk.Muted.Render("(no transitions available for " + m.currentBadge + ")"))
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

// Empty reports whether there are no transitions available — the
// App Shell short-circuits with a toast in that case.
func (m StatusPickerModel) Empty() bool { return len(m.transitions) == 0 }

// statusActionLabel renders a row label like "→ SUSPENDED  (Suspend
// user)". Target-status badge is the primary signal; the action verb
// is the muted parenthetical so the operator picks the desired
// *state* and only secondarily reads the *verb*.
func statusActionLabel(tr StatusTransition) string {
	if tr.TargetStatus == "" {
		// Delete pseudo-transition — no target status.
		return "✗ " + tr.ActionLabel
	}
	return "→ " + tr.TargetStatus + "  (" + tr.ActionLabel + ")"
}
