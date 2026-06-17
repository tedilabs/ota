# TESTING

**Version:** v1.1.0
**Status:** Final (Phase 4 + REQ-W01 addendum)
**Last updated:** 2026-06-17
**Owner:** test-engineer (주 작성) · developer (adapter·TUI 섹션 공동 기여)
**Sources:** `docs/PRD.md` v1.1.0 · `docs/TUI_DESIGN.md` v1.3.0 · `docs/ARCHITECTURE.md` v1.1.0 · `docs/PROJECT_STRUCTURE.md` v1.1.0 · `docs/CONVENTIONS.md` v1.1.0 · `docs/TECH_STACK.md` v1.0.0 · `_workspace/02_okta_domain_input.md` · `_workspace/edit-form-users/02_okta_domain_input.md` · `_workspace/edit-form-users/02_pm_prd_draft.md` · `_workspace/edit-form-users/03_tui_design_draft.md`

> 이 문서는 ota의 **테스트 단일 출처**다. 어떤 코드든 이 문서의 원칙·패턴·도구와 일치해야 한다. 새 테스트를 작성하기 전 본 문서를 읽고, 테스트 전략이 본 문서와 충돌하면 문서를 먼저 업데이트한다.

---

## 1. 테스트 철학

### 1.1. 세 가지 핵심 원칙

1. **Fail-First TDD** — 모든 테스트는 처음에 반드시 실패한다. 실패 이유가 "구현 부재"임을 확인한 후 구현으로 이동한다. 실패 로그 증거는 `_workspace/05_test_fail_log_<YYYY-MM-DD>.txt`에 append.
2. **경계면 정합성 (Three-Way Triangulation)** — PRD REQ ↔ 구현 ↔ 테스트가 삼각 일치해야 한다. QA(Phase 7)는 이 세 꼭짓점을 교차 읽기로 검증한다. 테스트는 REQ-ID를 반드시 주석 또는 함수명에 명시한다.
3. **회귀 방지 자동화** — 사용자·QA가 발견한 모든 버그는 먼저 실패 테스트로 재현된 뒤 수정된다. 회귀 테스트 없이 버그를 수정하지 않는다.

### 1.2. 비목표

- **100% 커버리지 추구 금지** — 커버리지는 결과물이지 목표가 아니다. 핵심 도메인 95%+, 서비스 85%+, 어댑터 70%+, TUI 60%+ (§9 참조).
- **SDK 자체 동작 테스트 금지** — Okta SDK는 Okta의 책임. 우리는 **SDK ↔ 어댑터 경계**만 검증한다.
- **픽셀 단위 UI 비교 금지** — ANSI 토큰 수준 골든 diff로 충분. 터미널 폰트·렌더러별 차이는 명세 밖.

### 1.3. Fail-First가 아닌 테스트

이미 통과하는 구현에 테스트를 덧붙이는 경우:
- 회귀 방지 목적은 정당하나 **설계 피드백 가치는 없음**.
- 파일 상단 주석 필수: `// Lock-in test (not Fail-First derived): <이유>`
- 예외는 소수로 유지. 새 요구사항은 항상 Red부터.

---

## 2. 테스트 피라미드

```
            ┌──────────────────────────┐
            │   E2E (실 Okta tenant)    │  minimal, opt-in (-tags=integration)
            │   소수, 느림, 수동 트리거    │  scripts + Developer Free tenant
            ├──────────────────────────┤
            │   TUI Flow (teatest)      │  인터랙션·상태 전이 검증
            │   중간, 1~3초/테스트        │  골든 파일 diff
            ├──────────────────────────┤
            │   TUI Render (View)       │  순수 렌더 스냅샷, teatest 없음
            │   많음, <50ms/테스트        │  빠른 회귀 체크
            ├──────────────────────────┤
            │   Adapter Integration     │  httptest.Server + 고정 JSON fixture
            │   중간, <200ms/테스트       │  Link 헤더·429·errorCode·페이지네이션
            ├──────────────────────────┤
            │   Unit                    │  순수 도메인 로직, 픽스처 없음
            │   압도적 다수, <10ms       │  파서·매퍼·필터·에러맵
            └──────────────────────────┘
```

### 2.1. 레이어별 책임과 산출물

| 레이어 | 대상 패키지 | 외부 의존 | 속도 | 예시 |
|-------|-----------|---------|------|------|
| Unit | `internal/domain/*`, `internal/okta/{ratelimit,pagination,errormap}` | 없음 | <10ms | UserStatus 파싱, Link 헤더 파서, errorCode 매핑 |
| Adapter Integration | `internal/okta/*` (HTTP 경계). Port 계약도 여기서 검증. | httptest.Server + JSON fixture | <200ms | OktaUsersAdapter가 Link 헤더 순회, 429+Retry-After 준수, Get→ErrNotFound |
| TUI Render | `internal/tui/*/view.go` | 주입된 fake Port | <50ms | UsersListModel.View() 골든 diff |
| TUI Flow | `internal/tui/*` Update 체인 | 주입된 fake Port | 1~3s | `/alice` → Enter → Detail 전환 |
| E2E (manual) | `cmd/ota` | 실 Okta Developer tenant | 수초~수분 | 스모크: 기동 → Users List 표시 |

> MVP에서 Port 구현체는 Okta 어댑터 1개뿐이므로 **별도의 "Contract" 레이어를 두지 않는다.** Port 계약 검증(Get→ErrNotFound, List→빈 slice 등)은 Adapter Integration 테스트가 그대로 담당한다. 대체 구현(예: v0.2 오프라인 데모용 in-memory fake 또는 v0.3+ 다른 IdP)이 등장하는 시점에 §13의 계약 테스트 패턴을 도입한다.

### 2.2. 비율 원칙

- **원칙:** Unit 압도적 다수, 경계면당 Integration 1개 이상, TUI Flow는 시나리오 대표성만.
- **강제 수치 아님:** 7:3(수량) / 1:1(시간)은 목표치. 의미 있는 테스트가 늘어나 비율이 깨져도 문제없음.

---

## 3. 도구 스택

| 용도 | 선정 | 버전 정책 | 비고 |
|------|-----|---------|------|
| Assertion | `github.com/stretchr/testify/assert`, `require` | latest | `assert` 기본, 후속 단언을 멈춰야 하면 `require` |
| Interface mock | **수동 fake** | — | 기본. `internal/*/fakes/` 패키지에 모음 (§3.2) |
| Interface mock (예외) | `github.com/stretchr/testify/mock` | latest | 계약이 극도로 복잡할 때만. `gomock`/`mockgen` 금지 |
| HTTP 흉내 | `net/http/httptest.Server` | stdlib | 기본. SDK 클라이언트의 base URL을 교체 |
| HTTP 흉내 (보조) | `github.com/jarcoal/httpmock` | latest | SDK 내부 transport 교체가 불가할 때만 |
| TUI flow | `github.com/charmbracelet/x/exp/teatest` | latest | §6 참조 |
| Deep diff | `github.com/google/go-cmp/cmp` | latest | JSON/구조체 비교 |
| 골든 파일 | `testdata/golden/<test-name>.golden` | — | `-update` 플래그로만 갱신 |
| Goroutine leak | `go.uber.org/goleak` | latest | `TestMain`에서 `VerifyTestMain` |
| Race detector | `go test -race` | go toolchain | CI 필수 |
| Coverage | `go test -coverprofile` + `go tool cover` | go toolchain | 목표치 gate는 CI에서 체크 |
| Lint | `golangci-lint` | latest | testfx 하위 포함. `depguard`로 SDK 직접 import 차단 |
| Vulnerability | `govulncheck` | latest | CI 주 1회 |

### 3.1. 선정 근거

- **testify** — 표준 assertion, 팀 친숙도. 단, `testify/mock`은 stringly-typed 호출이 리팩터링 취약 → fake 우선.
- **teatest** — Bubbletea 공식 실험적 패키지. 현재 유일한 실용 옵션. 내부 goroutine이 goleak와 상호작용하므로 §9.3 allowlist 필요.
- **httptest.Server** — SDK가 `*http.Client` 교체를 허용하므로 base URL만 httptest 서버로 돌려 실제 SDK 경로를 검증. `httpmock`은 SDK가 내부 transport를 직접 쓸 때 최후수단.
- **go-cmp** — `reflect.DeepEqual`보다 해석력 높은 diff. 중첩 JSON 검증에 유리.

### 3.2. 수동 fake 템플릿

```go
// internal/service/fakes/users_port_fake.go
package fakes

import (
    "context"
    "testing"

    "github.com/tedilabs/ota/internal/domain"
)

// UsersPortFake는 domain.UsersPort를 테스트 용도로 구현한다.
// Func 필드를 설정하지 않은 메서드가 호출되면 t.Fatal로 즉시 실패한다.
type UsersPortFake struct {
    t *testing.T

    ListFunc    func(ctx context.Context, f domain.UserFilter) ([]domain.User, domain.PageInfo, error)
    GetFunc     func(ctx context.Context, id string) (domain.User, error)
    GroupsFunc  func(ctx context.Context, id string) ([]domain.Group, error)
    FactorsFunc func(ctx context.Context, id string) ([]domain.Factor, error)
}

func NewUsersPort(t *testing.T) *UsersPortFake {
    t.Helper()
    return &UsersPortFake{t: t}
}

func (f *UsersPortFake) List(ctx context.Context, filter domain.UserFilter) ([]domain.User, domain.PageInfo, error) {
    f.t.Helper()
    if f.ListFunc == nil {
        f.t.Fatalf("UsersPortFake.List called but ListFunc is not set")
    }
    return f.ListFunc(ctx, filter)
}
// ... Get, Groups, Factors 동일 패턴
```

**이점:** IDE 탐색·리팩터링 친화, 명시적, 컴파일러가 인터페이스 변경을 잡아준다.

---

## 4. 디렉토리 구조

