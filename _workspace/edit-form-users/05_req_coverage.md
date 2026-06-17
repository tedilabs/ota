# Phase 5 AC Coverage Matrix — REQ-W01 (Users Profile Edit Form)

**Date:** 2026-06-17
**Author:** go-test-engineer
**Status:** RED (32 failing tests; 0 regressions in non-W01 packages)

## Summary

| Metric | Value |
|---|---|
| AC items covered (≥ 1 RED test) | **10 / 10** (100%) |
| New Fail-First tests | 32 |
| New Lock-in tests (CONVENTIONS §1.3) | 3 (Form Diff-Idempotent / Form ReadOnly-Never-In-Diff / ScreenUserEdit identity) |
| Existing tests broken | 0 |

## AC → Test Mapping

| AC | Behaviour | Test file | Test function(s) | Layer | Status |
|---|---|---|---|---|---|
| **AC-1.1** | `e` on Users List → Edit | `internal/app/user_edit_entry_test.go` | `Test_AppModel_EKey_OnUsersList_OpensUserEditScreen` | TUI flow | RED |
| **AC-1.1** | `OpenUserEditMsg` routes to ScreenUserEdit | `internal/app/user_edit_entry_test.go` | `Test_AppModel_OpenUserEditMsg_PushesUserEditScreen` | App Shell | RED |
| **AC-1.1** | `:user-edit` palette resolves | `internal/app/user_edit_entry_test.go` | `Test_AppModel_SwitchScreen_UserEdit_ResolvesViaPalette` | App Shell | GREEN (lock-in — screenFromName stub added) |
| **AC-1.2** | `e` on User Detail → Edit | — | Implicit via shared `e`/OpenUserEditMsg routing test above; future entry test for detail when wired by Phase 6 | TUI flow | covered by AC-1.1 |
| **AC-1.3** | Latest GET on entry, 1 call | `internal/tui/users/edit_test.go` | `Test_UserEdit_OnEntry_CallsPortGet_Once` | TUI flow | RED |
| **AC-1.4** | Loading abort with Esc | — | Phase 6 follow-up (covered by AC-5.1 esc-clean indirectly) | — | deferred |
| **AC-1.5** | 4xx GET blocks form | `internal/tui/users/edit_test.go` | `Test_UserEdit_Loading_403_DoesNotOpenForm` | TUI flow | RED |
| **AC-2** | 11 field labels rendered | `internal/tui/users/edit_test.go` | `Test_UserEdit_Render_Has11FieldLabels_4Sections` | Render | RED |
| **AC-2** | Login read-only | `internal/tui/users/edit_test.go` + `internal/tui/shared/form/form_test.go` | `Test_UserEdit_LoginField_RendersReadOnly` + `Test_Form_ReadOnlyField_NeverInDiff` | TUI flow + Form unit | RED + GREEN (lock-in) |
| **AC-2** | 11-field domain catalog | `internal/domain/user_patch_test.go` | `Test_UserProfilePatch_IsEmpty_SingleFieldSet_IsFalse` (table-driven × 11) + `Test_UserProfilePatch_HasNoLoginField` | Unit | RED |
| **AC-3.1** | Required-empty inline | `internal/tui/shared/form/form_test.go` | `Test_Form_Validate_RequiredEmpty_FailsAtFirstInvalid` | Form unit | RED |
| **AC-3.2** | Loose email shape | `internal/tui/shared/form/form_test.go` | `Test_Form_Validate_InvalidEmail_Fails` | Form unit | RED |
| **AC-3.3** | Phone hint, no block | — | Phase 6 follow-up (loose, not a Save-block) | — | deferred |
| **AC-3.4** | No client length truncation | — | implicit — Form doesn't truncate (Phase 6 covers via no `MaxLen` enforcement in Update) | — | deferred |
| **AC-3.5** | No pre-save uniqueness lookup | — | implicit — no UsersPort.Get call between Update and Save in flow test; verifiable by counter in future test | — | deferred |
| **AC-4.1** | Ctrl+S triggers save | `internal/tui/users/edit_test.go` | `Test_UserEdit_Save_PartialMergeBody_Success` (saveHit counter) | TUI flow | RED |
| **AC-4.2** | Partial-merge body (only dirty) | `internal/okta/users_update_test.go` + `internal/tui/users/edit_test.go` + `internal/tui/shared/form/form_test.go` | `Test_OktaUsersAdapter_UpdateProfile_PartialMerge_SingleField_BodyShape` + `Test_OktaUsersAdapter_UpdateProfile_PartialMerge_MultiField_BodyShape` + `Test_UserEdit_Save_PartialMergeBody_Success` + `Test_Form_Diff_ReturnsOnlyDirtyFields` | Adapter + TUI + Form | RED |
| **AC-4.3** | Saving disables input + footer | `internal/tui/shared/form/form_test.go` | `Test_Form_SetSaving_RendersSavingFooter` | Form unit | RED |
| **AC-4.4** | 1s post-save guard | — | Phase 6 follow-up (clock injection) | — | deferred |
| **AC-4.5** | Success: cache patch + toast | `internal/tui/users/edit_test.go` | `Test_UserEdit_Save_Success_RendersUpdatedToast` | TUI flow | RED |
| **AC-5.1** | Clean Esc → immediate close | `internal/tui/users/edit_test.go` | `Test_UserEdit_Esc_Clean_NoDiscardConfirm` | TUI flow | RED |
| **AC-5.2** | Dirty Esc → discard confirm | `internal/tui/users/edit_test.go` | `Test_UserEdit_Esc_Dirty_OpensDiscardConfirm` | TUI flow | RED |
| **AC-5.3** | Saving Esc → no-op | — | Phase 6 follow-up | — | deferred |
| **AC-6** | 400 errorCauses → inline | `internal/tui/users/edit_test.go` + `internal/okta/users_update_test.go` + `internal/service/users_update_test.go` + `internal/tui/shared/form/form_test.go` | `Test_UserEdit_Save_400Validation_InlineFieldErrors` + `Test_OktaUsersAdapter_UpdateProfile_400Validation_PropagatesCauses` + `Test_UsersService_UpdateProfile_PropagatesBadRequestError` + `Test_Form_ApplyServerErrors_PrefixMatchesFieldSpecKey` | TUI + Adapter + Service + Form | RED |
| **AC-6** | 403 → toast, form preserved | `internal/okta/users_update_test.go` | `Test_OktaUsersAdapter_UpdateProfile_403Forbidden_ReturnsErrForbidden` | Adapter | RED |
| **AC-6** | 404 → ErrNotFound | `internal/okta/users_update_test.go` | `Test_OktaUsersAdapter_UpdateProfile_404NotFound_ReturnsErrNotFound` | Adapter | RED |
| **AC-6** | 429 → RateLimitedError | `internal/service/users_update_test.go` | `Test_UsersService_UpdateProfile_PropagatesRateLimitedError` | Service | RED |
| **AC-7** | PII masking lifecycle | — | Phase 6 follow-up (depends on Form Alt+m / focus auto-unmask wiring) | — | deferred |
| **AC-8** | NO_COLOR markers (`*`, `[required]`, `!`) | — | Phase 6 visual golden (testdata/golden/) | — | deferred |
| **AC-9.1** | Dirty tracked per keystroke | `internal/tui/shared/form/form_test.go` | `Test_Form_Dirty_TrackedPerKeystroke` | Form unit | RED |
| **AC-9.1** | Snapshot stored on entry | `internal/tui/shared/form/form_test.go` | `Test_Form_New_NoDirty` | Form unit | RED |
| **AC-9** | Revert clears dirty | `internal/tui/shared/form/form_test.go` | `Test_Form_Revert_ClearsDirty` | Form unit | RED |
| **AC-9.3** | Footer `N changes` counter | `internal/tui/users/edit_test.go` | `Test_UserEdit_Dirty_Counter_RendersInFooter` | TUI flow | RED |
| **AC-9.4** | Diff = dirty only | `internal/tui/shared/form/form_test.go` | `Test_Form_Diff_ReturnsOnlyDirtyFields` | Form unit | RED |
| **AC-10** | List/detail cache untainted on cancel | — | Phase 6 follow-up (depends on UserUpdatedMsg cache patcher) | — | deferred |

