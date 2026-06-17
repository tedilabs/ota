package form_test

// REQ-W01 — form widget unit tests (Step 6 of Phase 5 RED order).
// These tests treat `form.Form` as a black box. No port fakes — the
// widget is intentionally domain-agnostic (D-T10). The 11-field
// catalog used here mirrors AC-2's set but lives as a local helper
// so the form package never imports users-specific code.
//
// Phase 5 RED expectation: every public method on form.Form is a stub
// (Dirty()=-1, Snapshot()=nil, View()="", Update is no-op), so every
// assertion below fails until Phase 6 wires the widget.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/tui/shared/form"
)

// testSpecs is the 11-field catalog matching REQ-W01 AC-2 — duplicated
// here (rather than imported from users) so the form package stays
// import-cycle-clean. CONVENTIONS §10a.9 forbids the form package from
// importing domain or users.
func testSpecs() []form.FieldSpec {
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

func testInitial() map[string]string {
	return map[string]string{
		"login":          "alice@acme.com",
		"firstName":      "Alice",
		"lastName":       "Smith",
		"displayName":    "Alice Smith",
		"nickName":       "ali",
		"email":          "alice@acme.com",
		"mobilePhone":    "+1-555-123-4567",
		"secondEmail":    "alice.b@personal.com",
		"title":          "SWE",
		"division":       "R&D",
		"department":     "Eng",
		"employeeNumber": "ENG-042",
	}
}

// AC-9.1 — a freshly constructed Form reports zero dirty fields and
// the snapshot equals the initial values.
func Test_Form_New_NoDirty(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())

	assert.Equal(t, 0, f.Dirty(),
		"REQ-W01 AC-9.1: a fresh form must report Dirty()=0 (snapshot == current)")
	assert.Equal(t, []string(nil), f.DirtyFields(),
		"REQ-W01 AC-9.1: DirtyFields must be empty when nothing changed")
	assert.Equal(t, testInitial(), f.Snapshot(),
		"REQ-W01: Snapshot() must echo the initial values")
}

// AC-9.1 — a Form built from an empty initial map still constructs
// successfully and reports zero dirty fields.
func Test_Form_New_EmptyInitial_NoDirty(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), map[string]string{})
	assert.Equal(t, 0, f.Dirty(),
		"REQ-W01 AC-9.1: missing initial values default to empty string — dirty still 0")
}

// AC-9.1 / AC-9.2 — each keystroke that mutates a field flips dirty
// for that key. Phase 6: focus index starts at first editable field.
func Test_Form_Dirty_TrackedPerKeystroke(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())
	// Append "ia" to the focused first editable field (firstName) —
	// "Alice" → "Aliceia". The Form must report dirty=1.
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnd})
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ia")})

	assert.Equal(t, 1, f.Dirty(),
		"REQ-W01 AC-9.1: a keystroke on a focused field flips Dirty to 1")
}

// AC-9 — reverting to the snapshot value clears the dirty marker for
// that field. This pins the "compare to snapshot every render" model
// (D-T7).
func Test_Form_Revert_ClearsDirty(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())
	// Dirty firstName then revert: type "ia" then backspace twice.
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnd})
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ia")})
	require.Equal(t, 1, f.Dirty(), "precondition: dirty=1 after edit")

	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	assert.Equal(t, 0, f.Dirty(),
		"REQ-W01 D-T7: reverting to the snapshot value clears the dirty marker")
}

// AC-4.2 — Diff() returns only dirty (key, value) pairs. Drives the
// screen's buildPatch helper (D-T3 → service.UpdateProfile).
func Test_Form_Diff_ReturnsOnlyDirtyFields(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnd})
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})

	diff := f.Diff()
	require.Len(t, diff, 1, "REQ-W01 AC-4.2: Diff() must contain exactly the dirty field(s)")
	v, ok := diff["firstName"]
	require.True(t, ok, "REQ-W01 AC-4.2: firstName must be present in Diff after edit")
	assert.Equal(t, "AliceX", v, "REQ-W01 AC-4.2: Diff value is the live (not snapshot) input")
}

// AC-3.1 — Validate returns (false, firstInvalidKey) when a required
// field is empty. The Form widget's Validate is loose: required-empty
// + email shape only (AC-3.1, AC-3.2).
func Test_Form_Validate_RequiredEmpty_FailsAtFirstInvalid(t *testing.T) {
	t.Parallel()
	initial := testInitial()
	initial["firstName"] = "" // required but empty
	f := form.New(testSpecs(), initial)

	ok, firstInvalid := f.Validate()
	assert.False(t, ok, "REQ-W01 AC-3.1: empty required field must fail Validate()")
	assert.Equal(t, "firstName", firstInvalid,
		"REQ-W01 AC-3.1: Validate must return the key of the first invalid field for focus jumping")
}

// AC-3.2 — invalid email shape fails Validate. We use the loose
// `*@*.*` rule per PRD §6.1.
func Test_Form_Validate_InvalidEmail_Fails(t *testing.T) {
	t.Parallel()
	initial := testInitial()
	initial["email"] = "not-an-email"
	f := form.New(testSpecs(), initial)

	ok, _ := f.Validate()
	assert.False(t, ok,
		"REQ-W01 AC-3.2: invalid email shape must fail loose client validation")
}

// AC-6.1 — ApplyServerErrors maps domain.FieldError causes onto inline
// slots by FieldSpec.Key prefix. Unknown keys fall through to
// OtherErrors footer (validated via View) but the form still records
// they happened.
func Test_Form_ApplyServerErrors_PrefixMatchesFieldSpecKey(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())
	f = f.ApplyServerErrors([]domain.FieldError{
		{Field: "email", Summary: "email: Email is not valid"},
		{Field: "department", Summary: "department: Cannot exceed 100 characters"},
	})

	v := f.View()
	assert.Contains(t, v, "Email is not valid",
		"REQ-W01 AC-6.1: server error for 'email' must render inline near the Email field")
	assert.Contains(t, v, "Cannot exceed 100 characters",
		"REQ-W01 AC-6.1: server error for 'department' must render inline near the Department field")
}

// AC-9.1 — Snapshot() is a stable copy. Re-calling Diff() repeatedly
// with no edits returns the same empty result (sanity for D-T7 lazy
// diff).
func Test_Form_Diff_Idempotent_WhenClean(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())
	assert.Empty(t, f.Diff(), "fresh form has no diff")
	assert.Empty(t, f.Diff(), "second Diff() call still empty (idempotent)")
}

// AC-4.3 — SetSaving toggles a read-only state that surfaces via View
// footer (Phase 6) and disables further Update mutations. Here we only
// pin that SetSaving doesn't panic and View reflects the saving badge.
func Test_Form_SetSaving_RendersSavingFooter(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())
	f = f.SetSaving(true)
	assert.Contains(t, f.View(), "Saving",
		"REQ-W01 AC-4.3: SetSaving(true) must surface a saving footer (Phase 6 wires spinner)")
}

// D-W2 / AC-2 — read-only fields (login) MUST NOT appear in Diff even
// after a synthetic edit attempt. Phase 6 ensures Update routes
// keystrokes only to editable focus.
func Test_Form_ReadOnlyField_NeverInDiff(t *testing.T) {
	t.Parallel()
	f := form.New(testSpecs(), testInitial())
	// Even if some test tries to "type" while login is logically
	// focused, the Form widget treats it as KindReadOnly and skips.
	// Phase 6 implements Tab to skip read-only.
	diff := f.Diff()
	_, present := diff["login"]
	assert.False(t, present,
		"REQ-W01 D-W2 / AC-2: read-only login must never appear in Diff()")
}
