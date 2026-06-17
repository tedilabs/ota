# QA Findings — REQ-W01 Users Profile Edit Form (SCR-012)

**Date:** 2026-06-17
**Reviewer:** qa-inspector (Phase 7)
**Scope:** PRD §5.6 REQ-W01 (AC-1 … AC-10, D-W1 … D-W16) ↔ TUI_DESIGN §SCR-012 / §11.2a / §3.4 / §3.6 / §3.7 / §10.1 / §13 ↔ implementation under `internal/domain`, `internal/okta`, `internal/service`, `internal/tui/shared/form`, `internal/tui/users`, `internal/app`.
**Method:** Cross-read PRD AC ↔ Design row ↔ implementation line by line (양쪽 동시 읽기). Production behaviour validated by lifting EditModel into the App Shell (the Phase 5 teatest scaffolds exercised it in isolation, which masks several App Shell coordination bugs).
**Verification:** New regression tests at `internal/app/user_edit_qa_regression_test.go` (4 FAIL-FIRST tests that reproduce QA-W01-01 … QA-W01-04). Full suite otherwise green at HEAD.

---

## 0. Headline

| Severity | Count | Ship blocker? |
|----------|-------|--------------|
| **Critical** | 1 | YES — silent data loss on a primary user gesture |
| **High** | 5 | YES — primary AC violations / production-only behaviour gaps |
| **Medium** | 6 | NO — UX / consistency; Phase 7+ patch acceptable |
| **Low** | 4 | NO — backlog |
| **Total** | 16 | |

**AC coverage today: 7 / 10 fully met.** AC-1.4 (Loading Esc abort), AC-4.3/4.4/5.3 (saving-state contract), AC-5.2 (dirty Esc), AC-7.3/7.4 (PII focus lifecycle), AC-8.2 (NO_COLOR markers), AC-10.1 (cache untainted on cancel) are partial or broken in production.

**Regression tests added (FAIL-FIRST):** 4 — `internal/app/user_edit_qa_regression_test.go`.

---

## 1. Methodology

Each finding follows this layout:

```
QA-W01-NN  <title>           Severity: <C|H|M|L>
  AC / D / §:  <which spec(s) it touches>
  Producer   :  <file:line of the spec / contract side>
  Consumer   :  <file:line of the implementation side>
  Repro      :  steps
  Expected   :  what the spec asks for
  Actual     :  what code does today
  Root cause :  one-sentence
  Fix sketch :  pointer
  Test       :  added FAIL-FIRST regression / proposed
```

The Phase 5 teatest scenarios exercise `EditModel` in isolation via `teatest.NewTestModel(m)`. That scaffold bypasses the App Shell's Esc precedence, palette routing, and overlay router — which is exactly where most of the production bugs live. The new regression suite in `internal/app/user_edit_qa_regression_test.go` wires EditModel through `app.New(...)` so the Shell's `escIsCritical` / `popNav` / `screenFromName` participate.

---

## 2. Findings — Critical

### QA-W01-01 — `Esc` on a dirty edit form silently discards unsaved changes (data loss)

- **Severity:** Critical — silent data loss on the most common cancel gesture.
- **AC / D:** PRD §5.6 AC-5.2, D-W4; TUI_DESIGN §SCR-012 상태머신 (`editing(dirty) → confirmDiscard`); §10.1 row "Users Profile Edit — Discard (REQ-W01, AC-5.2) | L1 | dirty 상태 ESC → 'Discard N changes? y/N' modal".
- **Producer (spec):** `docs/PRD.md:529`, `docs/TUI_DESIGN.md:1062,1219`.
- **Consumer (impl):**
  - `internal/app/app.go:2113-2122` — Esc precedence pops nav whenever the stack has > 1 frame, **regardless of `EscapeWillAct()`**.
  - `internal/app/app.go:1411-1432` — `escIsCritical()` only considers overlay / filter-input / query-input / server-filter-input / visual-line states. EditModel's `EditStateEditing(dirty)` is **not** in the critical list.
  - `internal/tui/users/edit.go:205-215` — EditModel's dirty-Esc branch sets `EditStateDiscardConfirm`, but the App Shell's pop already fires first, so EditModel never sees the Esc keystroke.
- **Repro (regression test reproduces this):**
  1. Launch ota → Users list active.
  2. Press `e` on the selected row → `ScreenUserEdit` pushes (nav stack = [`users`, `user-edit`]).
  3. Wait for the form to render.
  4. Type one rune into First Name → form is dirty (`Dirty()=1`).
  5. Press `Esc`.
