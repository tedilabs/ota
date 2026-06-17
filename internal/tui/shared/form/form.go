package form

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
)

// FieldKind classifies the validation and rendering treatment a field
// receives.
type FieldKind int

// Field kinds — keep stable; CONVENTIONS §10a.5 lists them.
const (
	KindText FieldKind = iota
	KindEmail
	KindPhone
	KindReadOnly
)

// FieldSpec describes a single field in the catalog. Catalogs are
// declared by the owning screen (e.g. users/edit_spec.go for REQ-W01).
//
// Section groups fields for the rendered separator headers (Identity /
// Contact / Organization / Status in Users). PII flips the default
// masking lifecycle (focus auto-unmask, blur re-mask if unchanged,
// Alt+m toggle).
type FieldSpec struct {
	Key      string    // API field name, e.g. "firstName"
	Label    string    // human label, e.g. "First Name"
	Kind     FieldKind // KindText / KindEmail / KindPhone / KindReadOnly
	Required bool      // surfaces "[required]" prefix + empty-string inline error
	PII      bool      // mobilePhone / secondEmail toggle group (Alt+m)
	Section  string    // header label
	Hint     string    // tokens.Muted advisory text under the field
	MaxLen   int       // 0 = no client limit
}

// Option configures a Form at New() time. Placeholder — no concrete
// options needed for REQ-W01 yet.
type Option func(*Form)

// fieldState is the per-key internal state. cursor is the byte position
// within current (no IME / wide-rune handling yet — the App Shell
// renders ASCII-style cursors elsewhere).
type fieldState struct {
	spec    FieldSpec
	cursor  int    // byte position within current
	piiMask bool   // when true, render value as bullets (PII display only)
}

// Form is the Bubbletea sub-model. All public methods are value-
// receiver to keep the App Shell's reducer style consistent.
type Form struct {
	specs    []FieldSpec
	snapshot map[string]string
	current  map[string]string
	state    map[string]*fieldState
	order    []string // FieldSpec order — focus index walks this

	focus       int // index into order; -1 when no editable field
	saving      bool
	piiAllUnmasked bool // Alt+m global toggle

	inlineErr  map[string]string // server / client field errors
	otherErrs  []string          // non-matched server errors
}