```
ota/
├── internal/
│   ├── domain/
│   │   ├── user.go
│   │   ├── user_test.go                      # 외부 테스트 (package domain_test)
│   │   ├── ports.go                          # UsersPort, GroupsPort, ...
│   │   ├── errors.go                         # Error, ErrNotFound, ...
│   │   └── errors_test.go
│   ├── service/
│   │   ├── users_service.go
│   │   ├── users_service_test.go
│   │   └── fakes/                            # Port fake 집합
│   │       ├── users_port_fake.go
│   │       ├── groups_port_fake.go
│   │       └── ...
│   ├── okta/
│   │   ├── users_adapter.go
│   │   ├── users_adapter_test.go             # httptest 기반 integration
│   │   ├── ratelimit/
│   │   │   ├── monitor.go
│   │   │   └── monitor_test.go               # 순수 유닛
│   │   ├── pagination/
│   │   │   ├── link.go
│   │   │   └── link_test.go
│   │   ├── errormap/
│   │   │   ├── map.go
│   │   │   └── map_test.go
│   │   ├── testfx/                           # 테스트 전용, 비-prod 보장
│   │   │   ├── fake_server.go                # NewFakeOktaServer(t, scenario)
│   │   │   ├── fixtures.go                   # LoadFixture(t, path) 헬퍼
│   │   │   └── scrub.go                      # record 스크럽 유틸
│   │   └── integration/
│   │       └── live_test.go                  # //go:build integration
│   ├── tui/
│   │   ├── shared/
│   │   ├── users/
│   │   │   ├── list_model.go
│   │   │   ├── list_view.go
│   │   │   ├── list_render_test.go           # View() 스냅샷
│   │   │   ├── list_flow_test.go             # teatest
│   │   │   └── testdata/
│   │   │       ├── Test_UsersList_Initial.golden
│   │   │       └── Test_UsersList_FilterThenDetail.golden
│   │   └── ...
│   ├── clock/
│   │   ├── clock.go                          # Clock, Jitter interface + real impls
│   │   └── fake.go                           # FakeClock (Advance, NewTimer, ...)
│   ├── mask/
│   │   └── mask_test.go
│   └── logger/
│       ├── logger.go
│       ├── masking_handler.go
│       └── masking_handler_test.go
├── testdata/
│   └── oktaapi/
│       ├── fixtures_manifest.yaml            # 캡처 시각·scrub 상태·tenant edition
│       ├── users/
│       │   ├── list_page1.json
│       │   ├── list_page1_link_header.txt
│       │   ├── list_page2.json
│       │   ├── detail_active.json
│       │   ├── factors_all_types.json
│       │   └── groups_of_user.json
│       ├── groups/
│       ├── grouprules/
│       ├── policies/
│       │   ├── okta_sign_on.json
│       │   ├── access_policy.json
│       │   ├── password.json
│       │   ├── mfa_enroll.json
│       │   ├── profile_enrollment.json       # raw-only
│       │   ├── post_auth_session.json        # raw-only
│       │   └── idp_discovery.json            # raw-only
│       ├── logs/
│       │   ├── tail_initial.json
│       │   ├── tail_poll_next.json
│       │   ├── rate_limited_429.json
│       │   └── failed_signins.json
│       └── errors/
│           ├── E0000001_validation.json
│           ├── E0000004_auth.json
│           ├── E0000006_forbidden.json
│           ├── E0000007_not_found.json
│           ├── E0000011_token_expired.json
│           ├── E0000022_delete_blocked.json
│           ├── E0000038_feature_disabled.json
│           └── E0000047_rate_limit.json
├── scripts/
│   ├── record-fixture.go                     # 실 tenant → testdata 캡처
│   └── record-fixture.md                     # 스크럽 규칙
└── Makefile
```

### 4.1. 내부 vs 외부 테스트 패키지

- **외부 (`package foo_test`) 기본** — 인터페이스 사용자 관점. 노출 API만 사용.
- **내부 (`package foo`) 예외** — unexported 심볼 직접 검증이 불가피할 때만. 테스트 파일 상단 주석으로 정당화.

### 4.2. testfx 패키지 규약

- 실제 런타임 경로에서 제외되어야 한다. 2가지 방법 중 하나 채택:
  1. 별도 패키지 (`internal/okta/testfx`) — `*_test.go`가 아닌 일반 `.go` 파일이지만 production 진입점(`cmd/ota`)이 import하지 않으면 실제 바이너리에 포함되지 않음. 더 간단.
  2. 빌드 태그 `//go:build testfx`.
- **채택:** 방법 1 (패키지 분리). lint 규칙으로 `cmd/ota`에서 `internal/okta/testfx` import 금지.

### 4.3. Port 인터페이스 위치

- `internal/domain/ports.go`에 `UsersPort`, `GroupsPort`, `RulesPort`, `PoliciesPort`, `LogsPort` 선언.
- `internal/okta/*_adapter.go`가 구현체 제공.
- Service 레이어 및 TUI는 도메인 인터페이스만 import.
- **lint 강제:** `internal/app/`, `internal/tui/`, `internal/service/`에서 `github.com/okta/okta-sdk-golang` 직접 import 금지 (`golangci-lint` `depguard`).

> ※ Port 위치는 **`internal/domain/ports.go`로 확정** (2026-04-24, developer + test-engineer 합의). ARCHITECTURE §6.2 · PROJECT_STRUCTURE §2에도 동일 반영.

---

## 5. 픽스처 관리

### 5.1. Record/Replay 전략

- **Record:** `scripts/record-fixture.go --scenario <name>`로 실 Developer tenant에서 1회 캡처 → 자동 스크럽 → `testdata/oktaapi/`에 저장.
- **Replay:** 기본 `go test`는 오프라인. `testfx.NewFakeOktaServer(t, scenario)`가 fixture를 로드하여 httptest.Server로 재생.
- **검증:** `//go:build integration` 태그로 실 tenant 호출 테스트 별도 제공 (§9.4).

### 5.2. 스크럽 규칙

모든 캡처는 저장 전 아래 치환을 거친다. `scripts/record-fixture.md`에서 상세 규칙 관리.

| 필드 | 치환 |
|-----|------|
| 이메일 (`profile.email`, `profile.login`, `actor.alternateId`) | `user-<hash>@redacted.example.com` (hash는 stable) |
| 전화 (`profile.mobilePhone`, factor.profile.phoneNumber) | `+1-555-000-NNNN` (NNNN은 결정론적 4자리) |
| `Authorization` 헤더 | 완전 제거 |
| `Set-Cookie` | 완전 제거 |
| `X-Okta-Request-Id` | `00000000-0000-0000-0000-<index>` 고정 |
| `ipAddress`, `geographicalContext` | `198.51.100.1` / 비워둠 |
| 조직 커스텀 profile 필드 | 값만 `***REDACTED***`, 키는 보존 |
| `uuid` (Log event) | 결정론적 UUID로 재생성 |

### 5.3. fixture manifest

```yaml
# testdata/oktaapi/fixtures_manifest.yaml
fixtures:
  - path: users/list_page1.json
    captured_at: 2026-05-02T10:00:00Z
    tenant_edition: developer-free
    endpoint: GET /api/v1/users?limit=200
    scenario: users-list-basic
    scrubbed:
      - emails
      - request_ids
    notes: |
      3 users, all ACTIVE. Used by UsersRepo_List_Basic tests.
```

Phase 7 QA가 manifest를 읽어 fixture drift(실 tenant 응답 스키마 변화)를 추적할 수 있다.

### 5.4. 필수 시드 데이터 (Phase 5 시작 전 필수)

#### users/
- `00u_active_alice` — ACTIVE, factors=[push(Okta Verify), sms], groups=[eng]
- `00u_active_bob` — ACTIVE, factors=[webauthn]
- `00u_suspended` — SUSPENDED (REQ-R01 AC-2 색상 분기)
- `00u_locked` — LOCKED_OUT
- `00u_password_expired` — PASSWORD_EXPIRED
- `00u_deprovisioned` — DEPROVISIONED (REQ-R01 AC-2 DELETED와 혼동 방지)
- `00u_staged`, `00u_provisioned` — 경계 상태

#### groups/
- `00g_engineering` — OKTA_GROUP, 동적 (Group Rule target, RULE 배지 테스트)
- `00g_sales` — OKTA_GROUP, 정적
- `00g_everyone` — BUILT_IN (REQ-R02 AC-3 대용량 배너)
- `00g_app_synced` — APP_GROUP

#### grouprules/
- `0pr_active` — ACTIVE, expression=`user.department == "Engineering"`
- `0pr_inactive` — INACTIVE
- `0pr_invalid` — INVALID (REQ-R03 AC-2 경고색)

#### policies/ (7종 ≥ 1개씩)
- OKTA_SIGN_ON × 1 (system=true) + 1 custom
- ACCESS_POLICY × 2
- PASSWORD × 1
- MFA_ENROLL × 1
- PROFILE_ENROLLMENT, POST_AUTH_SESSION, IDP_DISCOVERY × 각 1 (raw-only)

#### logs/
- `user.session.start` SUCCESS × 3, FAILURE × 2
- `group.rule.deactivate` × 1 (REQ-R05 AC-5 프리셋 iii)
- `system.api_token.create` × 1 (REQ-C04 AC-5 토큰 수명 힌트)

#### errors/
- 8종 전부 (§7 에러 매핑 테이블)

---

## 6. teatest 패턴

### 6.1. 언제 쓰는가

- **쓴다:** 화면 전환, 키 입력 시퀀스, tail 폴링 등 **이벤트 루프가 필요한 시나리오**.
- **쓰지 않는다:** 단일 상태에서의 렌더링 검증. View() 출력을 직접 비교하는 편이 10~20배 빠르고 명확하다.

### 6.2. View() 스냅샷 (fast path)

Model은 **생성자 경로로만** 초기 상태를 주입한다. 테스트-only setter(`SetUsers` 등) 금지 (CONVENTIONS §8.1 · §10.1 Elm 원칙). 초기 데이터가 필요하면 Port fake가 동기적으로 반환한 뒤 `UsersLoadedMsg`를 직접 `Update(msg)`로 전달하거나, 생성자에 `WithInitialUsers(xs)` Option을 추가한다.

```go
// internal/tui/users/list_render_test.go
func Test_UsersListModel_Render_WithActiveAndSuspendedRows(t *testing.T) {
    t.Parallel()

    fakeClock := clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))

    // 방법 A: 생성자 Option으로 초기 데이터 주입
    model := users.NewListModel(users.Deps{
        Clock:  fakeClock,
        Width:  120, Height: 30,
    }, users.WithInitialUsers([]domain.User{
        {ID: "00u1", Profile: domain.UserProfile{Login: "alice@acme.com"}, Status: domain.StatusActive},
        {ID: "00u2", Profile: domain.UserProfile{Login: "bob@acme.com"},   Status: domain.StatusSuspended},
    }))

    // 방법 B (동등): Update를 통해 상태 유도
    // m := users.NewListModel(deps)
    // m, _ = m.Update(users.UsersLoadedMsg{Items: xs}).(users.ListModel), ...

    out := model.View()
    golden.Assert(t, out, "testdata/Test_UsersListModel_Render_Mixed.golden")
}
```

> 두 방법 중 팀은 **방법 A (Option)**를 기본으로 한다. `Update` 경유(방법 B)는 메시지 타입까지 테스트에서 구성해야 해 번잡하며, Option은 production에서도 부트스트랩(예: 캐시 hydration)에 재사용 가능한 경로다.

### 6.3. teatest 전체 플로우 (slow path)

