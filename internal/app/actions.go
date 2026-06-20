package app

// Actions — destructive lifecycle dispatch (Users + Group Rules) the
// App Shell gates behind a Danger-toned confirmation modal. Issue #A2
// v0.2.4 — extracted from app.go.

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
)

// openActionConfirm opens the destructive-action gate (issue #125)
// for the currently-selected user. Falls back to a transient toast
// when no Users target is available (e.g., the operator fired the
// command from a non-Users screen).
func (m Model) openActionConfirm(kind UserActionKind) (tea.Model, tea.Cmd) {
	child, ok := m.screens[m.active]
	if !ok {
		return m, toastCmdInfo("no active screen")
	}
	stater, ok := child.(SelectedUserStater)
	if !ok {
		return m, toastCmdInfo("action not available on this screen")
	}
	user, ok := stater.SelectedUser()
	if !ok {
		return m, toastCmdInfo("no user selected")
	}
	m.pendingAction = pendingUserAction{Kind: kind, User: user}
	m.pendingRule = pendingRuleAction{}
	m.overlay = OverlayActionConfirm
	return m, nil
}

// openRuleActionConfirm is the Group Rule sibling (issue #188).
func (m Model) openRuleActionConfirm(kind RuleActionKind) (tea.Model, tea.Cmd) {
	child, ok := m.screens[m.active]
	if !ok {
		return m, toastCmdInfo("no active screen")
	}
	stater, ok := child.(SelectedRuleStater)
	if !ok {
		return m, toastCmdInfo("action not available on this screen")
	}
	rule, ok := stater.SelectedRule()
	if !ok {
		return m, toastCmdInfo("no rule selected")
	}
	m.pendingRule = pendingRuleAction{Kind: kind, Rule: rule}
	m.pendingAction = pendingUserAction{}
	m.overlay = OverlayActionConfirm
	return m, nil
}

// userActionLabel returns a human-readable label for the action kind,
// rendered in the confirmation modal and the post-action toast.
func userActionLabel(k UserActionKind) string {
	switch k {
	case UserActionResetPassword:
		return "Reset password"
	case UserActionUnlock:
		return "Unlock account"
	case UserActionResetFactors:
		return "Reset MFA factors"
	case UserActionActivate:
		return "Activate user"
	case UserActionDeactivate:
		return "Deactivate user"
	case UserActionExpirePassword:
		return "Expire password"
	case UserActionDelete:
		return "Delete user"
	case UserActionSuspend:
		return "Suspend user"
	case UserActionUnsuspend:
		return "Unsuspend user"
	}
	return ""
}

// ruleActionLabel returns a human-readable label for the action.
func ruleActionLabel(k RuleActionKind) string {
	switch k {
	case RuleActionActivate:
		return "Activate rule"
	case RuleActionDeactivate:
		return "Deactivate rule"
	case RuleActionDelete:
		return "Delete rule"
	}
	return ""
}

// policyActionLabel / appActionLabel / authenticatorActionLabel
// produce the confirm-modal headline ("Activate policy for …",
// "Deactivate app for …") for the per-resource status picker
// dispatch paths. Symmetric with userActionLabel / ruleActionLabel.
func policyActionLabel(k PolicyActionKind) string {
	switch k {
	case PolicyActionActivate:
		return "Activate policy"
	case PolicyActionDeactivate:
		return "Deactivate policy"
	}
	return ""
}

func appActionLabel(k AppActionKind) string {
	switch k {
	case AppActionActivate:
		return "Activate app"
	case AppActionDeactivate:
		return "Deactivate app"
	}
	return ""
}

func authenticatorActionLabel(k AuthenticatorActionKind) string {
	switch k {
	case AuthenticatorActionActivate:
		return "Activate authenticator"
	case AuthenticatorActionDeactivate:
		return "Deactivate authenticator"
	}
	return ""
}

// actionCompletedMsg wraps the toast a successful action emits so
// the App Shell can chain a screen refresh in the same Update pass
// (issue #192 v0.2.3). The handler turns it into a ToastMsg AND a
// RefreshScreenMsg via tea.Batch.
type actionCompletedMsg struct {
	toast ToastMsg
}

// actionFailedMsg pairs the error toast with the failed resource's
// ID so the App Shell can broadcast a shared.ActionFailedMsg to the
// active list, which flashes the row red (#U11 v0.2.4).
type actionFailedMsg struct {
	toast    ToastMsg
	targetID string
}

