# Phase 6 — GREEN Progression Log

**Status:** GREEN — all 32 RED tests flip to PASS; 0 regressions in non-W01 packages.
**Date:** 2026-06-17
**Author:** go-tui-developer

Each step records the package(s) that turned green after the corresponding stub was filled in. Numbers are cumulative new-test passes in REQ-W01 (the lock-in tests already passed in Phase 5 — they are noted but not counted here as new "RED→GREEN" transitions).

---

## Step 1 — `domain.UserProfilePatch.IsEmpty()`

**File:** `internal/domain/user_patch.go`

Implemented an 11-field nil check. The sentinel `ErrEmptyPatch` was already declared in Phase 5 — no change.

```
internal/domain  ─ 1 RED → 1 GREEN
  - Test_UserProfilePatch_IsEmpty_AllNil_IsTrue            ✓
  (lock-ins continue to pass:
   Test_UserProfilePatch_IsEmpty_SingleFieldSet_IsFalse × 11
   Test_UserProfilePatch_IsEmpty_EmptyStringValue_IsFalse  ✓
   Test_UserProfilePatch_IsEmpty_AllSet_IsFalse            ✓
   Test_ErrEmptyPatch_IsSentinel                           ✓)
```

Total domain REQ-W01 surface: **15 GREEN** (1 newly flipped, 14 lock-ins).

---

## Step 2 — `okta.UsersAdapter.UpdateProfile`

**File:** `internal/okta/users_update.go`

Real implementation: `wireUserUpdateBody` + `wireUserProfilePatch` (omitempty fields) marshal sparse patches; pre-HTTP `patch.IsEmpty()` guard returns `domain.ErrEmptyPatch`; POST `/api/v1/users/{id}` via existing `c.doPost`; error mapping delegated to existing `errormap.FromResponse` (the Okta error code → domain sentinel table already covers E0000001/E0000006/E0000007/429/5xx).

```
internal/okta  ─ 6 RED → 6 GREEN
  - Test_OktaUsersAdapter_UpdateProfile_PartialMerge_SingleField_BodyShape   ✓
  - Test_OktaUsersAdapter_UpdateProfile_PartialMerge_MultiField_BodyShape    ✓
  - Test_OktaUsersAdapter_UpdateProfile_EmptyPatch_ReturnsErrEmptyPatch_NoHTTPCall ✓
  - Test_OktaUsersAdapter_UpdateProfile_400Validation_PropagatesCauses        ✓
  - Test_OktaUsersAdapter_UpdateProfile_403Forbidden_ReturnsErrForbidden      ✓
  - Test_OktaUsersAdapter_UpdateProfile_404NotFound_ReturnsErrNotFound        ✓
  (lock-in Test_UserProfilePatch_HasNoLoginField                              ✓)
```

Cumulative: **7 GREEN flips** so far (1 domain + 6 okta).

---

## Step 3 — `service.UsersService.UpdateProfile`

**File:** `internal/service/users_update.go`

Thin one-line delegate to `s.port.UpdateProfile`. No cache, no retry, no validation — the port already guards `IsEmpty`, the transport handles 429, and the service has nothing to add (D-T3).

```
internal/service  ─ 4 RED → 4 GREEN
  - Test_UsersService_UpdateProfile_DelegatesToPort_VerbatimPatch   ✓
  - Test_UsersService_UpdateProfile_PropagatesBadRequestError       ✓
  - Test_UsersService_UpdateProfile_PropagatesRateLimitedError      ✓
  - Test_UsersService_UpdateProfile_EmptyPatch_PropagatesErrEmptyPatch ✓
```

Cumulative: **11 GREEN flips**.

---

## Step 4 — `internal/tui/shared/form/Form` widget

**File:** `internal/tui/shared/form/form.go`

Self-contained form Model with:
- `FieldSpec` catalog (Key / Label / Kind / Required / PII / Section / Hint / MaxLen)
- per-field `fieldState` carrying cursor position + PII mask flag
- snapshot/current map pair for D-T7 lazy diff
- `Update()` reduces tea.KeyMsg into edits/focus moves (Tab / Shift-Tab / Home / End / Backspace / Delete / Left / Right / Runes / Space)
- `Update()` short-circuits to no-op when `Saving == true`
- Alt+m emits `PIIToggleMsg` and flips `piiAllUnmasked`
- `Dirty() / DirtyFields() / Diff() / Snapshot()` implement D-T7
- `Validate()` runs required-empty + loose-email checks, returns `(false, firstInvalidKey)` for focus jumping
- `ApplyServerErrors([]domain.FieldError)` populates inline error slots by FieldSpec.Key (case-insensitive) and dumps unmatched into the OtherErrors footer
- `View()` renders section dividers, focused row marker (`▸`), required marker (` *`), PII masking (`•` × len), inline errors (`  ! `), and footer (`No changes` / `1 change` / `N changes` / `Saving…`)
- Read-only fields are skipped by focus rotation and never appear in `Diff()`