```go
// internal/tui/users/list_flow_test.go
func Test_UsersListFlow_FilterAlice_OpensDetail(t *testing.T) {
    t.Parallel()

    port := fakes.NewUsersPort(t)
    port.ListFunc = func(_ context.Context, _ domain.UserFilter) ([]domain.User, domain.PageInfo, error) {
        return []domain.User{
            {ID: "00u1", Profile: domain.UserProfile{Login: "alice@acme.com"}, Status: domain.StatusActive},
            {ID: "00u2", Profile: domain.UserProfile{Login: "bob@acme.com"},   Status: domain.StatusActive},
        }, domain.PageInfo{}, nil
    }
    port.GetFunc = func(_ context.Context, id string) (domain.User, error) {
        return domain.User{ID: id, Profile: domain.UserProfile{Login: "alice@acme.com"}, Status: domain.StatusActive}, nil
    }

    fakeClock := clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))

    model := users.NewListModel(users.Deps{Port: port, Clock: fakeClock})

    tm := teatest.NewTestModel(t, model,
        teatest.WithInitialTermSize(100, 30),
    )

    // 초기 fetch 결과 도착 대기
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("alice@acme.com"))
    }, teatest.WithCheckInterval(10*time.Millisecond), teatest.WithDuration(2*time.Second))

    // '/alice' 입력 후 Enter
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alice")})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
    // 선택 상태에서 Enter로 상세 진입
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // 최종 출력 골든 비교
    out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
    require.NoError(t, err)
    teatest.RequireEqualOutput(t, out)
    // golden: testdata/Test_UsersListFlow_FilterAlice_OpensDetail.golden
    // 업데이트: go test -update ./internal/tui/users/
}
```

### 6.4. 골든 파일 규약

- 경로: `internal/tui/<resource>/testdata/<TestName>.golden`
- 업데이트: `go test -update ./...` — CI에서는 금지
- 업데이트 시 git diff를 PR 본문에 포함하여 리뷰어가 검토
- 터미널 크기는 `teatest.WithInitialTermSize`로 고정 (기본 100×30)
- ANSI 이스케이프 포함 그대로 저장 (스타일 회귀도 감지)

### 6.4a. 시각 골든 (Phase 6d)

Phase 6c에서 발견된 회귀(테스트는 PASS인데 사용자에게는 빈 화면처럼 보임)를 막기 위해 Phase 6d는 **시각 충실도 골든**을 별도 트랙으로 도입한다. 이전 6.4의 `*.golden` 파일은 teatest 출력 전체(ANSI 포함)를 저장하지만, 시각 골든은 다음과 같이 분리되어 사람이 읽고 리뷰할 수 있는 형태로 유지된다.

- **경로:** `internal/tui/<resource>/testdata/golden/<scenario>.txt`
- **포맷:** NO_COLOR + ANSI strip 후의 plain text (박스 보더 `╭─╮│╰─╯├┤` 유지, 컬럼 정렬 보존)
- **갱신:** `go test -update ./internal/tui/...` — `internal/testfx` 가 `-update` 플래그를 처리하여 기존 파일을 덮어쓴다.
- **컬러 프로필 고정:** `testfx.PinTestEnvironment()` 가 `NO_COLOR=1` 과 `lipgloss.SetColorProfile(termenv.Ascii)` 를 같이 적용. 각 `*_golden_test.go` 의 `init()` 에서 호출.
- **ANSI Strip:** `testfx.StripANSI(view)` — `charmbracelet/x/ansi.Strip` 래퍼. 컬러 프로필 고정과 이중 안전장치.
- **소스 오브 트루스:** 각 골든은 `docs/TUI_DESIGN.md` §16 의 ASCII 와이어프레임을 그대로 옮긴 것. 디자이너가 §16 을 갱신하면 -update 로 동기화.

**Fail-First 운영 모드:** Phase 6d-3 (App Shell chrome) ~ 6d-6 (Error surfacing) 가 진행 중인 동안 골든 비교는 `t.Skip` 으로 비활성화하고 spec lock-in (`assert.Contains` 기반) 만 Active 로 둔다. 개발자가 해당 Phase 를 마치면 골든 테스트의 Skip 을 제거하고 -update 한 번 실행하여 골든을 활성화. 이 절차는 `_workspace/06d_test_red_log.txt` 에 기록된다.

### 6.5. 비동기 Cmd 대기 전략

- **예측가능한 조건**을 기다린다: 특정 문자열/패턴 등장, 카운터 변화
- **`time.Sleep` 금지**: 본질적으로 flaky
- **타임아웃:** 네트워크 없으므로 2s로 충분. 그보다 오래 걸리면 설계 문제

### 6.6. teatest 실측 (Phase 6 Users list 플로우 기준, 2026-04-24)

첫 teatest `Test_UsersListFlow_FilterAlice_OpensDetail` 를 3회 연속 실행하여 측정한 실측치. 각 실행 **~20ms**로 매우 빠르고 안정적. flaky 없음.

#### 타이밍 및 일반
- **단일 실행 시간:** 0.02s (-race 포함). 플로우 내부 cmd 3~4개(Init → fetch → filter → Get) 수행.
- **`WaitFor` 타임아웃:** 2s로 선언했지만 실제로는 10ms 미만에 predicate 매치. fake Port가 Cmd 내부 I/O 없이 즉시 반환하므로 대기가 거의 없음.
- **`FinalOutput` 타임아웃:** 3s로 선언. 실제 드레인은 즉시. 실제 I/O 기반 Integration에서는 값 조정 필요 가능.
- **3회 반복 안정성:** `go test -count=3` 3회 모두 0.02s PASS. 타이밍 기반 flakiness 없음.

#### ANSI 처리
- **stripping 불필요** — Bubbletea가 터미널 폭에 맞춘 ANSI 이스케이프 포함 바이트를 Output에 기록하지만, `bytes.Contains` 기반 predicate + 골든 파일이 ANSI 포함 비교를 자연 수용.
- 골든 비교 시 `teatest.RequireEqualOutput`이 정규화 도움을 주지만, 본 테스트는 `require.Contains` 사용이 충분.
- 터미널 크기 `teatest.WithInitialTermSize(100, 30)` 고정. 이 값 변경 시 줄바꿈 위치가 달라지므로 골든 파일 함께 재생성.

#### 초기 `tea.Cmd` 실행 순서
1. `users.NewListModel(Deps{Port, Clock})` → Init() → `fetchUsersCmd` 트리거
2. teatest 내부가 Cmd 실행 → fake `ListFunc` 호출 → `UsersLoadedMsg` 반환
3. Model.Update가 Msg 수신 → 내부 상태 `.items` 채움 → View에 렌더
4. `WaitFor` predicate `Contains("alice@...")` 매치 시점에 다음 키 주입 진행

#### `goleak` allowlist (Phase 6 실측 결과)
3회 실행 모두 누출 없음. 현재 시점 TestMain에서 goleak 실행 시 **추가 allowlist 불필요**. Phase 7 QA 중 teatest 동시 복수 호출 경로 발견 시 재조사. 예비용 스니펫:

```go
// internal/tui/users/main_test.go (예비 — 현재는 불필요)
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m,
        // 필요 시 아래 추가:
        // goleak.IgnoreTopFunction("github.com/charmbracelet/bubbletea.(*Program).Run"),
    )
}
```

#### 터미널 크기·Resize
- 본 테스트는 Resize 이벤트를 명시적으로 주입하지 않음. `WithInitialTermSize` 후 고정 폭 사용.
- Resize 전파 신뢰성은 별도 시나리오 테스트가 필요 (Phase 7 시 추가). Bubbletea가 `tea.WindowSizeMsg`를 Update에 전달하는 경로를 model이 store + View에서 width 참조하는지 확인.

#### 결론
teatest는 fake Port + FakeClock 조합에서 **결정론적이고 빠름**. Phase 6 TUI Flow 계층의 기반으로 안정.

### 6.7. Form 화면 teatest 패턴 (REQ-W01)

REQ-W01 Users Edit (SCR-012)은 ota의 첫 mutation 화면이자 `internal/tui/shared/form/` 위젯의 첫 사용처. 테스트 레이어가 셋으로 분리된다:

| 레이어 | 위치 | 도구 | 검증 대상 |
|--------|------|------|----------|
| **Form 위젯 단위** | `internal/tui/shared/form/form_test.go` | 순수 unit | dirty 추적·검증·키 가로채기·focus 이동 — Port fake 없음 |
| **Edit screen 통합** | `internal/tui/users/edit_*_test.go` | **teatest + UsersPortFake** | fetch/save Cmd·상태머신 전이·에러 매핑·navStack pop |
| **계약/Port** | `internal/okta/users_test.go` + `internal/service/users_test.go` | httptest.Server + fake | `UpdateProfile` partial-merge body·errormap·재시도 정책 |

#### 6.7.1. Form 위젯 unit 테스트 (예: dirty 추적)

```go
// internal/tui/shared/form/form_test.go
func Test_Form_Dirty_TrackedPerKeystroke(t *testing.T) {
    t.Parallel()

    specs := []form.FieldSpec{
        {Key: "firstName", Label: "First Name", Kind: form.KindText, Required: true},
        {Key: "lastName",  Label: "Last Name",  Kind: form.KindText, Required: true},
    }
    initial := map[string]string{"firstName": "Alice", "lastName": "Smith"}

    f := form.New(specs, initial)
    require.Equal(t, 0, f.Dirty())

    // type "Alicia" — append "ia" to firstName (assuming first field focused)
    f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ia")})
    require.Equal(t, 1, f.Dirty())
    require.Equal(t, "Aliceia", f.Snapshot()["firstName"])  // depends on impl

    // revert by Ctrl+U (textinput clears) + type "Alice"
    f, _ = f.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
    f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Alice")})
    assert.Equal(t, 0, f.Dirty(), "reverting to snapshot value clears dirty")
}
```

핵심: Port fake가 없다. Form은 도메인을 모르므로 순수 메시지 → 상태 검증으로 충분.

#### 6.7.2. Edit screen teatest (예: full save flow)

```go
// internal/tui/users/edit_save_test.go
func Test_UserEdit_Save_PartialMergeBody_Success(t *testing.T) {
    t.Parallel()

    var capturedPatch domain.UserProfilePatch
    port := fakes.NewUsersPort(t)
    port.GetFunc = func(_ context.Context, id string) (domain.User, error) {
        return domain.User{
            ID: id,
            Status: domain.UserStatusActive,
            Profile: domain.UserProfile{
                Login: "alice@acme.com", Email: "alice@acme.com",
                FirstName: "Alice", LastName: "Smith",
                Department: "Eng",
                // ... 11 fields populated
            },
        }, nil
    }
    port.UpdateProfileFunc = func(_ context.Context, _ string, patch domain.UserProfilePatch) (domain.User, error) {
        capturedPatch = patch
        // simulate server echoing the updated user
        return domain.User{ID: "00u1", Profile: domain.UserProfile{
            Login: "alice@acme.com", Email: "alice@acme.com",
            FirstName: *patch.FirstName, LastName: "Smith",
            Department: "Eng",
        }}, nil
    }

    svc := service.NewUsersService(port)
    model := users.NewEditModel(users.EditDeps{Svc: svc, UserID: "00u1"})

    tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 30))

    // wait for loaded
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("Alice"))
    }, teatest.WithDuration(2*time.Second))

    // edit firstName: focus already on first field (Identity / First Name)
    tm.Send(tea.KeyMsg{Type: tea.KeyEnd})               // cursor to end
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ia")})  // Alice → Alicia
    // wait dirty marker
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("1 change"))
    }, teatest.WithDuration(1*time.Second))

    // Ctrl+S
    tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

    // wait pop (e.g., toast "Updated alice")
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("Updated"))
    }, teatest.WithDuration(2*time.Second))

    // AC-4.2: partial-merge body only contains firstName
    require.NotNil(t, capturedPatch.FirstName)
    assert.Equal(t, "Alicia", *capturedPatch.FirstName)
    assert.Nil(t, capturedPatch.LastName, "unchanged fields must be omitted")
    assert.Nil(t, capturedPatch.Email,    "unchanged fields must be omitted")
    assert.Nil(t, capturedPatch.Department,"unchanged fields must be omitted")
}
```

