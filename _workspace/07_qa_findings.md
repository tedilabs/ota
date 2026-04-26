# 07. QA Findings — Cycle 1 (internal)

**Author:** qa
**Date:** 2026-04-24
**Target:** Git HEAD @ end of Phase 6
**Companion:** `docs/QA_REPORT.md` (external, PM/team-lead facing)

This file preserves raw evidence gathered during cross-boundary verification
(PRD ↔ TUI_DESIGN ↔ code ↔ tests). It is retained for regression lookups and
post-mortem reviews.

## Sources read

- `docs/PRD.md` (v1.0.0, 766 lines)
- `docs/TUI_DESIGN.md` (v1.0.0, 2195 lines)
- `docs/ARCHITECTURE.md`, `docs/TESTING.md`, `docs/CONVENTIONS.md` (headline sections)
- `_workspace/06_implementation_report.md` (developer self-attest)
- All files under `internal/` + `cmd/`

## Gate commands (2026-04-24)

| Command | Result |
|---------|--------|
| `go build ./cmd/ota` | PASS (8.2 MB arm64 binary) |
| `go vet ./...` | PASS |
| `go test -race -count=1 ./...` | PASS (13 packages) |
| `go test -cover ./...` | PASS; see coverage table below |
| `golangci-lint run` | **NOT EXECUTED — binary missing locally** |
| `gofumpt -l -d .` | **NOT EXECUTED — binary missing** |
| `govulncheck` | **NOT EXECUTED — binary missing** |
| `./ota --help` | Usage printed, **exits 1** (flag.ContinueOnError) |
| `./ota` (no env) | "ota: no profile available … set OKTA_ORG_URL" — acceptable UX |

Coverage snapshot:

| Package | % | TESTING target | Verdict |
|---------|----|----|----|
| internal/domain | 100.0 | 95 | MEETS |
| internal/keys | 100.0 | — | MEETS |
| internal/okta/pagination | 100.0 | — | MEETS |
| internal/okta/errormap | 100.0 | — | MEETS |
| internal/okta/ratelimit | 87.9 | — | MEETS |
| internal/mask | 86.4 | — | MEETS |
| internal/app | 79.6 | — | OK |
| internal/logger | 75.0 | — | OK |
| internal/tui/users | 74.5 | 60 | MEETS |
| internal/service | 62.0 | 85 | **MISS (-23)** |
| internal/config | 53.3 | — | LOW |
| internal/okta | 43.7 | 75 | **MISS (-31)** |
| cmd/ota | 0 | — | not tested |
| internal/cache | 0 | — | panic stub |
| internal/clock | 0 | — | not tested |
| internal/tui/{groups,rules,policies,logs,overlay,shared} | 0 | 60 | **MISS (-60)** |
| internal/okta/testfx, service/fakes | 0 | — | test helpers |
| internal/security | (no statements) | — | peek test only |

## Cross-boundary matrix — raw field-level observations

### B1. Okta API ↔ adapter ↔ domain ↔ view

Evidence from reading `internal/okta/mapping.go` against `internal/domain/*.go`
side by side:

| Adapter field | Domain field | Observation |
|---|---|---|
| `wireUser.ID → domain.User.ID` | string | OK |
| `wireUser.Status → domain.User.Status` | UserStatus (7 enum values) | OK — all 7 states in domain/user.go match AC-2 |
| `wireUser.Profile` | UserProfile + Extras map | OK — custom fields preserved |
| `wireUser.Credentials.Provider.{Name,Type}` | UserCredentials.{Provider, ProviderType} | **Mapping inverted**: `Provider` filled from JSON `provider.name`, `ProviderType` from JSON `provider.type`. Okta's JSON is `credentials.provider.type` (e.g., OKTA) and `provider.name` (e.g., "OKTA"). Naming swap is cosmetic, but consumers relying on "provider vendor" vs "provider enum" may be surprised. NO-IMPACT in current MVP (detail/factors view not implemented). Keep as Low. |
| `wireUser.LastLogin / Activated / ...` | `*time.Time` | OK — nil preserved via parseOktaTimePtr |
| `wireLogEvent.Transaction` | `json.RawMessage` domain.LogEvent.Transaction | OK, but **View layer never consumes Transaction/Request/Debug** (stub) |
| `wireFactor.Profile.PhoneNumber` | FactorProfile.PhoneNumber | OK at boundary, **no mask.Phone integration in any View** — masking policy (REQ-R01 AC-6, TUI_DESIGN §7.2) is not enforced because no Factors view renders |

