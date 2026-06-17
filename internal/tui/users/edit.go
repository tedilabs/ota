package users

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/shared/form"
)

// EditState classifies the high-level lifecycle phase of EditModel
// (TUI_DESIGN §2.3 state machine).
type EditState int

// EditState lifecycle values (REQ-W01 / TUI_DESIGN §2.3).
const (
	// EditStateLoading is the initial state — GET /api/v1/users/{id}
	// in flight. Esc aborts and pops nav. 4xx blocks form open.
	EditStateLoading EditState = iota
	// EditStateEditing is the live form state. Esc → discard confirm
	// if dirty>0, else immediate popNav. Ctrl+S → save when valid.
	EditStateEditing
	// EditStateSaving is the POST /api/v1/users/{id} in flight.
	// All input disabled; only Ctrl+C aborts (preserves draft).
	EditStateSaving
	// EditStateDiscardConfirm shows the y/N overlay when the
	// operator pressed Esc on a dirty form (AC-5.2).
	EditStateDiscardConfirm
	// EditStateErrored is the terminal state when the initial Get
	// returned 4xx — form is NOT built (AC-1.5).
	EditStateErrored
)

// EditDeps bundles the EditModel's required dependencies (CONVENTIONS
// §8.1 — Deps struct pattern). REQ-W01 / D-T3.
type EditDeps struct {
	// Svc is the service-layer entry point. EditModel calls Get on
	// entry and UpdateProfile on save.
	Svc *service.UsersService
	// UserID is the resource the form is editing. Required.
	UserID string
	// Clock is optional — falls back to clock.Real.
	Clock clock.Clock
	// Logger is optional — Phase 6 wires debug logging via
	// internal/logger's masking handler.
	Logger *slog.Logger
	// Width/Height are the initial terminal dimensions. EditModel
	// updates them via tea.WindowSizeMsg.
	Width  int
	Height int
}

// EditModel is the Bubbletea tea.Model for SCR-012 Users Edit Form
// (REQ-W01). It composes a `form.Form` widget for the 11-field
// catalog and orchestrates the fetch → edit → save lifecycle.
type EditModel struct {
	deps  EditDeps
	state EditState
	form  form.Form

	// loadedUser is the snapshot the form was built from. Used to
	// reconstruct the patch on save and to surface PII display.
	loadedUser domain.User

	// statusMsg surfaces transient outcome strings ("Updated",
	// "Insufficient permissions", etc.). Cleared on the next keystroke.
	statusMsg string

	// loadErr stores the failure from the initial GET — drives the
	// AC-1.5 "do not open form" branch and the AC-6 toast text.
	loadErr error

	// saveErr stores the failure from UpdateProfile — drives the
	// post-save inline error / toast rendering.
	saveErr error
}

// NewEditModel constructs an EditModel.
func NewEditModel(deps EditDeps) EditModel {
	return EditModel{
		deps:  deps,
		state: EditStateLoading,
	}
}

// State exposes the current lifecycle phase — used by tests and the
// App Shell's overlay router.
func (m EditModel) State() EditState { return m.state }

// Form exposes the embedded form widget — used by tests to inspect
// Dirty/DirtyFields/Snapshot/Diff.
func (m EditModel) Form() form.Form { return m.form }

// UserID reports the user the form is editing — used by the App
// Shell to route UserUpdatedMsg to the right cache slot.
func (m EditModel) UserID() string { return m.deps.UserID }

// Init implements tea.Model — fires the initial GET /api/v1/users/{id}
// (AC-1.3).
func (m EditModel) Init() tea.Cmd {
	if m.deps.Svc == nil || m.deps.UserID == "" {
		return nil
	}
	return fetchUserForEditCmd(m.deps.Svc, m.deps.UserID)
}

