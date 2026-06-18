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

	// discardCursor selects which discard-confirm option is
	// highlighted. 0 = "Discard and exit" (destructive), 1 = "Keep
	// editing" (safe default). Defaults to 1 on entry so a stray
	// Enter doesn't destroy work.
	discardCursor int
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
		// Seed the cursor on the safe option so a stray Enter keeps
		// the operator's draft intact (AC-5.2 / D-W4).
		m.discardCursor = 1
		return m, nil
	}
	// Forward to form for everything else (Tab / chars / etc.).
	updated, cmd := m.form.Update(msg)
	m.form = updated
	m.statusMsg = "" // any keypress clears transient feedback
	return m, cmd
}

func (m EditModel) handleDiscardConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Arrow-key picker: ← / → / h / l / Tab move the cursor between
	// the two options; Enter applies the highlighted one. Esc is a
	// shortcut for "keep editing" (the safe default — D-W4).
	// y / Y / n / N stay as muscle-memory accelerators.
	//
	// On confirm-discard the model emits shared.UserEditDiscardedMsg
	// — the App Shell handles it by popping the ScreenUserEdit frame
	// (popNav). EditModel can't pop nav on its own; the message is
	// the routed handoff.
	switch msg.Type {
	case tea.KeyEsc:
		m.state = EditStateEditing
		return m, nil
	case tea.KeyLeft, tea.KeyShiftTab:
		m.discardCursor = 0
		return m, nil
	case tea.KeyRight, tea.KeyTab:
		m.discardCursor = 1
		return m, nil
	case tea.KeyEnter:
		if m.discardCursor == 0 {
			return m, discardAndExitCmd()
		}
		m.state = EditStateEditing
		return m, nil
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "h":
			m.discardCursor = 0
			return m, nil
		case "l":
			m.discardCursor = 1
			return m, nil
		case "y", "Y":
			return m, discardAndExitCmd()
		case "n", "N":
			m.state = EditStateEditing
			return m, nil
		}
	}
	return m, nil
}

// discardAndExitCmd builds the Cmd that emits
// shared.UserEditDiscardedMsg. The App Shell picks it up and pops
// the user-edit frame so the operator lands back on the previous
// screen with their draft thrown away.
func discardAndExitCmd() tea.Cmd {
	return func() tea.Msg { return shared.UserEditDiscardedMsg{} }
}

// View implements tea.Model.
//
// View is the *plain* (no App Shell) rendering — teatest's golden
// loop reads this output directly. Production goes through
// RenderModal via the App Shell's composeBody router (per the v2
// redesign, `_workspace/edit-form-users/redesign/03_tui_design_v2.md`).
// Keep this output stable: tests in `internal/tui/users/edit_test.go`
// search for "First Name", "Discard", "1 change", "Updated", and the
// 400-validation inline error literals via teatest.WaitFor.
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
		b.WriteString("\n\nUnsaved changes — pick: ")
		if m.discardCursor == 0 {
			b.WriteString("[▸ Discard and exit]    Keep editing")
		} else {
			b.WriteString("Discard and exit    [▸ Keep editing]")
		}
	}
	if m.statusMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(m.statusMsg)
	}
	return b.String()
}

