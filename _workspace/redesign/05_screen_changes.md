# 05 Screen Changes — before / after, per surface

For each screen: visible delta, behaviour delta, what stays the same. Read alongside `04_redesign_spec.md` §2-7.

Convention: every inventory §A-G item survives; entries here only call out the diff. Marker prefixes: V = visible, B = behaviour, S = same.

---

## Users · List

- V: Status row appears under body — `[SORT: status↑] [FILTER: al] [hscroll 0]` when active, blank otherwise.
- V: Cursor row colour uniform across normal / abnormal / VISUAL — `RowCursor` applied last over `RowDanger`/`RowWarning`/`RowMuted`. No more "is the highlight winning?" ambiguity.
- V: Empty / loading / no-match states use centred `shared/empty.go` body.
- B: Cursor row goes through `shared.RenderRowWithCursor`. Removes the `padOrTruncateVisible + tk.Accent.Render(composed)` chain.
- B: Esc on list with no filter shows one-shot Muted toast `nothing to close` in status row (silent today — pain T7).
- B: `[hscroll N]` in status row when h/l offset > 0 (silent today).
- S: Column lineup `LOGIN / NICKNAME / DIVISION / DEPARTMENT / TITLE / STATUS / LAST LOGIN / LAST UPDATED`, drop priorities, sort glyphs, row-status backgrounds, scrollbar gutter, refresh tick.

## Users · Detail

- V: Tab bar tightens — thin horizontal divider directly under tabs, always rendered (incl. JSON/YAML).
- V: Pretty extras (Groups + Apps) get a Muted underline header `── Groups (lazy)  6 of 6 ──` instead of side-by-side rounded boxes. Same rows, same scrollbars, no extra border budget.
- V: `[FOCUS: extras]` in status row when extras focused (replaces green box-border tint).
- V: `[VISUAL N]` in status row replaces inline `-- VISUAL --`.
- B: `]`/`[` route through `shared.Cursor.StepRegion` — single dispatcher.
- B: Visual region-scoped (no accidental cross-column yanks). Single-line `y` outside Visual still yanks cursor row (KEEP).
- B: `r` JSON-toggle restores cursor to last region on return (improves on today landing at line 0).
- B: Action menu (`a`) posts `[ACTION: <id>]` to status row before confirm — visible even if modal dismisses.
- S: Pretty section ordering, single-col fallback at 80, masked-line annotation, `:unmask`/`:mask`, lazy fetch + per-user keying, `OpenGroupDetailMsg`/`OpenAppDetailMsg`, JSON syntax highlight, YAML 2-space indent.

## Groups · List

- V/B: Same chrome facelift, status row, cursor unification.
- B: `m` (members) sub-screen renders inside the unified chrome with shared cursor (was a one-off list).
- B: Sort N glyph integrates with `[SORT: name↑]` in status row (in-header glyph KEEP).
- S: Columns `TYPE / NAME / DESCRIPTION / MEMBERS / UPDATED`, RULE badge for dynamic, `m` shortcut, `OpenGroupDetailMsg`.

## Groups · Detail

- V: Tabs `[Pretty] [ JSON ] [ YAML ]` consistent with Users (today trails).
- B: Detail body via unified Cursor — line cursor, Visual + yank identical to Users (lifts to "shared mixin"; brief Open Q4: contract enforced via `RenderRowWithCursor` test).
- S: Group attributes content, JSON / YAML output.

## Rules · List

- V: Status row `[SORT] [FILTER] [hscroll]`, status badges (INVALID > ACTIVE > INACTIVE) keep colours; `RowDanger` for INVALID applies the same way.
- B: Cursor + scrollbar via shared renderer.
- S: Columns `STATUS / NAME / TARGETS / EXPRESSION / UPDATED`, sort S+N, target id→name resolve+cache, expression truncation, `R` refresh.

## Rules · Detail

- V/B: Same Pretty/JSON/YAML triplet, status row, line cursor + Visual + yank consistent.
- S: Expression rendering, target list, content.

## Apps · Type Picker

