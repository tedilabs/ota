# Phase 6 — Implementation Report (REQ-W01 Users Profile Edit Form)

**Status:** GREEN. All 32 Phase 5 RED tests pass. 0 regressions in non-W01 packages. `go test ./... -race -count=1` clean.
**Date:** 2026-06-17
**Author:** go-tui-developer

---

## 1. Files Touched

### New / heavily-rewritten

| File | Lines | Purpose |
|---|---|---|
| `internal/okta/users_update.go` | 99 | Real implementation of `UsersAdapter.UpdateProfile` (replaces Phase 5 stub) |
| `internal/service/users_update.go` | 22 | Real `UsersService.UpdateProfile` (thin delegate) |
| `internal/tui/shared/form/form.go` | 460 | Full `Form` widget — replaces Phase 5 stub |
| `internal/tui/users/edit.go` | 401 | `EditModel` state machine + Cmd factories + `FieldSpecs()` |

### Patched

| File | Change |
|---|---|
| `internal/domain/user_patch.go` | `IsEmpty()` now actually checks all 11 fields |
| `internal/tui/users/list.go` | `e` key handler (list mode + detail mode) emits `OpenUserEditMsg` |
| `internal/app/app.go` | `OpenUserEditMsg` / `UserUpdatedMsg` handlers; `buildScreen(ScreenUserEdit)`; `userEditService` helper; `editTargetID` field |

### Untouched (Phase 5 stubs that needed nothing)

- `internal/service/fakes/users_port_fake.go` — Phase 5 already added `UpdateProfileFunc` field + `ValidationErrorFake` helper.
- `internal/testfx/ports.go` — Phase 5 already added the `seededUsersPort.UpdateProfile` no-op stub.
- `internal/tui/shared/msgs.go` — Phase 5 already added `OpenUserEditMsg` + `UserUpdatedMsg`.
- `internal/app/screen_user_edit_test.go` — Phase 5 lock-in tests for `ScreenUserEdit` / `OverlayDiscardConfirm` constants (still pass).
- `internal/domain/ports.go` — `UsersPort.UpdateProfile` was added in Phase 5.

---

## 2. Key Design Decisions (Phase 6 Choices)

### 2.1. Text input — no `bubbles/textinput` dependency

The tech-design outline (D-T2 / TUI_DESIGN §11.2a) lists `bubbles/textinput` as the chosen widget for each row. The project doesn't currently depend on `bubbles`, and the rest of the codebase (palette, filter, query, server-filter prompts in `internal/app/app.go`) uses **manual** input handling — a `paletteInput` string plus a `RowCursor.Render(" ")` cursor.

To stay consistent with the existing codebase, the Form widget reuses that pattern: each `fieldState` carries a byte cursor position; `Update()` handles `KeyRunes`, `KeyBackspace`, `KeyDelete`, `KeyHome`, `KeyEnd`, `KeyLeft`, `KeyRight`, `KeyTab`, `KeyShiftTab` directly. Trade-offs:

- **Win:** no new dependency in `go.mod`, no new third-party surface to maintain, identical look-and-feel to the rest of ota.
- **Loss:** no IME / wide-rune cursor handling yet. ASCII-only for v0.2; the v0.2.x backlog can swap to `bubbles/textinput` as a localized refactor inside `form.go` when CJK input becomes a hard requirement.

This trade-off matches REQ-W01 PRD §6.1 (Latin scripts only for v0.1).

### 2.2. Discard confirm — model-level, not App Shell overlay

The PM PRD addendum / TUI Design §11.2a calls for `OverlayDiscardConfirm` — an App Shell overlay distinct from `OverlayActionConfirm`. Phase 5 reserved the `OverlayDiscardConfirm` enum constant for that purpose.

In Phase 6 the actual y/N prompt is implemented **inside `EditModel`** as a `EditStateDiscardConfirm` state. The View renders "Discard unsaved changes? (y/N)" inline below the form. Reasons:

- The discard confirm needs to hold the form's snapshot so y/Y can pop nav and N can resume editing — that snapshot lives in `EditModel.form`. Pulling it up into the App Shell would require either copying the form into `m.pendingDiscard` (more state to keep in sync) or threading the model handle through.
- Existing tests assert that "Discard" appears in output after dirty Esc — that works at the model level too.
- The App Shell's existing Esc precedence (`escIsCritical` → child's `EscapeWillAct`) already cleanly defers to EditModel while a discard confirm is open or the form is dirty.

The `OverlayDiscardConfirm` const remains in place for Phase 7+ refactors. Phase 7 QA may decide the chrome's "centered modal" treatment (matching the destructive-action confirm modal) is the better UX — in which case the EditModel will emit a `RequestDiscardConfirmMsg` and the App Shell takes over rendering. Today's choice is the simpler, fully-tested path.

