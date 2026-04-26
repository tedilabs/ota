# ota Project Structure

**Version:** v1.0.0
**Status:** Final (Phase 4)
**Last updated:** 2026-04-24
**Authors:** developer (lead), test-engineer (testdata/ review)

---

## 변경 이력

| 버전 | 날짜 | 변경 | 작성자 |
|------|------|------|-------|
| v0.1.0-draft | 2026-04-24 | 초안 | developer |
| v1.0.0 | 2026-04-24 | D-A 반영: `internal/domain/ports.go` + `queries.go`. testdata 분리(루트 공유 vs 패키지-로컬 golden). testfx 패키지 명시. service/fakes 경로 확정. | developer |

---

## 1. 철학

- **`internal/` 최대 활용.** ota 내부 코드는 외부 프로젝트에서 import 되지 못하도록 차단. `pkg/`는 MVP에서 비워 둔다 (필요 시 승격).
- **책임 경계가 디렉토리에 드러나야 한다.** 파일을 열기 전에 역할을 유추 가능해야 함.
- **"새 리소스 추가는 한 디렉토리 안에 끝난다"** — Okta Applications(v0.2) 추가 시 `internal/tui/apps/`, `internal/okta/apps.go`, `internal/service/apps.go` 만 건드려도 되도록.
- **Go 관용구:** 인터페이스는 소비자 패키지가 선언, 작은 패키지 · 명확한 이름.

---

## 2. 디렉토리 트리

