# Changelog

All notable changes to **ota** (Okta TUI) are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Planned (v0.1.x patches)
- QA-012: warn (log-only, non-blocking) when `~/.config/ota/config.yaml` has loose permissions (anything other than `0600`).
- QA-013: numeric rate-limit display in the header (`[RL: 586/600]`) and `:ratelimit` modal with last-observed values per category.

### Planned (v0.2)
- `OpenResourceMsg` routing for detail screens (graduates from inline `ListModel.opened` mode used in v0.1.1).
- Apps resource (list, detail, User → Apps tab).
- Interactive token entry prompt as the third fallback for `OKTA_API_TOKEN` (PRD REQ-C04 AC-1 step 3).
- Runtime `:profile` switching with cache invalidation (PRD REQ-C02 AC-3).
- `HealthPort` production implementation backing `:healthcheck`.
- First Write actions, in domain-risk order:
  1. Group static member add / remove.
  2. User lifecycle: `unlock`, `unsuspend`, `activate`, `deactivate`.
  3. Group Rule `activate` / `deactivate` (with double-confirm + impact estimate; deactivation removes rule-granted memberships).
- Rich renderers for `PROFILE_ENROLLMENT`, `POST_AUTH_SESSION`, `IDP_DISCOVERY` policy types.
- OAuth 2.0 Service App (Private Key JWT) authentication alongside SSWS.
- `M!` toggle (`:unmask <field>` / `:mask`) for selective PII reveal in Detail Raw tab.

---

## [0.1.1] — 2026-04-26

User-driven iteration on top of v0.1.0. Adds responsive TUI, column sort,
Detail entry / Raw tab, and palette aliases. Targets Users / Groups /
Group Rules; Policies and Logs unchanged from v0.1.0.

### Added

**Responsive layout (TUI_DESIGN §15.0a)**
- Chrome now fills 100% of the terminal width — `clampWidth` no longer caps at
  200, and `tea.WindowSizeMsg` propagates through every cached child screen.
- New `internal/tui/shared/columns.go` implements a Min/Weight + drop-priority
  layout algorithm shared across all list views.
- Users gains a 6th column `DEPARTMENT` once usable width ≥ 130; falls off
  before `LAST LOGIN` / `CHANGED` / `DISPLAY NAME` as the terminal narrows.

**Column sort (TUI_DESIGN §3.5)**
- `Shift+S/N/L/C` cycles each column off → asc → desc → off. The active
  column header carries a `↑` or `↓` glyph (one cell, no padding impact).
- Stable sort (`sort.SliceStable`); cursor resets on cycle change, preserves
  position on repeat. Status orderings follow operational rank
  (Users: `ACTIVE` first, Rules: `INVALID` first).
- Users supports all four keys; Groups: `Shift+N` only; Rules: `Shift+S`
  + `Shift+N`. Unmapped keys are no-op.

**`d` for inline Detail (TUI_DESIGN §3.6, §3.6a Note)**
- `d` and `Enter` both open a resource's detail in inline mode. `Esc`
  returns to the list. `Ctrl-c` quits the program globally.
- Full `OpenResourceMsg` routing through the App Shell remains a v0.2
  goal; v0.1.1 ships the simpler `ListModel.opened` pattern Phase 6c
  already deferred for v0.1.0.

**Detail Raw tab (TUI_DESIGN §15.7)**
- Every detail surface gains a final `[Raw]` tab — Users (7), Groups (5),
  Rules (4) — that renders `json.MarshalIndent` of a stable shape struct.
- `Tab` / `Shift+Tab` cycle through tabs forward / backward. `r` toggles
  between Raw and the most recently viewed non-Raw tab.
- PII (phone numbers, emails) is masked at the projection step before
  marshalling, and masked lines are annotated `# masked` in the rendered
  output for human readers.

**Palette aliases (TUI_DESIGN §3.4)**
- `:user`, `:users`, `:u` → Users.
- `:group`, `:groups`, `:g` → Groups.
- `:rule`, `:rules`, `:grouprule`, `:grouprules`, `:group-rule`,
  `:group-rules`, `:group_rule`, `:group_rules`, `:gr` → Group Rules.
- Existing `:policy`, `:policies`, `:log`, `:logs`, `:l` aliases retained.

### Changed

- Removed the unused `IDSearchNext` / `IDSearchPrev` (`n` / `N`) keys —
  they were never wired and the incremental `/` filter makes match-step
  navigation moot. Frees the uppercase `N` rune for `IDSortName`.
- Help overlay row updated to advertise `Shift+S/N/L/C` sort and `d`
  detail.
- `internal/app/clampBodyLines` reserves 7 rows of chrome instead of 6
  to match the §15.0a layout.

### Internals

- 6 new feature commits + 1 spec commit, each one a single work unit.
- Production `internal/tui/users/list.go` shrunk by 168 lines after
  migrating to the shared column layout.
- Test scoreboard: 21 packages race-PASS, 0 fail. New spec lock-ins for
  responsive widths, sort cycling, d-key detail, and palette resolution.

---

## [0.1.0] — 2026-04-25

Initial public release. Read-only Okta TUI MVP for Workforce Identity (OIE).

### Added

**TUI core**
- k9s-style resource navigation with `:` command palette, `/` incremental search, and Vim motion (`hjkl`, `gg`, `G`, `Ctrl-d/u`, `Ctrl-f/b`).
- Context-aware Help modal (`?`) with four tabs: Screen / Global / Commands / Status icons.
- Quit confirmation when a Logs tail is active; `Ctrl-c` double-tap force-exits.
- Status bar limited to six keys per screen; remainder discoverable via `?`.

