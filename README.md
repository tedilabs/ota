# ota
♥️  k9s for Okta — a Go/Bubbletea TUI to inspect Okta admin resources

**ota** is a keyboard-driven, k9s-style Terminal UI for exploring and auditing Okta Workforce Identity organizations. It lets IAM operators, security auditors, and SREs navigate Users, Groups, Group Rules, Policies, and System Logs without leaving the terminal.

> **Status:** v0.1.0 MVP — **read-only**. Write actions (group membership edits, user lifecycle, etc.) are planned for v0.2. See [Roadmap](#roadmap).

---

## Why ota

Okta's Admin Console requires many clicks to correlate a user's group membership with their recent login events, and shared `curl + jq` snippets lose context fast. ota brings the same keyboard-first, "press `:`, type a resource, drill down" workflow that k9s established for Kubernetes — applied to Okta's identity model.

Typical loops it accelerates:
- "Why can't `alice@example.com` reach this app?" → `:users` → `/alice` → Enter → Groups tab.
- "What failed sign-ins happened in the last 24h?" → `:logs` → `P` → preset *Failed Sign-ins (24h)*.
- "Is this Group Rule still valid?" → `:grouprules` → look for the red `⚠ INVALID` badge.

---

## Install

### From source (recommended for v0.1.0)

```bash
git clone https://github.com/tedilabs/ota.git
cd ota
go build -o ota ./cmd/ota
./ota --version
```

Requires Go 1.24+.

### Via `go install` (when published)

```bash
go install github.com/tedilabs/ota/cmd/ota@v0.1.0
```

---

## Quick start

### Step 1 — Get an Okta API token

1. Sign up for a free Okta Developer tenant: <https://developer.okta.com/signup/>
2. In the Admin Console, create a **Read-Only Administrator** account (Security → Administrators).
3. Sign in *as that read-only admin*, then go to **Security → API → Tokens → Create Token**.
4. Name it `ota-readonly` and copy the value once — Okta will not show it again.

> **Why Read-Only Admin?** ota is read-only in v0.1.0. Issuing the token from a Read-Only Admin account means a leaked token cannot be used to mutate your tenant. PRD §7.6 / domain §4 cover the rationale.

### Step 2 — Set environment variables

```bash
export OKTA_ORG_URL="https://dev-NNNNNN.okta.com"   # or your custom domain
export OKTA_API_TOKEN="<paste the token from step 1>"
```

> Both variables are required. `OKTA_ORG_URL` accepts `<org>.okta.com`, `<org>.oktapreview.com`, and custom domains.

### Step 3 — Run

```bash
./ota
```

You should land on the Users list (`SCR-010`). Press `?` for help, `:` for the command palette, `q` to quit.

---

## Configuration file

ota looks for a YAML config at `$XDG_CONFIG_HOME/ota/config.yaml` (defaults to `~/.config/ota/config.yaml`). The file is optional — environment variables alone are enough for a single-tenant setup.

### Example `~/.config/ota/config.yaml`

```yaml
profiles:
  dev:
    org_url: "https://dev-123456.okta.com"
    api_token_env: "OKTA_API_TOKEN"        # name of the env var holding the token
    default_log_filter: ""
  prod:
    org_url: "https://acme.okta.com"
    api_token_env: "OKTA_PROD_API_TOKEN"

ui:
  theme: "dark"                            # dark | high_contrast | monochrome
  pii_masking:
    enabled: true                          # phone/email masked by default
    default_unmask_on_copy: false
    logs_actor_email: false                # toggle on for stricter compliance

keybindings:                               # override any keys.go ID
  nav.down: "j"
  nav.up: "k"
  app.quit: "q"
  search.open: "/"

logs:
  poll_interval_seconds: 7                 # tail interval (5–10 recommended)

debug: false                               # writes ~/.cache/ota/debug.log when true
```

Pick a profile at startup with `--profile <name>`. Keybindings under `keybindings:` use the IDs defined in `internal/keys/keys.go` (`nav.down`, `app.quit`, etc.). See [`docs/CONVENTIONS.md` §7](docs/CONVENTIONS.md) and [`docs/TUI_DESIGN.md` §3](docs/TUI_DESIGN.md) for the full catalogue.

---

## Keyboard cheat sheet

ota's defaults are k9s-compatible with Vim navigation — press `?` in any screen for the live, context-aware list.

### Global

| Key | Action |
|---|---|
| `:` | Open command palette |
| `/` | Incremental search (lists only) |
| `?` | Help modal |
| `Esc` | Cancel current mode/modal |
| `q` | Close current screen / app quit (with confirm) |
| `Ctrl-c` | Soft quit; double-tap to force-exit |
| `Ctrl-l` | Force redraw (after tmux resize) |

### Navigation

| Key | Action |
|---|---|
| `j` `k` (or `↓` `↑`) | Down / up one row |
| `h` `l` (or `←` `→`) | Tab / column left / right |
| `gg` | Top of list |
| `G` | Bottom of list |
| `Ctrl-d` `Ctrl-u` | Half-page down / up |
| `Ctrl-f` `Ctrl-b` | Full-page down / up |
| `Enter` | Open detail / drill down |
| `Tab` `Shift-Tab` | Next / previous tab |

### Observe & yank

| Key | Action |
|---|---|
| `R` | Refresh current resource (invalidate cache) |
| `r` | Toggle rich ↔ raw JSON (Policies / Logs detail) |
| `y` / `yy` / `yf` | Copy to clipboard: selection / row / focused field |
| `o` | Open Admin Console link in browser |
| `e` | Expand / collapse detail (e.g. Factor IDs) |
| `s` | Logs: toggle tail mode |
| `f` | Logs: toggle auto-follow |
| `n` `N` | Next / previous search match |

### Command palette

| Command | Effect |
|---|---|
| `:users` `:u` | Users list |
| `:groups` `:g` | Groups list |
| `:grouprules` `:gr` | Group Rules list |
| `:policies [TYPE]` | Policy type selector (or jump straight to e.g. `OKTA_SIGN_ON`) |
| `:logs` `:l` | Logs search / tail |
| `:search <SCIM-expr>` | Server-side search (Users/Groups). Note: Users `search` is *eventually consistent* — newly-created users may take minutes to appear. |
| `:filter <SCIM-expr>` | Server-side filter (Groups/Apps/Logs) |
| `:unmask <field>` | Reveal masked PII for the current session only |
| `:mask` | Re-mask everything in the session |
| `:raw` | Toggle raw JSON view in detail screens |
| `:refresh` | Drop cache and reload |
| `:about` | App/token/rate-limit summary |
| `:errors` | Session error history |
| `:debug open` | Print debug log path |
| `:help` `:?` | Help modal |
| `:quit` `:q` | Quit |

Full key map and screen catalogue: [`docs/TUI_DESIGN.md` §3 & §4](docs/TUI_DESIGN.md).

---

## Supported resources (v0.1.0, read-only)

| Resource | List | Detail | Notes |
|---|---|---|---|
| **Users** | ✅ | ✅ + 6 tabs | Profile (split into fixed + custom fields), Credentials, Timestamps, Groups, **Factors** (PII masked by default), Recent activity |
| **Groups** | ✅ | ✅ + 4 tabs | OKTA_GROUP / APP_GROUP / BUILT_IN icons, `RULE` / `SYS` / `LARGE` badges. Members tab uses progressive load with `Esc` to stop |
| **Group Rules** | ✅ | ✅ | `ACTIVE` / `INACTIVE` / **`INVALID`** colour-coded; INVALID counter banner; expression rendered monospace |
| **Policies** | ✅ (per type) | ✅ | All 7 types listed; rich render for `OKTA_SIGN_ON` / `ACCESS_POLICY` / `PASSWORD` / `MFA_ENROLL`; raw-JSON-only for `PROFILE_ENROLLMENT` / `POST_AUTH_SESSION` / `IDP_DISCOVERY` |
| **System Logs** | ✅ + tail | ✅ | Adaptive 7s polling (auto-stretches to 15s on low-quota tenants), hole-free resume after 429, 5 built-in filter presets including `Group Rule Deactivations` (highlighted) |

**Authentication:** Okta SSWS API tokens via env vars (and optional `api_token_env` profile mapping). Tokens are never written to disk and are scrubbed from panic stack traces and debug logs (PRD §6.2 / REQ-C05).

---

## Known limitations (v0.1.0)

These are the explicit gaps tracked in [PRD §11.3.1](docs/PRD.md):

- **Token input is environment-only.** Interactive `--prompt` token entry will arrive in v0.2 (QA-005).
- **`:profile` runtime switch is not implemented.** Pick a profile at start with `--profile <name>` and re-launch to switch (QA-009).
- **`:ratelimit` and `:healthcheck` modals are partial / missing.** The `[RL]` badge in the header may also be absent in some builds — being added during v0.1.x patches (QA-013, QA-016).
- **Config file permissions are not validated.** ota does not store tokens, but a `0600` permissions check (warn-only) is queued for v0.1.x (QA-012).
- **Rendering for `PROFILE_ENROLLMENT` / `POST_AUTH_SESSION` / `IDP_DISCOVERY` policies is raw JSON only.** Press `r` for the JSON view; rich renderers are tracked for v0.2.

---

## Roadmap

**v0.1.x (patches):**
- QA-012 config file `0600` permission warning
- QA-013 rate-limit numeric display + `:ratelimit` modal

**v0.2 (Q3 2026 target):**
- Apps resource (list, detail, User → Apps tab)
- Interactive token prompt (QA-005)
- Runtime `:profile` switch (QA-009)
- HealthPort production implementation for `:healthcheck` (QA-016)
- **First Write actions (domain risk ascending):**
  1. Group static member add / remove
  2. User lifecycle: `unlock`, `unsuspend`, `activate`, `deactivate`
  3. Group Rule activate / deactivate (with double-confirm + impact estimate)
- Rich renderers for the remaining 3 policy types
- OAuth 2.0 Service App (Private Key JWT) authentication

**v0.3+:** Custom views (DSL), Event Hook based pseudo-streaming, shareable URI scheme.

---

## Troubleshooting

- **`E0000004` / `401 — API token invalid or revoked`** → rotate the token in the Admin Console (Security → API → Tokens).
- **`E0000006` / `403 — Insufficient permissions`** → token role lacks read scope for that resource. Check `:about`.
- **`E0000007` / `404`** → resource was likely deleted by another admin; press `R` to refresh.
- **`429 — Rate limited`** → ota auto-pauses tail polling and resumes from the last `published` timestamp; no data is lost (PRD REQ-E01 AC-3).
- **Garbled rendering after `tmux resize`** → press `Ctrl-l` to force a full redraw.
- **Detailed error history** → run `:errors` inside the app, or inspect `~/.cache/ota/debug.log` (enable with `--debug`).

---

## Contributing & issues

- Source: <https://github.com/tedilabs/ota>
- Bug reports / feature requests: <https://github.com/tedilabs/ota/issues>
- Conventions, architecture, and testing rules: see [`docs/`](docs/).

When filing bugs, please include:
- ota version (`./ota --version`)
- Okta tenant type (Developer Free / Production / Preview)
- Steps to reproduce + expected vs observed
- Relevant lines from `~/.cache/ota/debug.log` (tokens are auto-redacted)

---

## License

Apache License 2.0 — see [`LICENSE`](LICENSE).

---

## Acknowledgements

- [k9s](https://k9scli.io/) for popularising the resource-navigation TUI pattern this project mimics.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss) — the Charm stack that makes ota's UI possible.
- The Okta developer documentation, especially the [Core API](https://developer.okta.com/docs/reference/core-okta-api/) and [System Log](https://developer.okta.com/docs/reference/api/system-log/) references.