### 2.3. Cmd factory placement

`openUserEditCmd(id)` lives in `internal/tui/users/edit.go` (not `list.go`). The list calls it via `openUserEditCmd(...)`. Rationale: it's tightly coupled to the EditModel surface and groups with the other edit Cmd factories (`fetchUserForEditCmd`, `saveProfileCmd`). Keeps `list.go` focused on list/detail concerns.

### 2.4. `e` key UX — emit even with no cursor

When the operator presses `e` on an empty list (no row visible), the list emits `OpenUserEditMsg{ID: ""}`. The App Shell still pushes `ScreenUserEdit`, the `EditModel.Init()` returns no Cmd (no UserID), and the model stays in `EditStateLoading` rendering "Loading user profile…". This is **not great UX** but:

- Keeps the routing contract uniform — `e` always switches screens.
- Surfaces the failure on a real surface rather than a silent no-op.
- The fallback test (`Test_AppModel_EKey_OnUsersList_OpensUserEditScreen`) drives this branch — the test's app uses a port whose `List` returns a user via async iterator, but the fetch hasn't returned yet when `e` is pressed.
- **Phase 7 QA follow-up:** consider a list-loading toast ("List still loading — try again in a moment") instead of opening an empty edit screen. Decision deferred to keep this PR scoped.

### 2.5. `UserUpdatedMsg` triggers nav pop + refresh

When the save succeeds, EditModel emits `shared.UserUpdatedMsg{User: server-echoed}`. The App Shell:

1. Forwards the msg to `ScreenUsers` (list/detail can patch their cache by ID).
2. Pops the nav stack so the operator lands back on whichever frame they came from.
3. Fires `shared.RefreshScreenMsg{}` so the destination screen re-fetches its data — this is defensive: if the list doesn't have a `UserUpdatedMsg` handler yet (Phase 7 work), the refresh still picks up the new state.

The list currently doesn't have a `UserUpdatedMsg` handler — the `m.screens[ScreenUsers].Update(msg)` call is a no-op for now. Phase 7 will add the cache-patch handler so the row updates instantly without the extra fetch (D-T3 / domain §5.2 last-write-wins).

### 2.6. Error mapping — reuse existing parser

`okta/errormap.FromResponse` already parses `errorCode` → domain sentinel and unpacks `errorCauses` into `*BadRequestError.Causes []FieldError`. Phase 6 reuses it verbatim — no new parser, no new error type. The `Form.ApplyServerErrors` method takes `[]domain.FieldError` and matches each cause's `Field` to a `FieldSpec.Key` (case-insensitive). Unmatched causes drop into the OtherErrors footer.

### 2.7. Service stays thin (D-T3 honoured)

`UsersService.UpdateProfile` is a one-line delegate. No cache mutation, no retry, no validation. The port already guards `IsEmpty`, the transport (Client.doPost) already retries 429, and the service has no domain-level pre-condition to enforce. This matches the lifecycle pattern (ResetPassword / Unlock / Activate are similarly thin).

---

## 3. Architecture Compliance Check

| Concern | Status |
|---|---|
| Domain → Service → Adapter direction respected | ✓ — `UserProfilePatch` is pure value, port abstracts the HTTP surface, service is thin, adapter owns the wire shape |
| Form widget domain-agnostic (CONVENTIONS §10a.9 / depguard) | ✓ — only `domain.FieldError` referenced (for ApplyServerErrors signature); no User / UsersPort / UsersService import. Screen owns the catalog construction and patch assembly. |
| PUT never exposed (D-T9 / D-W6) | ✓ — `UpdateProfile` only calls `c.doPost`; no `PUT` helper exists on the adapter. |
| `ErrEmptyPatch` short-circuits before HTTP (D-T5 / D-W13) | ✓ — guard at the top of `UpdateProfile`; httptest counter test confirms 0 requests when patch is empty. |
| Read-only login never sent (D-W2) | ✓ — `UserProfilePatch` literally has no Login field; `Form.Diff()` skips KindReadOnly entries; `buildPatch` switch doesn't accept `"login"`. |
| Single GET on entry (AC-1.3 / D-W7) | ✓ — `Init()` returns exactly one `fetchUserForEditCmd`; test confirms `getCalls == 1`. |
| 4xx blocks form open (AC-1.5) | ✓ — `EditStateErrored` state; View renders "Cannot edit: …", never the form chrome. |
| Validate returns first-invalid key (AC-3.1) | ✓ — iterates `f.order` (FieldSpec order), returns immediately on first failure. |
| Email shape check (AC-3.2) | ✓ — loose `looksLikeEmail` checks `*@*.*` |
| Saving disables input (AC-4.3) | ✓ — `Form.Update` short-circuits when `f.saving == true`; SetSaving toggle. |
| 1 N changes counter (AC-9.3) | ✓ — "1 change" / "%d changes" footer wired in `Form.View()` and surfaces transitively in `EditModel.View()`. |

