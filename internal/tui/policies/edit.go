package policies

// Policy Edit Form — parallels groups.EditModel / rules.EditModel
// but targets the Policy resource. v0.2 scope: name + description +
// priority (int) + status. Rule editing (priority, conditions inside
// individual PolicyRules) is deferred — the form edits the policy's
// metadata only.
//
// System policies refuse status / priority changes upstream; the
// 400 returned by Okta is mapped onto the field inline via the
// standard ErrorMapper.

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/shared/form"
)

type EditState int

const (
	EditStateLoading EditState = iota
	EditStateEditing
	EditStateSaving
	EditStateDiscardConfirm
	EditStateErrored
)

type EditDeps struct {
	Svc      *service.PoliciesService
	PolicyID string
	Clock    clock.Clock
	Logger   *slog.Logger
	Width    int
	Height   int
}

type EditModel struct {
	deps  EditDeps
	state EditState
	form  form.Form

	loadedPolicy domain.Policy
	statusMsg    string
	loadErr      error
	saveErr      error

	discardCursor int
}

func NewEditModel(deps EditDeps) EditModel {
	return EditModel{deps: deps, state: EditStateLoading}
}

func (m EditModel) State() EditState { return m.state }
func (m EditModel) Form() form.Form  { return m.form }
func (m EditModel) PolicyID() string { return m.deps.PolicyID }

func (m EditModel) Init() tea.Cmd {
	if m.deps.Svc == nil || m.deps.PolicyID == "" {
		return nil
	}
	return fetchPolicyForEditCmd(m.deps.Svc, m.deps.PolicyID)
}

func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	switch msg := msg.(type) {
	case policyEditLoadedMsg:
		m.loadedPolicy = msg.policy
		m.form = form.New(PolicyFieldSpecs(), policyToInitial(msg.policy))
		m.state = EditStateEditing
		return m, nil
	case policyEditLoadFailedMsg:
		m.loadErr = msg.err
		m.state = EditStateErrored
		return m, nil
	case policyEditSaveSucceededMsg:
		m.loadedPolicy = msg.policy
		m.saveErr = nil
		m.state = EditStateEditing
		m.form = m.form.SetSaving(false)
		m.statusMsg = "Updated " + policyDisplay(msg.policy)
		broadcast := func() tea.Msg { return shared.PolicyUpdatedMsg{Policy: msg.policy} }
		return m, broadcast
	case policyEditSaveFailedMsg:
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
		update, err := buildPolicyUpdate(m.loadedPolicy, m.form.Current())
		if err != nil {
			m.form = m.form.Focus("priority")
			m.statusMsg = "Priority must be an integer between 1 and 100"
			return m, nil
		}
		m.state = EditStateSaving
		m.form = m.form.SetSaving(true)
		m.statusMsg = ""
		return m, savePolicyCmd(m.deps.Svc, m.deps.PolicyID, update)
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

func (m EditModel) View() string {
	switch m.state {
	case EditStateLoading:
		return "Loading policy…"
	case EditStateErrored:
		return "Cannot edit: " + errorSummary(m.loadErr)
	}
	var b strings.Builder
	b.WriteString("Edit Policy · ")
	b.WriteString(m.loadedPolicy.Name)
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
			Title:  "Edit Policy · error",
			Body:   "Cannot edit: " + errorSummary(m.loadErr),
			Footer: "Esc to close",
			Tone:   shared.ModalToneDanger,
			Width:  width,
			Tokens: tk,
		})
	case EditStateLoading:
		body := form.RenderPlaceholderBody(tk, PolicyFieldSpecs(), contentWidth, labelCol, inputCol)
		return shared.MountModal(shared.ModalIn{
			Title:  "Edit Policy · Loading…",
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
	title := "Edit Policy  ·  " + policyDisplay(m.loadedPolicy)

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

func (m EditModel) EscapeWillAct() bool {
	switch m.state {
	case EditStateDiscardConfirm:
		return true
	case EditStateEditing:
		return m.form.Dirty() > 0
	}
	return false
}

func (m EditModel) EscapeBlocksPop() bool {
	switch m.state {
	case EditStateSaving, EditStateDiscardConfirm:
		return true
	case EditStateEditing:
		return m.form.Dirty() > 0
	}
	return false
}

// PolicyFieldSpecs lists the editable fields.
func PolicyFieldSpecs() []form.FieldSpec {
	return []form.FieldSpec{
		{Key: "name", Label: "Name", Kind: form.KindText, Required: true, Section: "Identity", MaxLen: 255},
		{Key: "description", Label: "Description", Kind: form.KindText, Section: "Identity", MaxLen: 1024},
		{Key: "priority", Label: "Priority", Kind: form.KindText, Required: true, Section: "Behavior", MaxLen: 4},
		{Key: "status", Label: "Status", Kind: form.KindText, Required: true, Section: "Behavior", MaxLen: 8},
	}
}

func policyToInitial(p domain.Policy) map[string]string {
	return map[string]string{
		"name":        p.Name,
		"description": p.Description,
		"priority":    strconv.Itoa(p.Priority),
		"status":      string(p.Status),
	}
}

// buildPolicyUpdate constructs the strict-replace body. Errors when
// priority isn't a valid integer (1..100) — the caller redirects the
// operator to fix the field before the save fires.
func buildPolicyUpdate(_ domain.Policy, current map[string]string) (domain.PolicyUpdate, error) {
	pr, err := strconv.Atoi(strings.TrimSpace(current["priority"]))
	if err != nil || pr < 1 || pr > 100 {
		return domain.PolicyUpdate{}, errors.New("invalid priority")
	}
	return domain.PolicyUpdate{
		Name:        current["name"],
		Description: current["description"],
		Priority:    pr,
		Status:      domain.PolicyStatus(strings.ToUpper(strings.TrimSpace(current["status"]))),
	}, nil
}

func policyDisplay(p domain.Policy) string {
	if p.Name != "" {
		return p.Name
	}
	return p.ID
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

type policyEditLoadedMsg struct{ policy domain.Policy }
type policyEditLoadFailedMsg struct{ err error }
type policyEditSaveSucceededMsg struct{ policy domain.Policy }
type policyEditSaveFailedMsg struct{ err error }

// OpenPolicyEditCmd is emitted by the Policies list / detail on `e`.
func OpenPolicyEditCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenPolicyEditMsg{ID: id} }
}

func fetchPolicyForEditCmd(svc *service.PoliciesService, id string) tea.Cmd {
	return func() tea.Msg {
		p, err := svc.Get(context.Background(), id)
		if err != nil {
			return policyEditLoadFailedMsg{err: err}
		}
		return policyEditLoadedMsg{policy: p}
	}
}

func savePolicyCmd(svc *service.PoliciesService, id string, update domain.PolicyUpdate) tea.Cmd {
	return func() tea.Msg {
		p, err := svc.UpdatePolicy(context.Background(), id, update)
		if err != nil {
			return policyEditSaveFailedMsg{err: err}
		}
		return policyEditSaveSucceededMsg{policy: p}
	}
}

func discardAndExitCmd() tea.Cmd {
	return func() tea.Msg { return shared.PolicyEditDiscardedMsg{} }
}