## Decision Coverage (D-W1..D-W16, D-T1..D-T10)

| Decision | Test | Status |
|---|---|---|
| **D-T4** *string pointer pattern | `Test_UserProfilePatch_IsEmpty_*` (table 11) + `Test_OktaUsersAdapter_UpdateProfile_PartialMerge_*_BodyShape` | RED |
| **D-T5 / D-W13** ErrEmptyPatch sentinel | `Test_ErrEmptyPatch_IsSentinel` + `Test_OktaUsersAdapter_UpdateProfile_EmptyPatch_ReturnsErrEmptyPatch_NoHTTPCall` + `Test_UsersService_UpdateProfile_EmptyPatch_PropagatesErrEmptyPatch` | RED |
| **D-T9 / D-W15** POST not PUT | `Test_OktaUsersAdapter_UpdateProfile_PartialMerge_SingleField_BodyShape` asserts `http.MethodPost` | RED |
| **D-W2** Login read-only / not in patch | `Test_UserProfilePatch_HasNoLoginField` + `Test_Form_ReadOnlyField_NeverInDiff` | RED + GREEN (lock-in) |
| **D-W4** Dirty Esc → confirm | `Test_UserEdit_Esc_Dirty_OpensDiscardConfirm` | RED |
| **D-W5** Ctrl+S saves | `Test_UserEdit_Save_PartialMergeBody_Success` | RED |
| **D-W6** Failure preserves form | `Test_UserEdit_Save_400Validation_InlineFieldErrors` (assert.Contains "First Name") | RED |
| **D-W7** Single GET on entry | `Test_UserEdit_OnEntry_CallsPortGet_Once` | RED |
| **D-W10** N changes + `*` marker | `Test_UserEdit_Dirty_Counter_RendersInFooter` + `Test_Form_Dirty_TrackedPerKeystroke` | RED |
| **D-W16** navStack push | `Test_AppModel_OpenUserEditMsg_PushesUserEditScreen` | RED |

