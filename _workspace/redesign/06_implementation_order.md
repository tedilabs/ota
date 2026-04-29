# 06 Implementation Order — single-PR sequence

The redesign lands in **one commit / one PR** so we get one-commit rollback. Inside the PR work is sequenced so the build is green at every step. Top of list = lowest-risk shared infrastructure. Bottom = per-screen application.

---

## Step 0 — Snapshot

Before touching code, run the existing golden / teatest suite and snapshot outputs we aim to keep stable (overlay content, About body, palette command list, key labels, monochrome rendering). Diff after every step against this snapshot. No code changes.

---

## Step 1 — Tokens: `RowHighlight` → `RowCursor` rename

`internal/tui/shared/styles.go`. Rename field on `Tokens`; update Dark / HighContrast / Monochrome constructors and all call sites. Compiles + tests pass with no behavioural change.

Risk: Low. ~6 files, ~12 lines.

## Step 2 — Cursor model: `shared/cursor.go` (NEW)

~120 lines. Define `Region`, `Cursor`, `RegionStep`, transition table. Add `RenderRowWithCursor(line, isCursor, statusBg, tk) string` — single function every screen calls. ANSI-strip-aware, scrollbar-gutter-aware. Move `▸ ` / `  ` gutter and `padOrTruncateVisible` from each screen here.

Tests: `cursor_test.go` exhaustive transitions; `RenderRowWithCursor` golden at 80 / 100 / 140 with normal / abnormal / cursor combos. Reference: `internal/app/visual_*` and `internal/tui/users/visual_test.go.new` for the strip-CSI ordering today's tests pass.

Risk: Medium. Strip-CSI ordering (data → status-bg → cursor-bg) must match today's visual tests.

## Step 3 — Chrome status row: 5-zone layout

`internal/tui/shared/chrome.go`. Extend `ChromeInput` with `StatusBadges []StatusBadge` (`Key, Value, Tone`). Add status-row emission (5-zone instead of 4-zone). Body row count drops by 1; update `app.go` `BodyLines` callers.

Tests: update `internal/app/golden_test.go` for chrome shape; new `TestRenderChrome_StatusRow_BadgeOrdering` feeds saturated badge list and asserts ordering at 80 / 100 / 140 with `…` truncation at 80.

Risk: Medium. Body row reduction is the most visible behavioural change.

## Step 4 — Empty / loading / error: `shared/empty.go` (NEW)

~80 lines. `RenderEmpty(kind EmptyKind, body string, hints []string) string` returns a centred body using `lipgloss.Place`. Kinds: `Loading`, `NoRows`, `NoMatches`, `Error`. Tests: golden per kind at 80x24.

Risk: Low.

## Step 5 — Overlay mount protocol: `shared.MountModal`

Extend `internal/tui/shared/modal.go`. Add `ModalIn { Title, Body, Footer string; Tone ToneKind; Width int }` and `MountModal(in ModalIn) string`. Update `internal/tui/overlay/overlay.go` so palette / help / action menu / confirm / about call `MountModal` instead of `shared.Modal`. Mechanical replacement.

Tests: existing palette / help / confirm goldens stay green (content identical); new `Test_MountModal_AnchorsCenter` locks centring at 80 / 100 / 140.

Risk: Medium. Easy to get vertical anchoring wrong on narrow terminals.

## Step 6 — Status badge contributors: `StatusBadgeStater`

`internal/app/app.go`. Add interface `StatusBadgeStater interface { StatusBadges() []shared.StatusBadge }` next to `FilterStater`, `LastUpdatedStater`. `app.View()` queries the active screen and passes badges to the chrome. App Shell still owns action-pending and offline toasts (top of list).

No screen implements `StatusBadgeStater` yet — chrome renders the empty status row.

Risk: Low.

## Step 7 — List screens migrate (per-screen)

Apply most-evolved first so issues surface against the richest content:

1. **Users** (`internal/tui/users/list.go`) — replace inline `▸ ` / Accent.Render in View with `shared.RenderRowWithCursor`. Implement `StatusBadges()` returning `[SORT]`, `[FILTER]`, `[hscroll]`. Keep in-header sort glyph. Drop now-unused `padOrTruncateVisible` mid-row pad logic.
2. **Logs** (`internal/tui/logs/logs.go`) — biggest visible change. Remove the two inline status lines (`renderTailState`, `renderFollowState`, `timeRangeLabel` line, hint line); keep helpers (still produce badge values). Implement `StatusBadges()` returning `[RANGE]`, `[TAIL]`, `[FOLLOW]`, `[FILTER]`, `[hscroll]`. Reclaim 2 body rows. Verify `eeb2197` cursor-on-newest, `af76ebd` UUID dedup, range presets unchanged.
3. **Groups** (`internal/tui/groups/groups.go`)
4. **Rules** (`internal/tui/rules/rules.go`)
5. **Apps** (`internal/tui/apps/apps.go`) — `ListModel` and `TypeSelectModel`.
6. **Policies** (`internal/tui/policies/policies.go`) — `ListModel` and picker.

Each migration: ~40 lines removed (cursor markup, ad-hoc status), ~20 added (`StatusBadges()`). Tests: existing teatest flows pass; one NEW visual test per screen asserting status row content.

Risk: Medium per screen, accumulating. Run snapshot diff after each step — a regression here is easier to fix in isolation.

## Step 8 — Detail screens migrate

In the same order:

1. **Users detail** (`internal/tui/users/detail.go` + `list.go::renderDetailWithColumnFlowCursor`) — pull column-flow cursor (`detailExtrasFocused`, `detailExtrasCur`, `prettyColumns`) into shared Cursor model. Replace side-by-side rounded boxes for Groups + Apps with inline `── Groups (lazy) N of M ──` underline. Render Visual ranges via `RenderRowWithCursor` with VisualAnchor.
2. **Groups / Rules / Apps / Policies detail** — adopt Pretty/JSON/YAML triplet via the now-shared mixin. Some already have it (Apps, Policies); add it where missing. Contract: every detail's View() consumes `shared.DetailFrame(tabBar, body)` so the tab bar + thin divider is identical.
3. **Logs detail** — same triplet. Pretty groups (Actor / Target / Client / Outcome / Debug) preserved.

Tests: visual goldens per detail tab × screen; Visual mode + yank teatest per screen.

Risk: Medium. The 2-col Pretty cursor is the most subtle piece — keep `internal/tui/users/visual_test.go.new` green throughout.

## Step 9 — Help / palette / action menu / about polish

`internal/tui/overlay/overlay.go`. Help title includes active screen `Help · <Screen>`. Palette: drop alias commands into Muted second tier in suggestion list. Action menu: post `[ACTION: <id>]` to status row via a new message that App Shell forwards. About: render through `MountModal` with Header title.

Risk: Low. Cosmetic plumbing.

## Step 10 — Esc-no-op toast + nothing-to-close

`internal/app/app.go` + list.go siblings. When a screen receives Esc with no overlay / filter / Visual / detail open: post one-shot toast `nothing to close` to status row (1.5s linger via existing tick infra, or until next key). Reuse for all list screens via `EscapeOpStater` interface (NEW, returns whether Esc would do something).

Risk: Low.

## Step 11 — Final acceptance

1. Full test suite — all goldens pass.
2. Teatest flows: tail/follow, cursor flows in User Detail, drill-down to group/app, action menu + confirm.
3. 80x24 manual smoke at every screen.
4. Verify every key in `04_redesign_spec.md` §8 still works.
5. Verify every palette command in inventory §F still routes.
6. Visual diff against Step 0 snapshot — call out unintended changes.

---

## Why this order

- **Tokens (1) → cursor (2) → chrome (3) → modal (5)** is the dependency stack. Each step depends only on what's above; pause / squash any time and the build is green.
- **Empty states (4)** is independent of cursor / status, so it can land in parallel with steps 2-3.
- **Per-screen migration (7-8)** is the longest stretch but each screen is isolated; if Logs is hairy, ship Users first and verify the model in production-feel before touching Logs.
- **Polish (9-10)** at the end so we don't fight the redesign mid-flight.

Total expected diff: ~600 lines deleted, ~400 lines added, net **−200 LoC** (unification reclaims duplicate cursor / status / modal code).

End implementation order.
