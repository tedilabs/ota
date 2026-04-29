# 01 Feature Inventory — ota v0.1.17

Contract: every item MUST survive the redesign. Replacements cover original intent 1:1.

Sources: PRD v1.0.1, TUI_DESIGN v1.2.0, QA_REPORT, `internal/{app,tui/*}` HEAD `9d81a01`.

## A. Chrome

- Single-row k9s TitleBar: `ota vX · <tenant> · <principal>  [profile]` | `[RL]  TZ  vX`
- Env badge by classifier (prod red / stage yellow / dev green)
- Principal slot from one-shot `/me` probe on first WindowSizeMsg, collapses if absent
- Resource label in upper divider: `── Users · 42 of 1,205 · q="al" ──`
- Right-divider "updated HH:MM:SS UTC" stamp from last fetch
- KeyHints bottom strip per-screen, with `[offline]` red token when offline
- RL badge ok/warn/danger by Remaining/Limit ratio (never raw numbers)
- Rounded outer frame, 100% width fill (min 80), body-line cap so top divider never scrolls off

## B. Lists (Users / Groups / Rules / Policies / Apps / Logs)

Common: `j k` + arrows · Ctrl-f/b page, Ctrl-d/u half · `gg` chord top, `G` bottom · `h l` + arrows hscroll (clamped) · `/` floating filter input, client-side incremental · Esc cancels filter input or clears committed filter · Enter opens detail, `d` alias · cursor `▸ ` + Accent tint + full-row pad · status-driven row bg tint when abnormal (LOCKED_OUT/SUSPENDED/PASSWORD_EXPIRED/DEPROVISIONED) · viewport windowing keeps header visible · right-edge scrollbar glyph · bold accent headers, 2-cell cursor gutter · auto-fit cols to observed width with priority drop · auto-refresh tick (`Deps.RefreshInterval`, gen-guarded) · `R` manual refresh · sort cycle Shift-S/N/L/C (off→asc→desc→off), ↑↓ glyph (asc green, desc red), single active sort.

Per-resource:

- **Users**: STATUS / LOGIN / DISPLAY NAME / TITLE / DIVISION / LAST LOGIN / STATUS_CHANGED. Badges ●○⚠✗+colour. Sort S(rank)/N(login)/L/C.
- **Groups**: TYPE / NAME / DESCRIPTION / MEMBERS / UPDATED. Sort N. RULE badge for dynamic. `m` → members.
- **Rules**: STATUS (INVALID>ACTIVE>INACTIVE rank) / NAME / TARGETS (id→name resolved+cached) / EXPRESSION trunc / UPDATED. Sort S, N.
- **Policies**: per-type wrapper; `:policies <TYPE>` skips picker; 7-type modal. Cols PRIORITY / STATUS / NAME / SYSTEM / UPDATED.
- **Apps**: type-select wrapper (SAML/OIDC/Bookmark/SWA/SCIM/Other), per-type palette (`:saml-app` etc.), client-side type filter.
- **Logs**: TIMESTAMP / SEVERITY colour / EVENT TYPE / ACTOR / TARGET / OUTCOME / IP. `s` tail, `f` follow auto-scroll, `r` abs/relative time, `0/1/3/c` range presets, `e` expand. Newest-at-bottom, inline range/follow status, UUID dedup in follow.

## C. Detail

- Universal Pretty/JSON/YAML; Tab/Shift-Tab cycles; `r` toggles JSON↔previous (DetailTabRaw=JSON alias)
- Pretty: curated 2-col, semantic groupings (Identity/Contact/Address/Org/Custom alpha-sorted)
- JSON: MarshalIndent + masked-line annotation + HighlightJSON. YAML: 2-space indent + syntax highlight
- Line cursor `j k`, full-row highlight on every tab; `gg G` in body
- Vim Visual `v V` + range yank `y`, `-- VISUAL --` indicator; single-line `y` also yanks; "yanked N lines" toast
- Pretty column-flow cursor (#181): walks LEFT then RIGHT col rows, wraps; only active half tints
- User Detail extras: side-by-side scrollable Groups + Apps boxes under 2-col Pretty
- `]` enter extras / jump right, `[` jump left, Esc returns to grid
- Extras drill-down: Enter on Groups → `OpenGroupDetailMsg`; Apps → `OpenAppDetailMsg`
- Per-user keying prevents stale-fetch clobber; independent scrollbars per box
- `:unmask <field>` / `:mask` toggle PII, session-scoped, persisted on ListModel
- `:reset-password` / `:unlock` / `:reset-mfa` via action menu + typed-confirm
- Action menu (`a`): resource-specific picker from screen's `Actioner`
- Logs detail: same triplet; Pretty has Actor/Target/Client/Outcome/Debug sections
- Esc bubbles back to list (App Shell forwards unhandled keys to child)

## D. Overlays

- Palette (`:`): floating input doesn't shift chrome; Tab/Shift-Tab cycles canonical-only autocomplete; right-border aligned to chrome
- Help (`?`): context-aware 3-column wide, `/` filter within, Resource/General/Navigation groupings
- Action menu (`a`): rounded modal, j/k cursor, Enter run, Esc cancel, hint trail
- Confirm: typed y/N with "cannot be undone"
- Quit confirm (Ctrl-c soft + tail running)
- About (`:about`): version / commit / build / profile / token source / RL summary
- Standalone Search (`/`-style)
- Boot error screen (token missing / bad org_url)

## E. Global keys

`:` `/` `?` palette/filter/help · Esc cancel/clear · `q` close · Ctrl-c soft quit · Ctrl-l redraw · Tab/Shift-Tab next/prev tab or palette suggestion · Enter select · `j k h l` + arrows · `gg G` · Ctrl-f/b/d/u page · Shift-S/N/L/C sort · `R` refresh · `r` toggle JSON · `y` yank · `v V` Visual · `d` detail · `e` expand · `s` tail · `f` follow · `0 1 3 c` logs range · `]` `[` extras · `m` members · `a` action menu

## F. Palette commands

`:users :groups :grouprules :policies [TYPE] :logs :apps [TYPE] :saml-app :oidc-app :bookmark-app :swa-app :scim-app :other-app :profile [name] :search <expr> :filter <expr> :unmask <field> :mask :raw :refresh :about :ratelimit :errors :healthcheck :debug open :help :quit :reset-password :unlock :reset-mfa`. Aliases: singular/plural/hyphen/underscore per resource.

## G. Cross-screen routing & actions

- `OpenResourceMsg` (Kind = user/group/rule/policy/log) → detail
- `OpenGroupDetailMsg` / `OpenAppDetailMsg` cross-screen IDs (User-detail extras drill-down)
- `OpenPolicyTypeMsg` / `OpenAppTypeMsg` rebuild per-type wrapper
- `shared.Actioner` publishes actions; menu enumerates; destructive actions fire confirm before `RunUserActionMsg`