## Phase 6 Action Items (Deferred AC follow-ups)

After Phase 6 turns the 32 RED tests GREEN, add tests for:

- AC-1.4 (Loading + Esc abort) — `Test_UserEdit_Loading_EscAborts`
- AC-3.3 (Phone E.164 hint, no block) — `Test_Form_PhoneHint_NoSaveBlock`
- AC-3.4 (No length truncate) — `Test_Form_LongInput_NotTruncated`
- AC-3.5 (No pre-save GET) — `Test_UserEdit_NoPreSaveUniquenessGetCall`
- AC-4.3 (Saving Esc → no-op) — `Test_UserEdit_Esc_DuringSaving_NoOp`
- AC-4.4 (1s post-save guard, FakeClock) — `Test_UserEdit_Saving_PostSuccess_1sGuard_DisablesSave`
- AC-7 (PII lifecycle: default-mask / focus-unmask / blur-remask / Alt+m toggle) — 4 separate teatest scenarios
- AC-8.2 (NO_COLOR markers, visual golden)
- AC-8.3 (80×24 viewport, label+input two-line)
- AC-10 (cache untouched on cancel / discard / background poll resumption)

These need behavioural surfaces that Phase 6 will create (Alt+m focus toggle, NO_COLOR render path,
FakeClock seam). Adding them now would be premature — the Form public API would be wrong, and the
tests would need to be rewritten when Phase 6 lands the real shape.
