# 04 Redesign Spec — ota v0.2.0 surface

Implementation target. Single source of truth for chrome, cursor, list, detail, overlays, tokens, keys. References: `01_feature_inventory.md` (must-preserve), `02_pain_points.md` (T1-T7), `03_redesign_brief.md`. Every key from inventory §E and command from §F survives — delta table at the end marks every change.

---

## 1. Design principles

1. **One cursor, one focus, one badge row.** Lists, detail body, 2-col Pretty, extras, JSON / YAML — driven by `shared.Cursor` (region + index + visual range). `▸ ` gutter and `RowCursor` token are the only highlight in any region. Mode badges live in exactly one row above key hints.
2. **Chrome owns layout; screens own content.** Chrome reserves a fixed budget per zone. Screens render into a body width handed to them; they never compute chrome geometry. Overlays request a centred rectangle.
3. **State is visible or it is not state.** Sort, filter, follow, tail, focus, Visual, action-pending, hScroll, offline — each maps to one badge or divider segment. If a mode exists it has a glyph; if no mode is active the cell is empty (not hidden).
4. **Density without ambiguity.** Drop padding, never information. Cursor gutter 2 cells, scrollbar gutter 1 cell, every other cell carries data. Empty values render as `—`.
5. **Keymap is muscle memory.** `: / ? q Esc Tab j k h l gg G Ctrl-f/b/d/u Shift-S/N/L/C R r y v V d e s f 0 1 3 c ] [ m a` non-negotiable. New behaviour reuses an old key or earns a new one with one-line justification.

---

## 2. Chrome layout

### 2.1 Five zones, fixed heights

```
Row  0       ╭─────...─────╮              top frame                       1
Row  1       │ <title>     │              title bar                       1
Row  2       ├─ <label> ───┤              upper divider + count + filter  1
Row  3..N+2  │ <body>      │              body                            N
Row  N+3     │ <status>    │              transient status row (NEW)      1
Row  N+4     ├─────────────┤              divider                         1
Row  N+5     │ <key hints> │              key hints                       1
Row  N+6     ╰─────...─────╯              bottom frame                    1
```

Total chrome cost: 7 rows. Body budget on a 24-row terminal: `24 - 7 = 17`. Today's chrome costs 6 (no status); we add 1 row, accepting body − 1 in exchange for permanent state legibility (brief Open Q2). The status row is always rendered — when no badges are active it renders blank so the chrome height never twitches.

### 2.2 Title bar

```
ota v0.2.0  ·  acme.okta.com  ·  alice@acme.com  [prod]      [RL: ok]   UTC
```

Cells (left → right): `ota v0.2.0` (Header), `· <tenant>` (Muted), `· <principal>` (Accent, collapses if /me pending), `[prod|stage|dev]` env badge, `[RL: ok|warn|limited|?]` (Success/Warning/Danger/Muted, right-anchored), `UTC` (Muted).

| width | left                                              | right          |
|-------|---------------------------------------------------|----------------|
| 80    | `ota v0.2.0 · acme.okta.com [prod]`               | `[RL: ok] UTC` |
| 100   | `… · alice@acme.com [prod]`                       | `[RL: ok]  UTC` |
| 140   | full + extra mid-gap                               | full           |

Drop order under width pressure: principal → version suffix → tenant trim with `…`.

### 2.3 Upper divider

```
├─ Users · 42 of 1,205 · q="al"  ─────────  updated 12:34:56 UTC ─┤
```

Left: `<Resource> · <visible> of <total> · q="<filter>"` (KEEP). Right: `updated HH:MM:SS UTC` (KEEP, `LastUpdatedStater`). Sub-paths like `Policies › OKTA_SIGN_ON` collapse right-stamp first under width pressure (KEEP `dividerWithLabels` overflow rule).

### 2.4 Status row (NEW — single mode-badge cell)

```
 [SORT: status↑] [FILTER: al] [FOCUS: extras] [VISUAL 3 lines] [hscroll 4]   yanked 5 lines
```

- `[KEY: value]`, Accent for value, Muted for key, single space gap.
- Right-anchored toasts (`yanked 5 lines`, `copied`, `refreshed`). Toasts auto-clear on next key.
- Empty row stays rendered — never collapses.