#### 6.7.3. Validation error 시나리오 (server-side)

```go
// internal/tui/users/edit_save_test.go
func Test_UserEdit_Save_400Validation_InlineFieldErrors(t *testing.T) {
    t.Parallel()

    port := fakes.NewUsersPort(t)
    // Get: same as success above
    port.UpdateProfileFunc = func(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
        return domain.User{}, &domain.BadRequestError{
            Raw: "Api validation failed: profile",
            Causes: []domain.FieldError{
                {Field: "email",      Summary: "email: Email is not valid"},
                {Field: "department", Summary: "department: Cannot exceed 100 characters"},
            },
        }
    }
    // ... wire model, tm, edit some field, send Ctrl+S ...

    // wait field-level inline error attached to "Email" row
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("! Email is not valid"))
    }, teatest.WithDuration(2*time.Second))

    // also for Department
    out := string(takeFrame(tm))
    assert.Contains(t, out, "! Cannot exceed 100 characters")

    // AC-6: form must NOT close — Esc still goes to discard confirm
    tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("Discard"))
    }, teatest.WithDuration(1*time.Second))
}
```

#### 6.7.4. 단축키 매트릭스

| 단축키 | 시점 | 테스트 | 예상 동작 |
|--------|------|--------|----------|
| `e` | List/Detail 화면 | `Test_UsersList_eKey_EmitsOpenUserEditMsg`, `Test_UserDetail_eKey_EmitsOpenUserEditMsg` | `OpenUserEditMsg{ID}` 발송 |
| `Tab` / `Shift+Tab` | editing | `Test_Form_TabCyclesFocus_SkipsReadOnly` | focus 이동 (read-only `login` skip) |
| `Ctrl+S` (Dirty=0) | editing | `Test_Form_CtrlS_Empty_NoSaveCommand_FooterHint` | save Cmd 미발사, footer "No changes to save" |
| `Ctrl+S` (Dirty>0, valid) | editing | `Test_UserEdit_Save_PartialMergeBody_Success` | save Cmd 발사 |
| `Ctrl+S` (validation fail) | editing | `Test_UserEdit_Save_ClientValidationFails_FocusFirstInvalid` | save Cmd 미발사, focus 첫 invalid 필드 |
| `Esc` (Dirty=0) | editing | `Test_UserEdit_Esc_Clean_PopsNav` | popNav, no modal |
| `Esc` (Dirty>0) | editing | `Test_UserEdit_Esc_Dirty_OpensDiscardConfirm` | `OverlayDiscardConfirm` open |
| `y` on DiscardConfirm | overlay | `Test_DiscardConfirm_Y_DiscardsAndPops` | popNav, changes lost |
| `n` / `Esc` on DiscardConfirm | overlay | `Test_DiscardConfirm_N_ReturnsToEditing` | overlay close, form preserved |
| `Alt+m` | editing | `Test_Form_AltM_TogglesAllPII` | mobilePhone + secondEmail mask/unmask 전환 |
| `Ctrl+C` | saving | `Test_UserEdit_CtrlC_DuringSaving_CancelsAndPreservesInput` | ctx cancel, draft 보존 |
| `Esc` | saving | `Test_UserEdit_Esc_DuringSaving_NoOp` | 무시, footer hint 표시 |

각 행은 **teatest 시나리오 1개**로 구현. table-driven으로 묶기 어려운 이유: 각 시나리오의 초기 상태 / 키 시퀀스가 다름. 다만 §8.7의 11 fields × dirty matrix는 table-driven.

#### 6.7.5. UsersPortFake에 UpdateProfile 추가 (Phase 5 시작 전 필수)

```go
// internal/service/fakes/users_port_fake.go — 확장
type UsersPortFake struct {
    t *testing.T

    // ... 기존 필드 ...
    UpdateProfileFunc func(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error)
}

func (f *UsersPortFake) UpdateProfile(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error) {
    f.t.Helper()
    if f.UpdateProfileFunc == nil {
        f.t.Fatalf("UsersPortFake.UpdateProfile called but UpdateProfileFunc is not set")
    }
    return f.UpdateProfileFunc(ctx, userID, patch)
}
```

**검증 시뮬레이션 헬퍼 (UsersPortFake에 추가):**

```go
// ValidationErrorFake returns a UpdateProfileFunc that always rejects with
// BadRequestError for the given (field → message) pairs. Used to drive
// AC-6 server-side validation tests.
func ValidationErrorFake(causes map[string]string) func(context.Context, string, domain.UserProfilePatch) (domain.User, error) {
    fields := make([]domain.FieldError, 0, len(causes))
    for k, v := range causes {
        fields = append(fields, domain.FieldError{Field: k, Summary: k + ": " + v})
    }
    return func(_ context.Context, _ string, _ domain.UserProfilePatch) (domain.User, error) {
        return domain.User{}, &domain.BadRequestError{Causes: fields, Raw: "Api validation failed"}
    }
}
```

---

## 7. Fail-First 프로세스

### 7.1. 표준 Red → Green → Refactor

```
1. REQ-ID 또는 AC 단위로 요구사항 선정
2. 테스트 파일에 실패 테스트 작성 (구현은 stub 또는 부재)
3. go test ./... → 실패 확인
4. 실패 로그를 _workspace/05_test_fail_log_YYYY-MM-DD.txt에 append
5. 최소 구현 작성 → Green
6. 리팩터링 (녹색 유지)
7. 커밋 (테스트 + 구현 + 문서 동시)
```

### 7.2. Fail 로그 규약

`_workspace/05_test_fail_log_YYYY-MM-DD.txt`에 REQ-ID별로 섹션을 만들고 append:

```
## REQ-R01 AC-1 Users 리스트 기본 컬럼
### 2026-05-01 10:23 (Red)
$ go test ./internal/tui/users/ -run Test_UsersListModel_Render
--- FAIL: Test_UsersListModel_Render (0.00s)
    list_render_test.go:38: undefined: users.NewListModel

### 2026-05-01 10:41 (Red 두번째 — 컴파일 통과, 단정 실패)
$ go test ./internal/tui/users/ -run Test_UsersListModel_Render
--- FAIL: Test_UsersListModel_Render (0.02s)
    golden mismatch (see diff):
    expected column 'STATUS' at index 0, got ''

### 2026-05-01 10:58 (Green)
$ go test ./internal/tui/users/ -run Test_UsersListModel_Render
ok  github.com/tedilabs/ota/internal/tui/users  0.123s

### 2026-05-01 11:12 (Refactor — 여전히 Green)
...
```

### 7.3. 흔한 함정

- **테스트가 처음부터 통과:** Fail-First 위반. 기존 구현이 우연히 충족시키는 경우라면 Lock-in 테스트(§1.3)로 명시하거나 assertion을 더 구체적으로.
- **테스트가 구현을 1:1로 반영:** 행동을 검증해야지 구조를 반영하면 안 된다. 내부 심볼 직접 호출 지양.
- **과도한 mock:** 전체 시스템을 mock으로 덮으면 통합 버그를 놓친다. 최소 1~2 레벨 위까지는 실제 코드.

---

## 8. 시나리오별 테스트 매트릭스 (요약)

전체 REQ-ID 매핑은 §12에. 여기서는 **고위험 시나리오 6종**의 설계 의도만 요약.

### 8.1. Rate Limit (REQ-E01, PRD §7.2)

| AC | 테스트 | 레이어 |
|----|-------|-------|
| AC-1 Remaining ≤ 10% 노란 경고 | `RateLimitMonitor_Warns_WhenRemainingBelowTenPercent` | Unit |
| AC-2 Retry-After 준수 + ±20% jitter | `Transport_Retries429_WithRetryAfterAndJitter` | Adapter integration (httptest) |
| AC-2 3회 실패 후 에러 | `Transport_GivesUp_After3Retries` | Adapter integration |
| AC-3 tail 일시정지 + 재개 시 `since` 유지 | `LogsTail_PausesOnRateLimit_ResumesWithSameSince` | Unit (fakeClock) |
| AC-4 카테고리별 last-observed | `RateLimitMonitor_RecordsPerCategory` | Unit |
| AC-5 `/logs` 분당 ~120 안전 마진 | `LogsTail_DefaultInterval_KeepsBudget` | Unit |
| AC-6 30초 TTL 캐시 + 강제 무효화 | `Cache_TTL_Expires` · `Cache_ForceInvalidate` | Unit |

### 8.2. Pagination (PRD §7.3, REQ-R01 AC-4, REQ-R02 AC-3)

| 시나리오 | 테스트 |
|---------|-------|
| Link 헤더 next 파싱 | `LinkHeader_ParsesNextCursor` · `LinkHeader_ReturnsEmpty_WhenNoNext` |
| 2~3페이지 순차 fetch | `UsersAdapter_ListAll_IteratesAllPages` |
| 병렬 요청 금지 (순차 보장) | `UsersAdapter_ListAll_IsSequential` |
| context 취소 전파 | `UsersAdapter_ListAll_CancelsGracefully` |
| 멤버 페이지 소진 중 사용자 중단 | `GroupMembersIterator_StopsOnSignal` |

### 8.3. 에러 매핑 (PRD §7.7, REQ-U04 AC-3, REQ-C04 AC-4)

에러 모델은 **sentinel 기본 + `errors.As` 타입으로 부가 정보 추출** (CONVENTIONS §4.2 · ARCHITECTURE §9). errorCode 8종 전부 테이블 드리븐:

```go
func Test_ErrorMap_FromResponse_MapsAllKnownCodes(t *testing.T) {
    t.Parallel()
    cases := []struct{
        name       string
        fixture    string   // testdata/oktaapi/errors/<file>.json
        wantSentinel error
        wantSummary string // substring match (사용자 메시지 원재료)
    }{
        {"validation",       "E0000001_validation.json",       domain.ErrBadRequest,      "Api validation failed"},
        {"auth",             "E0000004_auth.json",             domain.ErrTokenInvalid,    "API token invalid"},
        {"forbidden",        "E0000006_forbidden.json",        domain.ErrForbidden,       "Insufficient permissions"},
        {"not_found",        "E0000007_not_found.json",        domain.ErrNotFound,        "Resource not found"},
        {"token_expired",    "E0000011_token_expired.json",    domain.ErrTokenInvalid,    "Token expired"},
        {"delete_blocked",   "E0000022_delete_blocked.json",   domain.ErrBadRequest,      "Deactivate before"},
        {"feature_disabled", "E0000038_feature_disabled.json", domain.ErrFeatureDisabled, "feature is disabled"},
        {"rate_limit",       "E0000047_rate_limit.json",       domain.ErrRateLimited,     "Rate limit"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            resp := testfx.LoadHTTPResponse(t, "testdata/oktaapi/errors/"+tc.fixture)
            err := errormap.FromResponse(resp)
            // sentinel 확인
            require.ErrorIs(t, err, tc.wantSentinel)
            // errorSummary 보존 (사용자 메시지 원재료)
            assert.Contains(t, err.Error(), tc.wantSummary)
        })
    }
}

// E0000001의 errorCauses 필드별 보존은 전용 타입으로 검증
func Test_ErrorMap_BadRequest_PreservesCauses(t *testing.T) {
    t.Parallel()
    resp := testfx.LoadHTTPResponse(t, "testdata/oktaapi/errors/E0000001_validation.json")
    err := errormap.FromResponse(resp)

    var bre *domain.BadRequestErr
    require.ErrorAs(t, err, &bre)
    require.NotEmpty(t, bre.Causes, "E0000001 must include errorCauses")
    // Okta가 전달한 필드별 원인이 그대로 보존되어 UI에서 표시 가능해야 함 (REQ-U04 AC-3)
    assert.Contains(t, bre.Causes[0].Summary, "login")
}

// 429의 Retry-After는 RateLimitedErr로 꺼낸다
func Test_ErrorMap_RateLimit_ExposesRetryAfter(t *testing.T) {
    t.Parallel()
    resp := testfx.LoadHTTPResponse(t, "testdata/oktaapi/errors/E0000047_rate_limit.json")
    err := errormap.FromResponse(resp)

    var rle *domain.RateLimitedErr
    require.ErrorAs(t, err, &rle)
    assert.Greater(t, rle.RetryAfter, time.Duration(0))
}
```

### 8.4. PII 마스킹 (REQ-C05, REQ-R01 AC-6, TUI_DESIGN §7)

| 시나리오 | 테스트 | 레이어 |
|---------|-------|-------|
| SMS phoneNumber 기본 마스킹 | `Factor_SMS_MasksPhoneByDefault` | Unit |
| secondEmail 마스킹 포맷 | `Profile_MasksSecondEmail` | Unit |
| `:unmask` 후 [M!] 배지 | `UserDetail_Unmask_ShowsBadge` | TUI flow (teatest) |
| `y` 복사 (마스킹 상태) | `UserDetail_CopyMasked_CopiesMasked` | Unit (clipboard fake) |
| 화면 전환 시 자동 재마스킹 | `UserDetail_Navigate_AutoRemasks` | TUI flow |
| 60초 inactivity 자동 재마스킹 | `UserDetail_Inactivity_AutoRemasks` | Unit (fakeClock) |
| **peek test**: 로그·크래시에 raw PII 부재 | `Secrets_NeverLeakToDebugLog` | Unit (§11) |
| Authorization 헤더 마스킹 | `DebugLog_AuthHeader_IsMasked` | Unit |

### 8.5. Logs Tail (REQ-R05)

| AC | 테스트 |
|----|-------|
| AC-2 `since` 마지막 published + 1ms | `LogsTail_Since_AdvancesByMinPlusOneMs` |
| AC-2 기본 7초 간격 | `LogsTail_DefaultPollInterval_Is7s` |
| AC-2 adaptive: `X-Rate-Limit-Limit < 60` → 15초 | `LogsTail_AdaptivePolling_UpgradesTo15s_OnLowLimit` |
| AC-3 새 이벤트 카운터 | `LogsTail_NewEventsBadge_IncrementsOnArrival` (TUI flow) |
| AC-3 `f` 자동 스크롤 토글 | `LogsTail_FollowToggle_PausesAutoScroll` (TUI flow) |
| AC-3 429 자동 일시정지 | `LogsTail_Paused_WhenRateLimited` |
| AC-4 DESCENDING 최신순 히스토리 | `LogsList_Descending_LatestFirst` |
| AC-5 프리셋 5종 | `LogsPresets_ApplyFilter` (table-driven × 5) |
| AC-6 actor.id → User 점프 | `LogEvent_ActorJump_OpensUserDetail` (TUI flow) |
| AC-7 UTC/로컬 토글 | `LogTime_Format_RespectsTzSetting` |

### 8.6. Policies 7종 (REQ-R04)

- **Rich 4종:** table-driven `Test_Policies_RichType_<TYPE>_ActionSummary` — OKTA_SIGN_ON, ACCESS_POLICY, PASSWORD, MFA_ENROLL
- **Raw 3종:** `Test_Policies_RawOnlyType_ListColumns_UniformAcrossTypes`, `Test_Policies_RawOnlyType_DetailShowsRawJSON`, `Test_Policies_RawOnlyType_MenuShowsRawViewBadge`

### 8.7. Users Profile Edit Form (REQ-W01)

`internal/tui/shared/form/` 와 `internal/tui/users/edit*.go` 를 다층으로 검증. 핵심 매트릭스 3종:

#### 8.7.1. AC 매트릭스 (전체)

| AC | 테스트 함수 | 레이어 |
|----|------------|-------|
| AC-1.1 (e from list) | `Test_UsersList_eKey_EmitsOpenUserEditMsg` | TUI unit |
| AC-1.2 (e from detail) | `Test_UserDetail_eKey_EmitsOpenUserEditMsg` (table — Pretty/JSON/YAML 탭 × 3) | TUI unit |
| AC-1.3 (latest GET on entry) | `Test_UserEdit_OnEntry_CallsPortGet_Once` | TUI flow |
| AC-1.4 (loading abort with Esc) | `Test_UserEdit_Loading_EscAborts` | TUI flow |
| AC-1.5 (4xx blocks form open) | `Test_UserEdit_Loading_4xx_DoesNotOpenForm` (table — 401/403/404) | TUI flow |
| AC-2 (11 fields × 4 sections) | `Test_UserEdit_FieldCatalog_HasAll11Fields_In4Sections` | Render |
| AC-2 (login read-only) | `Test_UserEdit_LoginField_IsReadOnly_NotInDiff` | TUI unit |
| AC-3.1 (required empty) | `Test_UserEdit_RequiredField_EmptyShowsInline` (table — firstName/lastName/email × 3) | Form unit |
| AC-3.2 (email loose format) | `Test_UserEdit_EmailField_LooseValidation` (table — valid/invalid × 6) | Form unit |
| AC-3.3 (phone E.164 hint) | `Test_UserEdit_MobilePhone_E164Hint_NoBlock` | Form unit |
| AC-3.5 (no client uniqueness lookup) | `Test_UserEdit_NoPreSaveUniquenessGetCall` | TUI flow |
| AC-4.1 (Ctrl+S triggers save) | `Test_UserEdit_Save_PartialMergeBody_Success` | TUI flow |
| AC-4.2 (diff-only body, omit) | **§8.7.2 table-driven** | TUI flow + port assertion |
| AC-4.3 (saving disables input + Esc) | `Test_UserEdit_Saving_InputDisabled_EscNoop` | TUI flow |
| AC-4.4 (1s save guard) | `Test_UserEdit_Saving_PostSuccess_1sGuard_DisablesSave` (FakeClock) | TUI flow |
| AC-4.5 (success cache patch + toast) | `Test_UserEdit_Save_Success_BroadcastsUserUpdatedMsg` + `Test_UsersList_ReceivesUserUpdatedMsg_PatchesRow` | TUI flow |
| AC-5.1 (clean Esc) | `Test_UserEdit_Esc_Clean_PopsImmediately` | TUI flow |
| AC-5.2 (dirty Esc → modal) | `Test_UserEdit_Esc_Dirty_OpensDiscardConfirm` + `Test_DiscardConfirm_Y_Discards` / `Test_DiscardConfirm_N_Preserves` | TUI flow |
| AC-5.3 (saving Esc noop) | `Test_UserEdit_Esc_DuringSaving_NoOp` | TUI flow |
| AC-6 (errorCauses inline) | **§8.7.3 table-driven** | TUI flow |
| AC-6.1 (field prefix match) | `Test_Form_ApplyServerErrors_PrefixMatchesFieldSpecKey` | Form unit |
| AC-6.2 (inline clears on edit) | `Test_Form_InlineError_ClearsOnKeystrokeInThatField` | Form unit |
| AC-7.1~7.5 (PII mask lifecycle) | `Test_UserEdit_PII_DefaultMasked` + `Test_UserEdit_PII_FocusAutoUnmask` + `Test_UserEdit_PII_BlurRemasksUnmodified` + `Test_UserEdit_AltM_TogglesAllPII` | TUI flow |
| AC-7.6 (no PII in debug log) | `Test_UserEdit_DebugLog_NoRawPII` (peek test, §11) | Unit |
| AC-8.1 (keyboard only) | implicit — all teatest 시나리오가 키보드만 사용 | — |
| AC-8.2 (NO_COLOR markers) | `Test_UserEdit_Render_NoColor_ShowsDirtyAsterisk_RequiredText_InlineExclam` | Render (visual golden §6.4a) |
| AC-8.3 (80x24 viewport) | `Test_UserEdit_NarrowTerminal_LabelInputTwoLines_ViewportScroll` | Render |
| AC-9 (dirty/diff tracking) | **§8.7.4 table-driven** | Form unit |
| AC-10.1 (cache untainted on cancel) | `Test_UserEdit_Discard_DoesNotPatchListCache` | TUI flow |
| AC-10.2 (background polls continue) | `Test_UserEdit_DoesNotPauseLogsTailPolling` | TUI flow (LogsModel + EditModel 동시) |
| AC-10.3 (selected row restored on close) | `Test_UserEdit_AfterClose_RestoresSelectedRow` | TUI flow |

#### 8.7.2. AC-4.2 Partial-Merge Body 테이블 (11 fields × dirty matrix)

