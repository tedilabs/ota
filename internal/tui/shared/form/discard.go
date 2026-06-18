package form

// Generic edit-form helpers reused by every write surface (Users,
// Groups, Group Rules, Policies, future resources). Lifted here so
// per-resource edit screens stay tiny — they just supply the
// FieldSpec catalog + Service callbacks + title; this package
// renders the discard picker, placeholder skeleton, and the modal
// footer common to all of them.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tedilabs/ota/internal/tui/shared"
)

// BuildDiscardStrip emits the prominent confirm strip appended to
// the edit-form modal body when the operator presses Esc with
// unsaved changes. The cursor argument drives the arrow-key picker:
// 0 = "Discard and exit", 1 = "Keep editing" (safe default).
//
// dirtyKeys are listed under the headline so the operator sees what
// they're about to throw away. width is the modal's content cell
// budget (used for the dashed headline).
func BuildDiscardStrip(tk shared.Tokens, dirtyKeys []string, width, cursor int) string {
	headline := "─ Unsaved changes "
	if w := width - len([]rune(headline)); w > 0 {
		headline += strings.Repeat("─", w)
	}
	if tk.Warning.GetForeground() != nil {
		headline = tk.Warning.Render(headline)
	}
	var b strings.Builder
	b.WriteString(headline)
	if n := len(dirtyKeys); n > 0 {
		limit := n
		if limit > 5 {
			limit = 5
		}
		b.WriteString("\n  Modified: " + strings.Join(dirtyKeys[:limit], ", "))
		if n > limit {
			b.WriteString(fmt.Sprintf("  … and %d more", n-limit))
		}
	}
	discard := pickerOption(tk, "Discard and exit", cursor == 0, tk.Danger)
	keep := pickerOption(tk, "Keep editing", cursor == 1, tk.Success)
	b.WriteString("\n\n  " + discard + "     " + keep)
	return b.String()
}

// pickerOption renders one option on the side-by-side discard
// picker. The highlighted entry uses reverse video + a leading `▸`
// so it stays identifiable under NO_COLOR / monochrome.
func pickerOption(tk shared.Tokens, label string, active bool, tone lipgloss.Style) string {
	if active {
		body := "▸ " + label + " "
		hi := tone.Reverse(true).Bold(true)
		if fg := tone.GetForeground(); fg != nil {
			hi = hi.Foreground(fg)
			return hi.Render(body)
		}
		return body
	}
	body := "  " + label + " "
	if tk.Muted.GetForeground() != nil {
		return tk.Muted.Render(body)
	}
	return body
}