**Resources (read-only)**
- **Users** — list with status icons (`● ACTIVE`, `○ STAGED`, `✗ SUSPENDED`, `⚠ LOCKED_OUT`, `◒ PASSWORD_EXPIRED`, `⊘ DEPROVISIONED`), 6-tab detail (Profile / Credentials / Timestamps / Groups / Factors / Recent Logs), SCIM-style `:search`, fixed + custom profile field separation.
- **Groups** — type icons (`◆` `▣` `◈`), `RULE` / `SYS` / `LARGE` badges, progressive member loading with `Esc` to stop, special banner for `BUILT_IN` and `Everyone`.
- **Group Rules** — `ACTIVE` / `INACTIVE` / **`INVALID`** colour coding, INVALID-count banner, monospace expression view, prominent deactivation-impact warning banner reused across read-only and v0.2 write modes.
- **Policies** — type-as-namespace navigation through all 7 OIE types; rich rendering for `OKTA_SIGN_ON`, `ACCESS_POLICY`, `PASSWORD`, `MFA_ENROLL`; raw-JSON view (`r` / `:raw`) for `PROFILE_ENROLLMENT`, `POST_AUTH_SESSION`, `IDP_DISCOVERY`; `system=true` badge.
- **System Logs** — search + tail mode with adaptive 7s polling (auto-stretches to 15s when `X-Rate-Limit-Limit < 60`), hole-free `since` resume after 429 backoff, 5 built-in filter presets including the warning-coloured *Group Rule Deactivations*.

**Configuration & authentication**
- XDG config (`$XDG_CONFIG_HOME/ota/config.yaml`) with multi-tenant `profiles:` block.
- Token resolution order: profile-scoped `api_token_env` → `OKTA_API_TOKEN` env → (interactive prompt — *deferred to v0.2*).
- Token never written to disk; scrubbed from panic stack traces, `Stringer` output, and debug logs (REQ-C05).
- Customisable keybindings (`keybindings:` section maps key IDs in `internal/keys/keys.go` to user-defined keys).
- `--profile <name>` startup flag.

**Observability & resilience**
- Header rate-limit awareness with auto pause/resume of tail polling on HTTP 429 (`Retry-After` honoured + jitter).
- Per-category "last observed" rate-limit tracking surfaced in `:about`.
- 30-second TTL cache for list responses (`R` / `:refresh` to invalidate).
- `--debug` writes `~/.cache/ota/debug.log` (10 MB × 3 rotation, `0600`).
- Session error history viewable via `:errors`.

**Security & privacy**
- PII masked by default: `phoneNumber` (`+1-***-***-1234`), `secondEmail`, `mobilePhone` (`a***@example.com`).
- `:unmask <field>` reveals values for the current session only; `[M!]` warning badge marks unmasked state; auto re-mask after 60 s of inactivity or screen change.
- Read-Only Administrator account recommended in onboarding docs.
- Crash dump / core dump scrubbing guidance in README; `ulimit -c 0` recommended.

**Domain accuracy (Okta Identity Engine)**
- Error code translation table for `E0000001`, `E0000004`, `E0000006`, `E0000007`, `E0000011`, `E0000022`, `E0000038`, `E0000047` (PRD §7.7).
- Identifier prefixes documented per resource (`00u`, `00g`, `0pr`, `00p`).
- Eventually-consistent semantics of Users `search` surfaced in Help and the empty-state hint.
- Search syntax differentiation across `q` (free text), `search` (SCIM, Users/Groups), `filter` (SCIM, Groups/Apps/Logs).

### Known limitations

(verbatim from [PRD §11.3.1](docs/PRD.md))

- Token input is environment-only — interactive prompt deferred to v0.2 (QA-005).
- `:profile` runtime switch is not implemented — restart with `--profile <name>` (QA-009).
- `:ratelimit` and `:healthcheck` modals are partial or missing in v0.1.0 (QA-013, QA-016).
- Config file permission validation is informational only (QA-012, queued for v0.1.x).
- Local `golangci-lint` / `gofumpt` / `govulncheck` are validated through CI rather than enforced locally (QA-018).

### Internals

- Hexagonal architecture with `depguard` enforced SDK isolation; Okta calls go through `internal/okta` adapters that never leak SDK types into `internal/tui`.
- Direct `net/http` Okta client (Okta Go SDK v5 evaluated as a v0.2+ option).
- 19 packages, ~7,250 lines of production Go and ~3,660 lines of tests.
- Test coverage gates: `okta` 80.6 % · `service` 86.6 % · `domain` 100 % · `errormap` / `pagination` / `keys` 100 %.
- All quality gates green: `go build`, `go vet`, `go test -race -count=1 ./...` (17 packages), `golangci-lint` (CI).

### Acknowledgements

This release was built end-to-end through a multi-phase agent workflow: PRD authoring with Okta domain expert review, TUI design with REQ-ID traceability, TDD fail-first implementation, and three QA verification cycles. Detailed phase artifacts live under `docs/` and `_workspace/`.

---

## Versioning policy

- **MAJOR** — breaking change to PRD-tracked behaviour or config schema.
- **MINOR** — new functional REQ added (e.g. v0.2 Write actions, Apps resource).
- **PATCH** — Known-Limitations resolution, bug fix, or doc-only change. PRD `1.0.x` increments accompany ota `0.1.x` patches when AC clarifications ship.

[Unreleased]: https://github.com/tedilabs/ota/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/tedilabs/ota/releases/tag/v0.1.1
[0.1.0]: https://github.com/tedilabs/ota/releases/tag/v0.1.0
