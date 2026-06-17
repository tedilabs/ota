package domain

import "errors"

// UserProfilePatch is the partial-merge body for POST /api/v1/users/{id}
// (REQ-W01 AC-4.2 / D-T4). nil pointer = unchanged (omit from JSON);
// non-nil pointer = set to the dereferenced value.
//
// Login is intentionally absent — D-W2 keeps login read-only in MVP.
// Explicit-null-clear is deferred (D-W13 / domain §1.2).
//
// Field set matches AC-2's 11-field catalog.
type UserProfilePatch struct {
	FirstName      *string
	LastName       *string
	DisplayName    *string
	NickName       *string
	Email          *string
	Title          *string
	Division       *string
	Department     *string
	EmployeeNumber *string
	MobilePhone    *string
	SecondEmail    *string
}

// IsEmpty reports whether every patch field is nil (i.e. there is no
// mutation to send to Okta). D-T5 / D-W13: callers (or the adapter) MUST
// short-circuit empty patches without making an HTTP call and return
// ErrEmptyPatch instead.
//
// Pointer presence is the patch key; the dereferenced string is the
// payload. A non-nil pointer to an empty string still counts as "set"
// (today equivalent to "unchanged" per D-W13 — explicit-null-clear is
// deferred).
func (p UserProfilePatch) IsEmpty() bool {
	return p.FirstName == nil &&
		p.LastName == nil &&
		p.DisplayName == nil &&
		p.NickName == nil &&
		p.Email == nil &&
		p.Title == nil &&
		p.Division == nil &&
		p.Department == nil &&
		p.EmployeeNumber == nil &&
		p.MobilePhone == nil &&
		p.SecondEmail == nil
}

// ErrEmptyPatch is the sentinel returned by UsersPort.UpdateProfile and
// UsersService.UpdateProfile when the supplied patch carries no
// mutations (D-T5 / D-W13). The TUI guards this with `Save` disabled at
// dirty=0, but the sentinel is the defensive backstop.
var ErrEmptyPatch = errors.New("empty patch: no fields to update")
