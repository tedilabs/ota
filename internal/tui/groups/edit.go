package groups

// Group Edit Form — parallels users.EditModel (REQ-W01 / SCR-012)
// but trimmed to the two editable GroupProfile fields (name +
// description). Mounted as a v2 popup-over-dimmed-body via the
// App Shell's ModalRenderer interface.
//
// Constraint: only OKTA_GROUP type accepts profile updates.
// APP_GROUP / BUILT_IN are upstream-managed and return 403 from
// Okta. The Groups list / detail key handler guards on Type before
// emitting shared.OpenGroupEditMsg so the form never opens for a
// read-only group.

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

// EditState classifies the high-level lifecycle phase of EditModel.
type EditState int

const (
	EditStateLoading EditState = iota
	EditStateEditing
	EditStateSaving
	EditStateDiscardConfirm
	EditStateErrored
)

// EditDeps wires the screen to the service layer.
type EditDeps struct {
	Svc     *service.GroupsService
	GroupID string
	Clock   clock.Clock
	Logger  *slog.Logger
	Width   int
	Height  int
}

// EditModel is the tea.Model for the group profile edit form.
type EditModel struct {
	deps  EditDeps
	state EditState
	form  form.Form

	loadedGroup domain.Group
	statusMsg   string
	loadErr     error
	saveErr     error

	// discardCursor selects which discard-confirm option is
	// highlighted. 0 = "Discard and exit", 1 = "Keep editing"
	// (safe default — stray Enter won't destroy work).
	discardCursor int
}

// NewEditModel constructs an EditModel.
func NewEditModel(deps EditDeps) EditModel {
	return EditModel{deps: deps, state: EditStateLoading}
}

// State exposes the current lifecycle phase.
func (m EditModel) State() EditState { return m.state }

// Form exposes the embedded form widget.
func (m EditModel) Form() form.Form { return m.form }

// GroupID reports the resource the form targets.
func (m EditModel) GroupID() string { return m.deps.GroupID }

// Init fires the initial GET.
func (m EditModel) Init() tea.Cmd {
	if m.deps.Svc == nil || m.deps.GroupID == "" {
		return nil
	}
	return fetchGroupForEditCmd(m.deps.Svc, m.deps.GroupID)
}

// Update implements tea.Model.
func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	switch msg := msg.(type) {
	case groupEditLoadedMsg:
		m.loadedGroup = msg.group
		m.form = form.New(GroupFieldSpecs(), groupProfileToInitial(msg.group))
		m.state = EditStateEditing
		return m, nil
	case groupEditLoadFailedMsg:
		m.loadErr = msg.err
		m.state = EditStateErrored
		return m, nil
	case groupEditSaveSucceededMsg:
		m.loadedGroup = msg.group
		m.saveErr = nil
		m.state = EditStateEditing
		m.form = m.form.SetSaving(false)
		m.statusMsg = "Updated " + groupDisplay(msg.group)
		broadcast := func() tea.Msg { return shared.GroupUpdatedMsg{Group: msg.group} }
		return m, broadcast
	case groupEditSaveFailedMsg:
		m.saveErr = msg.err
		m.state = EditStateEditing
		m.form = m.form.SetSaving(false)
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
	case EditStateLoading, EditStateErrored, EditStateSaving:
		return m, nil
	case EditStateDiscardConfirm:
		return m.handleDiscardConfirm(msg)
	}

	switch msg.Type {
	case tea.KeyCtrlS:
		if m.form.Dirty() == 0 {
			return m, nil
		}
		ok, firstInvalid := m.form.Validate()
		if !ok {
			m.form = m.form.Focus(firstInvalid)
			m.statusMsg = "Fix the highlighted field"
			return m, nil
		}
		update := buildGroupUpdate(m.loadedGroup, m.form.Current())
		m.state = EditStateSaving
		m.form = m.form.SetSaving(true)
		m.statusMsg = ""
		return m, saveGroupCmd(m.deps.Svc, m.deps.GroupID, update)
	case tea.KeyEsc:
		if m.form.Dirty() == 0 {
			return m, nil
		}
		m.state = EditStateDiscardConfirm
		m.discardCursor = 1
		return m, nil
	}
	updated, cmd := m.form.Update(msg)
	m.form = updated
	m.statusMsg = ""
	return m, cmd
}