// RenderModal is the v2 redesign entrypoint — the App Shell stamps the
// returned modal over a dimmed backdrop of the previous screen
// (D-W17, D-W18). The output is the full MountModal box (rounded
// border + title + body + footer), already cell-correct at `width`.
//
//   - EditStateLoading        → placeholder underscores + spinner footer
//   - EditStateEditing        → live field rows (focus lift), dirty-aware
//                               section headers, single-line footer
//   - EditStateSaving         → non-focused rows + "Saving…" footer
//   - EditStateDiscardConfirm → editing body + strong confirm strip
//                               appended to the modal body
//   - EditStateErrored        → small error modal
//
// bodyBudget is the maximum body line count the chrome can afford
// (clampBodyLines(height) - 4); for now the modal renders the full
// form regardless (viewport scroll is deferred — see §4.3 hint).
func (m EditModel) RenderModal(tk shared.Tokens, width, bodyBudget int) string {
	if width < 60 {
		width = 60
	}
	// Inner content width — subtract MountModal's 1 border + 1 pad +
	// 1 border (3) + a small safety margin for the focus border glyphs.
	contentWidth := width - 3
	if contentWidth < 40 {
		contentWidth = 40
	}
	labelCol := 16
	inputCol := contentWidth - labelCol - 6
	if inputCol < 16 {
		inputCol = 16
	}

	switch m.state {
	case EditStateErrored:
		return shared.MountModal(shared.ModalIn{
			Title:  "Edit User · error",
			Body:   "Cannot edit: " + errorSummary(m.loadErr),
			Footer: "Esc to close",
			Tone:   shared.ModalToneDanger,
			Width:  width,
			Tokens: tk,
		})
	case EditStateLoading:
		body := form.RenderPlaceholderBody(tk, FieldSpecs(), contentWidth, labelCol, inputCol)
		return shared.MountModal(shared.ModalIn{
			Title:  "Edit User · Loading…",
			Body:   body,
			Footer: "Loading from Okta…  ·  Esc cancel",
			Tone:   shared.ModalToneAccent,
			Width:  width,
			Tokens: tk,
		})
	}

	// Editing / Saving / DiscardConfirm — render the live form via
	// the shared form.RenderFormBody helper so every edit screen
	// produces identical chrome from the same Form snapshot.
	body := form.RenderFormBody(tk, m.form, contentWidth, labelCol, inputCol,
		m.state == EditStateEditing, maskPII)
	footer := m.composeFooter(tk)
	title := "Edit User  ·  " + m.loadedUser.Profile.Login

	if m.state == EditStateDiscardConfirm {
		// D-W24 — instead of a separate nested modal stamp (which
		// would require nontrivial coordinate splicing inside the
		// rendered string), we append an emphasized confirm strip to
		// the end of the modal body. The QA-W01-1 regression test
		// looks for "Discard" inside m.View(); this satisfies the
		// AC-5.2 contract while keeping the layout simple.
		strip := "\n" + form.BuildDiscardStrip(tk, m.form.DirtyFields(), contentWidth, m.discardCursor)
		body = body + strip
		footer = "<←/→> select  ·  <Enter> apply  ·  <Esc> keep editing"
	}

	return shared.MountModal(shared.ModalIn{
		Title:  title,
		Body:   body,
		Footer: footer,
		Tone:   shared.ModalToneAccent,
		Width:  width,
		Tokens: tk,
	})
}

// composeFooter delegates to the lifted form.ComposeFooter helper —
// every edit screen uses the same footer wording so the keymap hint
// is consistent across resources.
func (m EditModel) composeFooter(_ shared.Tokens) string {
	var st form.FooterState
	switch m.state {
	case EditStateSaving:
		st = form.FooterStateSaving
	case EditStateDiscardConfirm:
		st = form.FooterStateDiscardConfirm
	default:
		st = form.FooterStateEditing
	}
	return form.ComposeFooter(st, m.form.Dirty(), m.statusMsg)
}

// maskPII produces the form's display masking. Mirrors form.maskString
// but lives here because we render the field row outside the Form's
// own View() path now.
func maskPII(s string) string {
	if s == "" {
		return ""
	}
	return strings.Repeat("•", len([]rune(s)))
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

// openStatusPickerCmd is emitted on `s` from the Users list / detail
// when the operator wants to flip the selected user's lifecycle
// state. Carries the whole User so the picker can render the
// current status badge in its title without another fetch.
func openStatusPickerCmd(u domain.User) tea.Cmd {
	return func() tea.Msg { return shared.OpenStatusPickerMsg{User: u} }
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

