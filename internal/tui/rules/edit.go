package rules

// Group Rule Edit Form — parallels groups.EditModel but targets the
// Group Rule resource. Okta requires INACTIVE / INVALID status
// before accepting a rule update; the list / detail key handler
// guards on Status before emitting shared.OpenRuleEditMsg.
//
// Editable fields: name + expression. Target group IDs are
// displayed read-only in the form (operators set them via the Okta
// console — comma-separated multi-value editing is deferred to a
// future round, see CONVENTIONS for the multi-value picker pattern).

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

// EditState classifies the lifecycle phase of EditModel.
type EditState int

const (
	EditStateLoading EditState = iota
	EditStateEditing
	EditStateSaving
	EditStateDiscardConfirm
	EditStateErrored
)

// EditDeps wires the screen to the service.
type EditDeps struct {
	Svc    *service.GroupRulesService
	RuleID string
	Clock  clock.Clock
	Logger *slog.Logger
	Width  int
	Height int
}

// EditModel is the tea.Model for the group rule edit form.
type EditModel struct {
	deps  EditDeps
	state EditState
	form  form.Form

	loadedRule domain.GroupRule
	statusMsg  string
	loadErr    error
	saveErr    error

	discardCursor int
}

// NewEditModel constructs an EditModel.
func NewEditModel(deps EditDeps) EditModel {
	return EditModel{deps: deps, state: EditStateLoading}
}

func (m EditModel) State() EditState { return m.state }
func (m EditModel) Form() form.Form  { return m.form }
func (m EditModel) RuleID() string   { return m.deps.RuleID }

func (m EditModel) Init() tea.Cmd {
	if m.deps.Svc == nil || m.deps.RuleID == "" {
		return nil
	}
	return fetchRuleForEditCmd(m.deps.Svc, m.deps.RuleID)
}

func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		return m, tea.Sequence(tea.Println(m.View()), tea.Quit)
	}
	switch msg := msg.(type) {
	case ruleEditLoadedMsg:
		m.loadedRule = msg.rule
		m.form = form.New(RuleFieldSpecs(), ruleToInitial(msg.rule))
		m.state = EditStateEditing
		return m, nil
	case ruleEditLoadFailedMsg:
		m.loadErr = msg.err
		m.state = EditStateErrored
		return m, nil
	case ruleEditSaveSucceededMsg:
		m.loadedRule = msg.rule
		m.saveErr = nil
		m.state = EditStateEditing
		m.form = m.form.SetSaving(false)
		m.statusMsg = "Updated " + ruleDisplay(msg.rule)
		broadcast := func() tea.Msg { return shared.RuleUpdatedMsg{Rule: msg.rule} }
		return m, broadcast
	case ruleEditSaveFailedMsg:
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
		update := buildRuleUpdate(m.loadedRule, m.form.Current())
		m.state = EditStateSaving
		m.form = m.form.SetSaving(true)
		m.statusMsg = ""
		return m, saveRuleCmd(m.deps.Svc, m.deps.RuleID, update)
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
		return "Loading rule…"
	case EditStateErrored:
		return "Cannot edit: " + errorSummary(m.loadErr)
	}
	var b strings.Builder
	b.WriteString("Edit Rule · ")
	b.WriteString(m.loadedRule.Name)
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
			Title:  "Edit Rule · error",
			Body:   "Cannot edit: " + errorSummary(m.loadErr),
			Footer: "Esc to close",
			Tone:   shared.ModalToneDanger,
			Width:  width,
			Tokens: tk,
		})
	case EditStateLoading:
		body := form.RenderPlaceholderBody(tk, RuleFieldSpecs(), contentWidth, labelCol, inputCol)
		return shared.MountModal(shared.ModalIn{
			Title:  "Edit Rule · Loading…",
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
	title := "Edit Rule  ·  " + ruleDisplay(m.loadedRule)

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

// RuleFieldSpecs lists the editable fields. Targets render
// read-only — multi-value editing is deferred (a future round
// introduces a picker widget for slice fields).
func RuleFieldSpecs() []form.FieldSpec {
	return []form.FieldSpec{
		{Key: "name", Label: "Name", Kind: form.KindText, Required: true, Section: "Identity", MaxLen: 255},
		{Key: "expression", Label: "Expression", Kind: form.KindText, Required: true, Section: "Logic", MaxLen: 1024},
		{Key: "targets", Label: "Target Groups", Kind: form.KindReadOnly, Section: "Logic"},
	}
}

func ruleToInitial(r domain.GroupRule) map[string]string {
	return map[string]string{
		"name":       r.Name,
		"expression": r.Expression,
		"targets":    strings.Join(r.TargetGroupIDs, ", "),
	}
}

// buildRuleUpdate constructs the strict-replace body. Target group
// IDs come from the loaded snapshot (read-only in this form); name +
// expression come from the form's current values.
func buildRuleUpdate(loaded domain.GroupRule, current map[string]string) domain.GroupRuleUpdate {
	return domain.GroupRuleUpdate{
		Name:           current["name"],
		Expression:     current["expression"],
		TargetGroupIDs: loaded.TargetGroupIDs,
	}
}

func ruleDisplay(r domain.GroupRule) string {
	if r.Name != "" {
		return r.Name
	}
	return r.ID
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

type ruleEditLoadedMsg struct{ rule domain.GroupRule }
type ruleEditLoadFailedMsg struct{ err error }
type ruleEditSaveSucceededMsg struct{ rule domain.GroupRule }
type ruleEditSaveFailedMsg struct{ err error }

// OpenRuleEditCmd is emitted on `e` from the Rules list/detail when
// the target rule is in INACTIVE or INVALID status. ACTIVE rules
// must be deactivated first; the caller surfaces a toast instead.
func OpenRuleEditCmd(id string) tea.Cmd {
	return func() tea.Msg { return shared.OpenRuleEditMsg{ID: id} }
}

// OpenRuleStatusPickerCmd is emitted on `s` from the Rules list /
// detail. Carries the full rule snapshot so the App Shell's picker
// can render the current status badge in its title without a fetch.
func OpenRuleStatusPickerCmd(r domain.GroupRule) tea.Cmd {
	return func() tea.Msg { return shared.OpenRuleStatusPickerMsg{Rule: r} }
}

func fetchRuleForEditCmd(svc *service.GroupRulesService, id string) tea.Cmd {
	return func() tea.Msg {
		r, err := svc.Get(context.Background(), id)
		if err != nil {
			return ruleEditLoadFailedMsg{err: err}
		}
		return ruleEditLoadedMsg{rule: r}
	}
}

func saveRuleCmd(svc *service.GroupRulesService, id string, update domain.GroupRuleUpdate) tea.Cmd {
	return func() tea.Msg {
		r, err := svc.UpdateRule(context.Background(), id, update)
		if err != nil {
			return ruleEditSaveFailedMsg{err: err}
		}
		return ruleEditSaveSucceededMsg{rule: r}
	}
}

func discardAndExitCmd() tea.Cmd {
	return func() tea.Msg { return shared.RuleEditDiscardedMsg{} }
}