// RenderPlaceholderBody draws the loading skeleton — section headers
// + underscore rows so the operator sees the modal's chrome shape
// before the GET resolves (REQ-W01 AC-1.4 + symmetric for future
// resources). specs governs the section / field layout; clean
// sections only since dirty state isn't meaningful before load.
func RenderPlaceholderBody(tk shared.Tokens, specs []FieldSpec, contentWidth, labelCol, inputCol int) string {
	var b strings.Builder
	currentSection := ""
	placeholder := strings.Repeat("_", inputCol)
	for _, s := range specs {
		if s.Section != "" && s.Section != currentSection {
			if currentSection != "" {
				b.WriteByte('\n')
			}
			b.WriteString(RenderSectionHeader(tk, s.Section, 0, contentWidth))
			b.WriteByte('\n')
			currentSection = s.Section
		}
		row := RenderFieldRow(tk, FieldRowOpts{
			Label:    s.Label,
			Value:    placeholder,
			LabelCol: labelCol,
			InputCol: inputCol,
		})
		b.WriteString(row)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// ComposeFooter returns the single-line footer per edit-form state.
// Centralised so every resource's modal shares the same wording +
// keymap hints.
//
// dirty: 0 = clean, ≥1 = number of modified fields.
// statusMsg: optional transient status ("Updated …", "Fix the
// highlighted field"). When present it leads the footer.
func ComposeFooter(state FooterState, dirty int, statusMsg string) string {
	switch state {
	case FooterStateSaving:
		return "Saving…  ·  Ctrl+C to abort (preserves draft)"
	case FooterStateDiscardConfirm:
		return fmt.Sprintf("%d changes  ·  <←/→> select  ·  <Enter> apply  ·  <Esc> keep editing", dirty)
	}
	if statusMsg != "" {
		return statusMsg + "  ·  Ctrl+S save  ·  Esc cancel"
	}
	if dirty <= 0 {
		return "No changes  ·  Ctrl+S save  ·  Esc cancel  ·  Tab next"
	}
	noun := "changes"
	if dirty == 1 {
		noun = "change"
	}
	return fmt.Sprintf("%d %s  ·  Ctrl+S save  ·  Esc cancel  ·  Tab next  ·  Alt+m PII", dirty, noun)
}

// RenderFormBody composes section headers + field rows for the
// editing / saving / discard-confirm states. Lifted from the
// per-resource RenderModal so every edit screen renders an
// identical body — only the FieldSpec catalog + Form snapshot
// differ.
//
// editing controls focus rendering: true during the live editing
// state, false during saving / discard-confirm (no row gets the
// `▎ ┃` highlight when input is disabled). maskFn handles PII
// display masking (called only for non-focused PII fields); nil
// disables masking for resources without PII fields.
func RenderFormBody(tk shared.Tokens, f Form, contentWidth, labelCol, inputCol int, editing bool, maskFn func(string) string) string {
	specs := f.Specs()
	current := f.Current()
	focusKey := f.FocusKey()
	dirtyByKey := map[string]bool{}
	for _, k := range f.DirtyFields() {
		dirtyByKey[k] = true
	}
	inlineErrs := f.InlineErrors()

	dirtyBySection := map[string]int{}
	for _, s := range specs {
		if dirtyByKey[s.Key] {
			dirtyBySection[s.Section]++
		}
	}

	var b strings.Builder
	currentSection := ""
	for _, s := range specs {
		if s.Section != "" && s.Section != currentSection {
			if currentSection != "" {
				b.WriteByte('\n')
			}
			b.WriteString(RenderSectionHeader(tk, s.Section, dirtyBySection[s.Section], contentWidth))
			b.WriteByte('\n')
			currentSection = s.Section
		}
		focused := editing && focusKey == s.Key
		readOnly := s.Kind == KindReadOnly
		dirty := dirtyByKey[s.Key]
		value := current[s.Key]
		masked := false
		if maskFn != nil && s.PII && !f.PIIAllUnmasked() && !focused {
			value = maskFn(value)
			masked = true
		}
		cursorPos := -1
		if focused && !readOnly {
			cursorPos = f.CursorPos()
		}
		row := RenderFieldRow(tk, FieldRowOpts{
			Label:     s.Label,
			Value:     value,
			Focused:   focused,
			Dirty:     dirty,
			ReadOnly:  readOnly,
			Masked:    masked,
			InlineErr: inlineErrs[s.Key],
			LabelCol:  labelCol,
			InputCol:  inputCol,
			CursorPos: cursorPos,
		})
		b.WriteString(row)
		b.WriteByte('\n')
	}

	if extras := f.OtherErrors(); len(extras) > 0 {
		b.WriteByte('\n')
		for _, e := range extras {
			b.WriteString("! " + e + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// FooterState classifies the edit-form's lifecycle state for
// ComposeFooter. Mirrors the screen-side EditState (which is owned
// by each per-resource edit screen) but stays inside the form
// package so the helpers don't have to import the screen.
type FooterState int

// Footer states the edit form distinguishes when picking footer
// text. The default zero value is FooterStateEditing — the most
// common state and the safest fallback.
const (
	FooterStateEditing FooterState = iota
	FooterStateSaving
	FooterStateDiscardConfirm
)