// New constructs a Form from a spec catalog and a snapshot of initial
// values. Missing keys default to empty string. Options control
// optional behaviours (focus-on, masking policies).
func New(specs []FieldSpec, initial map[string]string, opts ...Option) Form {
	if initial == nil {
		initial = map[string]string{}
	}
	snap := make(map[string]string, len(specs))
	cur := make(map[string]string, len(specs))
	state := make(map[string]*fieldState, len(specs))
	order := make([]string, 0, len(specs))
	firstEditable := -1
	for i, s := range specs {
		v := initial[s.Key]
		snap[s.Key] = v
		cur[s.Key] = v
		st := &fieldState{
			spec:    s,
			cursor:  len(v),
			piiMask: s.PII, // PII fields start masked (focus auto-unmasks)
		}
		state[s.Key] = st
		order = append(order, s.Key)
		if firstEditable == -1 && s.Kind != KindReadOnly {
			firstEditable = i
		}
	}
	f := Form{
		specs:    append([]FieldSpec(nil), specs...),
		snapshot: snap,
		current:  cur,
		state:    state,
		order:    order,
		focus:    firstEditable,
		inlineErr: map[string]string{},
	}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

// --- Bubbletea interface --------------------------------------------

// Init returns no Cmd — focus is established eagerly in New().
func (f Form) Init() tea.Cmd { return nil }

// Update reduces a tea.Msg into a new Form + Cmd. While Saving the
// form ignores edit-related keys.
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	if f.saving {
		// Disable all editing while saving; the owning screen still
		// processes Ctrl+C / Esc at a higher level.
		return f, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return f, nil
	}
	// Global Alt+m PII toggle (D-T8). When Alt is set the rune routes
	// here, NOT to the focused textinput.
	if km.Alt && km.Type == tea.KeyRunes && string(km.Runes) == "m" {
		f.piiAllUnmasked = !f.piiAllUnmasked
		return f, func() tea.Msg { return PIIToggleMsg{} }
	}

	switch km.Type {
	case tea.KeyTab:
		f.focus = f.nextEditableFocus(1)
		return f, nil
	case tea.KeyShiftTab:
		f.focus = f.nextEditableFocus(-1)
		return f, nil
	case tea.KeyDown:
		f.focus = f.nextEditableFocus(1)
		return f, nil
	case tea.KeyUp:
		f.focus = f.nextEditableFocus(-1)
		return f, nil
	}

	st := f.focusedState()
	if st == nil {
		return f, nil
	}
	key := st.spec.Key
	val := f.current[key]

	switch km.Type {
	case tea.KeyRunes:
		// Plain printable runes append at the cursor. Empty Alt
		// (handled above) was already filtered.
		ins := string(km.Runes)
		val = val[:st.cursor] + ins + val[st.cursor:]
		st.cursor += len(ins)
	case tea.KeySpace:
		val = val[:st.cursor] + " " + val[st.cursor:]
		st.cursor++
	case tea.KeyBackspace:
		if st.cursor > 0 {
			// Naive backspace by 1 byte (ASCII-aware; runes handled
			// in v0.2 when textinput swap lands).
			val = val[:st.cursor-1] + val[st.cursor:]
			st.cursor--
		}
	case tea.KeyDelete:
		if st.cursor < len(val) {
			val = val[:st.cursor] + val[st.cursor+1:]
		}
	case tea.KeyHome:
		st.cursor = 0
	case tea.KeyEnd:
		st.cursor = len(val)
	case tea.KeyLeft:
		if st.cursor > 0 {
			st.cursor--
		}
	case tea.KeyRight:
		if st.cursor < len(val) {
			st.cursor++
		}
	default:
		return f, nil
	}

	f.current[key] = val
	// Server / client error for this field clears on the next keystroke
	// — operator is actively addressing it.
	delete(f.inlineErr, key)
	return f, nil
}

// View renders the form body — section dividers, field rows with
// labels + values, inline errors, and a footer showing Saving / dirty
// state. Layout is intentionally simple ASCII so NO_COLOR mode (AC-8)
// reads cleanly without depending on lipgloss tints.
func (f Form) View() string {
	var b strings.Builder
	currentSection := ""
	for i, s := range f.specs {
		if s.Section != "" && s.Section != currentSection {
			if currentSection != "" {
				b.WriteByte('\n')
			}
			b.WriteString("── ")
			b.WriteString(s.Section)
			b.WriteString(" ──\n")
			currentSection = s.Section
		}
		focused := f.focus == i
		b.WriteString(f.renderRow(s, focused))
		b.WriteByte('\n')
		if msg, ok := f.inlineErr[s.Key]; ok {
			b.WriteString("  ! ")
			b.WriteString(msg)
			b.WriteByte('\n')
		}
		if s.Hint != "" {
			b.WriteString("  ")
			b.WriteString(s.Hint)
			b.WriteByte('\n')
		}
	}
	// OtherErrors footer — server errors that didn't match any field.
	if len(f.otherErrs) > 0 {
		b.WriteString("\n")
		for _, e := range f.otherErrs {
			b.WriteString("! ")
			b.WriteString(e)
			b.WriteByte('\n')
		}
	}
	// Footer — dirty / saving badge.
	b.WriteString("\n")
	if f.saving {
		b.WriteString("Saving…")
	} else {
		d := f.Dirty()
		switch {
		case d <= 0:
			b.WriteString("No changes")
		case d == 1:
			b.WriteString("1 change")
		default:
			b.WriteString(fmt.Sprintf("%d changes", d))
		}
	}
	return b.String()
}

