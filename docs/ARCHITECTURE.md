# ota Architecture

**Version:** v1.0.0
**Status:** Final (Phase 4)
**Last updated:** 2026-04-24
**Authors:** developer (lead), test-engineer (review)
**Scope:** ota v0.1.0 MVP (Read-Only)

---

## 변경 이력

| 버전 | 날짜 | 변경 | 작성자 |
|------|------|------|-------|
| v0.1.0-draft | 2026-04-24 | 초안 (Phase 4) | developer |
| v1.0.0 | 2026-04-24 | D-A 합의 반영: Port 위치 `internal/domain/ports.go` 확정. depguard 린트 규칙 명시. TESTING v0.9.1 정합성 확보. | developer |
| v1.0.1 | 2026-04-24 | Phase 6 결정 반영: MVP는 직접 `net/http`, SDK는 v0.2+ 옵션 (§6.5). team-lead 승인. | developer |

---

## 1. 시스템 개요

**ota** (Okta TUI)는 운영자가 Okta Workforce Identity 테넌트를 **터미널에서 읽기 전용으로** 탐색하도록 돕는 단일 바이너리 CLI 애플리케이션이다. k9s의 UX(리소스 리스트 → 상세 → 드릴다운, `:` 커맨드, `/` 검색, Vim 단축키)를 Okta 5개 리소스(Users, Groups, Group Rules, Policies, System Logs)에 적용한다.

핵심 특징:

- **단일 실행 파일**: 외부 데몬·서버 없음. 설정 파일 + 환경변수만으로 동작.
- **Read-Only**: v0.1 MVP는 모든 mutation 호출 금지. 쓰기 경로 자체가 코드에 없다.
- **도메인 플러그인 가능성**: Okta 외 IdP(Entra, JumpCloud 등)를 향후 대체 가능하도록 Port/Adapter로 격리.

근거: PRD §1, §4.

---

## 2. 설계 목표 및 비목표

### 2.1. 목표

| 목표 | 동인 (REQ/근거) |
|------|----------------|
| **테스트 가능성** | 도메인 규칙·어댑터·TUI를 각기 독립 테스트. `teatest`로 화면 전환까지 검증. |
| **유지보수성** | 작은 패키지, 명시적 의존성 방향. 새 리소스 추가가 한 디렉토리 안에서 완결. |
| **낮은 지연** | 키 입력 렌더 < 16ms, 리스트 초기 렌더 < 500ms — PRD §6.1. |
| **보안** | 토큰·PII 누출 방지가 아키텍처 수준에서 강제 — PRD §6.2, REQ-C05. |
| **확장성** | IdP 도메인 교체, 새 Okta 리소스 추가 비용이 선형. PRD §12. |
| **관측성** | 상관 ID 기반 디버그 로그, rate limit 상태의 런타임 가시성 — REQ-O01, REQ-E01. |

### 2.2. 비목표 (명시적 배제)

- Write / mutation (v0.2+)
- 멀티 창 / split pane (터미널 단일 뷰)
- 백그라운드 데몬 / 상주 서비스
- 다중 tenant 동시 조회 (동시 하나의 활성 프로필)
- 모바일 · 웹 UI
- 플러그인 핫리로드 (v0.3+)
- i18n (영어만, PRD §6.4)

---

## 3. 아키텍처 패턴

### 3.1. 왜 Hexagonal인가

선택: **Hexagonal (Ports & Adapters)** + Elm Architecture(Bubbletea) 위에 얹음.

**근거:**

1. **Okta SDK/HTTP는 밖으로 밀어낸다.** 도메인 규칙(예: "Group Rule의 INVALID 상태는 경고")이 SDK 타입에 얽매이면 SDK 버전 업그레이드마다 도메인 테스트가 흔들린다.
2. **TUI도 Adapter로 본다.** `tea.Model`은 inbound adapter(사람의 입력을 도메인으로 변환). Okta SDK는 outbound adapter(도메인 요청을 외부로 변환).
3. **확장성.** `EntraPort`, `JumpCloudPort` 구현 추가만으로 도메인이 재사용 가능.
4. **테스트 가능성.** 각 어댑터는 인터페이스(Port) 구현이므로 상위 레이어 테스트는 순수 fake로 가능.

### 3.2. 고려했으나 채택하지 않은 대안