```go
// internal/tui/users/edit_save_test.go
func Test_UserEdit_Save_PartialMerge_OnlyDirtyFieldsInBody(t *testing.T) {
    t.Parallel()

    type fieldCase struct {
        Key   string  // FieldSpec.Key — Okta API field name
        Input string  // value typed by user
        Read  func(domain.UserProfilePatch) *string  // accessor on patch
    }

    cases := []fieldCase{
        {"firstName",      "Alicia",   func(p domain.UserProfilePatch) *string { return p.FirstName }},
        {"lastName",       "Smyth",    func(p domain.UserProfilePatch) *string { return p.LastName }},
        {"displayName",    "Ali S.",   func(p domain.UserProfilePatch) *string { return p.DisplayName }},
        {"nickName",       "aly",      func(p domain.UserProfilePatch) *string { return p.NickName }},
        {"email",          "x@y.com",  func(p domain.UserProfilePatch) *string { return p.Email }},
        {"title",          "Lead",     func(p domain.UserProfilePatch) *string { return p.Title }},
        {"division",       "RnD-2",    func(p domain.UserProfilePatch) *string { return p.Division }},
        {"department",     "Platform", func(p domain.UserProfilePatch) *string { return p.Department }},
        {"employeeNumber", "ENG-099",  func(p domain.UserProfilePatch) *string { return p.EmployeeNumber }},
        {"mobilePhone",    "+14155551234", func(p domain.UserProfilePatch) *string { return p.MobilePhone }},
        {"secondEmail",    "a@b.com",  func(p domain.UserProfilePatch) *string { return p.SecondEmail }},
    }

    for _, tc := range cases {
        t.Run("only "+tc.Key, func(t *testing.T) {
            t.Parallel()
            var got domain.UserProfilePatch
            port := newPortWithLoadedUser(t, loadedAlice)
            port.UpdateProfileFunc = func(_ context.Context, _ string, p domain.UserProfilePatch) (domain.User, error) {
                got = p
                return loadedAlice, nil
            }
            tm := startEditFlow(t, port, "00u1")
            focusField(tm, tc.Key)   // helper: Tab until labelMatches
            tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})  // clear field
            tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.Input)})
            tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})
            waitForSaveComplete(t, tm)

            // Only this field set; all others nil
            require.NotNil(t, tc.Read(got), "%s must be set in patch", tc.Key)
            assert.Equal(t, tc.Input, *tc.Read(got))
            for _, other := range cases {
                if other.Key == tc.Key {
                    continue
                }
                assert.Nil(t, other.Read(got), "%s must be nil (omit) when not edited", other.Key)
            }
        })
    }
}
```

#### 8.7.3. AC-6 errorCauses 매핑 테이블

```go
// internal/tui/users/edit_save_test.go
func Test_UserEdit_Save_BadRequestError_InlineFieldErrors(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name        string
        causes      map[string]string  // field -> reason
        wantInline  map[string]string  // expected substring per field in rendered output
        wantOther   []string           // non-field causes shown in footer "Other errors:"
    }{
        {
            name: "single field validation",
            causes: map[string]string{"email": "Email is not valid"},
            wantInline: map[string]string{"Email": "! Email is not valid"},
        },
        {
            name: "two field validations",
            causes: map[string]string{
                "email": "Email is not valid",
                "department": "Cannot exceed 100 characters",
            },
            wantInline: map[string]string{
                "Email": "! Email is not valid",
                "Department": "! Cannot exceed 100 characters",
            },
        },
        {
            name: "unknown field prefix → other errors",
            causes: map[string]string{"customField_x": "Forbidden"},
            wantOther: []string{"customField_x"},
        },
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            port := newPortWithLoadedUser(t, loadedAlice)
            port.UpdateProfileFunc = fakes.ValidationErrorFake(tc.causes)
            tm := startEditFlow(t, port, "00u1")
            editAnyField(tm)             // force dirty so Ctrl+S triggers save
            tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})
            frame := waitForFrame(t, tm)
            for label, want := range tc.wantInline {
                assert.Contains(t, frame, want, "%s inline error", label)
            }
            for _, other := range tc.wantOther {
                assert.Contains(t, frame, other, "other error must appear in footer")
            }
            assert.NotContains(t, frame, "popNav called", "form must NOT close on 400")
        })
    }
}
```

#### 8.7.4. AC-9 Dirty Tracking Matrix

```go
// internal/tui/shared/form/form_test.go
func Test_Form_DirtyMatrix_AllSections(t *testing.T) {
    t.Parallel()
    specs := usersedit.FieldSpecs()   // production catalog — exported test helper or testfx

    type op struct {
        field string
        value string  // empty = no edit; non-empty = SetValue
    }
    cases := []struct {
        name     string
        ops      []op
        wantDirty int
        wantKeys []string
    }{
        {"no edit", nil, 0, nil},
        {"one identity field", []op{{"firstName", "Alicia"}}, 1, []string{"firstName"}},
        {"one contact field PII", []op{{"mobilePhone", "+1..."}}, 1, []string{"mobilePhone"}},
        {"cross-section",
            []op{{"firstName", "Alicia"}, {"department", "Eng-2"}, {"secondEmail", "x@y"}},
            3, []string{"firstName", "department", "secondEmail"}},
        {"revert restores clean",
            []op{{"firstName", "Alicia"}, {"firstName", "Alice"}},
            0, nil},
        {"all 11 dirty",
            []op{ /* 11 ops */ },
            11, /* all keys */},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            f := form.New(specs, loadedAliceProfile)
            for _, o := range tc.ops {
                f = applyEdit(f, o.field, o.value)
            }
            assert.Equal(t, tc.wantDirty, f.Dirty())
            assert.ElementsMatch(t, tc.wantKeys, f.DirtyFields())
        })
    }
}
```

#### 8.7.5. UsersPortFake에 UpdateProfile 추가 패턴

§6.7.5 참조. Phase 5 진입 직전 `internal/service/fakes/users_port_fake.go` 에 `UpdateProfileFunc` 필드와 `UpdateProfile` 메서드 추가 + 검증 헬퍼 (`ValidationErrorFake`) 추가.

#### 8.7.6. Adapter integration (httptest)

```go
// internal/okta/users_test.go
func Test_OktaUsersAdapter_UpdateProfile_PartialMerge_BodyShape(t *testing.T) {
    t.Parallel()
    var capturedBody []byte
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        require.Equal(t, "POST", r.Method)
        require.Equal(t, "/api/v1/users/00u1", r.URL.Path)
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"id":"00u1","profile":{"login":"a@x","firstName":"Alicia","lastName":"Smith"}}`))
    }))
    defer srv.Close()

    client := okta.NewTestClient(t, srv.URL, "token")
    a := okta.NewUsersAdapter(client)

    first := "Alicia"
    patch := domain.UserProfilePatch{FirstName: &first}
    user, err := a.UpdateProfile(context.Background(), "00u1", patch)
    require.NoError(t, err)
    assert.Equal(t, "Alicia", user.Profile.FirstName)

    var sent struct {
        Profile map[string]any `json:"profile"`
    }
    require.NoError(t, json.Unmarshal(capturedBody, &sent))
    assert.Equal(t, "Alicia", sent.Profile["firstName"])
    assert.NotContains(t, sent.Profile, "lastName",  "unchanged fields must be omitted")
    assert.NotContains(t, sent.Profile, "email",     "unchanged fields must be omitted")
    // ... 9 more assertNotContains for the other 9 fields
}

func Test_OktaUsersAdapter_UpdateProfile_ErrorMapping(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name    string
        fixture string
        wantErr error
    }{
        {"400 validation", "errors/E0000001_validation.json", new(domain.BadRequestError)},
        {"403 forbidden",  "errors/E0000006_forbidden.json",  domain.ErrForbidden},
        {"404 not found",  "errors/E0000007_not_found.json",  domain.ErrNotFound},
        {"429 rate limited","errors/E0000047_rate_limit.json",new(domain.RateLimitedError)},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { /* serve fixture, assert errors.As/Is */ })
    }
}

// EMPTY PATCH GUARD — service-level only, but adapter should also panic-guard
func Test_OktaUsersAdapter_UpdateProfile_EmptyPatch_ReturnsErrEmptyPatch_NoHTTPCall(t *testing.T) {
    t.Parallel()
    called := false
    srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
        called = true
    }))
    defer srv.Close()
    a := okta.NewUsersAdapter(okta.NewTestClient(t, srv.URL, "tok"))
    _, err := a.UpdateProfile(context.Background(), "00u1", domain.UserProfilePatch{})
    assert.ErrorIs(t, err, domain.ErrEmptyPatch)
    assert.False(t, called, "no HTTP call must be made for empty patch (D-W13)")
}
```

---

## 9. CI 요구사항

### 9.1. GitHub Actions 잡

| 잡 | 명령 | 게이트 |
|----|------|-------|
| `test-unit` | `go test -race -count=1 ./...` | race free, 전부 통과 |
| `test-integration` (httptest) | 위 잡이 포함 (태그 없이 httptest 기반만) | 동일 |
| `lint` | `golangci-lint run` | 경고 0 |
| `vuln` | `govulncheck ./...` | 알려진 취약점 없음 |
| `cover-gate` | coverage thresholds 체크 (§9.2) | 하회 시 실패 |
| (manual) `e2e-live` | `go test -tags=integration ./...`, `workflow_dispatch` | OKTA_ORG_URL/TOKEN secret 필요 |

### 9.2. Coverage Gate

| 패키지 prefix | 최소 | 근거 |
|--------------|------|------|
| `internal/domain/...` | **95%** | 순수 로직, mock 불필요 |
| `internal/service/...` | **85%** | Port fake로 대부분 커버 가능 |
| `internal/okta/...` | **75%** | integration으로 보완. ARCHITECTURE §17 / CONVENTIONS 동일 기준과 통일 |
| `internal/okta/{ratelimit,pagination,errormap}` | **95%** | 경계 유닛, 철저 |
| `internal/tui/...` | **60%** | View 스냅샷 + teatest flow. 스타일 분기는 snapshot이 커버 |

> Gate는 PRD §6.3의 "핵심 도메인 ≥ 70%" 기준을 상향. Phase 5 말부터 활성화.

### 9.3. goleak 정책 (Phase 6 실측 결과 2026-04-24)

**현재 상태:** 실측 결과 teatest Users list 플로우 3회 연속 실행에서 goroutine 누출 **0건**. 현재 시점 allowlist가 필요하지 않음 — `goleak.VerifyTestMain(m)`을 allowlist 없이 호출하는 것으로 충분.

```go
// internal/<package>_test.go (필요한 패키지별로 1개)
// 현재 권장 형태 (2026-04-24 실측 기준):
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

**allowlist가 필요해지는 조건 (Phase 7 이후 관찰 예정):**
- 동일 테스트 안에서 teatest.NewTestModel을 2회 이상 호출하는 경우 — bubbletea Program goroutine이 완전히 종료되지 않을 수 있음
- `tea.Cmd` 안에서 수동 `go func` 스폰을 사용하는 경우 (CONVENTIONS §9 위반)
- Logs tail polling 테스트가 ctx cancel 없이 early-return하는 경우

위 중 하나라도 발견되면 아래 템플릿으로 allowlist 추가:
```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m,
        goleak.IgnoreTopFunction("github.com/charmbracelet/bubbletea.(*Program).Run"),
        goleak.IgnoreAnyFunction("github.com/charmbracelet/x/exp/teatest.(*TestModel).startLoop"),
    )
}
```

- 범위: 각 패키지 단위로 1개 `TestMain`
- allowlist는 최소한으로. 실제 leak은 반드시 수정 (CONVENTIONS §13.9).

### 9.4. Integration 태그

- **파일 헤더:**
  ```go
  //go:build integration
  // +build integration

  package okta_test
  ```