```
ota/
├── cmd/
│   └── ota/
│       ├── main.go              # 엔트리포인트: flag → config → wire → tea.Program
│       └── wire.go              # 의존성 조립 (명시적, DI 프레임워크 없음)
│
├── internal/
│   ├── app/                     # App Shell (최상위 tea.Model, Router)
│   │   ├── app.go               # rootModel, Update, View
│   │   ├── router.go            # 화면 전환 로직
│   │   ├── overlay.go           # cmd palette / help / confirm 통합 관리
│   │   ├── msg.go               # 전역 Msg 타입 (RateLimitObservedMsg 등)
│   │   ├── statusbar.go         # 상단/하단 공통 바
│   │   └── app_test.go
│   │
│   ├── tui/                     # Screen Models
│   │   ├── shared/              # 전역 공통 (styles, breadcrumb, spinner wrapper)
│   │   │   ├── styles.go        # Lip Gloss 토큰 1차 정의 (TUI_DESIGN §6.1)
│   │   │   ├── breadcrumb.go
│   │   │   ├── toast.go
│   │   │   └── keymap.go        # 화면 공통 키 렌더 helper
│   │   ├── users/               # SCR-010, SCR-011
│   │   │   ├── list.go          # UsersListModel
│   │   │   ├── detail.go        # UserDetailModel (+ tabs)
│   │   │   ├── factors.go       # Factors 탭 렌더러 (REQ-R01 AC-6)
│   │   │   ├── msg.go
│   │   │   └── list_test.go
│   │   ├── groups/              # SCR-020, SCR-021
│   │   │   ├── list.go
│   │   │   ├── detail.go
│   │   │   ├── members.go       # 대용량 경고 + progressive (REQ-R02 AC-3)
│   │   │   └── msg.go
│   │   ├── rules/               # SCR-030, SCR-031
│   │   │   ├── list.go
│   │   │   └── detail.go
│   │   ├── policies/            # SCR-040, SCR-041, SCR-042
│   │   │   ├── typeselect.go    # 타입 선택 모달
│   │   │   ├── list.go
│   │   │   ├── detail.go
│   │   │   └── renderers/       # 타입별 액션 렌더러 (REQ-R04 AC-5)
│   │   │       ├── access_policy.go
│   │   │       ├── okta_sign_on.go
│   │   │       ├── password.go
│   │   │       ├── mfa_enroll.go
│   │   │       └── raw.go       # fallback (3 타입 raw-only)
│   │   ├── logs/                # SCR-050, SCR-051
│   │   │   ├── search.go
│   │   │   ├── tail.go          # tail Cmd + since 재설정
│   │   │   └── detail.go
│   │   └── overlay/             # SCR-900..905, 910
│   │       ├── cmdpalette.go
│   │       ├── search.go
│   │       ├── help.go
│   │       ├── confirm.go
│   │       ├── errors.go
│   │       ├── about.go
│   │       └── quitconfirm.go
│   │
│   ├── service/                 # Use Cases (도메인 조합)
│   │   ├── users.go             # UsersService (캐시·정책 포함)
│   │   ├── groups.go
│   │   ├── rules.go
│   │   ├── policies.go
│   │   ├── logs.go              # tail 로직 (since 관리)
│   │   ├── bundle.go            # Services 한 번에 조립용 struct
│   │   ├── fakes/               # Port fake 구현 (test-only)
│   │   │   ├── users_port_fake.go
│   │   │   └── ...
│   │   └── users_test.go        # Port fake 주입 테스트
│   │
│   ├── domain/                  # 순수 도메인 (외부 import 금지)
│   │   ├── user.go              # User, UserStatus, UserProfile, Factor
│   │   ├── group.go
│   │   ├── rule.go              # Rule, RuleStatus (ACTIVE/INACTIVE/INVALID)
│   │   ├── policy.go            # Policy, PolicyType, PolicyRule
│   │   ├── logs.go              # LogEvent, Actor, Target, Outcome
│   │   ├── ports.go             # UsersPort, GroupsPort, ... (중립 위치)
│   │   ├── queries.go           # UsersQuery, LogsQuery (Port 파라미터)
│   │   ├── errors.go            # ErrNotFound, ErrForbidden, ErrRateLimited, ...
│   │   ├── iterator.go          # Iterator[T] interface, PageInfo
│   │   ├── filter.go            # 클라이언트측 substring/fuzzy 매처
│   │   └── user_test.go
│   │
│   ├── okta/                    # Outbound Adapter (SDK wrap)
│   │   ├── client.go            # Client + SDK config
│   │   ├── options.go           # NewClient Options (WithTimeout 등)
│   │   ├── users.go             # UsersAdapter (implements domain.UsersPort)
│   │   ├── groups.go
│   │   ├── rules.go
│   │   ├── policies.go
│   │   ├── logs.go              # Search, Iterator (since 기반)
│   │   ├── factors.go           # User factors
│   │   ├── pagination/          # Link 헤더 파서
│   │   │   ├── link.go
│   │   │   └── link_test.go
│   │   ├── ratelimit/           # RateLimitMonitor middleware
│   │   │   ├── monitor.go
│   │   │   └── monitor_test.go
│   │   ├── errormap/            # Okta errorCode → domain.Err* 매퍼
│   │   │   ├── map.go
│   │   │   └── map_test.go
│   │   ├── mapping.go           # SDK struct → domain struct 변환
│   │   ├── raw.go               # 필요 시 직접 HTTP fallback (Logs 확장 필드 등)
│   │   ├── testfx/              # (test-only 패키지, prod 미포함)
│   │   │   ├── fake_server.go   # NewFakeOktaServer(t, scenario)
│   │   │   ├── fixtures.go      # LoadFixture(t, path)
│   │   │   └── scrub.go
│   │   ├── integration/         # 실 tenant 테스트
│   │   │   └── live_test.go     # //go:build integration
│   │   └── users_test.go
│   │
│   ├── config/                  # YAML 로드
│   │   ├── config.go            # Config 구조체
│   │   ├── loader.go            # koanf layered merge (default → file → env)
│   │   ├── paths.go             # XDG 경로 resolver
│   │   ├── profile.go           # Profile 타입, 선택 로직
│   │   ├── keybinding.go        # 키 문자열 파서
│   │   ├── validate.go          # 구조 검증 (profile 이름, URL https, 등)
│   │   └── loader_test.go
│   │
│   ├── keys/                    # 키 바인딩 정의 (Key ID → KeyBind)
│   │   ├── keys.go              # KeyID 상수 (nav.down, app.quit, ...)
│   │   ├── defaults.go          # 기본 매핑
│   │   ├── resolver.go          # 사용자 override 병합
│   │   └── resolver_test.go
│   │
│   ├── mask/                    # PII 마스킹 유틸
│   │   ├── mask.go              # Phone, Email, Custom
│   │   └── mask_test.go
│   │
│   ├── logger/                  # slog 설정 + 민감값 스크럽
│   │   ├── logger.go            # New, session_id 부여
│   │   ├── mask_attr.go         # slog.ReplaceAttr 기반 마스킹
│   │   └── logger_test.go
│   │
│   ├── clock/                   # Clock 주입 (tail/factor/토큰수명)
│   │   ├── clock.go             # Clock interface, realClock
│   │   └── fake.go              # FakeClock (test)
│   │
│   └── version/                 # ldflags 주입 변수
│       └── version.go
# Note: a separate `internal/cache` package was considered for service TTL
# caching but was removed in QA-014 (Phase 7 Cycle 2). Each service holds its
# own per-query map (UsersService.cache) which is lighter and avoids a thin
# wrapper. Re-introduce if a multi-service eviction policy is needed.
│
├── pkg/                         # (v0.1에서 비움. 외부 공개가 필요해지면 승격)
│
├── testdata/                    # 공유 fixture (여러 패키지가 사용)
│   ├── oktaapi/                 # Adapter/Service integration 테스트용 HTTP 응답
│   │   ├── fixtures_manifest.yaml
│   │   ├── users/               # list_page1.json, detail_active.json, ...
│   │   ├── groups/
│   │   ├── grouprules/
│   │   ├── policies/            # 7 타입 각각
│   │   ├── logs/                # tail_initial, rate_limited_429, ...
│   │   └── errors/              # E0000001..E0000047 8종
│   └── config/                  # 설정 로더 테스트용
│       ├── valid_minimal.yaml
│       ├── valid_full.yaml
│       └── invalid_profile.yaml
# 주: TUI 렌더 golden 파일은 각 패키지 로컬 testdata/에 위치 (§7 참조)
# 예: internal/tui/users/testdata/Test_UsersListModel_Render_Mixed.golden
│
├── scripts/
│   ├── record-fixture.go        # 실 tenant 응답 캡처 + PII 스크럽 (D-I)
│   └── update-golden.sh         # golden 파일 일괄 재생성
│
├── docs/
│   ├── PRD.md
│   ├── TUI_DESIGN.md
│   ├── ARCHITECTURE.md
│   ├── TECH_STACK.md
│   ├── PROJECT_STRUCTURE.md
│   ├── CONVENTIONS.md
│   ├── TESTING.md
│   └── QA_REPORT.md             # (Phase 7에서 생성)
│
├── _workspace/                  # 중간 산출물
│
├── go.mod
├── go.sum
├── Makefile
├── .golangci.yml
├── .editorconfig
├── .gitignore
└── README.md
```