- V: Picker rendered inside chrome (today: minimal one-off). Resource label `Apps · pick a type`. Cursor row shows type with sample count if available.
- V: Status row blank — picker has no transient state.
- B: `:saml-app`, `:oidc-app`, … bypass the picker (KEEP). Picker is one route in.
- S: 6 type entries, palette aliases, `OpenAppTypeMsg`.

## Apps · List (per type)

- V: Resource label `Apps › SAML  ·  3 of 17  ·  q="okta"`. Status row carries `[SORT]`/`[FILTER]`/`[hscroll]`.
- B: Client-side type filter (KEEP `b45bdeb`). Cursor + scrollbar uniform.
- S: Per-type column set, app status badges, palette aliases.

## Apps · Detail

- V: Pretty / JSON / YAML triplet — same density and tab bar as Users detail. SAML / OIDC sensitive fields stay masked unless `:unmask`-ed (KEEP).
- B: Cross-screen entry (`OpenAppDetailMsg` from User Detail extras) lands cursor at first field.
- S: Field set per app type, credential-setup booleans (`b45bdeb`: bool not string), JSON / YAML output.

## Policies · Type Picker

- V: Inside chrome like Apps picker. Each row shows label and a small "rich-rendered" tag where applicable. Resource label `Policies · pick a type`.
- B: `:policies <TYPE>` skips picker (KEEP). Emits `OpenPolicyTypeMsg`.
- S: 7 policy types, modal vs inline resolution.

## Policies · List (per type)

- V: Columns `PRIORITY / STATUS / NAME / SYSTEM / UPDATED` (KEEP). Status row pattern matches Users / Apps.
- B: Cursor + scrollbar via shared renderer.
- S: Type-scoped routing, sort, palette `:policies`.

## Policies · Detail

- V/B: Same triplet of tabs, per-type rich rendering preserved (rules / conditions / actions sections in Pretty), unified line cursor.
- S: Per-type Pretty content, JSON / YAML output.

## Logs · List (Search) — biggest delta

- V: The two inline status lines today (one with `range … · TAIL … · FOLLOW …`, one with the key hint) are replaced by:
  - `[RANGE: 1h]  [TAIL 7s]  [FOLLOW]` in the status row (Accent / Success when on, Muted when off).
  - The hint line moves into key hints at width ≥ 100; at width 80 collapses to `<?>` help only.
- V: Body reclaims 2 rows — Logs list shows up to 16 events at 80x24 (was 14).
- V: Tail freshness ticks visibly via `[TAIL Ns]` updating per `pollInterval`.
- B: Newest-at-bottom + cursor-on-newest preserved (KEEP `eeb2197`). UUID dedup in follow preserved (KEEP `af76ebd`). `f` toggles follow without changing cursor's distance from bottom.
- S: Columns `PUBLISHED / SEV / MESSAGE / ACTOR TYPE / ACTOR / OUTCOME / IP / WHEN`, severity colours, range presets `0/1/3/c`, default 30m, abs/rel `r` toggle, `/` filter, `e` expand.

## Logs · Detail

- V: `[Pretty] [ JSON ] [ YAML ]` consistent. Pretty groups: Actor / Target / Client / Outcome / Debug (KEEP `c03a2e6`).
- B: Detail line cursor uniform.
- S: Same data sections, JSON / YAML output, `e`/`Esc` round-trip.

## Boot error / About / Quit confirm

- V/B: About via `shared.MountModal` (same content). Quit confirm same scaffold: Danger title `Quit ota?`, body `tail running — are you sure?` (KEEP soft-quit).
- B: Boot error stays as today (no chrome) — only path that runs before chrome init. Mark exempt in tests.

---

## Cross-cutting cleanup

- Every list / detail screen drops bespoke cursor renderer for `shared.RenderRowWithCursor` — 6 list views + 6 detail views, ~20 lines each lost.
- Every screen with a status surface returns `StatusRowStater` to the chrome instead of printing inline `[TAIL]`/`[FOLLOW]`/`[VISUAL]`/`[SORT]`. Logs is the most invasive — ~50 lines removed, ~30 added.
- Empty / loading / error rendering moves to `shared.RenderEmpty(kind, hints)`.
- Overlays drop hand-rolled `shared.Modal(title, body, width)` for `shared.MountModal(in)`. Existing overlays render the same content.

End screen change list.