- **환경:** `OKTA_ORG_URL`, `OKTA_API_TOKEN` (Read-Only Admin)
- **실행:** `make test-integration` (로컬/manual dispatch)
- **CI 기본 미실행** — PRD §10.1의 E2E를 수동/옵션으로 유지

### 9.5. Flaky 정책

- **재시도 금지.** 한 번이라도 flaky면 근본 원인 조사.
- 임시 격리 수단: `t.Skip("flaky: <link to issue>")` + `//go:build flaky` 태그
- 원인 수정 후 복원. 수정 전 merge 금지.

---

## 10. 로컬 개발자 워크플로

### 10.1. Makefile 타겟

```makefile
.PHONY: test test-short test-race test-integration test-cover test-update-golden lint vuln

test:
	go test -race -count=1 ./...

test-short:
	go test -race -short -count=1 ./...

test-race:
	go test -race -count=1 -timeout=60s ./...

test-integration:
	OKTA_ORG_URL=$${OKTA_ORG_URL:?set OKTA_ORG_URL} \
	OKTA_API_TOKEN=$${OKTA_API_TOKEN:?set OKTA_API_TOKEN} \
	go test -race -tags=integration -count=1 ./...

test-cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -n 1
	go tool cover -html=coverage.out -o coverage.html

test-update-golden:
	go test -update ./internal/tui/...

lint:
	golangci-lint run

vuln:
	govulncheck ./...
```

### 10.2. 일반 개발 사이클

1. `make test-short` — 작업 중 빠른 피드백 (`-short`로 teatest 스킵 가능)
2. 커밋 전 `make test` — race 포함 전체
3. 골든 변경 시 `make test-update-golden` + `git diff testdata/` 리뷰

### 10.3. `-short` 정책

- `testing.Short()`가 true면 teatest flow 테스트를 스킵
- 순수 unit·render·adapter integration은 항상 실행
- CI는 `-short` 없이 실행 (teatest 포함)

---

## 11. peek test (보안 QA)

### 11.1. 목적

PRD §6.2 + TUI_DESIGN §7에 정의된 PII/secret 마스킹이 **실제로 모든 출력 경로에서** 작동함을 기계적으로 검증.

### 11.2. 검증 경로

- 디버그 로그 파일 (`~/.cache/ota/debug.log`, REQ-O01)
- panic/crash 스택트레이스
- 세션 에러 히스토리 (`:errors`)
- 골든 파일 (teatest)
- 클립보드 복사 (마스킹 상태에서 `y`)
- tea.Model 내부 상태 (덤프 지점)

### 11.3. 표준 테스트

```go
func Test_Secrets_NeverLeakToDebugLog(t *testing.T) {
    t.Parallel()

    const (
        rawToken = "00a-SECRET-TOKEN-VALUE-xyz"
        rawPhone = "+1-555-123-4567"
        rawEmail = "alice@secret.com"
    )

    var buf bytes.Buffer
    handler := logger.NewMaskingHandler(slog.NewJSONHandler(&buf, nil))
    log := slog.New(handler)

    // 실제 HTTP 요청 로깅 시뮬레이션
    log.Error("okta request failed",
        "authorization", "SSWS "+rawToken,
        "phone",          rawPhone,
        "email",          rawEmail,
        "user_id",        "00u123")

    out := buf.String()

    // raw 값이 어떤 경로로도 로그에 나타나면 실패
    assert.NotContains(t, out, rawToken, "raw token must not appear in log")
    assert.NotContains(t, out, rawPhone, "raw phone must not appear in log")
    assert.NotContains(t, out, rawEmail, "raw email must not appear in log")

    // 마스킹 흔적은 있어야
    assert.Contains(t, out, "***", "masked marker must appear")
}
```

### 11.4. 골든 파일 peek

```go
func Test_Secrets_NeverLeakToGoldenFiles(t *testing.T) {
    t.Parallel()
    // 모든 testdata/**/*.golden 파일을 grep
    patterns := []*regexp.Regexp{
        regexp.MustCompile(`(?i)\bAuthorization: SSWS \S+`),
        regexp.MustCompile(`\+1-\d{3}-\d{3}-\d{4}`),               // raw US phone
        regexp.MustCompile(`[a-z0-9._-]+@(?!redacted\.example\.com)[a-z0-9-]+\.[a-z]+`),
    }
    filepath.WalkDir("../../", func(path string, d fs.DirEntry, err error) error {
        if !strings.HasSuffix(path, ".golden") { return nil }
        b, err := os.ReadFile(path)
        require.NoError(t, err)
        for _, p := range patterns {
            if m := p.Find(b); m != nil {
                t.Errorf("potential PII leak in %s: %q", path, string(m))
            }
        }
        return nil
    })
}
```

### 11.5. 크래시 스택트레이스

- `runtime/debug.Stack()`으로 캡처 → 마스킹 처리 적용된 Writer로 출력
- 테스트: panic을 유발하는 구조체에 토큰 필드를 넣고, 크래시 로거 출력에 원본 없음 검증

---

## 12. 수용 기준 매핑 매트릭스

> **규약:** 각 REQ-ID의 AC에 최소 1개 테스트 함수를 매핑. AC가 복합적이면 여러 테스트로 분할. Phase 5 시작 전 테스트 이름을 **예약**(stub + `t.Skip`)하고, 작성 시 하나씩 풀어감.

| REQ-ID | AC | 테스트 파일 | 테스트 함수 | 레이어 |
|--------|----|------------|-----------|-------|
| REQ-U01 | AC-1 | `internal/keys/keys_test.go` | `Test_Navigation_ArrowKeysEquivalentToVim` | Unit |
| REQ-U01 | AC-2 | `internal/config/keybinding_test.go` | `Test_Keybindings_OverrideFromConfig` | Unit |
| REQ-U01 | AC-3 | `internal/tui/*/list_flow_test.go` | `Test_*_NavigationConsistent` | TUI flow |
| REQ-U01 | AC-4 | `internal/tui/shared/textinput_test.go` | `Test_TextInput_UsesReadlineKeysInInsertMode` | Unit |
| REQ-U02 | AC-1~4 | `internal/tui/cmdpalette/palette_test.go` | `Test_CmdPalette_Commands` (table) · `Test_CmdPalette_TabComplete` · `Test_CmdPalette_PartialMatch` · `Test_CmdPalette_HistoryPersists` | TUI flow |
| REQ-U03 | AC-1~4 | `internal/tui/shared/incsearch_test.go` | `Test_IncSearch_TypeResponseUnder50ms` · `Test_IncSearch_EnterConfirmsEscCancels` · `Test_IncSearch_CaseToggle` · `Test_IncSearch_NextPrevMatch` | Unit + TUI |
| REQ-U04 | AC-1 | `internal/tui/users/list_flow_test.go` | `Test_UsersList_SlashKeyTriggersQSearch` | TUI flow |
| REQ-U04 | AC-2 | `internal/service/users_service_test.go` | `Test_UsersService_SearchExpression_UsesScimSearchParam` | Unit |
| REQ-U04 | AC-3 | `internal/okta/errormap/map_test.go` | `Test_ErrorMap_FromResponse_MapsAllKnownCodes` | Unit |
| REQ-U04 | AC-4 | `internal/tui/logs/presets_test.go` | `Test_LogsPresets_AppliesFilter` (table × 5) | Unit |
| REQ-U04 | AC-5 | `internal/tui/users/help_test.go` | `Test_UsersHelp_IncludesEventualConsistencyNote` | Unit |
| REQ-U05 | AC-1~3 | `internal/tui/users/detail_flow_test.go` | `Test_UserDetail_TabTransitionUnder300ms_Cached` · `Test_UserDetail_LoadingIndicatorAndCancel` · `Test_UserDetail_BreadcrumbPath` | TUI flow |
| REQ-U06 | AC-1~3 | `internal/tui/help/help_test.go` | `Test_Help_ContextSensitive` · `Test_Help_Searchable` · `Test_Help_ReflectsUserBindings` | TUI flow |
| REQ-U07 | AC-1~2 | `internal/tui/quit_test.go` | `Test_Quit_CtrlCDoubleSkipsProtection` · `Test_Quit_ProtectionDisabledByConfig` | Unit |
| REQ-R01 | AC-1 | `internal/tui/users/list_render_test.go` | `Test_UsersList_DefaultColumns` | Render |
| REQ-R01 | AC-2 | `internal/tui/users/list_render_test.go` | `Test_UsersList_StatusColors` (table × 7 상태) | Render |
| REQ-R01 | AC-3 | `internal/tui/users/detail_render_test.go` | `Test_UserDetail_TabsPresent` (Profile/Credentials/Timestamps/Groups/Factors/Logs) | Render |
| REQ-R01 | AC-4 | `internal/okta/users_adapter_test.go` | `Test_UsersAdapter_ListFirstPage_Under1s` | Adapter integration |
| REQ-R01 | AC-5 | `internal/service/users_service_test.go` | `Test_UsersService_QVsSearchModes` | Unit |
| REQ-R01 | AC-6 | `internal/tui/users/factors_render_test.go` | `Test_UsersFactors_AllTypes_Rendered` (table × 7 factor types) · `Test_UsersFactors_PhoneNumberMasked` · `Test_UsersFactors_IDHiddenByDefault` | Render + Unit |
| REQ-R01 | AC-7 | `internal/tui/users/help_test.go` | `Test_UsersHelp_ExplainsDeletedVsDeprovisioned` | Unit |
| REQ-R02 | AC-1 | `internal/tui/groups/list_render_test.go` | `Test_GroupsList_Columns_WithRuleBadgeWhenDynamic` | Render |
| REQ-R02 | AC-2 | `internal/service/groups_service_test.go` | `Test_GroupsService_FilterParam_UsesScimFilter` | Unit |
| REQ-R02 | AC-3 | `internal/tui/groups/members_flow_test.go` | `Test_Members_BuiltInShowsLargeBanner` · `Test_Members_EveryoneLabeled` · `Test_Members_ProgressiveLoadingCanAbort` | TUI flow |
| REQ-R02 | AC-4 | `internal/tui/groups/detail_flow_test.go` | `Test_GroupDetail_AppsTabLazyLoads_DashOnForbidden` | TUI flow |
| REQ-R02 | AC-5 | `internal/tui/groups/members_flow_test.go` | `Test_Members_CountShownAfterPageExhaust` | TUI flow |
| REQ-R03 | AC-1~6 | `internal/tui/rules/list_render_test.go`, `detail_render_test.go` | `Test_Rules_ListColumns` · `Test_Rules_StatusColors` (ACTIVE/INACTIVE/INVALID) · `Test_Rules_DetailExpressionMonospace` · `Test_Rules_TargetGroupIDResolution` · `Test_Rules_DetailDeactivationWarningBanner` · `Test_RulesAdapter_DefaultLimit200` | Mixed |
| REQ-R04 | AC-1~8 | `internal/tui/policies/*_test.go`, `internal/domain/policies/catalog_test.go` | 7종 × 렌더 + 4 rich × action summary + raw 3종 × 3 tests + `Test_PolicyCatalog_AddNewType_NoUIChange` | Mixed |
| REQ-R05 | AC-1~9 | `internal/tui/logs/*_test.go`, `internal/service/logs_service_test.go` | §8.5 전부 | Mixed |
| REQ-C01 | AC-1~4 | `internal/config/loader_test.go` | `Test_Config_ParseErrorReportsLineColumn` · `Test_Config_CLIFlagOverridePath` · `Test_Config_Sections_Required` · `Test_Config_CommentsPreserved` | Unit |
| REQ-C02 | AC-1~4 | `internal/config/profile_test.go`, `internal/app/profile_switch_test.go` | `Test_Profile_PerTenantFields` · `Test_Profile_CLIFlag` · `Test_Profile_SwitchResetsStateUnder2s` · `Test_Profile_TokenNotInFile` | Unit + TUI flow |
| REQ-C03 | AC-1~4 | `internal/keys/*_test.go` | `Test_Keys_DefaultsDocumented` · `Test_Keys_UserMappingWinsOnConflict` · `Test_Keys_InvalidKeyNameWarns` · `Test_Keys_ReloadRequiresRestart` | Unit |
| REQ-C04 | AC-1~6 | `internal/auth/*_test.go` | `Test_Auth_PriorityFallback` · `Test_Auth_InteractiveTokenNotPersisted` · `Test_Auth_MissingTokenExitsWithGuide` · `Test_Auth_ErrorMapping` (table × errorCode) · `Test_Auth_TokenAgeHintBestEffort` · `Test_Auth_OAuth2NotInMVP` | Unit |
| REQ-C05 | AC-1~4 | `internal/logger/masking_handler_test.go`, `internal/mask/*_test.go` | `Test_Secrets_TokenZeroCopyLifecycle` · `Test_DebugLog_AuthHeader_IsMasked` · `Test_CrashTrace_NoTokenInStack` · `Test_ConfigExamples_NoPlaintextSecret` | Unit |
| REQ-E01 | AC-1~6 | §8.1 전부 | 상동 | Unit + Adapter integration |
| REQ-E02 | AC-1~3 | `internal/tui/shared/toast_test.go` | `Test_Toast_3SecondAutoDismiss` · `Test_Toast_DuplicateShowsCounter` · `Test_ErrorsCommand_ShowsHistory` | TUI flow |
| REQ-E03 | AC-1~3 | `internal/tui/shared/offline_test.go` | `Test_Offline_StatusBarIndicator` · `Test_Offline_CachedDataStillReadable` · `Test_Offline_AutoRefreshOnRecovery` | TUI flow |
| REQ-O01 | AC-1~4 | `internal/logger/debuglog_test.go` | `Test_DebugLog_DisabledByDefault` · `Test_DebugLog_CapturesHTTPAndState` · `Test_DebugLog_RotatesAt10MB` · `Test_DebugLog_TailCommand` | Unit |
| **REQ-W01** | **AC-1~10** | **§8.7 전부 (`edit_flow_test.go` / `edit_save_test.go` / `edit_pii_test.go` / `form_test.go` / `errmap_test.go` / `users_test.go`)** | **§8.7 매트릭스 — 11 fields × 4 sections × dirty matrix table-driven + 6 HTTP 에러 시나리오** | **Mixed (Form unit + TUI flow + Adapter integration)** |

