# 03 Redesign Brief — for tui-designer

Not adding features. Tightening what's there. 88 commits of catching up on visual debt — stop catching up.

## What "better" means

ota is k9s for Okta. Operators come to answer one question per minute and leave. Better = fewer mental beats to find the row, open detail, tail something. Priorities, in order:

1. **Cursor flow.** ONE cursor model across lists, detail bodies, 2-col Pretty, JSON/YAML, extras boxes. Always the most prominent element, including under abnormal-status tints. Today patched per-screen — make it a system.
2. **State legibility.** Sort, filter, tail, follow, focus region, Visual — each gets one unambiguous indicator. No silent state. Esc either closes something visible OR shows "nothing to close". Upper-divider for persistent state; consider a slim status row above KeyHints for transient mode badges.
3. **Column composition.** ONE column spec per resource, one renderer that knows width / priority / observed-fit / hscroll / sort glyph / header alignment / cursor gutter / East-Asian-Wide. Centralise.
4. **Overlay layout.** Palette / filter / help / action menu / confirm / about all align to chrome inner-width and never push the frame off-screen. Define one mounting protocol.
5. **Detail tab consistency.** Pretty / JSON / YAML universal. Same tab bar, `r` toggle, Visual+yank, line cursor. users-detail is master; others trail. Lift to shared mixin or write a contract every screen meets and test it.
6. **Density.** k9s is dense. ota has empty cells in User Detail Pretty. Don't drop information; drop padding.

## Constraints

- Preserve every item in `01_feature_inventory.md`. Replacements cover original intent 1:1. Don't remove silently — flag in open questions instead.
- One-commit rollback per change.
- 80x24 minimum. Test every wireframe at 80x24.
- Read-only MVP (Users lifecycle exception, typed-confirm gated).
- Bubbletea + Lipgloss. No mouse, no images, no kitty graphics.
- Keymap stability. No binding removed unless replaced 1:1 with same intent.

## Anti-goals

Don't add features. No mouse. No split-panel (v0.2). Don't redesign colour theme from scratch — refine existing tokens (Accent / Header / Muted / Warning / Danger / RowHighlight). Don't merge Pretty/JSON/YAML (operators yank JSON). Don't replace palette with a menu (`:` is muscle memory). Don't reorder global keymap (`:` `/` `?` `q` Esc Tab j k h l).

## Open questions

1. Cursor abstraction — list / detail-line / 2-col column-flow / extras linear-cross-box: one thing or three?
2. Mode-badge location — `-- VISUAL --`, `▶ tail`, `[FILTER]`, `[FOCUS: extras]`: above body, upper divider, or slim status row? Pick one, apply everywhere.
3. Sort indicator — `↑↓` with `^v` ASCII fallback or bracketed `[A]/[D]` next to header?
4. Detail tab consolidation — shared mixin or contract? If contract: where written, how tested?
5. Logs status line — propose condensed format, e.g., `[TAIL 7s · FOLLOW · 30m] · 142 events · last 12s ago`.
6. Help discoverability — operators don't open `?`. KeyHints too short. Transient hint after misfired key?
7. Empty / loading / error layouts — define one set, apply everywhere.
8. Action menu vs palette — `:reset-password` and `a → Reset Password` both exist. Intentional or consolidate?

## Process

Output: `_workspace/redesign/04_design_proposal.md` (or v2 of `docs/TUI_DESIGN.md`). ASCII wireframes at 80x24 AND 140x40, with responsive drop order explicit. For each behaviour change, point to the inventory item it replaces and how the replacement covers the original intent. Flag anything you can't preserve.