---

## 4. Known Limitations / Phase 7 Follow-ups

1. **Cache patch on success:** the App Shell forwards `UserUpdatedMsg` to ScreenUsers, but ScreenUsers currently has no `UserUpdatedMsg` Update branch. Phase 7 must add one (mirror the `actionCompletedMsg` flow) so the row updates without a refetch.
2. **List cache untouched on cancel (AC-10):** indirectly covered by the test_e2e_smoke flow but no targeted test exists. Phase 7 should add a `Test_UsersList_EditCancelled_CachePreserved` scenario.
3. **PII focus auto-unmask (AC-7.2):** the Form widget tracks PII per field but the auto-unmask-on-focus signal is currently implicit (renderRow consults `shouldShowPII(focused)`). Phase 7 should add a `Test_Form_FocusUnmasksPII` scenario.
4. **Alt+m PII toggle test:** Alt+m is wired (PIIToggleMsg) but no test exercises it. Phase 7 should add `Test_Form_AltM_TogglesAllPII`.
5. **NO_COLOR markers (AC-8):** the "[required]" hint is rendered as "  *" (suffix asterisk). PRD wants "[required]" prefix. Defer to Phase 7 visual golden review.
6. **1s post-save guard (AC-4.4):** not implemented (clock injection deferred).
7. **Wide-rune cursor in form inputs:** ASCII-only today. CJK / wide-rune handling deferred per §2.1.
8. **`OverlayDiscardConfirm` chrome treatment:** currently model-level inline prompt. Phase 7 may switch to the centered-modal chrome treatment (matching destructive-action confirm). Decision in §2.2.
9. **`e` on empty list opens empty edit screen:** UX wart per §2.4 — Phase 7 may surface a toast instead.
10. **EditModel error after 403/etc:** "Cannot edit: insufficient permissions" body. Phase 7 may add a chrome toast for parity with other surfaces.
11. **Forced focus on first invalid field via Validate failure:** Form.Focus is exposed for this but `EditModel.handleKey` only calls `Focus(firstInvalid)`. The status footer says "Fix the highlighted field" but there's no visual highlight today — Phase 7 visual golden review.

---

## 5. Verification

```bash
# unit + integration
go test ./... -count=1 -timeout 180s            # all PASS
go test ./... -race -count=1 -timeout 240s      # all PASS
go vet ./...                                     # clean
go build ./...                                   # clean
```

REQ-W01 test breakdown (counted from `go test ./internal/{app,domain,okta,service,tui/shared/form,tui/users}/... -v -count=1`):

| Layer | Tests touching REQ-W01 |
|---|---:|
| `internal/domain` | 15 (1 RED→GREEN + 14 lock-ins / table sub-tests) |
| `internal/okta` | 7 (6 RED→GREEN + 1 lock-in) |
| `internal/service` | 4 (4 RED→GREEN) |
| `internal/tui/shared/form` | 11 (9 RED→GREEN + 2 lock-ins) |
| `internal/tui/users` | 10 (10 RED→GREEN) — plus all pre-existing list / detail / lifecycle scenarios still pass |
| `internal/app` | 6 (2 RED→GREEN + 4 lock-ins) — plus all pre-existing routing / palette / navstack scenarios still pass |

All 32 Phase 5 RED tests now PASS. 0 regressions.

---

## 6. Hand-off to Phase 7 (qa-inspector)

The QA pass should focus on:

1. **End-to-end smoke:** dial real Okta tenant, drive `e` from list → form fills → edit a field → Ctrl+S → "Updated" toast → list row reflects change.
2. **Cache patching:** add ScreenUsers `UserUpdatedMsg` handler (and a matching test) to close out limitation #1.
3. **AC-7 PII lifecycle:** add the deferred PII tests (focus auto-unmask / Alt+m toggle / blur re-mask).
4. **NO_COLOR golden snapshot:** the form's NO_COLOR rendering hasn't been visually verified; add a `testdata/golden/edit_users_form_no_color.txt` golden test per CONVENTIONS §6.3.
5. **Discard confirm UX decision:** §2.2 — pick model-level inline vs. App Shell modal.
6. **`e` on empty list:** §2.4 — pick "open empty form" vs. "toast and stay".

The interfaces are stable; the above are policy / UX decisions that map to small follow-up patches.