// Update implements tea.Model.
func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Ctrl+C: hard quit — mirrors users/list.go pattern. When EditModel
	// runs as the teatest root (no App Shell wrapping it) Ctrl+C is the
	// only way to drain teatest's FinalOutput. The App Shell intercepts
	// Ctrl+C earlier in production and routes it to the QuitConfirm
	// overlay.
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	switch msg := msg.(type) {
	case userEditLoadedMsg:
		m.loadedUser = msg.user
		m.form = form.New(FieldSpecs(), profileToInitial(msg.user))
		m.state = EditStateEditing
		return m, nil
	case userEditLoadFailedMsg:
		m.loadErr = msg.err
		m.state = EditStateErrored
		return m, nil
	case userEditSaveSucceededMsg:
		m.loadedUser = msg.user
		m.saveErr = nil
		m.state = EditStateEditing
		m.form = m.form.SetSaving(false)
		m.statusMsg = "Updated " + displayName(msg.user)
		broadcast := func() tea.Msg { return shared.UserUpdatedMsg{User: msg.user} }
		return m, broadcast
	case userEditSaveFailedMsg:
		m.saveErr = msg.err
		m.state = EditStateEditing
		m.form = m.form.SetSaving(false)
		// Map *BadRequestError → inline errors. Other errors stay in
		// statusMsg footer.
		var bre *domain.BadRequestError
		if errors.As(msg.err, &bre) {
			m.form = m.form.ApplyServerErrors(bre.Causes)
			m.statusMsg = ""
		} else {
			m.statusMsg = "Save failed: " + msg.err.Error()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.deps.Width = msg.Width
		m.deps.Height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes keystrokes per state.
func (m EditModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case EditStateLoading:
		// Loading: Esc cancels (handled at App Shell via nav pop);
		// other keys are ignored. Ctrl+C still flows up to teatest.
		return m, nil
	case EditStateErrored:
		// 4xx: do not open form. Any key forwards (App Shell-level
		// Esc handler pops nav).
		return m, nil
	case EditStateSaving:
		// All input disabled while saving (AC-4.3 / AC-5.3). Ctrl+C
		// still aborts at the App Shell level.
		return m, nil
	case EditStateDiscardConfirm:
		return m.handleDiscardConfirm(msg)
	}

	// Editing — global shortcuts first, then forward to form.
	switch msg.Type {
	case tea.KeyCtrlS:
		// Save: validate locally, then dispatch UpdateProfile if
		// dirty>0 (AC-4.1). Empty Diff() short-circuits — no API
		// hit, no toast.
		if m.form.Dirty() == 0 {
			return m, nil
		}
		ok, firstInvalid := m.form.Validate()
		if !ok {
			m.form = m.form.Focus(firstInvalid)
			m.statusMsg = "Fix the highlighted field"
			return m, nil
		}
		patch := buildPatch(m.form.Diff())
		m.state = EditStateSaving
		m.form = m.form.SetSaving(true)
		m.statusMsg = ""
		return m, saveProfileCmd(m.deps.Svc, m.deps.UserID, patch)
	case tea.KeyEsc:
		// Dirty → discard confirm; clean → close (App Shell pops nav
		// via canPopNav). The App Shell-level Esc handler will only
		// reach this branch when escIsCritical / overlay logic lets
		// it; for now we emit a no-op cmd and let the shell pop nav.
		if m.form.Dirty() == 0 {
			return m, nil
		}
		m.state = EditStateDiscardConfirm
		return m, nil
	}
	// Forward to form for everything else (Tab / chars / etc.).
	updated, cmd := m.form.Update(msg)
	m.form = updated
	m.statusMsg = "" // any keypress clears transient feedback
	return m, cmd
}

func (m EditModel) handleDiscardConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Esc on confirm = "no, keep editing" (D-W4).
		m.state = EditStateEditing
		return m, nil
	case tea.KeyEnter:
		// Enter = confirm discard.
		return m, func() tea.Msg { return form.DiscardRequestedMsg{Confirmed: true} }
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "y", "Y":
			return m, func() tea.Msg { return form.DiscardRequestedMsg{Confirmed: true} }
		case "n", "N":
			m.state = EditStateEditing
			return m, nil
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m EditModel) View() string {
	switch m.state {
	case EditStateLoading:
		return "Loading user profile…"
	case EditStateErrored:
		// AC-1.5 — do NOT render form chrome or any field labels.
		// Surface a brief error so the operator knows why the form
		// didn't open; the App Shell-level toast renders alongside.
		return "Cannot edit: " + errorSummary(m.loadErr)
	}

	var b strings.Builder
	b.WriteString("Edit User · ")
	b.WriteString(m.loadedUser.Profile.Login)
	b.WriteString("\n\n")
	b.WriteString(m.form.View())
	if m.state == EditStateDiscardConfirm {
		b.WriteString("\n\nDiscard unsaved changes? (y/N)")
	}
	if m.statusMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(m.statusMsg)
	}
	return b.String()
}

// EscapeWillAct implements the App Shell's EscapeOpStater contract.
// During edit/discard states Esc has meaning (open discard confirm,
// close it); during loading/saving/errored Esc lets the shell pop nav.
func (m EditModel) EscapeWillAct() bool {
	switch m.state {
	case EditStateDiscardConfirm:
		return true
	case EditStateEditing:
		return m.form.Dirty() > 0
	}
	return false
}

// EscapeBlocksPop implements the App Shell's escapeBlocksPopStater
// contract. Returns true when Esc must short-circuit the nav-pop —
// REQ-W01 AC-5.2 (dirty form → discard confirm comes first) and
// AC-4.3 / AC-5.3 (saving → Esc is inert; only Ctrl+C aborts).
func (m EditModel) EscapeBlocksPop() bool {
	switch m.state {
	case EditStateSaving, EditStateDiscardConfirm:
		return true
	case EditStateEditing:
		return m.form.Dirty() > 0
	}
	return false
}

