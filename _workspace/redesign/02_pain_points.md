# 02 Pain Points — themes the commit history kept fixing

88 commits, `872e14f` (v0.1.0) → `9d81a01` (v0.1.17). Same themes repeat — systemic.

**T1. Cursor visibility & full-row coverage.** `f265975` extend highlight across JSON/YAML row · `6b9d0e5` accent+▸ · `868a66d` no indent shift under cursor · `132780e` 2-cell gutter aligns header with data · `d8bf046` RowHighlight token · `9d81a01` detail box border + column-flow cursor · `9fb092f`/`06f9d42` Pretty fills width. Highlight kept fighting padding, ANSI strip order, column composition. Need ONE cursor model across list rows, detail body, 2-col Pretty, extras boxes, JSON/YAML.

**T2. Column overflow & responsive width.** `91517b1` East-Asian-Wide/emoji width · `ab8f2a5`/`0ccb78e`/`1550c1d`/`a4f1a98` auto-fit / observed-fit / reorder · `c6f7881` h/l hscroll · `83b707d`/`0ad48a2` overlays push chrome off-screen, pipe Width/Height to children · `fd692b4` 100% fill + Min/Weight cols · `63534c8` viewport-window rows · `4dcbc84` revert cols to stop API burst. Width math is ad-hoc per screen. Centralise into one column-spec → render pipeline.

**T3. Chrome / overlay alignment.** `d21d54a` palette right border to chrome · `2a4cc7d`/`3001c13` 3-column help, modal centred · `60cac3f` k9s header, label-in-divider · `5b35021` count+filter stamped into divider. Overlays still feel like a separate coordinate space. One layout engine that knows chrome inner-width; overlays request space from it.

**T4. Logs noise / dupes / range UX.** `af76ebd` UUID dedup follow-mode · `eeb2197` visible f/r/s feedback + cursor on newest · `52c7888` `/` filter + tail/follow/range status line · `32dbf1c` newest-at-bottom + 30m default + 1/3/12/24h · `c03a2e6` Pretty/JSON/YAML on Log Detail. Tail mode is fragile — which mode, where's the cursor, did this event already render. Logs status line is first-class.

**T5. Palette discoverability.** `0936f7a` canonical-only autocomplete · `57bf733` Tab cycle · `8697076` floating input · `ba009a5` singular/plural/hyphen/underscore aliases · `13ca5ee` per-type palette skips picker · `9bf98d9` arrows + `:group-rule`. Operators don't read help. Palette is the discovery surface. Autocomplete + canonical hint list must stay tightly curated.

**T6. Drill-down & cross-screen flow.** `1480550` Enter on Groups/Apps drills · `c8cf807` side-by-side extras boxes · `f29365f` assigned Groups+Apps · `b45bdeb`/`02e9561` Apps API quirks · `4dcbc84` revert cols to stop API burst. Drill-down is high-value but expensive. Per-user keying, fetch dedup, lazy-load, cancel-on-back are non-negotiable. Make patterns explicit.

**T7. State / mode visibility.** `eeb2197` visible f/r/s · `cfa32a5` Esc clears filter to full set · `5b35021` count+filter in divider · `f684eab` principal on ContextBar · `ed43883` Esc-to-close + d-key to Policies/Logs · `3758edb`/`763b065` forward Esc + unhandled keys. Filter / sort / tail / focus / Visual — each had to be made visible after the fact. Don't repeat that pattern.