Per-screen badge inventory (replaces today's ad-hoc surfaces):

| Badge        | Trigger                                | Tone     | Replaces                   |
|--------------|----------------------------------------|----------|----------------------------|
| `[SORT: K↑]` | Sort active                            | Accent   | header glyph (KEEP both)   |
| `[FILTER: q]`| `/` committed                          | Accent   | divider `q="..."` (KEEP)   |
| `[VISUAL N]` | `v`/`V` active                         | Warning  | `-- VISUAL --` inline      |
| `[FOCUS: extras]` | Extras boxes focused              | Accent   | green box border tint      |
| `[TAIL Ns]`  | Logs tail mode on                      | Success  | inline tail line           |
| `[FOLLOW]`   | Logs follow mode on                    | Success  | inline follow line         |
| `[RANGE: 1h]`| Logs time range                        | Accent   | inline range line          |
| `[hscroll N]`| h/l offset > 0                         | Muted    | (was silent — pain T2)     |
| `[ACTION: reset-password]` | After `a` pick, before y/N | Danger | (was overlay-only)         |

Cell budget at width 80: ≤ 78 visible cells. Drop priority (right→left when tight): hscroll → focus → action → range → filter → sort → visual → tail → follow. Anything that doesn't fit becomes `…` at right edge. Tests assert badge order is stable.

### 2.5 Key hints

```
 <:> cmd  </> filter  <?> help  <a> action  <R> refresh  <q> close      [offline]
```

Width 80 carries 6 hints + offline. Width 100 adds `<gg>/<G>`; width 140 adds the screen-specific hints (sort, tail, etc) that today scatter through the body. Hints are stable per screen — they don't flash. `[offline]` stays right-anchored (KEEP).

---

## 3. Unified cursor model

### 3.1 Contract

```go
// shared/cursor.go (NEW)
type Region int
const (
    RegionListRows Region = iota   // list table
    RegionDetailLines              // JSON / YAML / single-col Pretty
    RegionDetailColLeft            // 2-col Pretty left half
    RegionDetailColRight           // 2-col Pretty right half
    RegionExtrasGroups             // User Detail Groups box
    RegionExtrasApps               // User Detail Apps box
)

type Cursor struct {
    Region        Region
    Index         int
    VisualAnchor  int // -1 when not Visual
    Hidden        bool
}
```

The model owns one `Cursor`. `Update` delegates by region; transitions explicit.

### 3.2 Region transitions

```
list rows ─Enter─▶ detail-lines (JSON / YAML / 1-col Pretty)
list rows ─Enter─▶ detail-col-left (User Detail 2-col, default)
detail-col-left ─j past end─▶ detail-col-right (top of right)
detail-col-right ─k past start─▶ detail-col-left (bottom of left)
detail-col-left  ─]─▶ extras-groups
detail-col-right ─]─▶ extras-apps
extras-groups ─]─▶ extras-apps
extras-groups ─[─▶ detail-col-left
extras-apps   ─[─▶ extras-groups
extras-*  ─Esc─▶ detail-col-left (or detail-lines)
detail-*  ─Esc─▶ list rows
```

Pretty column flow (issue #181) preserved 1:1. Inventory §C row 5+ all-checked.

### 3.3 Highlight rendering

Single function `RenderRowWithCursor(line, isCursor, statusBg, tk)`:
1. Compute the row body (no cursor styling yet).
2. Pad to `chromeContentWidth - 2` visible cells (room for scrollbar).
3. Prepend `▸ ` (cursor) or `  ` (gutter) — 2 cells exactly.
4. Apply `statusBg` if abnormal-status row.
5. If `isCursor`, wrap whole row in `tk.RowCursor` last (cursor wins).

Strip-CSI order fixed: data → status-bg → cursor-bg. Codified once so screens can't get it wrong.

### 3.4 Visual mode

`v` enters Visual at `Index`, sets `VisualAnchor = Index`. `j`/`k` extend `Index`. `y` yanks `[min, max]` joined by `\n`. `Esc` exits (anchor → -1). Single-line `y` outside Visual yanks the cursor row only (KEEP). Status row shows `[VISUAL N]` where N = `|index - anchor| + 1`. Region-scoped — entering Visual on detail-col-left and pressing `j` past the end stays inside left col (prevents accidental cross-column yanks).

---

## 4. List screen layout

### 4.1 Wireframe — width 80, healthy

```
╭──────────────────────────────────────────────────────────────────────────────╮
│ ota v0.2.0 · acme.okta.com [prod]                            [RL: ok]    UTC │
├─ Users · 42 of 1,205 · q="al"  ─────────────────  updated 12:34:56 UTC ────┤
│  LOGIN          NICKNAME    DEPARTMENT   STATUS↑       LAST LOGIN   UPDATED │
│▸ alice@acme.io  alice       Eng          ● ACTIVE          5m ago      2h   │
│  alex@acme.io   alex        Eng          ● ACTIVE          1h ago      3h ▌ │
│  alan@acme.io   —           Sales        ⚠ LOCKED_OUT      3d ago      1d ▌ │
│  alma@acme.io   alma        Ops          ● ACTIVE         12d ago      5d   │
│  ...                                                                        │
│  alvaro@acme.io alvaro      Eng          ⊘ DEPROVISIONED   90d ago     30d  │
│                                                                             │
│ [SORT: status↑]  [FILTER: al]  [hscroll 0]                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│ <:> cmd  </> filter  <?> help  <a> action  <R> refresh  <q> close           │
╰─────────────────────────────────────────────────────────────────────────────╯
```

Body row count at 80x24: 17 budget − 1 column header = 16 data rows.

### 4.2 Row anatomy (80-cell terminal, 78-cell inner content)

| Segment           | Cells | Notes                                      |
|-------------------|-------|--------------------------------------------|
| Cursor gutter     | 2     | `▸ ` or `  `                               |
| Column data       | 73    | sum of `LayoutColumnsTight` widths         |
| Pad + scrollbar   | 2     | one space + scrollbar glyph (`▌`/` `)      |
| Right border      | 1     | chrome `│`                                 |
| **Total**         | 78    | + 2 chrome borders = 80                    |

Per-resource column widths via `usersColumnSpecs()` etc — unchanged. All screens go through `shared.RenderRowWithCursor`, so highlight covers the full 73+2+2 = 77 cells. Pain T1 disappears because no screen renders cursor markup any more.

### 4.3 Header / scrollbar

Bold (`tk.Header`)-wrapped formatted row, 2-cell gutter aligns with data (KEEP). Sort glyph: `↑` Success asc, `↓` Danger desc (KEEP). Brief Open Q3: glyph stays — bracketed-letter alternative slipped under bold style. Unicode fallback `^`/`v` already in `shared.SortGlyph`. Scrollbar: 1-cell `▌` thumb, ` ` track, Accent (KEEP).

### 4.4 Empty / loading / error (NEW: unified)

```
╭────────────────────────────────────────────────╮
├─ Users · loading…  ────────────────────────────┤
│                                                │
│                fetching users…                 │
│                                                │
├────────────────────────────────────────────────┤
│ <:> cmd  </> filter  <?> help                  │
╰────────────────────────────────────────────────╯
```

One renderer in `shared/empty.go`. Kinds:
- **Loading** — divider `loading…`, body centred Muted message.
- **NoRows** — `0 of 0`, body `no users found · <R> refresh`.
- **NoMatches** — `no matches for q="<filter>" · Esc clears filter`.
- **Error** — body `shared.ErrorPanel(...)` (KEEP); status row `[ERROR]` Danger.

All four use the same chrome scaffolding — no per-screen rewrite.

### 4.5 Width responsiveness

- width 80: 16 data rows, 73 cells of column data, drop kicks in (UPDATED → LAST LOGIN → TITLE order).
- width 100: 16 rows, 93 cells, all 8 columns at observed widths.
- width 140: 16 rows, 133 cells, columns puff to spec.Min, gap right of last col.

Drop priorities unchanged from `usersColumnSpecs()`. Width math centralised in `shared.LayoutColumnsTight`/`HScroll`.

---

## 5. Detail screen layout

### 5.1 Wireframe — User Detail Pretty, width 100

```
╭────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ota v0.2.0 · acme.okta.com · alice@acme.com [prod]                              [RL: ok]      UTC │
├─ User › alice@acme.com  ──────────────────────────────────────────  updated 12:34:56 UTC ────────┤
│  [Pretty] [ JSON ] [ YAML ]                                                                       │
│  ──────────────────────────────────────────────────────────────────────────────────────────────── │
│  Status                            │  Identity                                                    │
│▸ status        ● ACTIVE            │   login        alice@acme.com                                │
│  created       2024-01-12 09:01    │   email        alice@acme.com                                │
│  activated     2024-01-12 09:05    │   firstName    Alice                                         │
│  statusChanged 2024-03-04 14:22    │   lastName     Adams                                         │
│  lastLogin     2026-04-28 09:14    │   displayName  Alice Adams                                   │
│                                                                                                   │
│  ── Groups (lazy)  6 of 6 ────── │ ── Apps (lazy)  4 of 4 ──────────────────────────────────────│
│   Engineers                       │   GitHub Cloud                                                │
│   SSO-Eng                         │   Okta Admin                                                  │
│   ...                             │   AWS Prod                                                    │
│                                                                                                   │
│ [FOCUS: grid-left]                                                                                │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ <Tab> tabs  <r> JSON  <]> extras  <a> action  <y> yank  <v> visual  <q> back                     │
╰────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

Body rows at 100x24: 17 budget − 1 tab bar − 1 tab divider = 15 rows.

### 5.2 Tab bar / Pretty / JSON / YAML / Action

- Tab bar: active `[Pretty]`, inactive `[ JSON ]` / `[ YAML ]`. Tab order stable; Tab cycles forward, Shift-Tab back, `r` toggles JSON ↔ previous (KEEP).
- Pretty 2-col: Left = Status + Identity (priority 1); Right = Contact + Address + Organization + Custom (priority 2). 16-cell key width (KEEP `composeProfileTab`). Extras render inline as `── Groups (lazy)  N of M ──` Muted underline (replaces today's rounded boxes — saves border budget; same data path, same per-box scrollbar).
- JSON / YAML: single-col scrolling body. Cursor wraps already-styled lines via `RenderRowWithCursor` (existing strip-CSI path stays).
- Action menu (`a`) opens centred over body. `[ACTION: <id>]` posts to status row while y/N is open — visible even if modal dismisses.

### 5.3 80x24 fallback

2-col Pretty collapses to one column (`pickPrettySectionWidth` already does this). Extras boxes stack one-line-each. Cursor regions remain — they just walk linearly.

---

## 6. Overlays

Common visual language: rounded border (`tk.Header.RoundedBorder`), 1-cell padding, Accent bold title row, Muted footer hints `<key> action  ·  <key> action`. Centred horizontally; vertically anchored one-third from top. Width = `min(80, contentWidth - 6)`.

Mounting protocol: overlay calls `shared.MountModal(in shared.ModalIn) string`. The chrome composites via `lipgloss.Place` over the body; chrome reserves zero rows for overlays.

### 6.1 Palette (`:`)

```
                ╭─ Command Palette ──────────────────────────╮
                │ : grou_                                    │
                │                                            │
                │  :groups                                   │
                │  :grouprules                               │
                │  :group-rules     (alias)                  │
                │                                            │
                │ <Tab> complete  <Enter> run  <Esc> cancel  │
                ╰────────────────────────────────────────────╯
```

Floating box; height = 4 + matches (cap 8). Width 60 default; 70 at width ≥ 100. Suggestions: canonical first (Header), aliases dimmed (Muted) suffix `(alias)`. Tab completes longest unique prefix then cycles. Right border aligned to chrome inner edge − 2 (KEEP `d21d54a`).

### 6.2 Help (`?`)

```
        ╭─ Help · Users  (Press Esc to close)  ─────────────────────────────╮
        │ Resource          │ General             │ Navigation              │
        │ <a> action menu   │ <:> palette         │ <j>/<k> down/up         │
        │ <m> members       │ </> filter          │ <h>/<l> hscroll         │
        │ <Shift-S> sort St │ <?> help            │ <gg> top                │
        │ <Shift-N> sort Nm │ <q> close           │ <G>  bottom             │
        │ <R>     refresh   │ <Esc> cancel/clear  │ <Ctrl-f/b> page         │
        │ <r>     toggle JSON│<Tab> next tab      │ <Ctrl-d/u> half page    │
        │                   │                     │                         │
        │ filter: /<text>                                                  │
        ╰────────────────────────────────────────────────────────────────────╯
```

3-column (KEEP issue #147). `/` filters all 3 (KEEP). Width fits chrome inner − 6.

### 6.3 Action menu (`a`)

```
                  ╭─ Actions · alice@acme.com ────────────────╮
                  │  Reset password — send reset email        │
                  │▸ Unlock account — clear LOCKED_OUT        │
                  │  Reset MFA factors — destructive          │
                  │                                           │
                  │ <Enter> run  <Esc> cancel                 │
                  ╰───────────────────────────────────────────╯
```

Cursor `▸ ` + Accent (KEEP). Destructive items get Danger label suffix. Enter → posts `[ACTION: <id>]` to status row, then opens confirm.

### 6.4 Confirm

```
                       ╭─ Confirm ──────────────────────────╮
                       │  Reset password for                │
                       │   ALICE@ACME.COM                   │
                       │                                    │
                       │  This cannot be undone.            │
                       │  Type the login to confirm:        │
                       │  > alice@acme.com_                 │
                       │                                    │
                       │ <Enter> confirm  <Esc> cancel      │
                       ╰────────────────────────────────────╯
```

Typed-confirm preserved (KEEP). Title Danger for destructive ops.

### 6.5 Quit / About / boot error

Same scaffold. About content unchanged (version / commit / build / profile / token source / RL summary). Boot error owns the entire viewport — only path that runs before chrome init.

---

## 7. Color / token roles

No new themes. One rename (`RowHighlight` → `RowCursor`) to clarify the role.

| Role             | Used for                                                | Dark color                  |
|------------------|----------------------------------------------------------|-----------------------------|
| `Header`         | brand, env, section labels, column headers (bold)        | #88c0d0 bold                |
| `Accent`         | principal, sort↑, palette suggestions, scrollbar thumb   | #81a1c1                     |
| `Muted`          | divider stamps, alias hints, time-ago, `—` placeholders  | #5c6a7a                     |
| `Success`        | ACTIVE dot, RL ok, TAIL/FOLLOW on, sort↑                 | #a3be8c                     |
| `Warning`        | SUSPENDED, env [stage], visual badge, RL warn            | #ebcb8b                     |
| `Danger`         | LOCKED_OUT/INVALID, sort↓, ACTION badge, `[offline]`     | #bf616a bold                |
| `Info`           | toasts (`copied`, `refreshed`)                           | #88c0d0                     |
| `Magenta`        | unmask badge backdrop                                    | #b48ead                     |
| `RowCursor` (was RowHighlight) | active cursor row across every region    | bg #2e3440 fg #88c0d0 bold  |
| `RowDanger`      | LOCKED_OUT / INVALID rows                                | bg #4c1f21 fg #f0d4d6       |
| `RowWarning`     | SUSPENDED / PASSWORD_EXPIRED rows                        | bg #4a3a17 fg #f5e7c1       |
| `RowMuted`       | DEPROVISIONED / INACTIVE rows                            | bg #2a2f38 fg #7a8290       |

Precedence: `data → status-row bg → cursor bg`. Codified once in `RenderRowWithCursor`. Monochrome / HighContrast inherit the rename — reverse video on `RowCursor`.

---

## 8. Key-binding map (delta vs current)

Marker: KEEP / RENAME / REPLACE / NEW. **Every key in inventory §E is KEEP.** Palette commands inventory §F all KEEP — canonical-only autocomplete behaviour unchanged. The only behavioural extensions:

| Key             | Extension                                                              | Marker |
|-----------------|------------------------------------------------------------------------|--------|
| `Esc`           | now also pops cursor region (extras → grid → list); at list with no overlay/filter posts toast `nothing to close` | REPLACE — was silent today (pain T7) |
| `]` / `[`       | semantics generalised to "step right / left region" (list → grid-left → grid-right → extras-groups → extras-apps) | KEEP — same physical bindings, single dispatcher |
| `h` / `l`       | cursor model dispatches: hscroll on lists, region step in detail        | KEEP   |
| `j` past last row of detail-col-left | enters detail-col-right (KEEP issue #181)               | KEEP   |

No keys removed. No keys added.

---

## 9. Acceptance gates

1. Chrome height invariant: status row always rendered; body rows = `height − 7`.
2. Cursor renders identically across all 6 regions — same RowCursor token, gutter, scrollbar.
3. Status row badges follow §2.4 order; at width 80 low-priority badges drop with `…`.
4. Every screen routes its rows through `shared.RenderRowWithCursor`. No screen owns cursor markup.
5. Empty / loading / error states render via `shared/empty.go`.
6. Overlays mount through `shared.MountModal`. No overlay touches chrome geometry.
7. 80x24 wireframes in this doc reproduce byte-for-byte from goldens.
8. Every inventory item still works; key-binding delta marks every change as KEEP.

End spec.