- **Expected:** Discard confirm modal appears ("Discard 1 unsaved changes? y/N"). Active screen remains `user-edit`. Unsaved edits preserved until operator answers `y`/`Y`/`Enter`.
- **Actual:** Nav stack pops to `users`. EditModel.Update is never called. Operator's edit is lost without any confirmation. Active screen is `users`.
- **Why Phase 5 tests didn't catch this:** Phase 5 `Test_UserEdit_Esc_Dirty_OpensDiscardConfirm` (`internal/tui/users/edit_test.go:268-291`) wraps the bare EditModel in `teatest.NewTestModel(m)` — the App Shell isn't in the message path, so Esc goes directly into EditModel.handleKey which correctly transitions to `EditStateDiscardConfirm`. Production routes Esc through `app.handleKey` first (line 2113) where the nav pop wins.
- **Root cause:** The 2026-05-04 commit `a68426b` ("tighten Esc precedence") deliberately made Esc-pops-nav unconditional when stack depth > 1 ("Applied filters survive the pop — they're persistent screen state"). REQ-W01 introduces the first child screen whose Esc semantics are *not* idempotent — dirty edits are NOT persistent state, they're operator drafts in flight. The Esc precedence must defer to `EscapeWillAct()` for `ScreenUserEdit`.
- **Fix sketch (one of):**
  - **Option A (recommended):** Extend `escIsCritical()` to consult `EscapeOpStater` when the active screen is `ScreenUserEdit` so the in-screen Esc handler runs first.
  - **Option B:** Promote dirty-Esc to the App Shell — EditModel emits a `RequestDiscardConfirmMsg` (already declared but never used), App Shell raises `OverlayDiscardConfirm` (already declared but never used). This matches the original TUI_DESIGN §SCR-012 "신규 식별자" intent and the implementation_report §2.2 explicit deferral.
  - In either case, `Test_AppShell_Esc_OnDirtyEditForm_OpensDiscardConfirm` (added below) must turn green.
- **Affected files (fix touches):** `internal/app/app.go` (escIsCritical / new overlay handler), optionally `internal/tui/users/edit.go` (emit RequestDiscardConfirmMsg).
- **Regression test added:** `internal/app/user_edit_qa_regression_test.go::Test_AppShell_Esc_OnDirtyEditForm_OpensDiscardConfirm` — FAIL today (asserts `ActiveScreenName == "user-edit"` after dirty Esc; gets `"users"`).

---

## 3. Findings — High

### QA-W01-02 — `Esc` during the saving state pops nav while a POST is in flight

- **Severity:** High — races a confirmed mutation against UI navigation; UserUpdatedMsg arrives at a screen the operator already left.
- **AC / D:** PRD §5.6 AC-4.3 ("ESC도 비활성화 (race 방지)"), AC-5.3 ("저장 중 ESC: 비활성. footer hint 'Saving… use Ctrl+C to abort'"); TUI_DESIGN §SCR-012 상태머신 (saving 상태 ESC 비활성).
- **Producer (spec):** `docs/PRD.md:517,530`, `docs/TUI_DESIGN.md:1101`.
- **Consumer (impl):**
  - `internal/tui/users/edit.go:177-180` — EditModel.handleKey for `EditStateSaving` returns no-op. Correctly ignores Esc *if reached*.
  - `internal/app/app.go:2113-2117` — App Shell pops nav before EditModel sees the Esc, because `escIsCritical()` does not classify the saving state as critical.
- **Repro:** drive Edit form → fill a field → Ctrl+S → while save Cmd is in flight (blocked / slow Okta), press Esc.
- **Expected:** Active screen stays `user-edit`. Saving footer ("⠋ Saving…") stays visible. Only Ctrl+C aborts (cancelling ctx + preserving draft).
- **Actual:** App Shell pops nav to `users` immediately. The save POST completes in the background; `UserUpdatedMsg` arrives at the App Shell which forwards to ScreenUsers (no-op, no handler) and re-pops nav (no-op since stack is already at root). The operator believes they "cancelled the save" but Okta wrote the change.
- **Same root cause as QA-W01-01:** App Shell's `escIsCritical()` doesn't know about EditStateSaving.
- **Bonus violation:** `EditModel.Update` at `internal/tui/users/edit.go:121-123` intercepts Ctrl+C with `tea.Quit` (with a comment that the App Shell intercepts first in production). In production Ctrl+C fires the QuitConfirm overlay (`internal/app/app.go:2125`), so AC-4.3's "Ctrl+C만 강제 abort 허용 (요청 cancel + 폼 입력 보존)" is **not implemented at all** — Ctrl+C quits the whole app instead of aborting the save and preserving the form. Independent High-severity defect tracked under this finding.
- **Fix sketch:** Same hook as QA-W01-01 (`escIsCritical` / overlay-promote). Additionally, the App Shell's Ctrl+C handler must route to the active screen when `EditStateSaving`, and the save Cmd must accept a cancellable ctx (today both `fetchUserForEditCmd` and `saveProfileCmd` hard-code `context.Background()` — `internal/tui/users/edit.go:404,416`).
- **Regression test added:** `internal/app/user_edit_qa_regression_test.go::Test_AppShell_Esc_DuringSaving_DoesNotPopNav` — FAIL today.