### B2. Keymap (TUI_DESIGN §3) ↔ `internal/keys/keys.go` ↔ `defaults.go`

TUI_DESIGN specified key IDs (count + representative samples):

Global (§3.1): `global.cmd_palette`, `global.search`, `global.help`, `global.cancel`, `global.close`, `global.hard_quit`, `global.redraw`, `help.search` (8)
Nav (§3.2): `nav.down/up/left/right/top/bottom/half_down/half_up/page_down/page_up/select/tab_next/tab_prev/line_home/line_end` (15)
Action (§3.3): `action.refresh/toggle_raw/yank/yank_field/yank_row/open_web/expand`, `logs.follow`, `logs.tail_toggle`, `search.next/prev` (11)

Total specified: **34 IDs**.

Implemented in `internal/keys/keys.go`:

```
IDNavDown, IDNavUp, IDNavLeft, IDNavRight, IDNavTop, IDNavBottom, IDNavPageUp, IDNavPageDn   (8)
IDAppQuit, IDAppHelp, IDAppRefresh, IDAppBack                                                 (4)
IDCmdOpen, IDSearchOpen, IDSearchNext, IDSearchPrev                                           (4)
IDLogsTailToggle, IDLogsFollowToggle                                                          (2)
IDPIIUnmask, IDPIIMask                                                                        (2)
```

Total implemented: **20 IDs**.

**Gap: 14 IDs not defined in implementation.** Worse, several that ARE defined differ in key string from TUI_DESIGN:

| Key ID (design) | Design key | Implementation | Mismatch? |
|---|---|---|---|
| `logs.tail_toggle` | `s` | `"t"` (IDLogsTailToggle) | **YES — bug** |
| `nav.half_down` (Ctrl-d) | spec'd | missing | missing ID |
| `nav.half_up` (Ctrl-u) | spec'd | missing | missing ID |
| `nav.page_down` (Ctrl-f) | spec'd | IDNavPageDn = "Ctrl-f" ✓ | label swap only |
| `nav.page_up` (Ctrl-b) | spec'd | IDNavPageUp = "Ctrl-b" ✓ | OK |
| `nav.select` (Enter) | spec'd | missing — app uses tea.KeyEnter directly | missing ID |
| `nav.tab_next` / `nav.tab_prev` | spec'd | missing | missing ID |
| `global.hard_quit` (Ctrl-c) | spec'd | missing ID (but handled inline via tea.KeyCtrlC) | missing ID |
| `global.redraw` (Ctrl-l) | spec'd | missing | missing ID |
| `action.toggle_raw` (`r`) | spec'd | missing | missing ID |
| `action.yank` + `yank_field` + `yank_row` | spec'd | missing | missing ID |
| `action.open_web` (`o`) | spec'd | missing | missing ID |
| `action.expand` (`e`) | spec'd | missing | missing ID |

`IDAppBack` = "Esc" maps to design's `global.cancel`; `IDAppQuit` = "q" maps to
`global.close`. Usable but naming divergence adds friction in Help overlay.

Tests: `internal/keys/resolver_test.go` only exercises `IDNavDown`,
`IDNavUp`, `IDSearchOpen`, `IDCmdOpen`, `IDAppQuit`. It does not enforce the
design contract against the full ID set.

### B3. Command palette (TUI_DESIGN §3.4) ↔ implementation

TUI_DESIGN lists 17 `:` commands. Production implementation has zero —
`internal/tui/overlay/overlay.go` `CmdPaletteModel.Update` is a no-op, and
`internal/app/app.go` only emits `openCmdPaletteMsg{}` which nothing subscribes
to. `:users`, `:groups`, `:logs`, `:profile`, `:search`, `:filter`, `:about`,
`:ratelimit`, `:errors`, `:healthcheck`, `:refresh`, `:debug open`, `:unmask`,
`:mask`, `:raw`, `:help`, `:quit` — **none dispatched**.