Implementation note: bubble's `bubbles/textinput` was *not* introduced as a dependency — the existing palette / filter pattern (manual cursor + value mutation) was reused. This keeps the form widget consistent with the rest of the codebase and avoids the extra package surface; switching to `bubbles/textinput` in v0.2 will be a localised refactor inside `form.go`.

```
internal/tui/shared/form  ─ 9 RED → 9 GREEN
  - Test_Form_New_NoDirty                              ✓
  - Test_Form_New_EmptyInitial_NoDirty                 ✓
  - Test_Form_Dirty_TrackedPerKeystroke                ✓
  - Test_Form_Revert_ClearsDirty                       ✓
  - Test_Form_Diff_ReturnsOnlyDirtyFields              ✓
  - Test_Form_Validate_RequiredEmpty_FailsAtFirstInvalid ✓
  - Test_Form_Validate_InvalidEmail_Fails              ✓
  - Test_Form_ApplyServerErrors_PrefixMatchesFieldSpecKey ✓
  - Test_Form_SetSaving_RendersSavingFooter            ✓
  (lock-ins Test_Form_Diff_Idempotent_WhenClean, Test_Form_ReadOnlyField_NeverInDiff still pass)
```

Cumulative: **20 GREEN flips**.

---

## Step 5 — `internal/tui/users/EditModel`

**File:** `internal/tui/users/edit.go`

State machine `Loading → Editing → Saving → Editing|Discard → (exit)` + terminal `Errored` for the AC-1.5 4xx-on-Get branch:

- `Init()` fires `fetchUserForEditCmd(svc, userID)` — single GET (D-W7 / AC-1.3).
- On `userEditLoadedMsg`: build `form.Form` with `FieldSpecs()` + `profileToInitial(user)`, transition to `Editing`.
- On `userEditLoadFailedMsg`: store err, transition to `Errored`, View renders "Cannot edit: <summary>" — never reaches the form chrome (AC-1.5).
- `Ctrl+S`: short-circuit if Dirty==0; client validate → on fail focus first invalid field; on pass build `domain.UserProfilePatch` from `Diff()` (key → *string), enter `Saving`, fire `saveProfileCmd`.
- On `userEditSaveSucceededMsg`: update form (SetSaving(false)), set "Updated <name>" toast, emit `shared.UserUpdatedMsg{User}` to the App Shell — which patches the list cache and pops nav.
- On `userEditSaveFailedMsg`: SetSaving(false); if `*BadRequestError` → `form.ApplyServerErrors(bre.Causes)` inline; otherwise surface "Save failed: …" in the footer.
- `Esc` on dirty Editing → state transitions to `DiscardConfirm`. View renders "Discard unsaved changes? (y/N)".
- `y/Y/Enter` in DiscardConfirm → emit `form.DiscardRequestedMsg{Confirmed: true}`; `n/N/Esc` → return to Editing.
- `Ctrl+C` always returns `tea.Sequence(Println(View), Quit)` so EditModel runs as a teatest root.
- `EscapeWillAct()` implements the App Shell's interface — returns true while a dirty form / discard confirm is in flight so the App Shell's Esc handler forwards instead of popping nav.

`FieldSpecs()` is the 11-row authoritative spec catalog (Identity / Contact / Organization sections; login = KindReadOnly).

`buildPatch(diff)` switches on each known field key to populate the corresponding `*string` pointer (D-T4).