func (f Form) renderRow(s FieldSpec, focused bool) string {
	cur := f.current[s.Key]
	required := ""
	if s.Required {
		required = " *"
	}
	switch s.Kind {
	case KindReadOnly:
		return fmt.Sprintf("  %s%s: %s", s.Label, required, cur)
	}
	display := cur
	st := f.state[s.Key]
	if st != nil && st.spec.PII && !f.shouldShowPII(s, focused) {
		display = maskString(cur)
	}
	prefix := "  "
	if focused {
		prefix = "▸ "
	}
	return fmt.Sprintf("%s%s%s: [%s]", prefix, s.Label, required, display)
}

func (f Form) shouldShowPII(s FieldSpec, focused bool) bool {
	if !s.PII {
		return true
	}
	if f.piiAllUnmasked {
		return true
	}
	// Focus auto-unmask (AC-7.2).
	return focused
}

func maskString(s string) string {
	if s == "" {
		return ""
	}
	return strings.Repeat("•", len([]rune(s)))
}

// --- Inspection -----------------------------------------------------

// Dirty returns the count of fields whose current value differs from
// the snapshot (AC-9.1).
func (f Form) Dirty() int {
	n := 0
	for k, v := range f.current {
		if f.specs == nil {
			continue
		}
		// Skip read-only by spec — they shouldn't change anyway, but
		// defensive: don't count them.
		if st := f.state[k]; st != nil && st.spec.Kind == KindReadOnly {
			continue
		}
		if f.snapshot[k] != v {
			n++
		}
	}
	return n
}

// DirtyFields returns the keys of fields whose current value differs
// from the snapshot. Order matches FieldSpec order.
func (f Form) DirtyFields() []string {
	var out []string
	for _, k := range f.order {
		if st := f.state[k]; st != nil && st.spec.Kind == KindReadOnly {
			continue
		}
		if f.snapshot[k] != f.current[k] {
			out = append(out, k)
		}
	}
	return out
}

// Validate runs the loose client-side checks (AC-3): required-empty,
// email shape, etc. On failure returns (false, firstInvalidKey) —
// matches the spec catalog order so the caller can jump focus.
func (f Form) Validate() (ok bool, firstInvalid string) {
	for _, k := range f.order {
		st := f.state[k]
		if st == nil || st.spec.Kind == KindReadOnly {
			continue
		}
		v := f.current[k]
		if st.spec.Required && strings.TrimSpace(v) == "" {
			return false, k
		}
		switch st.spec.Kind {
		case KindEmail:
			if v == "" {
				continue
			}
			if !looksLikeEmail(v) {
				return false, k
			}
		}
	}
	return true, ""
}