| 대안 | 탈락 이유 |
|------|---------|
| **Layered (3-tier)** | UI가 직접 도메인과 외부 시스템을 호출하게 되어 Okta SDK 타입이 UI로 누출됨. |
| **Clean Architecture (uncle Bob)** | Hex과 사실상 동형이나 레이어 개수가 많고 "use case" 인터페이스 폭발. 프로젝트 규모 대비 오버엔지니어링. |
| **DDD Tactical Patterns (Aggregate/Repository 풀 세트)** | ota는 읽기 전용 + CRUD 없음 → Aggregate/Transaction 개념이 과잉. Repository 패턴만 차용. |
| **Bubbletea만 순수 MVC** | 화면마다 Okta SDK 호출이 흩어짐 → 테스트 · 재사용 난항. |

---

## 4. 레이어 구조

```
┌─────────────────────────────────────────────────────────────────┐
│ cmd/ota                                                         │
│   flag 파싱, config 로드, 의존성 조립, tea.Program 실행            │
└───────────────────────────────┬─────────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────────┐
│ internal/app  (inbound adapter — TUI shell)                    │
│   App Shell Model (Router), 전역 단축키, overlay 관리             │
└───────────────────────────────┬─────────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────────┐
│ internal/tui/<resource>  (inbound adapter — Screen Models)     │
│   UsersListModel, UserDetailModel, PoliciesTypeSelectModel, ... │
│   tea.Model composition · Msg·Cmd 기반 상태 전이                  │
└───────────────────────────────┬─────────────────────────────────┘
                                │ Service API (use cases)
┌───────────────────────────────▼─────────────────────────────────┐
│ internal/service                                                │
│   UsersService, GroupsService, RulesService,                   │
│   PoliciesService, LogsService (tail iterator)                 │
└──────┬─────────────────────────────────────────────────┬────────┘
       │                                                  │
       │ requires (Port interfaces)                       │ uses
       ▼                                                  ▼
┌──────────────────────────┐              ┌─────────────────────────┐
│ internal/domain          │              │ internal/config         │
│ (pure)                   │              │ (Config loader, profile)│
│  User, Group, Rule,      │              └─────────────────────────┘
│  Policy, LogEvent, Err*, │
│  Iterator[T], PageInfo   │
└──────────────────────────┘
       ▲
       │ implements Port
       │
┌──────┴───────────────────────────────────────────────────────────┐
│ internal/okta  (outbound adapter)                                │
│  Client, UsersAdapter, GroupsAdapter, ..., RateLimitMonitor,    │
│  Pagination iterator, errorCode → domain.Err 매퍼                │
└──────────────────────────────────────────────────────────────────┘
                                │ okta-sdk-golang/v5 + net/http
                                ▼
                        ┌───────────────┐
                        │ Okta Core API │
                        └───────────────┘
```

### 4.1. 레이어 책임 요약

| 레이어 | 단일 책임 | 금지 |
|--------|---------|------|
| `cmd/ota` | 부팅 및 조립 (wiring) | 비즈니스 로직 |
| `internal/app` | 화면 라우팅·오버레이·전역 단축키 | 도메인 로직 · 직접 HTTP |
| `internal/tui/<r>` | 리소스별 Screen Model·Msg 처리·뷰 렌더 | Okta SDK import · 직접 HTTP |
| `internal/service` | 유스케이스(여러 Port 조합·캐시·정책) | tea.* 참조 · SDK import |
| `internal/domain` | 순수 타입·불변식·필터 매처 | 모든 외부 import (stdlib 외) |
| `internal/okta` | SDK 호출·페이지네이션·rate limit·에러 매핑 | 도메인 규칙 |
| `internal/config` | YAML 로드·경로 해결·검증 | 런타임 상태 |

---

## 5. 의존성 방향

```
cmd → app → tui/* → service ──┐
                               │ domain.*Port 인터페이스
                   domain ◄────┤◄── okta (implements Port)
                               │
               config ◄──── cmd (loads), app (reads)
               keys, mask, logger, clock ◄── 횡단 유틸 (upward 허용)
```

**규칙:**

1. `internal/domain`은 **표준 라이브러리 외 어떤 것도 import 금지**. Port 인터페이스·쿼리 타입도 여기 선언.
2. `internal/tui/*`는 `internal/service`, `internal/domain`, `internal/keys`, `internal/mask`만 import.
3. `internal/service`는 `internal/domain`만 (외부 의존 없음). SDK import 금지.
4. `internal/okta`는 `internal/domain` + SDK. `domain.*Port`를 구현하지만 Service/TUI는 몰라도 됨.
5. 순환 의존 금지 — 컴파일러가 잡음.

**Go 관용구 준수:** "Accept interfaces, return structs". 인터페이스는 구현체가 아닌 **값 객체·도메인 경계 패키지**에 둔다 (Service·TUI 공통 소비). 어셈블리(wiring)는 `cmd/ota/wire.go`에서 명시적으로.