func (m EditModel) handleDiscardConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// View is the teatest-friendly plain rendering.
func (m EditModel) View() string {
	switch m.state {
	case EditStateLoading:
		return "Loading group profile…"
	case EditStateErrored:
		return "Cannot edit: " + errorSummary(m.loadErr)
	}
	var b strings.Builder
	b.WriteString("Edit Group · ")
	b.WriteString(m.loadedGroup.Profile.Name)
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

// RenderModal implements app.ModalRenderer — the v2 popup-over-
// dimmed-body entrypoint.
func (m EditModel) RenderModal(tk shared.Tokens, width, bodyBudget int) string {
	_ = bodyBudget
	if width < 60 {
		width = 60
	}
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
			Title:  "Edit Group · error",
			Body:   "Cannot edit: " + errorSummary(m.loadErr),
			Footer: "Esc to close",
			Tone:   shared.ModalToneDanger,
			Width:  width,
			Tokens: tk,
		})
	case EditStateLoading:
		body := form.RenderPlaceholderBody(tk, GroupFieldSpecs(), contentWidth, labelCol, inputCol)
		return shared.MountModal(shared.ModalIn{
			Title:  "Edit Group · Loading…",
			Body:   body,
			Footer: "Loading from Okta…  ·  Esc cancel",
			Tone:   shared.ModalToneAccent,
			Width:  width,
			Tokens: tk,
		})
	}

	body := form.RenderFormBody(tk, m.form, contentWidth, labelCol, inputCol,
		m.state == EditStateEditing, nil)
	footer := m.composeFooter()
	title := "Edit Group  ·  " + groupDisplay(m.loadedGroup)

	if m.state == EditStateDiscardConfirm {
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

func (m EditModel) composeFooter() string {
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

// EscapeWillAct implements the App Shell's EscapeOpStater contract.
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
// contract — same semantics as users.EditModel.
func (m EditModel) EscapeBlocksPop() bool {
	switch m.state {
	case EditStateSaving, EditStateDiscardConfirm:
		return true
	case EditStateEditing:
		return m.form.Dirty() > 0
	}
	return false
}

// GroupFieldSpecs is the editable-field catalog for OKTA_GROUP
// profiles. Only Identity section; no PII fields. Required: name.
func GroupFieldSpecs() []form.FieldSpec {
	return []form.FieldSpec{
		{Key: "name", Label: "Name", Kind: form.KindText, Required: true, Section: "Identity", MaxLen: 255},
		{Key: "description", Label: "Description", Kind: form.KindText, Section: "Identity", MaxLen: 1024},
	}
}

func groupProfileToInitial(g domain.Group) map[string]string {
	return map[string]string{
		"name":        g.Profile.Name,
		"description": g.Profile.Description,
	}
}

// buildGroupUpdate constructs the strict-replace body from the form
// snapshot. Okta's PUT requires every field present, so we read all
// of them from current (which reflects the loaded snapshot for
// unchanged fields).
func buildGroupUpdate(_ domain.Group, current map[string]string) domain.GroupProfileUpdate {
	return domain.GroupProfileUpdate{
		Name:        current["name"],
		Description: current["description"],
	}
}

func groupDisplay(g domain.Group) string {
	if g.Profile.Name != "" {
		return g.Profile.Name
	}
	return g.ID
}

func errorSummary(err error) string {
	if err == nil {
		return "(unknown error)"
	}
	msg := err.Error()
	if len(msg) > 80 {
		return msg[:77] + "…"
	}
	return msg
}

// --- Messages -------------------------------------------------------

type groupEditLoadedMsg struct{ group domain.Group }
type groupEditLoadFailedMsg struct{ err error }
type groupEditSaveSucceededMsg struct{ group domain.Group }
type groupEditSaveFailedMsg struct{ err error }

// OpenGroupEditCmd is emitted by the Groups list / detail when the
// operator presses `e` on an OKTA_GROUP row. Caller guards on Type
// (the App Shell route handler also re-checks).
func OpenGroupEditCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenGroupEditMsg{ID: id} }
}

func fetchGroupForEditCmd(svc *service.GroupsService, id string) tea.Cmd {
	return func() tea.Msg {
		g, err := svc.Get(context.Background(), id)
		if err != nil {
			return groupEditLoadFailedMsg{err: err}
		}
		return groupEditLoadedMsg{group: g}
	}
}

func saveGroupCmd(svc *service.GroupsService, id string, profile domain.GroupProfileUpdate) tea.Cmd {
	return func() tea.Msg {
		g, err := svc.UpdateProfile(context.Background(), id, profile)
		if err != nil {
			return groupEditSaveFailedMsg{err: err}
		}
		return groupEditSaveSucceededMsg{group: g}
	}
}

func discardAndExitCmd() tea.Cmd {
	return func() tea.Msg { return shared.GroupEditDiscardedMsg{} }
}