// looksLikeEmail is the loose `*@*.*` check per PRD §6.1 (AC-3.2).
// Intentionally permissive — strict validation is deferred to the
// server.
func looksLikeEmail(s string) bool {
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	host := s[at+1:]
	if !strings.Contains(host, ".") {
		return false
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	return true
}

// Snapshot returns a clone of the snapshot values (i.e., what the form
// was initialised with — does NOT reflect live edits).
func (f Form) Snapshot() map[string]string {
	if f.snapshot == nil {
		return nil
	}
	out := make(map[string]string, len(f.snapshot))
	for k, v := range f.snapshot {
		out[k] = v
	}
	return out
}

// Diff returns the (key, value) pairs whose current value differs from
// the snapshot — the inputs the screen turns into a partial-merge
// patch (AC-4.2). Read-only fields are never in the diff.
func (f Form) Diff() map[string]string {
	var out map[string]string
	for _, k := range f.order {
		if st := f.state[k]; st != nil && st.spec.Kind == KindReadOnly {
			continue
		}
		if f.snapshot[k] != f.current[k] {
			if out == nil {
				out = map[string]string{}
			}
			out[k] = f.current[k]
		}
	}
	return out
}

// SetSaving toggles the read-only saving state — input disabled,
// "Saving" rendered in the footer.
func (f Form) SetSaving(on bool) Form {
	f.saving = on
	return f
}

// Saving reports whether the form is currently in saving (read-only)
// state.
func (f Form) Saving() bool { return f.saving }

// FocusKey returns the currently focused field key, or "" when no
// editable field is focused.
func (f Form) FocusKey() string {
	if f.focus < 0 || f.focus >= len(f.specs) {
		return ""
	}
	return f.specs[f.focus].Key
}

// Focus moves focus to the named field (no-op if not found or
// read-only). Used by the screen to jump to the first invalid field
// returned by Validate.
func (f Form) Focus(key string) Form {
	for i, s := range f.specs {
		if s.Key == key && s.Kind != KindReadOnly {
			f.focus = i
			return f
		}
	}
	return f
}

// ApplyServerErrors maps domain.FieldError causes onto the form's
// inline error slots by FieldSpec.Key. Unmatched causes go to the
// OtherErrors footer. The reason is extracted from the Summary using
// the same "field: reason" split as the okta.errormap parser.
func (f Form) ApplyServerErrors(causes []domain.FieldError) Form {
	f.inlineErr = map[string]string{}
	f.otherErrs = nil
	for _, c := range causes {
		key := matchFieldKey(c.Field, f.specs)
		reason := extractReason(c.Summary)
		if key == "" {
			if c.Summary != "" {
				f.otherErrs = append(f.otherErrs, c.Summary)
			}
			continue
		}
		f.inlineErr[key] = reason
	}
	return f
}

// matchFieldKey resolves the okta-side field name to a FieldSpec.Key
// via simple case-insensitive equality. Aliases (e.g. "email_addr" →
// "email") can be added per CONVENTIONS §10a.6 when needed.
func matchFieldKey(name string, specs []FieldSpec) string {
	if name == "" {
		return ""
	}
	for _, s := range specs {
		if strings.EqualFold(s.Key, name) {
			return s.Key
		}
	}
	return ""
}

// extractReason strips the "field: " prefix when present so the inline
// view shows just the reason ("Email is not valid") rather than the
// duplicated key.
func extractReason(summary string) string {
	if i := strings.Index(summary, ":"); i > 0 {
		return strings.TrimSpace(summary[i+1:])
	}
	return summary
}

// --- Internal helpers -----------------------------------------------

// focusedState returns the focused field's state pointer, or nil when
// no editable field is focused.
func (f Form) focusedState() *fieldState {
	if f.focus < 0 || f.focus >= len(f.specs) {
		return nil
	}
	if f.specs[f.focus].Kind == KindReadOnly {
		return nil
	}
	return f.state[f.specs[f.focus].Key]
}

// nextEditableFocus walks the spec list by step (1 forward, -1 back),
// skipping read-only fields. Wraps around at either end.
func (f Form) nextEditableFocus(step int) int {
	if len(f.specs) == 0 {
		return -1
	}
	idx := f.focus
	for i := 0; i < len(f.specs); i++ {
		idx = (idx + step + len(f.specs)) % len(f.specs)
		if f.specs[idx].Kind != KindReadOnly {
			return idx
		}
	}
	return f.focus
}

// --- Messages (form-owned) ------------------------------------------

// SaveRequestedMsg is emitted by Form.Update on Ctrl+S when dirty>0
// and validation passes. The owning screen interprets it as
// "build the patch and call svc.UpdateProfile".
type SaveRequestedMsg struct{}

// DiscardRequestedMsg is emitted on Esc when dirty>0. The owning
// screen opens an OverlayDiscardConfirm. Confirmed is true after
// the operator answered y/Y on the modal — see app/app.go's
// modal handler.
type DiscardRequestedMsg struct {
	Confirmed bool
}

// PIIToggleMsg is emitted on Alt+m. The owning screen forwards it
// back so any sibling listeners (status bar) can reflect.
type PIIToggleMsg struct{}

// FieldFocusedMsg fires when focus lands on a field. PII fields use
// it for auto-unmask (AC-7.2).
type FieldFocusedMsg struct{ Key string }

// FieldBlurredMsg fires on focus-out — PII fields re-mask when the
// value hasn't changed (AC-7.3).
type FieldBlurredMsg struct{ Key string }