**린트 강제 (depguard):**
- `internal/app/**`, `internal/tui/**`, `internal/service/**`: `github.com/okta/okta-sdk-golang/**` import 금지.
- `internal/domain/**`: stdlib 외 import 금지.

---

## 6. 핵심 컴포넌트

### 6.1. `internal/domain` — 순수 도메인

**책임:** Okta 리소스 5종의 **ota-내부 표현**과 불변식.

**주요 타입:**

```go
type User struct {
    ID            string        // 00uXXXXXX
    Login         string        // profile.login (email)
    Status        UserStatus    // ACTIVE, SUSPENDED, ...
    Profile       UserProfile   // displayName, email, mobilePhone, custom...
    LastLogin     *time.Time
    StatusChanged *time.Time
    // ...
}

type UserStatus string // 상수: STAGED, PROVISIONED, ACTIVE, SUSPENDED,
                       //       LOCKED_OUT, PASSWORD_EXPIRED, DEPROVISIONED

type Group struct {...}
type Rule struct {...}
type Policy struct {...}
type LogEvent struct {...}

type Iterator[T any] interface {
    Next(ctx context.Context) (T, bool, error) // (item, hasMore, err)
}

type PageInfo struct {
    Cursor string
    Limit  int
}

var (
    ErrNotFound       = errors.New("not found")
    ErrForbidden      = errors.New("forbidden")
    ErrRateLimited    = errors.New("rate limited")
    ErrTokenInvalid   = errors.New("token invalid")
    ErrBadRequest     = errors.New("bad request")
    ErrOktaServer     = errors.New("okta server error")
    ErrNetwork        = errors.New("network error")
    ErrFeatureDisabled = errors.New("feature disabled")
)
```

**핵심 원칙:**

- 불변식은 type invariant로 강제 (생성자만 공개, 필드는 검증 후 채움).
- 어떤 SDK 타입도 여기 없다. 매핑은 `internal/okta/*`가 담당.
- 파일 I/O·네트워크·시간·난수 직접 사용 금지. 필요하면 인터페이스로.

**근거:** PRD §7.7 에러 매핑, REQ-R01~R05의 필드 명세.

### 6.2. `internal/service` — 유스케이스

**책임:** 도메인을 조합한 운영자 관점 유스케이스. `internal/domain/ports.go`에 선언된 Port 인터페이스를 **소비**한다.

**Port 위치 결정 (test-engineer와 합의, 2026-04-24):**
- `internal/domain/ports.go`에 모든 Port 인터페이스 선언.
- 근거: Service와 TUI 둘 다 Port를 참조(소비자가 여러 레이어). 중립 위치 필요.
- Adapter(`internal/okta`)는 이미 `domain` 타입을 매핑하므로 Port 선언이 같이 있어도 순환 의존 없음.
- `depguard` 린트로 `app/tui/service`에서 SDK 직접 import 차단이 단순해짐.

**주요 인터페이스 (Port):**

```go
// 중립 위치 — internal/domain/ports.go
package domain

type UsersPort interface {
    List(ctx context.Context, q UsersQuery) (Iterator[User], error)
    Get(ctx context.Context, idOrLogin string) (User, error)
    ListGroups(ctx context.Context, userID string) ([]Group, error)
    ListFactors(ctx context.Context, userID string) ([]Factor, error)
}

type GroupsPort interface { ... }
type RulesPort interface { ... }
type PoliciesPort interface {
    List(ctx context.Context, policyType PolicyType, pi PageInfo) (Iterator[Policy], error)
    Get(ctx context.Context, id string) (Policy, error)
    Rules(ctx context.Context, policyID string) ([]PolicyRule, error)
}
type LogsPort interface {
    Search(ctx context.Context, q LogsQuery) (Iterator[LogEvent], error)
}
```

쿼리 타입(`UsersQuery`, `LogsQuery` 등)도 domain에 둔다 — Port가 사용하는 값 객체이므로 같은 경계 안.

**주요 서비스:**

```go
type UsersService struct {
    port  domain.UsersPort
    cache *ttl.Cache
    clock clock.Clock
    log   *slog.Logger
}

func NewUsersService(p domain.UsersPort, opts ...Option) *UsersService { ... }

func (s *UsersService) Search(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
    // 1) 쿼리 정규화 ( /u -> q="u" 등)
    // 2) 캐시 조회 (30s TTL, REQ-E01 AC-6)
    // 3) port.List 호출
    // 4) cache + iterator wrap
}
```

**원칙:**

