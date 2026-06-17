# Phase 5 — Developer Stub Files (Compile-Pass Only, Implementations Required)

**상태:** RED — All stubs return errors / return zero values.
**작성일:** 2026-06-17
**작성자:** go-test-engineer

Phase 5 (TDD Fail-First) 는 **컴파일은 통과**하되 **모든 새 테스트가 의도된 RED 상태**가 되도록 stub 파일을 생성한다. Phase 6 (developer) 가 이 stub 들을 채워서 GREEN 으로 만든다.

---

## Stub Files Created (Step 1-8)

### 1. `internal/domain/user_patch.go` (NEW)

**Purpose:** PRD D-W1 11 fields, D-T4 `*string` partial patch, D-T5 `ErrEmptyPatch` sentinel.

```go
package domain

import "errors"

// UserProfilePatch is the partial-merge body for POST /api/v1/users/{id}
// (REQ-W01 AC-4.2). Nil pointer = unchanged (omit from JSON). Non-nil
// pointer = set to the dereferenced value. Explicit-null-clear is
// deferred (D-W13 / domain §1.2).
//
// Login is intentionally absent (D-W2 — read-only in MVP).
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

// IsEmpty reports whether every field in the patch is nil. Stub returns
// false so tests fail loudly until implemented.
func (p UserProfilePatch) IsEmpty() bool {
	return false // STUB: must return true when all fields nil
}

// ErrEmptyPatch is the domain sentinel returned by UsersPort.UpdateProfile
// when the caller passes an IsEmpty() patch (D-T5 / D-W13).
var ErrEmptyPatch = errors.New("empty patch: no fields to update")
```

### 2. `internal/domain/ports.go` (PATCH — UsersPort.UpdateProfile)

Add one method to the `UsersPort` interface. All implementations get a stub.

```go
type UsersPort interface {
    // ... existing methods ...

    // UpdateProfile applies a partial-merge profile patch to the user
    // (REQ-W01 / D-W13). When patch.IsEmpty() == true, returns
    // ErrEmptyPatch WITHOUT making an HTTP call.
    UpdateProfile(ctx context.Context, userID string, patch UserProfilePatch) (User, error)
}
```

### 3. `internal/okta/users_update.go` (NEW)

```go
package okta

// UpdateProfile is a STUB — always returns ErrEmptyPatch so tests fail
// with a clear "implement me" signal until Phase 6.
func (a *UsersAdapter) UpdateProfile(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error) {
	return domain.User{}, errors.New("not implemented: okta.UsersAdapter.UpdateProfile")
}
```

### 4. `internal/service/fakes/users_port_fake.go` (PATCH)

Add `UpdateProfileFunc` field + method + `ValidationErrorFake` helper.

### 5. `internal/testfx/ports.go` (PATCH)

Add stub `UpdateProfile` to `seededUsersPort` so production code compiles.

### 6. `internal/tui/shared/form/form.go` (NEW package)

Bare-minimum public API so test compiles. All methods return zero / "not implemented" error.

### 7. `internal/tui/shared/msgs.go` (PATCH)

Add `OpenUserEditMsg` and `UserUpdatedMsg`.

### 8. `internal/app/app.go` (PATCH)

Add `ScreenUserEdit` and `OverlayDiscardConfirm` constants. Stub
`screenFromName("user-edit")`.

### 9. `internal/tui/users/edit.go` (NEW)

`EditModel`, `EditDeps`, `NewEditModel`. All methods return zero / no-op so tests fail loudly.

### 10. `internal/service/users_update.go` (NEW)

`(*UsersService).UpdateProfile(ctx, id, patch)` — wraps port.

---

## Phase 6 Action Items (in order)

1. **`domain/user_patch.go`** — implement `IsEmpty()` correctly.
2. **`okta/users_update.go`** — implement POST /api/v1/users/{id} with partial-merge body marshalling. Guard `IsEmpty()` BEFORE HTTP.
3. **`service/users_update.go`** — delegate to port, propagate errors.
4. **`tui/shared/form/`** — implement Form/FieldSpec/Update/View/Diff/etc.
5. **`tui/users/edit.go`** — wire Form → Service → Form. State machine: loading/editing/saving/discardConfirm.
6. **`app/app.go`** — wire `e` key from List/Detail to `OpenUserEditMsg` → push `ScreenUserEdit`. `OverlayDiscardConfirm` handler.

---

## Compile-Pass Checks

- `go build ./...` succeeds (no missing types/methods).
- `go vet ./...` succeeds.
- All existing tests still pass (lifecycle / list / detail untouched).
- All new tests FAIL with RED message matching expectation.