### B4. Screen catalog (TUI_DESIGN §4) ↔ TUI Model code

Designed screens:

```
SCR-000 Profile Select
SCR-001 Error Boot
SCR-010 Users List            ← partial (list.go)
SCR-011 User Detail           ← stub (detail.go)
SCR-020 Groups List           ← stub
SCR-021 Group Detail          ← stub
SCR-030 Group Rules List      ← stub
SCR-031 Group Rule Detail     ← stub
SCR-040 Policy Type Select    ← stub
SCR-041 Policies List         ← stub
SCR-042 Policy Detail         ← stub
SCR-050 Logs Search/Tail      ← stub
SCR-051 Log Event Detail      ← stub
SCR-900 Command Palette       ← stub
SCR-901 Search Prompt         ← not present (search handled inline in users)
SCR-902 Help                  ← stub
SCR-903 Confirm Dialog        ← stub
SCR-904 Error Detail          ← not present
SCR-905 About/RateLimit/Healthcheck ← stub (single AboutModel)
SCR-910 Quit Confirm          ← not present
```

**Only SCR-010 has meaningful content; 19 screens are stubs or absent.**

### B5. State machine / transitions

Because screens are stubs, there is no routing between them. `internal/app/app.go`
Update handles only `tea.KeyCtrlC`, `":"`, `"?"`, `ErrorMsg`, `NetworkErrorMsg`,
`NetworkRestoredMsg` → emits Cmds/Msgs that no other model consumes. No child
Model composition, no back-stack, no breadcrumb. Design §2.3 Breadcrumb and
§2.5 back policy are unimplemented.

### B6. Error mapping

`internal/okta/errormap/map.go` covers all 8 codes in PRD §7.7 plus fallbacks
by HTTP status. Errors are wrapped so `errors.Is(err, domain.ErrX)` works.
Adapters (`internal/okta/*_adapter_test.go`) exercise the common paths.