### QA-W01-03 — `:edit` and `:e` palette commands do not resolve to ScreenUserEdit

- **Severity:** High — PRD AC-1 lists palette as one of two entry paths; TUI_DESIGN §3.4 row 335 explicitly defines `:edit` / `:e` with the "no user selected" toast contract. Today both palette inputs fail silently.
- **AC / D:** PRD §5.6 AC-1.2 / AC-1; TUI_DESIGN §3.4:335, §11.2a:2559 ("`e`/`:edit` 진입(AC-1.1/1.2)"), §12.3:2623 ("v0.2.0 REQ-W01 (Users Profile Edit Form) 진입으로 배정").
- **Producer (spec):** `docs/TUI_DESIGN.md:335`.
- **Consumer (impl):**
  - `internal/app/app.go:792-841` — `screenFromName` only matches `"user-edit"`, `"user_edit"`, `"useredit"`, `"edit-user"`, `"edit_user"`, `"edituser"`. The natural `"edit"` and `"e"` aliases are missing.
  - `internal/app/app.go:2307-2343` — `paletteCommandPool` does not list `edit` for autocomplete. Operator typing `:edit` gets no suggestion either.
- **Repro:** Press `:` → type `edit` → press Enter. The palette closes, but no screen switch fires.
- **Expected:** Palette resolves to `ScreenUserEdit`. When active screen has no selected user (e.g. on an empty Users list with no rows, or on a non-user screen), surface toast `"no user selected"` (§3.4) and stay put.
- **Actual:** `screenFromName("edit")` returns `false`, the palette dispatch falls through to the "unknown command" branch (`internal/app/app.go:2554-2604` resolves nothing); the operator sees no effect.
- **Fix sketch:** Add `case "edit", "e":` to `screenFromName` returning `ScreenUserEdit`. Add `"edit"` to `paletteCommandPool`. Additionally, when palette dispatch lands on `ScreenUserEdit`, the App Shell must resolve the target user from the active screen (Users list cursor row or Detail's `detailUser.ID`) — this isn't a `SwitchScreenMsg` no-op, it's effectively an `OpenUserEditMsg{ID: <inferred>}`. When no user is inferable, toast "no user selected" (§3.4).
- **Regression tests added:**
  - `Test_AppShell_PaletteEdit_ResolvesScreenUserEdit` — FAIL today.
  - `Test_AppShell_PaletteE_ResolvesScreenUserEdit` — FAIL today.
- **PM decision requested:** The "infer user from active screen" half is non-trivial — should `:edit` from the Logs screen toast "no user selected", or should it pivot to the most recently focused User screen? Recommendation: toast (matches TUI_DESIGN §3.4 literal). Add a separate `OpenUserEditFromActiveMsg` so the palette stays declarative.

### QA-W01-04 — `e` key in Logs screen collides with REQ-W01's reserved single meaning

- **Severity:** High (consistency / cognitive load); upgradable to Critical if operator habit conditioning causes wrong-screen edits.
- **AC / D:** TUI_DESIGN §12.1:2610 ("**v0.2.0 신규 (REQ-W01)**. v1.2.0의 §15.7 Factors `(e) expand` 메모는 v0.1.2 탭 통합 이후 dead UX — v1.3.0 cleanup (DR-1)."); §3.6:418 ("`e` 키 적용 (v0.2.0+, REQ-W01): Users만. … 다른 리소스(Groups/Rules)는 v0.2.0 시점 mutation 미지원이므로 `e`는 SCR-012 디자인 외 화면에서 no-op + 토스트 `\"no edit action for <resource>\"`"); §12.3:2623.
- **Producer (spec):** `docs/TUI_DESIGN.md:418,2610,2623`.
- **Consumer (impl):** `internal/tui/logs/logs.go:925-927` — `case "e": return m.setRange(24 * time.Hour)`. Predates REQ-W01 (commit `66633d49`, 2026-04-27) and was not updated when REQ-W01 reserved `e` as the single-meaning edit-entry key.
- **Expected (TUI_DESIGN §3.6:418):** `e` on Logs → toast `"no edit action for <resource>"`. The 24-hour history shortcut must move to a different key (`24`/`Shift+D`/etc.).
- **Actual:** Operator pressing `e` on Logs unexpectedly snaps the history window to 24h.
- **Fix sketch:**
  - Drop `case "e"` from `internal/tui/logs/logs.go:925`. Reassign 24h shortcut to a free key (suggestion: `1d` or `Shift+D`). PM decision.
  - Add a global App-Shell `e` toast fallback: when `e` reaches the active screen and the screen doesn't define an `e` handler, toast `"no edit action for <resource>"` (TUI_DESIGN §3.6:418 literal).
- **Regression test:** to be added — `Test_AppShell_EKey_OnLogs_ToastsNoEditAction` (Phase 7 next pass; the regression suite added here focuses on the EditModel-side critical path).

### QA-W01-05 — `*` is overloaded: same glyph for required-field marker and dirty-field marker

- **Severity:** High (AC-8 / AC-9 ambiguity; affects NO_COLOR mode primarily).
- **AC / D:** PRD §5.6 AC-8.2 ("required 필드: 라벨 좌측 `[required]` 또는 `!`"), AC-9.2 ("변경 필드는 라벨에 `*` 마커"). The two markers are distinct in the spec.
- **Producer (spec):** `docs/PRD.md:560-562,569-572`.
- **Consumer (impl):**
  - `internal/tui/shared/form/form.go:262-281` — `renderRow` appends a single ` *` suffix when `s.Required`, but **never renders any dirty marker**. Dirty count surfaces only in the footer ("N changes").
  - Required marker uses ` *` (suffix), not `[required]` prefix or `!`.
- **Repro:** Open edit form, edit firstName (dirty=1) — the label still shows `First Name *` (required asterisk). No visual distinction between required and dirty.
- **Expected:**
  - Required: label prefix `[required]` or `!` (AC-8.2).
  - Dirty: label prefix `*` (AC-9.2). Both can coexist on a required-and-dirty field.
- **Actual:** Required-only marker `*`. Dirty marker missing entirely from per-field rendering.
- **Fix sketch:** Refactor `renderRow` to use distinct glyphs: `[required]` (or `!`) for required, `*` for dirty. Add a dedicated `dirty` boolean derived from `f.snapshot[s.Key] != f.current[s.Key]`.
- **Regression test:** add `Test_Form_RequiredMarker_IsBracketRequired` + `Test_Form_DirtyMarker_LeadsLabel` to `internal/tui/shared/form/form_test.go` (proposed; not added here).

### QA-W01-06 — `OverlayDiscardConfirm`, `RequestDiscardConfirmMsg`, `DiscardRequestedMsg`, `FieldFocusedMsg`, `FieldBlurredMsg` declared but never used (dead spec scaffolding)

- **Severity:** High (architecture drift — the App Shell overlay surface the design promised exists in name only; PII focus/blur lifecycle has no message path).
- **AC / D:** PRD §5.6 D-W16 (mount mode), AC-7.2/7.3/7.4 (PII focus lifecycle); TUI_DESIGN §SCR-012 "신규 식별자" (line 1198-1209); implementation_report §2.2.
- **Consumer (impl):**
  - `internal/app/app.go:186` — `OverlayDiscardConfirm` declared, never referenced after `screen_user_edit_test.go` lock-in.
  - `internal/tui/shared/form/form.go:531-549` — `DiscardRequestedMsg`, `PIIToggleMsg`, `FieldFocusedMsg`, `FieldBlurredMsg`, `SaveRequestedMsg` all declared.
    - `SaveRequestedMsg` — declared, never emitted. EditModel calls `saveProfileCmd` directly.
    - `DiscardRequestedMsg` — emitted by EditModel.handleDiscardConfirm (line 231,235) but no handler in App Shell (`grep -rn DiscardRequestedMsg internal/app/` returns 0 hits). The y/Y / Enter inside the confirm state therefore never pops nav — once the operator has answered "y", the form silently sits there until they press Esc again, which (after the QA-W01-01 fix) might or might not pop depending on the discard-state branch.
    - `FieldFocusedMsg` / `FieldBlurredMsg` — declared, never emitted. AC-7.3/7.4 (PII re-mask on blur + unchanged) have no signal path.
- **Fix sketch:** Either wire them up or delete them. Recommendation: keep `OverlayDiscardConfirm` and **wire it** as part of QA-W01-01's Option B (App Shell promotes the modal). Wire `DiscardRequestedMsg` into an `app.handleDiscardRequested` that pops nav.
- **Regression test:** add `Test_AppShell_DiscardConfirmYes_PopsNav` (proposed).

---

## 4. Findings — Medium

### QA-W01-07 — `:edit` palette `"no user selected"` toast is unimplemented

- **Severity:** Medium (subsumes part of QA-W01-03's fix; called out separately so the toast UX isn't dropped during the palette routing patch).
- **AC / D:** TUI_DESIGN §3.4:335.
- **Fix sketch:** When the palette resolves `:edit` and the active screen returns no inferable user ID, toast `"no user selected"` (`internal/app/actions.go:30` shows the toastCmdInfo helper to reuse).

### QA-W01-08 — `e` on empty Users list pushes a stuck "Loading user profile…" form

- **Severity:** Medium (UX dead-end, but operator can Esc out — once QA-W01-01 is fixed).
- **AC / D:** PRD §5.6 AC-1.1 ("선택된 행에서 `e` 키 입력 시"); implementation_report §2.4 explicitly defers this decision.
- **Consumer (impl):**
  - `internal/tui/users/list.go:833-839` — `e` always emits `OpenUserEditMsg{ID: id}` even when `cursorUser()` returns nil (passing `id=""`).
  - `internal/tui/users/edit.go:107-112` — `EditModel.Init()` returns `nil` Cmd when `UserID == ""`, so the model remains in `EditStateLoading` forever rendering "Loading user profile…".
- **Expected:** Toast `"no user selected"` (or `"list still loading — try again in a moment"`); active screen unchanged.
- **Actual:** Operator sees a permanently-stuck loading screen until Esc.
- **Fix sketch:** In `list.go:833`, when `cursorUser()` returns nil, emit `toastInfoCmd("no user selected")` instead of `openUserEditCmd("")`.
- **PM decision (deferred from impl §2.4):** confirm toast vs alternative.

### QA-W01-09 — Save success: ScreenUsers has no `UserUpdatedMsg` handler — cache patch is bypassed by RefreshScreenMsg full refetch

- **Severity:** Medium (extra GET against `/api/v1/users`; D-T3 last-write-wins server echo wasted; surfaces as a brief flash if the list re-fetch is slow).
- **AC / D:** PRD §5.6 AC-4.5 ("응답 body의 `User` 객체로 detail/list 캐시 갱신 (다른 admin의 동시 변경 부분 반영, 도메인 §5.2-2)"); implementation_report §2.5 explicitly defers this.
- **Consumer (impl):**
  - `internal/app/app.go:714-732` — App Shell forwards `UserUpdatedMsg` to ScreenUsers, but `internal/tui/users/list.go` has zero `UserUpdatedMsg` cases. The forward is dead.
  - `internal/app/app.go:730` — falls back to `refreshScreenCmd()` for safety.
- **Fix sketch:** Add `case shared.UserUpdatedMsg:` to `list.Model.Update`. Walk the cached `m.users` (and `m.detailUser` when `m.opened` matches), replace by ID with `msg.User`. Suppress the `RefreshScreenMsg` flush in `app.go:730` once that handler exists.
- **Regression test:** add `Test_UsersList_UserUpdatedMsg_PatchesCache` (proposed).

### QA-W01-10 — `EditModel.Form.snapshot` not refreshed on save success — diff persists after save

- **Severity:** Medium (latent — invisible today because the App Shell pops nav before the operator sees the post-save form, but breaks the moment QA-W01-09 is fixed and the operator might stay on the form for the 3-second toast).
- **AC / D:** PRD §5.6 AC-4.5, AC-9.4, D-T7 (snapshot semantics).
- **Consumer (impl):** `internal/tui/users/edit.go:134-141` — on `userEditSaveSucceededMsg`, the model sets `m.state = EditStateEditing` and `m.form.SetSaving(false)` but does NOT rebuild the form with a new snapshot from `msg.user`. `Form.Dirty()` still reports the pre-save diff. Footer would show "1 change" even after a successful save.
- **Fix sketch:** Rebuild the form: `m.form = form.New(FieldSpecs(), profileToInitial(msg.user))` (or add a `Form.Resnap(...)` method).
- **Regression test:** add `Test_UserEdit_AfterSaveSuccess_DirtyIsZero` (proposed).

### QA-W01-11 — `fetchUserForEditCmd` / `saveProfileCmd` use `context.Background()` — Esc / Ctrl+C cannot cancel in-flight HTTP

- **Severity:** Medium (long-tail Okta calls block the operator from leaving the form; AC-1.4 "ESC로 진입 취소 가능" not really actionable; AC-4.3 "Ctrl+C로 abort" not actionable).
- **AC / D:** PRD §5.6 AC-1.4, AC-4.3; PRD §6.3 ("`context.Context` 전파로 사용자 `Esc` 즉시 취소").
- **Consumer (impl):** `internal/tui/users/edit.go:402-410, 414-422`.
- **Fix sketch:** Store an `context.CancelFunc` on EditModel; cancel from the Esc / Ctrl+C branch; pass `ctx` into the Cmd. Pattern: same as logs tail's tick-cancel.

### QA-W01-12 — PII `focus blur + modified` should keep value unmasked (AC-7.4), but current code re-masks regardless

- **Severity:** Medium (PII UX violates §6.2).
- **AC / D:** PRD §5.6 AC-7.4 ("focus out + 수정 → 마스킹 없이 계속 표시 + dirty 마커").
- **Consumer (impl):** `internal/tui/shared/form/form.go:284-293` — `shouldShowPII` returns `focused` unconditionally for PII fields; doesn't consider whether the value differs from snapshot.
- **Fix sketch:** Add a "modified" branch:
  ```go
  if f.snapshot[s.Key] != f.current[s.Key] { return true }
  ```
- **Regression test:** add `Test_Form_PII_BlurredAfterEdit_StaysUnmasked` (proposed).

---

## 5. Findings — Low

### QA-W01-13 — `fieldState.piiMask` field declared and set but never read (dead code)

- **Severity:** Low (clarity / future bug bait).
- **Consumer:** `internal/tui/shared/form/form.go:52,91`. The actual masking uses `f.shouldShowPII(s, focused)` via `s.PII` + `f.piiAllUnmasked` + focus.
- **Fix:** remove the field; the comment `"when true, render value as bullets"` is misleading because nothing consults it.

### QA-W01-14 — `Form.Dirty()` has a `if f.specs == nil` guard inside the per-key loop that can never trip

- **Severity:** Low (cosmetic; will produce wrong N if it ever did trip).
- **Consumer:** `internal/tui/shared/form/form.go:309-311` — the guard is inside the `range f.current` loop, so it just causes early termination of every iteration when specs is nil. Cleaner as a pre-loop check.

### QA-W01-15 — Saving footer renders `"Saving…"` but no `POST /api/v1/users/{id}` URL hint as design wireframe shows

- **Severity:** Low (TUI_DESIGN §SCR-012 Saving block explicitly shows `⠋ Saving…   POST /api/v1/users/00u…x8     use <Ctrl+C> to abort`). The current footer is just `Saving…`.
- **Consumer:** `internal/tui/shared/form/form.go:246-247`.
- **Fix:** Add the abort hint at minimum.

### QA-W01-16 — Section header rendered as `── Identity ──\n` (no horizontal rule chars before/after the line), missing trailing extension

- **Severity:** Low (matches design intent, but actual divider is two dashes vs design's many).
- **Consumer:** `internal/tui/shared/form/form.go:212-219`. Wireframe shows `─ Identity ──────────────────────────────────`. Minor visual mismatch.

---

## 6. AC Coverage Matrix (final)

Cross-reference with `_workspace/edit-form-users/05_req_coverage.md`. Each AC is "fully met" only when the **production path** (App Shell + EditModel + Form + Service + Adapter) honours it.

| AC | Spec | Production status | Findings touching |
|----|------|-------------------|------------------|
| AC-1.1 | `e` on Users list → SCR-012 | PASS | — |
| AC-1.2 | `e` on User Detail → SCR-012 | PASS | — |
| AC-1.2 | `:edit` / `:e` palette → SCR-012 | **FAIL** | QA-W01-03, QA-W01-07 |
| AC-1.3 | Single GET on entry | PASS | — |
| AC-1.4 | Loading Esc abort | PARTIAL (Esc pops nav but ctx not cancelled — GET runs to completion in background) | QA-W01-11 |
| AC-1.5 | 4xx GET blocks form | PASS | — |
| AC-2   | 11 fields × 4 sections + login read-only | PASS | — |
| AC-3.1 | required-empty inline | PASS (validation works); inline error rendering missing dirty/required marker distinction | QA-W01-05 |
| AC-3.2 | loose email | PASS | — |
| AC-3.3 | phone hint | NOT IMPLEMENTED (PRD acknowledges as Phase 6 follow-up, but Phase 6 didn't add it) | (defer to backlog) |
| AC-3.4 | no client truncate | PASS (Form has no MaxLen enforcement) | — |
| AC-3.5 | no pre-save GET | PASS | — |
| AC-4.1 | Ctrl+S saves | PASS | — |
| AC-4.2 | partial-merge body | PASS | — |
| AC-4.3 | saving disables input + footer | PASS for form input; **FAIL** for Esc/Ctrl+C semantics | QA-W01-02 |
| AC-4.4 | 1s post-save guard | NOT IMPLEMENTED (impl §4 #6) | (Medium; backlog) |
| AC-4.5 | success → popNav + toast + cache | PARTIAL — popNav and toast work; cache patch is via RefreshScreenMsg fallback, not UserUpdatedMsg | QA-W01-09 + QA-W01-10 |
| AC-5.1 | clean Esc closes | PASS (because the App Shell pops nav anyway) | — |
| AC-5.2 | dirty Esc → discard confirm | **FAIL** | **QA-W01-01** (Critical) |
| AC-5.3 | saving Esc no-op | **FAIL** | QA-W01-02 |
| AC-6   | error mapping | PASS at adapter/service; PARTIAL at TUI (BadRequestError causes mapped to inline; 403 toast + form-preserve verified by code-read; 404 popNav not exercised by App Shell test) | (acceptable for v0.2.0; recommend regression test) |
| AC-7.1 | PII default mask | PASS | — |
| AC-7.2 | focus auto-unmask | PASS | — |
| AC-7.3 | blur unchanged re-mask | PASS | — |
| AC-7.4 | blur modified stay-unmasked | **FAIL** | QA-W01-12 |
| AC-7.5 | `Alt+m` toggle | PASS | — |
| AC-7.6 | debug log mask | NOT TESTED (no debug logger wired in EditModel — `deps.Logger` is optional and unused in EditModel) | (Low; backlog) |
| AC-8.1 | keyboard only | PASS | — |
| AC-8.2 | NO_COLOR markers (`[required]` / `!` / `*`) | **FAIL** (only `*` suffix; required/dirty markers conflated) | QA-W01-05 |
| AC-8.3 | 80×24 layout | NOT TESTED (no golden file at 80×24 yet; impl §4 #5 defers) | (Medium; backlog) |
| AC-8.4 | focus visual | PARTIAL (`▸` prefix exists; no bold border) | (Low; backlog) |
| AC-9.1 | per-keystroke dirty | PASS | — |
| AC-9.2 | dirty `*` marker per label | **FAIL** | QA-W01-05 |
| AC-9.3 | `N changes` footer | PASS | — |
| AC-9.4 | diff = dirty only | PASS | — |
| AC-10.1 | cache untainted on cancel | NOT TESTED at App Shell level (impl §4 #2 defers) | (Medium; backlog) |
| AC-10.2 | background polling continues | NOT VERIFIED | (Low; backlog) |
| AC-10.3 | scroll/selection restored | PASS (navStack frame preservation) | — |

**Score: 7 / 10 ACs fully met. 3 (AC-5.2 critical, AC-4.3, AC-7.4) broken; rest partial or backlog.**

---

## 7. Decision Matrix (D-W1 … D-W16) Coverage

| Decision | Status |
|----------|--------|
| D-W1 (11 fields) | PASS |
| D-W2 (login read-only) | PASS |
| D-W3 (no email confirm) | PASS |
| D-W4 (dirty Esc → L1 confirm) | **FAIL** (QA-W01-01) |
| D-W5 (Ctrl+S save) | PASS |
| D-W6 (failure preserves form) | PASS |
| D-W7 (single GET on entry) | PASS |
| D-W8 (no custom fields) | PASS |
| D-W9 (status read-only badge) | PARTIAL — FieldSpec doesn't include status row; only a comment placeholder. Acceptable for v0.2.0 if the design's Status section is left out of MVP. |
| D-W10 (dirty `*` + footer) | PARTIAL (QA-W01-05) |
| D-W11 (last-write-wins) | PASS |
| D-W12 (no preflight permission) | PASS |
| D-W13 (empty patch guard) | PASS |
| D-W14 (no email→login sync) | PASS |
| D-W15 (PUT not exposed) | PASS |
| D-W16 (modal full-screen + navStack push) | PASS for the push; **FAIL** for the modal Esc semantic (QA-W01-01). |

**Score: 12 / 16 fully met. D-W4 / D-W16 broken or partial.**

---

## 8. Regression (non-W01 surfaces)

Spot-checked the surfaces most at risk from the REQ-W01 wiring:

| Surface | Result | Notes |
|---------|--------|-------|
| Users list `:reset-password` | PASS | unaffected by edit-form wiring |
| Users list `:unlock` | PASS | — |
| Users detail `e` shortcut (pre-W01 meaning) | n/a — `e` was unused in detail pre-W01; now correctly opens form |
| Groups `e` | NOT CHECKED IN CODE — TUI_DESIGN promises toast "no edit action for <resource>", which is not implemented today on any non-Users screen (depends on QA-W01-04 fix to land the global fallback) | LOW |
| Logs `e` | **BROKEN** — collides with 24h shortcut (QA-W01-04) |
| Apps / Authenticators / Logs detail | not affected |
| navStack push/pop | PASS (covered by `navstack_test.go`) |

---

## 9. PM decisions requested

1. **QA-W01-01 fix path:** Option A (extend `escIsCritical`) vs Option B (promote to `OverlayDiscardConfirm`). Option B is closer to the SCR-012 design intent and reuses the already-declared overlay/identifiers; Option A is the smaller patch. Recommendation: **Option B** — the OverlayDiscardConfirm constant and `DiscardRequestedMsg` exist precisely because the design anticipated this. Option A leaves the overlay surface as dead code.
2. **QA-W01-03 `:edit` argument resolution:** infer user from active screen or always toast "no user selected" off-Users-context? Recommendation: **toast** (verbatim TUI_DESIGN §3.4 row). Active-user inference is implicit on Users list/detail because they're already user-scoped; the palette dispatch can read `cursorUser()` / `detailUser.ID` via a new `editTargetFromActiveScreen()` helper.
3. **QA-W01-04 Logs `e` reassignment:** move `setRange(24h)` to which key? Suggestions: `d` (already taken by detail), `1d`, `Shift+D`. Recommend: **drop history shortcuts for keys that conflict with global key reservations**; logs has a `:history` command + numeric shortcuts (`0`/`1`/`3`/`c`) — `e` was an outlier.
4. **QA-W01-08 empty list `e`:** toast or stuck loading form? Recommend toast (matches QA-W01-03 pattern); implementation_report §2.4 left this open.
5. **AC-7.6 debug logging:** PII fields must mask in `debug.log`. EditModel today doesn't write to its logger at all. Should Phase 7 wire log lines (state transitions / save attempts) with the masking handler? Recommend yes, behind `slog.Debug`.

---

## 10. Open follow-ups for go-test-engineer

| ID | Test name (proposed) | Layer | Purpose |
|----|--------------------|-------|---------|
| T-W01-A | `Test_AppShell_Esc_OnDirtyEditForm_OpensDiscardConfirm` | app | **Added** (this report). |
| T-W01-B | `Test_AppShell_Esc_DuringSaving_DoesNotPopNav` | app | **Added.** |
| T-W01-C | `Test_AppShell_PaletteEdit_ResolvesScreenUserEdit` | app | **Added.** |
| T-W01-D | `Test_AppShell_PaletteE_ResolvesScreenUserEdit` | app | **Added.** |
| T-W01-E | `Test_AppShell_EKey_OnLogs_ToastsNoEditAction` | app | Pin the global `e`-fallback toast (QA-W01-04 fix). |
| T-W01-F | `Test_UsersList_UserUpdatedMsg_PatchesCache` | users | Pin AC-4.5 cache-patch behaviour after QA-W01-09 fix. |
| T-W01-G | `Test_UserEdit_AfterSaveSuccess_DirtyIsZero` | users | Pin snapshot resnap on save (QA-W01-10). |
| T-W01-H | `Test_Form_RequiredMarker_IsBracketRequired` | form | AC-8.2. |
| T-W01-I | `Test_Form_DirtyMarker_LeadsLabel` | form | AC-9.2. |
| T-W01-J | `Test_Form_PII_BlurredAfterEdit_StaysUnmasked` | form | AC-7.4. |
| T-W01-K | `Test_AppShell_DiscardConfirmYes_PopsNav` | app | Wire `DiscardRequestedMsg` (QA-W01-06). |
| T-W01-L | `Test_UsersList_EditCancelled_CachePreserved` | users | AC-10.1 (impl §4 #2). |
| T-W01-M | `Test_UserEdit_NO_COLOR_Golden` | users | AC-8.2 / 8.3 golden snapshot (80×24). |

---

## 11. Verification summary

```bash
go build ./...                              # PASS
go vet ./...                                # PASS
go test ./... -count=1 -timeout 180s        # PASS *except the 4 new regression tests below*
go test ./internal/app/ -run \
  'Test_AppShell_Esc_OnDirtyEditForm_OpensDiscardConfirm|\
   Test_AppShell_Esc_DuringSaving_DoesNotPopNav|\
   Test_AppShell_PaletteEdit_ResolvesScreenUserEdit|\
   Test_AppShell_PaletteE_ResolvesScreenUserEdit' \
  -count=1 -v                               # 4 FAIL (intentional, Phase 6→7 deliverable)
```

The 4 new fail-first tests in `internal/app/user_edit_qa_regression_test.go` are the regression-fence for QA-W01-01 … QA-W01-04 — they must turn green in the Phase 7→8 fix pass.

---

## 12. Inventory of files reviewed (양쪽 동시 읽기)

| Spec side | Impl side |
|-----------|-----------|
| `docs/PRD.md:455-612` (§5.6 REQ-W01 AC + D-W) | `internal/tui/users/edit.go`, `internal/tui/shared/form/form.go`, `internal/tui/users/list.go:585-611,810-839`, `internal/app/app.go:702-732,2029-2068,2113-2122` |
| `docs/TUI_DESIGN.md:913-1224` (SCR-012) | same as above + golden walkthrough |
| `docs/TUI_DESIGN.md:319-345` (§3.4 palette) | `internal/app/app.go:792-841,2282-2343` |
| `docs/TUI_DESIGN.md:401-421` (§3.6 detail keys) | `internal/tui/users/list.go:595-611` |
| `docs/TUI_DESIGN.md:473-484` (§3.7 conflict) | `internal/tui/logs/logs.go:925`, `internal/app/app.go:2113-2178` |
| `docs/TUI_DESIGN.md:2476-2520` (§10.1 confirm) | n/a — design declares L1, impl uses in-state instead of overlay |
| `docs/TUI_DESIGN.md:2555-2559` (§11.2a) | summary cross-check |
| `docs/TUI_DESIGN.md:2594-2628` (§12 conflict) | `internal/tui/logs/logs.go`, `internal/app/app.go` |
| `docs/TUI_DESIGN.md:2629-2666` (§13/14) | cross-check |
| Phase 5 spec | `internal/tui/users/edit_test.go`, `internal/tui/shared/form/form_test.go`, `internal/okta/users_update_test.go`, `internal/service/users_update_test.go`, `internal/domain/user_patch_test.go`, `internal/app/screen_user_edit_test.go`, `internal/app/user_edit_entry_test.go` |

End of REQ-W01 Phase 7 QA findings.