---

## 3. 최상위 디렉토리 규약

### 3.1. `cmd/`

**내용:** 실행 가능한 바이너리 진입점만. 비즈니스 로직 금지.

- `cmd/ota/main.go`: flag 파싱 → config 로드 → `wire.go` 호출 → `tea.NewProgram(...).Run()`.
- `cmd/ota/wire.go`: 의존성 조립. `func Wire(cfg Config) (*app.Model, error)` 단일 함수. DI 프레임워크 없음(명시 조립).

### 3.2. `internal/`

**내용:** ota 구현 전부. 외부 프로젝트 import 차단.

### 3.3. `pkg/`

**내용:** v0.1에서 비움. 향후 외부 재사용 가능 유틸(예: Okta 공통 rate limit 파서)이 생기면 이곳으로 승격.

### 3.4. `testdata/`

**내용:** 모든 테스트 픽스처. Go 빌드 도구가 `_`나 `.`로 시작하는 디렉토리를 무시하는 규칙 외에도 `testdata/`는 특별 취급되어 build 포함 안 됨.

### 3.5. `scripts/`

**내용:** 일회성·개발자 도구. 프로덕션 바이너리에 포함 안 됨.

### 3.6. `docs/`

**내용:** 프로젝트 공식 문서. 변경 시 PR에 문서 수정 포함.

---

## 4. 패키지 책임 경계 매트릭스

| 패키지 | 수용 (import OK) | 금지 (import 금지) |
|--------|-----------------|-------------------|
| `internal/domain` | stdlib만 (Port 인터페이스·Query 타입 포함) | 모든 외부 패키지, 다른 internal |
| `internal/service` | `domain`, stdlib, `clock`, `mask` | `okta`, `tui`, `app`, SDK |
| `internal/okta` | `domain`, SDK, stdlib, `clock`, `logger` | `service`, `tui`, `app` |
| `internal/tui/*` | `service`, `domain`, `keys`, `mask`, `shared`, Bubbletea 생태 | SDK, `okta` |
| `internal/app` | `tui/*`, `service`, `domain`, `keys`, `mask`, `logger`, Bubbletea | SDK, `okta` |
| `internal/config` | stdlib, koanf | domain 외 internal (설정은 cmd가 도메인으로 변환해 주입) |
| `internal/keys` | stdlib | 그 외 internal |
| `internal/mask` | stdlib | 그 외 internal |
| `internal/logger` | stdlib, slog, lumberjack | 그 외 internal |
| `internal/clock` | stdlib | 그 외 internal |
| `internal/version` | — | 모든 것 |
| `internal/okta/testfx` | `domain`, stdlib, test helpers | `cmd/ota` (lint에서 import 금지) |
| `cmd/ota` | 전부 (wiring) | `internal/okta/testfx` (lint 강제) |