**But user-facing message rendering is absent** — the errormap translates to
sentinels, not to the strings prescribed in REQ-C04 AC-4 ("API token invalid
or revoked. Rotate and retry.", etc.). No TUI code consumes these errors yet.
Design gap: errormap gives `wrap(err, summary)` → the summary is Okta's raw
summary, not ota's curated guidance text.

### B7. PII masking

- `internal/mask/mask.go`: `Phone`, `Email` — implemented and tested.
- `internal/security/peek_test.go`: scans testdata for raw PII — green.
- **No view consumes Phone/Email masking** — the SMS/Voice factor phone field
  is read through adapter → domain but never displayed, so masking policy
  exists on paper only.
- `internal/tui/users/factors.go` carries an `unmasked map[string]bool` but
  View() = `""`. Unmask state machine is not implemented.

### B8. Rate limit (REQ-E01)

- `internal/okta/ratelimit/monitor.go` observes `X-Rate-Limit-*` headers and
  categorizes by path — matches REQ-E01 AC-4.
- `internal/okta/client.go` retries 429 honoring Retry-After up to 3 attempts
  — matches AC-2. However the fallback `time.After(d)` safety branch
  (client.go:221) may cause a short wait in tests that pass a FakeClock
  without Advance. Not a prod concern.
- `internal/app/app.go` Deps includes `RateLimit domain.RateLimitPort` but
  **no code reads it**. No status bar render, no `:ratelimit` screen.
- `monitor.CategoryFromPath` returns `"other"` for unrecognized paths — OK
  but design §3.3 examples reference 5 categories (management/logs/policies/apps
  + "other") and there's no UI affordance, so this is a latent feature.

### B9. Logs tail (REQ-R05)

- `internal/service/logs_tail.go`: Initial query (since=now-5m, ASCENDING,
  limit=1000), NextSinceAfter(+1ms), ObserveRateLimit (→15s if limit<60),
  Pause/Resume. **Not exposed via Bundle or consumed by any Model.**
- Production code path never instantiates `NewLogsTail`. The adaptive
  polling, pause on 429, hole-free resume — all unreachable.

### B10. Authentication (REQ-C04)

- `internal/app/auth.go` `ResolveToken`: precedence CLI → profile env → (no
  interactive). Interactive path returns "not implemented" error. PRD AC-1
  step 3 is thus **not implemented**; given users get a clear error message
  and env fallback, acceptable for MVP per auth_test.go coverage.
- `:about` source string ("env OKTA_API_TOKEN") — generated correctly but
  never displayed (no :about screen).
- `Token rotation hint` (AC-5) — not implemented. PRD calls it "selective,
  best-effort" and "silently skip if unavailable", so this is acceptable.
- OAuth 2.0 (AC-6) — explicitly out-of-scope for MVP.

### B11. Secrets hygiene (REQ-C05)

- Authorization header: set as `"SSWS "+c.token` in
  `internal/okta/client.go:138`. Token appended to memory string; no early
  scrub implemented (AC-1 "zero-copy…scrub early"). Given it lives the full
  session in the Client struct, AC-1 is partially met — Low.
- `internal/logger/mask_attr.go` scrubs keys `authorization`, `api_token`,
  `token`, `mobile_phone`, `second_email`, `phone_number`. Tests confirm
  (mask_attr_test.go).
- **Config file 0600 check: NONE.** `internal/config/loader.go` does not stat
  the file's mode. MVP treats tokens as env-only (AC-4), but `defaultLogFilter`
  or future fields may warrant a 0600 sanity check + warning. Recorded as
  Medium.
- Debug log file: lumberjack default mode 0600 (verified in vendor source).
  OK.
- Crash stack trace scrub (AC-3): no explicit mechanism. Go's default
  `panic` prints struct fields; `Client.token` is a plain string field. A
  panic in ota code would print the token. Recorded as High.

### B12. Tests that verify behaviour vs compile

- `internal/tui/users/list_flow_test.go`: REAL teatest — verifies filter flow
  and detail transition. Good.
- `internal/app/app_test.go` (4 tests), `auth_test.go`, `keymap_test.go`,
  `offline_test.go`: behaviour-level.
- `internal/service/*_test.go`: service orchestration with fakes. Good.
- `internal/okta/{users,logs}_adapter_test.go`: httptest-backed.
- Missing: Groups/Rules/Policies/Logs adapter integration tests. `ListGroups`,
  `ListFactors` paths are exercised only through users adapter test? Let me
  confirm — `users_adapter_test.go` + `logs_adapter_test.go` exist, but no
  `groups_adapter_test.go`, `policies_adapter_test.go`, `rules_adapter_test.go`
  at root of `internal/okta/`. Result: low adapter coverage (43.7%).

## Panic-only stubs (latent hazards)

- `internal/cache/ttl.go` — all 3 methods `panic("not implemented yet")`. Not
  used in production; used as example in docs only.
- `internal/tui/shared/styles.go` — all 3 theme factories `panic("not
  implemented yet")`.

Hitting these at runtime would crash the app. Unreachable currently but
should be deleted or filled (project rule in CLAUDE.md: "no half-finished
implementations").

## Documentation drift spotted

- `docs/TUI_DESIGN.md` §3.3 says `s` = logs.tail_toggle but
  `internal/keys/defaults.go` uses `t`. Either design or code is authoritative
  — needs reconciliation.
- `docs/PRD.md` §11.3 D-3 confirms 7-second default poll. Design and code
  (`LogsTail.interval`, `LogsService.pollInterval`) both 7s. OK.
- `docs/TUI_DESIGN.md` §3 lists ~34 key IDs; `internal/keys/keys.go` has 20.

## Recommendations to team-lead (executive summary)

The Phase 6 implementation covers **infrastructure layers (domain, okta
adapters, services, auth, logger)** solidly. **The presentation layer (every
Screen Model except users/list)** is an explicit stub, and the App Shell does
not compose screens. PRD v1.0.0 §4.1 requires Users/Groups/GroupRules/
Policies/Logs lists + details + search + tail. Shipping v0.1.0 as-is would
deliver a binary that boots into an empty alt-screen.

Three credible paths forward (from Cycle-1 perspective):

1. **Implement remaining screens.** Enormous scope (TUI_DESIGN §4 is ~1600
   lines of wireframes). Weeks of work. Blocks Phase 7 completion.
2. **Rescope v0.1.0 to "Users-only MVP"** and explicitly note in
   CHANGELOG/PRD that Groups/Rules/Policies/Logs screens are deferred to
   v0.1.1. QA can then certify Users-flow + infra gate quality. PM/team-lead
   decision.
3. **Release as "alpha infrastructure drop"** — document the gap honestly,
   ship the binary, and move to v0.2 dev cycle. Least professional option.

## Files referenced (absolute)

- /Users/austin/workspace/tedilabs/ota/docs/PRD.md
- /Users/austin/workspace/tedilabs/ota/docs/TUI_DESIGN.md
- /Users/austin/workspace/tedilabs/ota/docs/ARCHITECTURE.md
- /Users/austin/workspace/tedilabs/ota/docs/TESTING.md
- /Users/austin/workspace/tedilabs/ota/_workspace/06_implementation_report.md
- /Users/austin/workspace/tedilabs/ota/internal/app/app.go
- /Users/austin/workspace/tedilabs/ota/internal/app/auth.go
- /Users/austin/workspace/tedilabs/ota/internal/app/keymap.go
- /Users/austin/workspace/tedilabs/ota/internal/app/msg.go
- /Users/austin/workspace/tedilabs/ota/internal/keys/keys.go
- /Users/austin/workspace/tedilabs/ota/internal/keys/defaults.go
- /Users/austin/workspace/tedilabs/ota/internal/keys/resolver.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/users/list.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/users/detail.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/users/factors.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/groups/groups.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/rules/rules.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/policies/policies.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/logs/logs.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/overlay/overlay.go
- /Users/austin/workspace/tedilabs/ota/internal/tui/shared/styles.go
- /Users/austin/workspace/tedilabs/ota/internal/okta/client.go
- /Users/austin/workspace/tedilabs/ota/internal/okta/users.go
- /Users/austin/workspace/tedilabs/ota/internal/okta/logs.go
- /Users/austin/workspace/tedilabs/ota/internal/okta/mapping.go
- /Users/austin/workspace/tedilabs/ota/internal/okta/errormap/map.go
- /Users/austin/workspace/tedilabs/ota/internal/okta/ratelimit/monitor.go
- /Users/austin/workspace/tedilabs/ota/internal/domain/ports.go
- /Users/austin/workspace/tedilabs/ota/internal/domain/errors.go
- /Users/austin/workspace/tedilabs/ota/internal/domain/user.go
- /Users/austin/workspace/tedilabs/ota/internal/domain/policy.go
- /Users/austin/workspace/tedilabs/ota/internal/domain/queries.go
- /Users/austin/workspace/tedilabs/ota/internal/service/users.go
- /Users/austin/workspace/tedilabs/ota/internal/service/logs.go
- /Users/austin/workspace/tedilabs/ota/internal/service/logs_tail.go
- /Users/austin/workspace/tedilabs/ota/internal/service/bundle.go
- /Users/austin/workspace/tedilabs/ota/internal/config/loader.go
- /Users/austin/workspace/tedilabs/ota/internal/config/config.go
- /Users/austin/workspace/tedilabs/ota/internal/config/paths.go
- /Users/austin/workspace/tedilabs/ota/internal/logger/logger.go
- /Users/austin/workspace/tedilabs/ota/internal/logger/mask_attr.go
- /Users/austin/workspace/tedilabs/ota/internal/mask/mask.go
- /Users/austin/workspace/tedilabs/ota/internal/security/peek_test.go
- /Users/austin/workspace/tedilabs/ota/internal/cache/ttl.go
- /Users/austin/workspace/tedilabs/ota/cmd/ota/main.go
- /Users/austin/workspace/tedilabs/ota/cmd/ota/wire.go