- Service는 **`domain.*Port` 인터페이스만** 참조. 구현체는 `cmd/ota/wire.go`에서 주입.
- 캐시는 Service 책임 (어댑터가 아님). 캐시 무효화 API 제공 (`:refresh`, `:profile` 전환).
- 로깅: Service 수준의 "유스케이스 경계" 로그(`user.list.started`, `user.list.cache_hit`). 구조화 필드.

**근거:** PRD REQ-E01 AC-6, REQ-U04, REQ-R01~R05.

### 6.3. `internal/app` — App Shell (Router)

**책임:** 최상위 `tea.Model`. 화면 간 전환·오버레이·전역 단축키·커맨드 팔레트.

**구조 (TUI_DESIGN §5.1 기반):**

```go
type Model struct {
    active     screen.ID      // users_list, user_detail, ...
    screens    map[screen.ID]tea.Model // lazy 생성
    overlay    overlay.Model  // cmd palette / help / confirm / errors / about
    statusBar  statusbar.Model
    profile    string         // active profile name
    rateLimits ratelimit.Snapshot // 최신 관찰값
    errBuf     errorBuffer    // :errors
    cancel     func()         // 현재 Cmd 취소용
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // 1) 전역 키: :/?Esc Ctrl-c → overlay or quit confirm
    // 2) RateLimitObservedMsg → m.rateLimits 갱신
    // 3) ProfileSwitchedMsg → 전체 스크린 리셋 + 캐시 무효화
    // 4) 그 외 → active screen Update
}
```

**오버레이 합성:**

오버레이는 active screen **위**에 그려지고, 키 입력을 가로챈다. 단 `Esc`는 항상 오버레이 dismiss로 우선 처리.

**근거:** TUI_DESIGN §5.1, §2.2, REQ-U01~U07.

### 6.4. `internal/tui/<resource>` — Screen Models

**구조 예 (users):**

```go
// internal/tui/users/list.go
package users

type ListModel struct {
    svc     *service.UsersService  // 서비스를 직접 주입. 테스트는 domain.UsersPort fake를 서비스에 주입.
    table   table.Model        // bubbles/table
    filter  filter.Model       // inline / prompt
    search  string             // :search expr
    loading bool
    items   []domain.User
    page    pageCursor
    err     error
    keys    keys.ResolvedMap
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKey(msg)
    case UsersLoadedMsg:
        m.items = append(m.items, msg.Users...)
        m.page = msg.Next
        m.loading = false
        return m, nil
    case ErrorMsg:
        m.err = msg.Err
        m.loading = false
        return m, nil
    }
}
```

**Msg 계약:**

각 Screen Model은 **자기 화면에서 발생하는 Msg 타입**을 선언하되, 전역 Msg(RateLimit, Error, Toast)는 `internal/app/msg.go`에 공통 정의.

### 6.5. `internal/okta` — Outbound Adapter

**책임:**

1. Okta HTTP API 호출 (MVP: 직접 `net/http`; SDK는 v0.2+ 옵션)
2. 도메인 타입 매핑 (`wireUser` → `domain.User`)
3. Link 헤더 기반 페이지네이션을 `domain.Iterator[T]`로 제공
4. Rate Limit 헤더 관찰 → `ratelimit.Monitor`로 전달
5. Okta `errorCode` → `domain.Err*` 매핑 (PRD §7.7)

**MVP 구현 메모 (Phase 6, 2026-04-24):** MVP는 **`net/http` 직접**. `okta-sdk-golang/v5`는 `go.mod`에만 남아있으며 런타임에서는 사용하지 않는다. 이유는 SDK의 host 주입 방식이 ota의 httptest 시나리오 드라이버와 잘 맞지 않아서. 얇은 Client 래퍼(`internal/okta/client.go`)가 SSWS 인증·429 Retry-After 재시도·rate limit 관찰·errormap 통합을 한 곳에서 수행하므로 Port 경계는 동일하게 유지된다. SDK는 v0.2에서 per-endpoint 선택적 전환 대상 (오픈 이슈).

**구성:**

```go
type Client struct {
    baseURL    string
    token      string
    http       *http.Client           // 주입 가능 (httptest 바인딩)
    monitor    *ratelimit.Monitor
    log        *slog.Logger
    clock      clock.Clock
    maxRetries int                    // REQ-E01 AC-2: 기본 3
}

func NewClient(ctx context.Context, cfg Config, opts ...Option) (*Client, error) { ... }

type UsersAdapter struct { client *Client }

// implements domain.UsersPort
func (a *UsersAdapter) List(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
    // 1) q → query string (Limit, Q, Search, Filter, After)
    // 2) doGet → 429 자동 재시도 + rate limit 헤더 관찰 + errormap
    // 3) pagedIterator[T] 반환 (Next는 Link 헤더 next 자동 follow)
}
```