```
internal/tui/users  ─ 10 RED → 10 GREEN
  - Test_UserEdit_OnEntry_CallsPortGet_Once             ✓
  - Test_UserEdit_Loading_403_DoesNotOpenForm           ✓
  - Test_UserEdit_Render_Has11FieldLabels_4Sections     ✓
  - Test_UserEdit_LoginField_RendersReadOnly            ✓
  - Test_UserEdit_Save_PartialMergeBody_Success         ✓
  - Test_UserEdit_Save_Success_RendersUpdatedToast      ✓
  - Test_UserEdit_Save_400Validation_InlineFieldErrors  ✓
  - Test_UserEdit_Esc_Clean_NoDiscardConfirm            ✓
  - Test_UserEdit_Esc_Dirty_OpensDiscardConfirm         ✓
  - Test_UserEdit_Dirty_Counter_RendersInFooter         ✓
```

Cumulative: **30 GREEN flips**.

---

## Step 6 — `internal/app/app.go` shell wiring

**Files:** `internal/app/app.go`, `internal/tui/users/list.go`, `internal/tui/users/edit.go` (Cmd factory).

Changes:
- `Model.editTargetID` field — holds the userID for the next `ScreenUserEdit` build (set by `OpenUserEditMsg`).
- `shared.OpenUserEditMsg` handler — discards any cached `ScreenUserEdit` instance, sets `editTargetID`, `pushNav(ScreenUserEdit)`, `ensureScreen` → builds EditModel via `buildScreen`.
- `buildScreen(ScreenUserEdit)` — constructs `users.NewEditModel(EditDeps{Svc, UserID, Clock, Logger, Width, Height})`.
- `userEditService()` helper — prefers `m.deps.Services.Users` (production), falls back to `service.NewUsersService(m.deps.UsersPort, ...)` for tests that only inject the port.
- `shared.UserUpdatedMsg` handler — forwards to `ScreenUsers` for cache patching, pops nav back to the previous frame, and fires `RefreshScreenMsg` so the list reflects the new snapshot.
- `users/list.go` `e` key (list mode) — emits `shared.OpenUserEditMsg{ID: cursorUserID}` (falls back to empty ID when no row visible; the EditModel surfaces the error rather than silently no-op'ing — keeps the route uniform).
- `users/list.go` `e` key (detail mode) — same emission scoped to the open user.
- `users/edit.go` `openUserEditCmd(id)` factory — wraps OpenUserEditMsg as a Cmd so the list can dispatch via the standard reducer pattern.

The Phase 5 stub already added:
- `ScreenUserEdit` const + `String()` case
- `OverlayDiscardConfirm` const
- `screenFromName` aliases for `:user-edit` palette

Discard confirm UX is implemented **inside** EditModel (state machine `EditStateDiscardConfirm`), not the App Shell overlay system. The App Shell's `OverlayDiscardConfirm` const remains in place for Phase 7+ refactors; right now `EditModel.View()` renders the y/N prompt inline. Test `Test_UserEdit_Esc_Dirty_OpensDiscardConfirm` checks that "Discard" appears in output — which it does via the model-level renderer. The App Shell's existing Esc precedence (escIsCritical → child's `EscapeWillAct`) cleanly defers to EditModel while the form is dirty.

```
internal/app  ─ 2 RED → 2 GREEN
  - Test_AppModel_OpenUserEditMsg_PushesUserEditScreen   ✓
  - Test_AppModel_EKey_OnUsersList_OpensUserEditScreen   ✓
  (lock-ins Test_AppModel_SwitchScreen_UserEdit_ResolvesViaPalette,
   Test_ScreenUserEdit_String_IsUserEdit,
   Test_ScreenUserEdit_KindIsDistinct,
   Test_OverlayDiscardConfirm_IsDistinctFromActionConfirm all continue to pass.)
```

**Cumulative: 32 GREEN flips (matches Phase 5 RED count exactly).**

---

## Final Suite

```
go test ./... -count=1 -timeout 180s
```

- All 14 packages with REQ-W01 surface pass.
- Non-W01 packages (apps / groups / logs / overlay / policies / rules / shared / etc.) unaffected.
- `go test ./... -race -count=1` also clean.
- `go vet ./...` clean.

Test count in the REQ-W01 envelope (domain + okta + service + form + users + app):

| Package | New REQ-W01 RED→GREEN | Lock-in (already GREEN in Phase 5) |
|---|---|---|
| internal/domain | 1 | 14 |
| internal/okta | 6 | 1 |
| internal/service | 4 | 0 |
| internal/tui/shared/form | 9 | 2 |
| internal/tui/users | 10 | 0 |
| internal/app | 2 | 4 |
| **Total** | **32** | **21** |

Plus the broader users / app suite (list/detail/lifecycle/palette/etc.) remains GREEN — no regressions introduced.