// FieldSpecs returns the production 11-field FieldSpec catalog for
// REQ-W01 AC-2. Exported so the App Shell / tests can reference the
// canonical shape.
func FieldSpecs() []form.FieldSpec {
	return []form.FieldSpec{
		// Identity
		{Key: "login", Label: "Login", Kind: form.KindReadOnly, Section: "Identity"},
		{Key: "firstName", Label: "First Name", Kind: form.KindText, Required: true, Section: "Identity"},
		{Key: "lastName", Label: "Last Name", Kind: form.KindText, Required: true, Section: "Identity"},
		{Key: "displayName", Label: "Display Name", Kind: form.KindText, Section: "Identity"},
		{Key: "nickName", Label: "Nickname", Kind: form.KindText, Section: "Identity"},
		// Contact
		{Key: "email", Label: "Email", Kind: form.KindEmail, Required: true, Section: "Contact"},
		{Key: "mobilePhone", Label: "Mobile Phone", Kind: form.KindPhone, PII: true, Section: "Contact"},
		{Key: "secondEmail", Label: "Secondary Email", Kind: form.KindEmail, PII: true, Section: "Contact"},
		// Organization
		{Key: "title", Label: "Title", Kind: form.KindText, Section: "Organization"},
		{Key: "division", Label: "Division", Kind: form.KindText, Section: "Organization"},
		{Key: "department", Label: "Department", Kind: form.KindText, Section: "Organization"},
		{Key: "employeeNumber", Label: "Employee Number", Kind: form.KindText, Section: "Organization"},
	}
}

// profileToInitial materializes the initial value map from a User. The
// 11 fields match FieldSpecs() — read-only `login` is included so the
// form can render it.
func profileToInitial(u domain.User) map[string]string {
	return map[string]string{
		"login":          u.Profile.Login,
		"firstName":      u.Profile.FirstName,
		"lastName":       u.Profile.LastName,
		"displayName":    u.Profile.DisplayName,
		"nickName":       u.Profile.NickName,
		"email":          u.Profile.Email,
		"mobilePhone":    u.Profile.MobilePhone,
		"secondEmail":    u.Profile.SecondEmail,
		"title":          u.Profile.Title,
		"division":       u.Profile.Division,
		"department":     u.Profile.Department,
		"employeeNumber": u.Profile.EmployeeNumber,
	}
}

// buildPatch assembles a sparse *string patch from the form's
// Diff()-output map. Only keys present in the diff become non-nil
// pointers (D-T4 / AC-4.2).
func buildPatch(diff map[string]string) domain.UserProfilePatch {
	var p domain.UserProfilePatch
	for k, v := range diff {
		v := v // local copy so the address survives the loop
		switch k {
		case "firstName":
			p.FirstName = &v
		case "lastName":
			p.LastName = &v
		case "displayName":
			p.DisplayName = &v
		case "nickName":
			p.NickName = &v
		case "email":
			p.Email = &v
		case "mobilePhone":
			p.MobilePhone = &v
		case "secondEmail":
			p.SecondEmail = &v
		case "title":
			p.Title = &v
		case "division":
			p.Division = &v
		case "department":
			p.Department = &v
		case "employeeNumber":
			p.EmployeeNumber = &v
		}
	}
	return p
}

// displayName picks the human label for the post-save toast.
func displayName(u domain.User) string {
	if u.Profile.DisplayName != "" {
		return u.Profile.DisplayName
	}
	if u.Profile.Login != "" {
		return u.Profile.Login
	}
	return u.ID
}

// errorSummary picks a short error label for the errored-state body.
func errorSummary(err error) string {
	if err == nil {
		return "unknown"
	}
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return "insufficient permissions"
	case errors.Is(err, domain.ErrNotFound):
		return "user not found"
	}
	return err.Error()
}

// --- Cmd factories --------------------------------------------------

type userEditLoadedMsg struct{ user domain.User }
type userEditLoadFailedMsg struct{ err error }
type userEditSaveSucceededMsg struct{ user domain.User }
type userEditSaveFailedMsg struct{ err error }

// openUserEditCmd is emitted by the Users list / detail when the
// operator presses `e` (REQ-W01 AC-1.1 / AC-1.2). The App Shell
// converts it into a ScreenUserEdit push.
func openUserEditCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenUserEditMsg{ID: id} }
}

// fetchUserForEditCmd is the AC-1.3 single GET on entry.
func fetchUserForEditCmd(svc *service.UsersService, id string) tea.Cmd {
	return func() tea.Msg {
		u, err := svc.Get(context.Background(), id)
		if err != nil {
			return userEditLoadFailedMsg{err: err}
		}
		return userEditLoadedMsg{user: u}
	}
}

// saveProfileCmd dispatches the partial-merge POST via the service
// (AC-4.1 / AC-4.2).
func saveProfileCmd(svc *service.UsersService, id string, patch domain.UserProfilePatch) tea.Cmd {
	return func() tea.Msg {
		u, err := svc.UpdateProfile(context.Background(), id, patch)
		if err != nil {
			return userEditSaveFailedMsg{err: err}
		}
		return userEditSaveSucceededMsg{user: u}
	}
}