**System Logs 예외:** `logs.debugContext.debugData` 등 free-form 필드는 `json.RawMessage`로 보존하여 도메인 타입에 그대로 전달.

**근거:** 도메인 §8.3, §8.4, PRD §11.4, Phase 6 구현 보고서.

### 6.6. `internal/config` — 설정 로드

**책임:** YAML 파싱, XDG 경로 해결, 프로필 스위칭, 검증.

**구조:**

```go
type Config struct {
    Profiles     map[string]Profile `koanf:"profiles"`
    UI           UIConfig           `koanf:"ui"`
    Keybindings  map[string]string  `koanf:"keybindings"` // key_id -> keybind str
    Logs         LogsConfig         `koanf:"logs"`
    Debug        bool               `koanf:"debug"`
}

type Profile struct {
    OrgURL           string `koanf:"org_url"`
    APITokenEnv      string `koanf:"api_token_env"`
    DefaultLogFilter string `koanf:"default_log_filter"`
}
```

**로드 순서 (REQ-C04):**

1. `--config <path>` flag
2. `$XDG_CONFIG_HOME/ota/config.yaml`
3. `~/.config/ota/config.yaml`
4. default (메모리)

**토큰 해결 순서 (REQ-C04):**

1. `--token-env=<VAR>` / profile.api_token_env
2. `OKTA_API_TOKEN` + `OKTA_ORG_URL`
3. 대화식 프롬프트 (마스킹, 메모리 only)

### 6.7. 횡단 유틸

| 패키지 | 책임 |
|--------|------|
| `internal/keys` | Key ID(`nav.down` 등) 상수, 기본 맵, 사용자 override resolver. |
| `internal/mask` | PII 마스킹 (`mask.Phone`, `mask.Email`). 로거·뷰 공통 사용. |
| `internal/logger` | slog 설정. file handler + masking middleware. 상관ID(session_id). |
| `internal/clock` | `Clock` 인터페이스 + `realClock` / `FakeClock`. tail 폴링·토큰 수명 추정 테스트용. |
| `internal/version` | ldflags 주입 값. `:about` 노출. |

---

## 7. 데이터 흐름

### 7.1. 리스트 초기 로드 (Users 예)

```
[User] j↓ on UsersListModel
   │
   ▼
Screen.Update(tea.KeyMsg{j}) ── nop (moves cursor) ──► view re-render
                                                             │
[User] /{query}{Enter} ────────► Screen.Update(SearchMsg{q})
                                                             │
                                     returns tea.Cmd{fetchUsers(ctx, svc, q)}
                                                             │
┌────────────────────── tea.Cmd goroutine ─────────────────────┐
│ svc.Search(ctx, q)                                           │
│   └► cache miss                                              │
│       └► port.List(ctx, q)                                   │
│           └► UsersAdapter.List                               │
│               └► sdk.UserAPI.ListUsers(ctx).Q(q).Execute()   │
│                   └► HTTP GET /api/v1/users?q=...            │
│                   └► parse Link header → pageCursor          │
│                   └► RateLimitMonitor.Observe(resp.Header)   │
│                   └► err := mapOktaErr(apiErr) || nil        │
│               └► domain.Iterator[User] 반환                  │
│           └► domain.Iterator wrap (cache on Next)            │
│   └► iterator 반환                                           │
│ return UsersLoadedMsg{iter, page}  or ErrorMsg{err}          │
└──────────────────────────────────────────────────────────────┘
   │
   ▼
Screen.Update(UsersLoadedMsg) ──► 상태 갱신 + view
```

### 7.2. Tail 폴링 (Logs)

```
User presses f (enter tail)
   │
   ▼
LogsListModel.Update(ToggleTailMsg) → Cmd{tea.Tick(7s)}
                                       │
                                       ▼
Tick → Cmd{fetchLogs(ctx, svc, LogsQuery{since: lastPublished+1ms})}
                                       │
                                       ▼
Msg{LogsPolledMsg{events, newSince}} → Append + re-tick
                                       │
                429 ERRoutcome        │
                 ▼                     │
RateLimitErrorMsg → Pause (UI badge) → tea.Tick(Retry-After+jitter)
                                       │
                   recover → resume with same `since` (hole-free, REQ-E01 AC-3)
```

### 7.3. 프로필 전환