// runUserActionCmd dispatches the active pendingAction against the
// UsersPort and emits an actionCompletedMsg with the toast. The Cmd
// returns nil when called without a wired UsersPort so tests can
// drive the flow without a network round-trip.
func runUserActionCmd(port domain.UsersPort, action pendingUserAction) tea.Cmd {
	if port == nil {
		return toastCmdInfo("UsersPort not wired — action skipped")
	}
	return func() tea.Msg {
		ctx := context.Background()
		login := action.User.Profile.Login
		if login == "" {
			login = action.User.ID
		}
		fail := func(msg string, err error) tea.Msg {
			return actionFailedMsg{
				toast:    toastErr(msg + ": " + err.Error()),
				targetID: action.User.ID,
			}
		}
		switch action.Kind {
		case UserActionResetPassword:
			if _, err := port.ResetPassword(ctx, action.User.ID, true); err != nil {
				return fail("reset password failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("reset password email sent to " + login)}
		case UserActionUnlock:
			if err := port.Unlock(ctx, action.User.ID); err != nil {
				return fail("unlock failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("unlocked " + login)}
		case UserActionResetFactors:
			if err := port.ResetFactors(ctx, action.User.ID); err != nil {
				return fail("reset MFA failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("MFA factors reset for " + login)}
		case UserActionActivate:
			if err := port.Activate(ctx, action.User.ID, true); err != nil {
				return fail("activate failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("activated " + login)}
		case UserActionDeactivate:
			if err := port.Deactivate(ctx, action.User.ID, false); err != nil {
				return fail("deactivate failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("deactivated " + login)}
		case UserActionExpirePassword:
			if err := port.ExpirePassword(ctx, action.User.ID); err != nil {
				return fail("expire password failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("password expired for " + login)}
		case UserActionDelete:
			if err := port.Delete(ctx, action.User.ID); err != nil {
				return fail("delete failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("deleted " + login)}
		case UserActionSuspend:
			if err := port.Suspend(ctx, action.User.ID); err != nil {
				return fail("suspend failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("suspended " + login)}
		case UserActionUnsuspend:
			if err := port.Unsuspend(ctx, action.User.ID); err != nil {
				return fail("unsuspend failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("unsuspended " + login)}
		}
		return nil
	}
}

// runRuleActionCmd dispatches the active pendingRule against the
// GroupRulesPort and emits a toast with the result.
func runRuleActionCmd(port domain.GroupRulesPort, action pendingRuleAction) tea.Cmd {
	if port == nil {
		return toastCmdInfo("GroupRulesPort not wired — action skipped")
	}
	return func() tea.Msg {
		ctx := context.Background()
		name := action.Rule.Name
		if name == "" {
			name = action.Rule.ID
		}
		fail := func(msg string, err error) tea.Msg {
			return actionFailedMsg{
				toast:    toastErr(msg + ": " + err.Error()),
				targetID: action.Rule.ID,
			}
		}
		switch action.Kind {
		case RuleActionActivate:
			if err := port.Activate(ctx, action.Rule.ID); err != nil {
				return fail("activate rule failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("activated rule " + name)}
		case RuleActionDeactivate:
			if err := port.Deactivate(ctx, action.Rule.ID); err != nil {
				return fail("deactivate rule failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("deactivated rule " + name)}
		case RuleActionDelete:
			if err := port.Delete(ctx, action.Rule.ID); err != nil {
				return fail("delete rule failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("deleted rule " + name)}
		}
		return nil
	}
}

// runPolicyActionCmd dispatches the active pendingPolicy against the
// PoliciesPort. Status picker only emits Activate / Deactivate
// today; system policies refuse with 403 which surfaces as the
// standard error toast.
func runPolicyActionCmd(port domain.PoliciesPort, action pendingPolicyAction) tea.Cmd {
	if port == nil {
		return toastCmdInfo("PoliciesPort not wired — action skipped")
	}
	return func() tea.Msg {
		ctx := context.Background()
		name := action.Policy.Name
		if name == "" {
			name = action.Policy.ID
		}
		fail := func(msg string, err error) tea.Msg {
			return actionFailedMsg{
				toast:    toastErr(msg + ": " + err.Error()),
				targetID: action.Policy.ID,
			}
		}
		switch action.Kind {
		case PolicyActionActivate:
			if err := port.Activate(ctx, action.Policy.ID); err != nil {
				return fail("activate policy failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("activated policy " + name)}
		case PolicyActionDeactivate:
			if err := port.Deactivate(ctx, action.Policy.ID); err != nil {
				return fail("deactivate policy failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("deactivated policy " + name)}
		}
		return nil
	}
}

// runAppActionCmd dispatches the active pendingApp against the
// AppsPort.
func runAppActionCmd(port domain.AppsPort, action pendingAppAction) tea.Cmd {
	if port == nil {
		return toastCmdInfo("AppsPort not wired — action skipped")
	}
	return func() tea.Msg {
		ctx := context.Background()
		label := action.App.Label
		if label == "" {
			label = action.App.ID
		}
		fail := func(msg string, err error) tea.Msg {
			return actionFailedMsg{
				toast:    toastErr(msg + ": " + err.Error()),
				targetID: action.App.ID,
			}
		}
		switch action.Kind {
		case AppActionActivate:
			if err := port.Activate(ctx, action.App.ID); err != nil {
				return fail("activate app failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("activated app " + label)}
		case AppActionDeactivate:
			if err := port.Deactivate(ctx, action.App.ID); err != nil {
				return fail("deactivate app failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("deactivated app " + label)}
		}
		return nil
	}
}

// runAuthenticatorActionCmd dispatches the active pendingAuthenticator
// against the AuthenticatorsPort. Org-wide change — the toast names
// the factor (e.g., "activated authenticator okta_verify").
func runAuthenticatorActionCmd(port domain.AuthenticatorsPort, action pendingAuthenticatorAction) tea.Cmd {
	if port == nil {
		return toastCmdInfo("AuthenticatorsPort not wired — action skipped")
	}
	return func() tea.Msg {
		ctx := context.Background()
		name := action.Authenticator.Name
		if name == "" {
			name = action.Authenticator.ID
		}
		fail := func(msg string, err error) tea.Msg {
			return actionFailedMsg{
				toast:    toastErr(msg + ": " + err.Error()),
				targetID: action.Authenticator.ID,
			}
		}
		switch action.Kind {
		case AuthenticatorActionActivate:
			if err := port.Activate(ctx, action.Authenticator.ID); err != nil {
				return fail("activate authenticator failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("activated authenticator " + name)}
		case AuthenticatorActionDeactivate:
			if err := port.Deactivate(ctx, action.Authenticator.ID); err != nil {
				return fail("deactivate authenticator failed", err)
			}
			return actionCompletedMsg{toast: toastInfo("deactivated authenticator " + name)}
		}
		return nil
	}
}