**Phase 5 선행 과제:** 위 매트릭스의 모든 테스트 함수명을 `_test.go`에 `t.Skip("not yet implemented — REQ-XXX AC-Y")`로 예약. 이로써 REQ → 테스트 → 구현 추적 경로가 Phase 5 시작 시점에 이미 성립한다.

---

## 13. 계약 테스트 (Contract Test)

### 13.1. 목적

Port 인터페이스(예: `domain.UsersPort`)의 구현체가 여러 개(실 Okta 어댑터 / in-memory fake / record-replay 기반) 생기더라도 **동일 계약**을 만족하는지 보장.

### 13.2. 패턴

```go
// internal/domain/ports_contract.go
// (테스트가 아니라 일반 함수. 각 구현체 _test.go가 import하여 호출)
package domain_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/tedilabs/ota/internal/domain"
)

type UsersPortFactory func(t *testing.T) domain.UsersPort

func RunUsersPortContract(t *testing.T, factory UsersPortFactory) {
    t.Run("Get nonexistent returns ErrNotFound", func(t *testing.T) {
        port := factory(t)
        _, err := port.Get(context.Background(), "00u-nonexistent")
        assert.ErrorIs(t, err, domain.ErrNotFound)
    })

    t.Run("List empty returns empty slice not nil", func(t *testing.T) {
        port := factory(t)
        users, _, err := port.List(context.Background(), domain.UserFilter{})
        require.NoError(t, err)
        assert.Equal(t, []domain.User{}, users)
    })

    t.Run("Context cancellation propagates", func(t *testing.T) {
        port := factory(t)
        ctx, cancel := context.WithCancel(context.Background())
        cancel()
        _, _, err := port.List(ctx, domain.UserFilter{})
        assert.ErrorIs(t, err, context.Canceled)
    })

    // 추가 계약: List의 PageInfo 형식, Filter의 연산자 지원 등
}
```

### 13.3. 구현체 테스트에서 호출

```go
// internal/okta/users_adapter_contract_test.go
func Test_OktaUsersAdapter_Contract(t *testing.T) {
    domain_test.RunUsersPortContract(t, func(t *testing.T) domain.UsersPort {
        srv := testfx.NewFakeOktaServer(t, testfx.ContractScenario)
        return okta.NewUsersAdapter(srv.Client(), srv.URL)
    })
}

// internal/service/fakes/inmem_users_port_test.go
func Test_InMemUsersPort_Contract(t *testing.T) {
    domain_test.RunUsersPortContract(t, func(t *testing.T) domain.UsersPort {
        return fakes.NewInMemUsersPort()
    })
}
```

### 13.4. 커버 대상 계약

- **Get:** 존재/부재/비활성 사용자
- **List:** 빈/필터 매치/복수 페이지/페이지 없음
- **Context 취소** → `errors.Is(err, context.Canceled)` 전파
- **Error 매핑:** 적어도 `ErrNotFound`, `ErrUnauthorized`는 모든 구현이 동일 매핑

---

## 14. 변경 이력

| 날짜 | 버전 | 변경 | 작성 |
|------|------|------|------|
| 2026-04-24 | v0.9 | 초안. D-A~D-L 합의 대부분 반영. | test-engineer |
| 2026-04-24 | v0.9.1 | developer 리뷰 B1~B5 반영: Contract 레이어 → Adapter Integration 병합(§2.1), View 스냅샷 예시 SetUsers 제거(§6.2), Coverage `okta` 75% 통일(§9.2), teatest 실측 Phase 5 초반으로 연기(§6.6), Appendix A 업데이트. | test-engineer |
| 2026-04-24 | v1.0.0 | developer 교차 리뷰 통과. ARCHITECTURE/PROJECT_STRUCTURE/CONVENTIONS/TECH_STACK v1.0.0 정합성 확인 (Port 위치·testdata 분리·fakes 경로·go-cmp 추가 전부 반영). Appendix A의 teatest/goleak/Makefile 항목은 Phase 5 초반 실측 후 본문 업데이트. | test-engineer |
| 2026-04-24 | v1.0.1 | Phase 6 Users list teatest 첫 Green 실측 반영: §6.6 타이밍·ANSI·Cmd 순서 구체 기록, §9.3 goleak은 현 시점 allowlist 불필요 확인. Appendix A의 B4/goleak 이월 항목 해소. | test-engineer |
| 2026-06-17 | v1.1.0 | REQ-W01 (Users Profile Edit) addendum: §6.7 Form 화면 teatest 패턴 신설 (Form unit / Edit screen / Adapter integration 3층 분리, UsersPortFake.UpdateProfile 확장 가이드 + ValidationErrorFake 헬퍼). §8.7 REQ-W01 AC 매트릭스 (11 fields × dirty matrix table-driven, errorCauses 매핑 table, partial-merge body assertion table). §12 매트릭스에 REQ-W01 행 추가. Sources에 REQ-W01 PRD/TUI Design addendum 포함. | test-engineer |

---

## Appendix A. 미해결 항목 (v1.0 finalize 전)

- [x] **D-A 최종 결정:** `internal/domain/ports.go` 확정 (2026-04-24, developer + test-engineer 합의). §4.3 반영 완료.
- [x] **D-M Error 타입:** developer 방식 수용 — sentinel 기본 + 추가 정보는 `errors.As`로 꺼내는 타입(`BadRequestErr{Causes}`, `RateLimitedErr{RetryAfter}`) 병행. CONVENTIONS §4.2 · ARCHITECTURE §6.1 기준. typed Error struct 제안은 철회.
- [x] **CONVENTIONS §13 테스트 섹션:** test-engineer 기여 완료 (2026-04-24, CONVENTIONS v0.1.1-draft).
- [x] **B1 Contract 레이어 병합:** 완료 (§2.1).
- [x] **B2 SetUsers 제거:** 완료 (§6.2 + CONVENTIONS §8.1).
- [x] **B3 Coverage 75%:** 완료 (§9.2).
- [x] **B5 testdata 위치:** TUI 패키지-로컬 + 루트 공유 분리 채택 완료 (CONVENTIONS §13.4). PROJECT_STRUCTURE §7는 developer가 업데이트 예정.
- [x] **B4 / §6.6 teatest 실측:** 완료 (2026-04-24, Phase 6). Users list 플로우 3회 실행 0.02s·flaky 없음·ANSI stripping 불필요 확정.
- [x] **goleak allowlist:** 완료 (2026-04-24). 실측 결과 누출 0건 → 현재 allowlist 불필요. 예비 스니펫 §9.3에 문서화.
- [x] **Makefile**: 완료 (2026-04-24, developer Task #23+#46). `/Makefile` 존재 · `build/test/test-short/test-race/test-integration/test-e2e/lint/vuln/ci/run/fmt/tidy/clean` 타겟 전부 정의. PROJECT_STRUCTURE §9 contract 준수.

## Appendix B. 참조 REQ-ID → 도메인 섹션 인덱스

- REQ-U04 AC-5 ↔ 도메인 §3.2 eventually consistent
- REQ-R01 AC-6 ↔ 도메인 §7 MFA Factors
- REQ-R02 AC-3 ↔ 도메인 §1.3 Everyone 그룹
- REQ-R03 전부 ↔ 도메인 §1.4 Group Rules + §5 EL
- REQ-R04 AC-1~8 ↔ 도메인 §1.6 Policies
- REQ-R05 전부 ↔ 도메인 §1.7 System Logs + §2 페이지네이션
- REQ-E01 전부 ↔ 도메인 §2.2 Rate Limit
- REQ-C04 AC-4 ↔ 도메인 §2.3 에러 응답

**END of TESTING.md v1.0.0.** Phase 5 첫 teatest PR이 Appendix A 잔여 항목(§6.6 실측, §9.3 goleak allowlist, Makefile dry-run)을 본문 값으로 치환한다.