```
User :profile prod
   │
   ▼
CmdPalette.Update → CmdExecutedMsg{"profile", "prod"}
                         │
                         ▼
App.Model.Update → ProfileSwitchStartedMsg
   └► 즉시 토스트: "Switching to prod… (invalidating cache)"
   └► Cmd{reinit(ctx, "prod")}
         │
         ▼
   - services.reset()       // 캐시 clear
   - okta.Client.reinit()   // 새 token + org_url
   - screens 재생성
   - ProfileSwitchedMsg → App 갱신, 토스트 "Switched to prod"
```

---

## 8. 동시성 모델

### 8.1. Goroutine 생성 규칙

- **tea.Cmd 내부 only.** Screen Model은 goroutine을 직접 spawn 하지 않는다.
- tea.Cmd는 호출 시점에 1회 실행되어 Msg를 돌려준다. 장기 실행(tail)은 `tea.Tick` + 재귀 Cmd로 표현.
- 예외: Okta Adapter 내부에서 병렬 페치가 필요한 경우(없는 것을 권장, 순차) — Okta는 병렬 페이지 요청 금지([확정], 도메인 §2.1).

### 8.2. 공유 상태

- **Screen Model의 상태는 그 Model이 owning.** 외부는 Msg로만 접근.
- 전역 상태(Profile, RateLimits, ErrorBuffer)는 App.Model이 owning. Screen에 읽기 전용 스냅샷을 Msg로 전파.
- `sync.Mutex`는 원칙적으로 사용하지 않는다 (Elm arch가 직렬화). 예외 발생 시 **코드 주석으로 이유 명시**.

### 8.3. Context 전파

- 모든 외부 I/O Cmd는 `context.Context`를 받는다.
- Esc 키: App.Model이 `cancel()` 호출 → 진행 중 Cmd가 cancelled. 타임아웃 + 취소 둘 다 처리.
- Profile 전환 시 이전 context 전체 cancel.

### 8.4. Tail 폴링 안정성

- tail Cmd는 각 tick에서 **새로운 context**를 얻음. 이전 tick이 남아있으면 취소.
- 429 시 `tea.Tick(Retry-After)`로 대기. 이 기간 타이머 충돌 없음.
- 리사이즈 시에도 tail은 영향 없음(별도 lane).

**근거:** PRD §11.4 기술 검증 필요 항목.

---

## 9. 에러 처리 전략

### 9.1. 레이어별 역할

```
Okta SDK error ─► okta.mapErr(err) → domain.Err*
                                  │
                                  ▼
                          service가 받아서 처리 여부 결정
                                  │
           ┌──────────────────────┼──────────────────────┐
           ▼                      ▼                      ▼
    retry (429,네트워크)    도메인 에러 반환       wrap with context
           │                      │                      │
           └──────────────────────┴──────────────────────┘
                                  │
                                  ▼
                           tui: ErrorMsg → statusbar 토스트 + :errors 로그
                                  (message = Okta errorCode 기반 문구, PRD §7.7)
```

### 9.2. 매핑 테이블 (PRD §7.7)

| Okta errorCode | HTTP | domain.Err | 사용자 메시지 |
|----------------|------|-----------|--------------|
| E0000001 | 400 | ErrBadRequest | errorCauses 파싱해서 필드별 표시 |
| E0000004 | 401 | ErrTokenInvalid | "API token invalid or revoked. Rotate and retry." |
| E0000006 | 403 | ErrForbidden | "Insufficient permissions for <resource>" |
| E0000007 | 404 | ErrNotFound | "Resource not found. Refreshing list…" |
| E0000011 | 401 | ErrTokenInvalid | "Token expired or revoked" |
| E0000022 | 400 | ErrBadRequest | "Deactivate before deleting" (읽기 전용 — 정보성) |
| E0000038 | 400 | ErrFeatureDisabled | "This feature is disabled for your organization." |
| E0000047 | 429 | ErrRateLimited | 자동 재시도 (REQ-E01) |
| (그 외 5xx) | | ErrOktaServer | "Okta server error. Retrying…" |

### 9.3. 비-재시도 / 재시도

| 상황 | 처리 |
|------|------|
| 429 rate limited | 최대 3회 자동 재시도, Retry-After + jitter (REQ-E01 AC-2) |
| 5xx 서버 에러 | idempotent GET만 지수 백오프 3회 |
| 네트워크 단절 | 폴링 중단, statusbar "offline", 복구 감지 시 재개 (REQ-E03) |
| 401/403 | 재시도 없음. 사용자에게 명확 안내. |
| 404 | 리스트 refresh 트리거 |

### 9.4. 패닉 처리

- `cmd/ota/main.go`가 `defer recover()` 마지막 안전망. 크래시 로그에 상관ID + 버전.
- Bubbletea 프로그램이 panic 시 터미널 상태 복원 후 친절한 메시지 출력.