### 4.1. 순환 의존 방지 — 그림

```
cmd
 │
 ▼
 app ──► tui/* ──► service ──► domain (hub)
 │        │           ▲          ▲
 │        │           │          │
 │        ▼           │          │
 │    keys/mask   (Port interface 선언)
 │                    │
 └──► config          │
                      │
  okta (outbound) ────┘  (Port implements)
```

- `okta`는 Port **구현체**이지만 `service` 패키지를 import하지 않는다. 구현체를 등록하는 곳은 `cmd/ota/wire.go`.
- `service`는 자신이 필요한 인터페이스(Port)를 자기 패키지에서 선언한다. 이것이 Go 관용구("Accept interfaces, return structs").

---

## 5. 파일 명명

### 5.1. 일반 파일
- 소문자, snake_case. 예: `access_policy.go`, `pagination.go`.
- 기능 단위 분할. 500~800 LoC 이하 유지 권장.

### 5.2. 테스트 파일
- 표준 테스트: `<file>_test.go` — 같은 패키지.
- Black-box 테스트: `<file>_external_test.go` — `<pkg>_test` 패키지.
- 통합 테스트: `<file>_integration_test.go` + `//go:build integration` 태그.

### 5.3. 빌드 태그 분리
- `okta/testserver.go`: `//go:build test` (자체 정의) — 테스트 바이너리에만 포함.
- e2e: `//go:build e2e`.

### 5.4. Mock / Fake
- 패키지 내부 fake: `internal/service/users_fake_test.go` (테스트 파일 형태).
- 공유 fake: `internal/service/fakes/` — 필요 시 생성. MVP에는 없음.

---

## 6. 타입 명명 규약

| 종류 | 규칙 | 예 |
|------|------|-----|
| 도메인 엔티티 | PascalCase 단수 | `User`, `Group`, `Policy` |
| 도메인 enum | PascalCase + 상수 UPPER_SNAKE | `UserStatus`, `ACTIVE` |
| 서비스 | `<Resource>Service` | `UsersService` |
| 인터페이스 (Port) | `<Resource>Port` | `UsersPort` |
| Screen Model | `<View>Model` | `UsersListModel`, `UserDetailModel` |
| tea.Msg | `<Noun>Msg` | `UsersLoadedMsg`, `ErrorMsg` |
| tea.Cmd 팩토리 함수 | `<verb><Noun>` (소문자 시작, 패키지 비공개 기본) | `fetchUsers(ctx, svc, q)` |
| 에러 | `Err<Reason>` | `ErrNotFound`, `ErrRateLimited` |
| Options | `<Target>Option` + `With<X>` | `ClientOption`, `WithTimeout(d)` |

---

## 7. testdata/ 세부 구조 (test-engineer 공동)

**원칙 (CONVENTIONS §13.4):**
- **공유 fixture (HTTP/Config)** → **루트 `testdata/`**. 여러 패키지가 공동 사용.
- **패키지 로컬 golden (TUI 렌더)** → **`internal/tui/<resource>/testdata/`**. 해당 패키지가 소유.
- 중앙 집중 `testdata/golden/` 불채택: TUI 리팩터가 루트 diff를 크게 만들고, `go test` 디폴트 경로와 어긋남.

### 7.1. 루트 `testdata/oktaapi/` — HTTP 응답 픽스처
- 파일명: `<endpoint>_<scenario>.json`
- 메타파일: `<scenario>.meta.json` 또는 인접 `<scenario>_headers.txt` (상태 코드, Link, X-Rate-Limit-* 헤더).
- 스크럽된 데이터만 커밋. `fixtures_manifest.yaml`로 캡처 시점·테넌트 에디션 추적 (TESTING §5.3).
- Record 도구: `scripts/record-fixture.go` (CONVENTIONS §13.4, TESTING §5.1).

**예:**
```
testdata/oktaapi/users/
├── list_active_page1.json        # 본문
├── list_active_page1.meta.json   # {"status": 200, "headers": {"Link": "<...>; rel=\"next\"", "X-Rate-Limit-Remaining": "598"}}
├── list_active_page2.json
├── list_active_page2.meta.json   # no Link header (last page)
├── get_00u123.json
├── get_not_found.json
├── get_not_found.meta.json       # {"status": 404, ...}
├── factors_multi.json
└── 429_rate_limited.meta.json    # {"status": 429, "headers": {"Retry-After": "2"}}
```

