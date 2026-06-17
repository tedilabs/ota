// Package form is the domain-agnostic reusable form widget
// (REQ-W01 / D-T2 / OI-W5 option C). Screens compose Form with a
// FieldSpec catalog, drive it via Update, render via View, and
// extract user input via Snapshot/Diff. The Form does not import
// `internal/domain` — depguard enforces this so the widget stays
// reusable for future mutation surfaces (lifecycle deactivate
// reason input, group rename, etc.).
//
// Tests for this package live in form_test.go and treat the Form
// as a black box: feed messages, inspect Dirty/DirtyFields/
// Snapshot/Diff. ApplyServerErrors accepts domain.FieldError
// strictly via the public API; the Form internally just stores
// (key, message) pairs.
package form