---

## 10. 설정·인증 흐름

### 10.1. 부팅

```
main()
  ├─ flag.Parse → --config, --profile, --token-env, --debug, --poll-interval
  ├─ config.Load(flagPath)
  │    ├─ XDG 경로 resolve
  │    ├─ koanf 로드 + 병합 (default → file → env)
  │    └─ Validate (profile 이름, keybinding 유효성)
  ├─ profile := pickProfile(flag, config)
  ├─ token, src := resolveToken(flag, profile, env, prompt)
  ├─ logger := logger.New(config.Debug, sessionID)
  ├─ clk := clock.Real{}
  ├─ oktaClient := okta.NewClient(okta.Config{OrgURL: profile.OrgURL, Token: token}, ...)
  ├─ svcs := service.Bundle{
  │     Users: service.NewUsersService(oktaClient.Users(), ...)
  │     Groups: ...
  │     ...
  │   }
  ├─ app := app.New(svcs, config, keys, clk, logger)
  └─ tea.NewProgram(app, opts...).Run()
```

**토큰 결정 노출:** `:about` 화면에 `token source: env OKTA_API_TOKEN` 같이 노출 (REQ-C04 AC-1).

### 10.2. 프로필 스위칭

- `:profile <name>`이 실행되면 전체 re-init (6.3 섹션 참조).
- < 2s 타겟 (REQ-C02 AC-3). 리셋 중에는 로딩 스피너 + 상태 메시지.

---

## 11. Rate Limit 전략

### 11.1. 관찰 → 전파

- `internal/okta/ratelimit.go`의 `RateLimitMonitor`가 모든 응답 헤더에서 `X-Rate-Limit-Limit/Remaining/Reset`를 읽어 **카테고리별 last-observed**로 보관.
- 카테고리: `management` (users/groups/rules), `logs`, `policies`, `apps` (v0.2).
- Monitor가 관찰 직후 `RateLimitObservedMsg`를 보낼 방법:
  - Screen Model이 Monitor의 채널을 구독하는 대신, **Monitor는 단순 메모리 저장소**이고 각 Cmd가 응답 후 snapshot을 Msg에 포함해 반환.
  - `statusbar.Model`이 메시지 수신 시 뱃지 렌더 (`:about` 상세 보기 제공).

### 11.2. 429 처리 (REQ-E01 AC-2)

- Okta 응답의 `Retry-After`(초 또는 HTTP-date) 준수.
- `wait = retryAfter ± 20% jitter`.
- 최대 3회 재시도. 실패 시 `ErrRateLimited` → statusbar 빨간 에러.

### 11.3. Tail 적응 (REQ-R05 AC-2)

- 첫 응답에서 `X-Rate-Limit-Limit < 60` 관찰 시 LogsService가 tail 주기를 7s → 15s 상향.
- 변경 사실은 1회성 토스트 + `:about`의 "Polling interval: 15s (adaptive, observed limit 50/min)" 표시.

### 11.4. 캐시 (REQ-E01 AC-6)

- UsersService/GroupsService/PoliciesService: 30s TTL 메모리 캐시.
- `:refresh` or `R` → 현재 화면 캐시 무효화.
- `:profile` 전환 → 전체 캐시 무효화.

---

## 12. 관측성

### 12.1. 로깅

- `log/slog` JSON handler, file sink `~/.cache/ota/debug.log` (REQ-O01).
- 모든 이벤트에 `session_id`(UUID) 필드.
- 레벨: ERROR/WARN/INFO/DEBUG. 기본 INFO. `--debug`로 DEBUG.
- **민감 필드 마스킹** middleware: `authorization` 헤더·`api_token`·`secondEmail`·`mobilePhone` 같은 키를 발견하면 값 `***` 대체.

### 12.2. 런타임 조회

- `:about` → 버전, build 시각, session_id, active profile, token source, rate limit snapshot, polling interval, config path.
- `:ratelimit` → 카테고리별 last-observed detail.
- `:healthcheck` → 짧은 GET `/api/v1/users/me` + 결과 표시.
- `:errors` → 세션 내 에러 히스토리 (REQ-E02 AC-3).

---

## 13. 확장 포인트

### 13.1. 새 Okta 리소스 추가 (예: Applications — v0.2)

1. `internal/domain/app.go` 타입 정의
2. `internal/domain/ports.go`에 `AppsPort` 추가
3. `internal/okta/apps.go` 어댑터 구현
4. `internal/service/apps.go` AppsService
5. `internal/tui/apps/` Screen Models
6. `internal/app/router.go`에 라우트 추가
7. `cmd/ota/wire.go` wiring 한 줄