### 7.2. 패키지 로컬 `<pkg>/testdata/*.golden` — TUI 렌더 스냅샷
- 위치: `internal/tui/<resource>/testdata/<Test_Name>.golden`
- 파일명: 테스트 함수명과 동일 (한 테스트당 하나).
- ANSI 이스케이프 포함 그대로 저장 (스타일 회귀 감지).
- 업데이트: `go test -update ./internal/tui/...` (CI는 금지).
- 업데이트 diff는 PR 본문에 포함하여 리뷰어가 검토.

### 7.3. 루트 `testdata/config/`
- 유효/무효 설정 파일. 파싱·검증 테스트. `internal/config` 소유.

### 7.4. `internal/<consumer>/fakes/` — Port Fake 공유
- 위치: 소비자 패키지 기준 (`internal/service/fakes/`, `internal/tui/shared/fakes/`).
- 명명: `<port>_port_fake.go` (예: `users_port_fake.go`).
- 구현 패턴: `Func` 필드 (TESTING §3.2).

---

## 8. 새 기능 추가 플레이북

### 8.1. 신규 Okta 리소스 (예: Applications v0.2)

순서:

1. `internal/domain/app.go` — `App` 엔티티 정의
2. `internal/domain/ports.go` — `AppsPort` 추가 선언
3. `internal/service/apps.go` — `AppsService` (캐시, 쿼리 정규화)
4. `internal/service/fakes/apps_port_fake.go` — Port fake
5. `internal/service/apps_test.go` — fake Port 주입 테스트
6. `internal/okta/apps.go` — Adapter 구현 (implements `domain.AppsPort`)
7. `internal/okta/mapping.go`에 SDK→domain 변환 추가
8. `internal/okta/apps_test.go` — httptest.Server + testdata/oktaapi/apps/
9. `internal/tui/apps/list.go`, `detail.go` — Screen Model
10. `internal/tui/apps/list_test.go` + `testdata/*.golden` — teatest + 스냅샷
11. `internal/app/router.go` — 라우트 등록
12. `cmd/ota/wire.go` — Service 바인딩 한 줄
13. `docs/` 업데이트 (ARCHITECTURE §13, PROJECT_STRUCTURE §2 트리, PRD 상응 REQ)

**변경 반경:** 신규 파일 7개, 수정 파일 4개.

### 8.2. 신규 Policy 타입 (예: CONTINUOUS_ACCESS)

1. `internal/domain/policy.go`의 `policyTypeCatalog` map에 타입 상수 추가
2. 만약 rich renderer 만들려면 `internal/tui/policies/renderers/continuous_access.go`
3. 없으면 자동 raw view로 fallback (REQ-R04 AC-8)

---

## 9. Makefile 타겟 계약

```make
.PHONY: all build test test-race test-integration test-e2e lint vuln ci run clean tidy fmt

all: build

build:      ## 정적 바이너리 빌드 (bin/ota)
test:       ## unit + adapter integration + tui component
test-race:  ## -race 추가
test-integration: ## -tags=integration 포함
test-e2e:   ## 실 Okta Sandbox — 환경변수 OKTA_ORG_URL/TOKEN 필요
lint:       ## gofumpt + golangci-lint
vuln:       ## govulncheck
ci:         ## lint + test + test-race + vuln
run:        ## 로컬 실행 (OKTA_ORG_URL/OKTA_API_TOKEN 요구)
fmt:        ## gofumpt -w
tidy:       ## go mod tidy
clean:      ## bin/ 제거
```

---

## 10. .gitignore 핵심 항목

- `bin/` — 빌드 산출물
- `coverage.out` / `coverage.html`
- `.DS_Store` / `Thumbs.db`
- `.idea/` / `.vscode/` (팀 공유 파일만 예외)
- `*.local.yaml` — 개발자 로컬 설정
- **특히 중요:** `tenant.yaml`, `*.token`, `*.secret` — 실수로 토큰 커밋 방지

---

## 11. README.md 권장 구조

1. 한 줄 설명
2. 설치 (Homebrew, go install, GitHub Release)
3. Quick Start (환경변수 + 3줄 실행)
4. 설정 파일 예제
5. 단축키 치트시트
6. 문서 링크 (`docs/` 각 파일)
7. 기여 가이드 (PR 규칙은 `CONVENTIONS.md` 참조)
8. 라이선스

MVP 기준 README는 **300줄 이하** 유지.

---

**END of PROJECT_STRUCTURE.md draft**