**통상 변경 범위:** 7개 파일, 1 디렉토리 신규.

### 13.2. 새 Policy 타입 추가 (예: CONTINUOUS_ACCESS)

REQ-R04 AC-8: 타입 카탈로그는 설정 가능 구조.

- `internal/domain/policy.go`의 `policyTypeCatalog` map에 추가.
- Rich renderer가 없으면 자동 raw-view 모드 (`policies/renderers/` 미등록).

### 13.3. 다른 IdP로 교체 (예: Microsoft Entra)

- `internal/entra/` 어댑터 신설. Port 인터페이스만 구현.
- 도메인 타입은 공통(User/Group 등) — 필요하면 `domain` 내에서 공통 필드만 유지, IdP 고유는 `extras map[string]any`.
- `cmd/ota/main.go`의 wiring 함수만 교체. TUI 레이어 변경 없음.

이 확장은 **v0.3+ 탐색**. 지금은 필요 인터페이스 모양을 비우지 않고 Okta에 맞게 좁게 둔다 (YAGNI). 단, **Port 경계는 유지**.

---

## 14. 보안 아키텍처 요약

| 요구 | 메커니즘 |
|------|---------|
| 토큰 파일 저장 금지 (REQ-C05 AC-4) | 설정 구조체에 `Token` 필드 없음. 오직 `APITokenEnv`(이름). |
| 로그 마스킹 (REQ-C05 AC-2) | `internal/logger` middleware가 인증 헤더·토큰 값·PII 스크럽. |
| 크래시 스택에 토큰 누출 방지 (REQ-C05 AC-3) | Client 구조체의 `token` 필드는 `fmt.Stringer`에서 `***` 반환. panic handler는 Authorization 헤더 scrub. |
| PII 기본 마스킹 (REQ-R01 AC-6) | `internal/mask`로 View 레이어에서 마스킹 후 렌더. `:unmask`는 세션/60s 타임아웃. |
| TLS only | `okta-sdk-golang`는 TLS 기본. ota는 http:// URL 거부 (config validate). |
| 디버그 로그 파일 권한 | `0600` (user-only). 생성 시 umask 명시. |

---

## 15. 비기능 요구사항 대응 요약

| NFR | 아키텍처적 대응 |
|-----|----------------|
| 초기 실행 < 500ms | lazy wiring, TLS 연결 pool 재사용, 초기 SDK client 생성 최소화. |
| 키 입력 < 16ms | Update 함수는 순수·비블로킹. 모든 I/O는 Cmd. |
| 리스트 → 상세 < 300ms (cached) | Service 캐시 30s TTL. Detail에서 연관 리소스는 탭 진입 시에만 호출. |
| 1,000행 필터 < 50ms | 클라이언트 필터는 메모리 substring match. fuzzy는 명시 토글 시만. |
| 메모리 < 200MB | 로그 버퍼는 TUI viewport(최대 N행), Iterator는 페이지 단위로 GC. |

---

## 16. 아키텍처 수준 테스트 포인트 (test-engineer 협업)

| 관점 | 검증 전략 |
|------|---------|
| Port 경계 | Service 테스트는 fake Port 주입. SDK 없는 단위 테스트. |
| Adapter 통합 | `httptest.Server` + JSON 픽스처(testdata/oktaapi/). 실제 SDK 호출 경로 검증. |
| TUI 인터랙션 | `teatest`로 키 시퀀스 → 렌더 비교. |
| 에러 매핑 | 테이블 드리븐: Okta 응답 fixture → domain.Err 매칭. |
| Rate Limit | 429 fixture + Retry-After 준수 / tail 복구 `since` 유지. |
| 프로필 스위칭 | Fake Clock + Fake Adapter 두 세트 → 스위치 후 캐시 리셋. |

상세는 `TESTING.md` (test-engineer 주도).

---

## 17. 열린 이슈 / Phase 5 검증 과제

PRD §11.4를 상속:

1. `teatest`의 스냅샷 안정성 (ANSI 정규화)
2. SDK v5의 Rate Limit 헤더 노출 정확성 — middleware 내 직접 `http.Response` 접근 필요성
3. SDK v5의 Link 헤더 헬퍼(`HasNextPage`/`Next`) 커버리지 — 미커버 엔드포인트가 있으면 수동 파서 fallback
4. Resize 중 tail 안정성
5. SystemLogs의 debugContext free-form 필드 — 직접 HTTP fallback 필요 여부
6. Policy Rule id prefix 일관성 (도메인 §12.1 잔존) — 픽스처로 검증

---

**END of ARCHITECTURE.md draft**
