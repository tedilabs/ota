# 04. Tech Docs — 공통 아웃라인 (developer + test-engineer)

**작성일:** 2026-04-24
**작성자:** developer (리드), test-engineer 공동 리뷰 예정
**목적:** 5개 기술 문서 간 일관된 관점(레이어, 명명, 용어)을 미리 못박기 위한 초안. 실제 문서는 본 아웃라인의 결정을 확장.
**근거:** `docs/PRD.md` v1.0.0, `docs/TUI_DESIGN.md` v1.0.0, `_workspace/02_okta_domain_input.md`

---

## 0. 공통 용어 (Glossary — 5개 문서 공유)

| 용어 | 의미 | 비고 |
|------|------|------|
| **Hex** / **Port / Adapter** | Hexagonal Architecture. `domain`(순수) ← `service`(유스케이스) ← `adapter`(외부). UI는 또 다른 inbound adapter. | §ARCHITECTURE §3 |
| **Model** (TUI) | Bubbletea `tea.Model` 구현체. UI 상태 + Update 함수. | TUI_DESIGN §5.1 |
| **Screen Model** | 리소스별 화면 (UsersListModel, PolicyDetailModel 등). root Model에 owning. | |
| **App Shell** / **Router** | 최상위 `tea.Model`. 메시지 라우팅·전역 단축키·오버레이(command palette) 관리. | TUI_DESIGN §2.2, §5.1 |
| **Adapter** (Okta) | Okta SDK/HTTP 호출을 도메인 타입으로 변환하는 경계면. `internal/okta/*` | 도메인 §8.3 |
| **Port** | 도메인/서비스가 요구하는 인터페이스. 소비자 패키지에 선언 (Go 관용). | |
| **Iterator** | `IterateUsers(ctx)` 같은 페이지네이션 자동화 반복자. Link 헤더 커서 은닉. | |
| **Cmd** | `tea.Cmd`. 비동기 I/O 수행 후 `tea.Msg` 반환하는 함수. | |
| **Msg** | `tea.Msg`. Update로 전달되는 강타입 이벤트. | |
| **Profile** | 설정 파일의 tenant 프로필 (REQ-C02). 인증·org_url·기본 필터 묶음. | |
| **Masking** | PII 마스킹. 기본 on, `:unmask`로 일시 해제, 세션/시간 기반 자동 재마스킹. | PRD §6.2 |
| **Adaptive polling** | tail 모드 rate limit 저한도 tenant에서 폴링 7s → 15s 자동 상향. | REQ-R05 AC-2 |

---

## 1. ARCHITECTURE.md 아웃라인

**핵심 메시지:** Hexagonal + Bubbletea. 도메인은 순수, 어댑터로 Okta 의존성 격리, Bubbletea는 "inbound adapter"로 본다.

| 섹션 | 핵심 내용 | 근거 |
|------|---------|------|
| 1. 시스템 개요 | k9s 스타일 읽기 전용 Okta TUI. 단일 바이너리. | PRD §1 |
| 2. 설계 목표/비목표 | 목표: 유지보수성, 테스트 가능성, 확장성(타 IdP), 낮은 지연. 비목표: Write(v0.2+), 멀티 화면, 데몬. | PRD §1.4, §4.2 |
| 3. 아키텍처 패턴 | Hexagonal (Ports & Adapters). Elm Architecture(Bubbletea) 위에서 UI는 또 하나의 driven adapter. | 도메인 §8.3 |
| 4. 레이어 다이어그램 | `TUI ← Service ← Domain ← Adapter`. ASCII + Mermaid 권장. | |
| 5. 의존성 방향 | `domain` import 없음(표준 라이브러리 외). `tui` → `service` → `domain`, `service` ↔ `port`, `adapter` implements `port`. 순환 금지. | |
| 6. 핵심 컴포넌트 | 6.1 domain, 6.2 service, 6.3 tui/app, 6.4 tui/<resource>, 6.5 okta adapter, 6.6 config, 6.7 logger | |
| 7. 데이터 흐름 | key press → Screen Model Update → tea.Cmd → Service → Port → Okta Adapter → SDK → HTTP → 역순 (Msg). | |
| 8. 동시성 모델 | goroutine 생성은 tea.Cmd 내부 only. 공유 상태는 모델 내 owning, 외부는 Msg로 접근. `context.Context` 전파. tail은 별도 tea.Tick + 취소 가능 Cmd. | |
| 9. 에러 처리 전략 | 어댑터: Okta error → `domain.ErrXxx` 매핑(errorCode 기반). 서비스: 도메인 에러 그대로 반환 또는 wrap. TUI: `ErrorMsg`를 statusbar + `:errors` 로그에 표시. | PRD §7.7 |
| 10. 설정/인증 흐름 | 부팅: flag → env → profile → interactive. 프로필 전환 시 캐시 무효화 + 전체 상태 리셋. | REQ-C01~C04 |
| 11. Rate Limit 전략 | Adapter middleware가 응답 헤더 관찰 → `RateLimitObservedMsg` 브로드캐스트. 429 시 Retry-After + jitter. | REQ-E01 |
| 12. 확장 포인트 | 새 Okta 리소스(Apps): Screen Model + Service + Adapter 메서드 추가. 다른 IdP: Adapter 교체만. | PRD §12 |
| 13. 변경 이력 | | |

---

## 2. TECH_STACK.md 아웃라인

**핵심 메시지:** 검증된 라이브러리만. 불필요한 추상화 금지. 모든 선택에 대안과 탈락 이유.

| 섹션 | 항목 | 결정 초안 |
|------|------|---------|
| Language | Go | **1.23+** (stdlib `slog`, `iter.Seq` 고려). toolchain 버전은 go.mod에 명시. |
| TUI | Bubbletea | latest stable. tea.Model composition. |
| TUI | Bubble (components) | latest. list/table/viewport/textinput/help/spinner/progress |
| TUI | Lip Gloss | latest. 모든 스타일 1차. |
| TUI | Huh | latest. 프로필 선택, 토큰 입력(EchoMode=password)만 MVP. |
| TUI | Glamour | latest. Help 모달 markdown 렌더. Fallback: plain text. |
| TUI | charmbracelet/x (teatest) | test-only. charmbracelet/x/exp/teatest. |
| HTTP/Okta | okta-sdk-golang | **v5** 기본. 문서 §8 권고. |
| HTTP/Okta | net/http + encoding/json | SDK 보완(System Logs raw 필드, 커스텀 헤더 관찰) |
| Config | knadh/koanf | **후보 1** 경량, 플러그인형 provider/parser, YAML+env merge. |
| Config | spf13/viper | 후보 2 업계 표준, 의존성 크다 |
| 결정: YAML 파서 | knadh/koanf | koanf 선택(의존성 작고 테스트 용이). YAML provider `parsers/yaml`. |
| Logger | log/slog | **stdlib**. PRD §6.6 (상관 ID). JSON handler + file sink. |
| Secret | 설정 파일에 평문 금지 | 환경변수 참조만 허용 (REQ-C05). |
| Test | testing + testify (assert/require) | |
| Test | charmbracelet/x/exp/teatest | TUI 통합 테스트 |
| Test | net/http/httptest | Okta API 모킹 |
| Test | jarcoal/httpmock | (보조) httpmock은 특정 URL 매칭 테스트에 한정 |
| Lint | golangci-lint + gofumpt | `.golangci.yml` 체크 |
| 취약점 | govulncheck | CI에서 주 단위 |
| CI | GitHub Actions | test + lint + build + govulncheck |
| 빌드 | Makefile | `build`/`test`/`lint`/`test-integration` 타겟 |
| 의존성 관리 | go modules + Renovate/Dependabot | 주간 마이너 업그레이드 PR |
| 의도적 배제 | Cobra | flag 패키지로 충분(CLI 플래그 3~4개). |
| 의도적 배제 | Wire/Fx | 명시적 생성자 + Options 패턴. |
| 의도적 배제 | SCIM 클라이언트 | Core API만 사용 (PRD §7.1). |

---

## 3. PROJECT_STRUCTURE.md 아웃라인

**핵심 메시지:** 책임 경계가 디렉토리에 드러나게. `internal/` 활용으로 외부 import 차단.

```
ota/
├── cmd/
│   └── ota/                      # main.go — flag 파싱, config 로드, tea.Program 기동
├── internal/
│   ├── app/                      # App Shell / Router tea.Model, 전역 단축키
│   │   ├── app.go                # rootModel
│   │   ├── router.go             # 화면 전환 로직
│   │   ├── overlay.go            # command palette, help, confirm 오버레이 관리
│   │   └── app_test.go
│   ├── tui/                      # 화면별 Screen Model (Bubbletea)
│   │   ├── shared/               # 공통 컴포넌트 (statusbar, breadcrumb, spinner wrappers)
│   │   ├── users/                # list.go, detail.go, factors.go
│   │   ├── groups/               # list.go, detail.go, members.go
│   │   ├── rules/                # list.go, detail.go
│   │   ├── policies/             # typeselect.go, list.go, detail.go, renderers/
│   │   └── logs/                 # search.go, tail.go, detail.go
│   ├── service/                  # 유스케이스 (도메인 조합)
│   │   ├── users.go              # UsersService: List, Get, Factors, Groups
│   │   ├── groups.go
│   │   ├── rules.go
│   │   ├── policies.go
│   │   └── logs.go
│   ├── domain/                   # 순수 도메인 (외부 import 없음)
│   │   ├── user.go               # domain.User + 상태 상수
│   │   ├── group.go
│   │   ├── rule.go
│   │   ├── policy.go
│   │   ├── logs.go
│   │   ├── errors.go             # ErrNotFound, ErrForbidden, ErrRateLimited 등
│   │   └── pagination.go         # Iterator[T], PageInfo
│   ├── okta/                     # Okta adapter (SDK wrapping)
│   │   ├── client.go             # SDK + middleware 조립
│   │   ├── users.go              # ListUsers, GetUser, ListUserFactors, ListUserGroups
│   │   ├── groups.go
│   │   ├── rules.go
│   │   ├── policies.go
│   │   ├── logs.go
│   │   ├── pagination.go         # Link 헤더 파서, IterateXxx helper
│   │   ├── ratelimit.go          # RateLimitMonitor middleware
│   │   ├── errors.go             # Okta errorCode → domain.Err 매핑
│   │   └── testdata_server.go    # (test-only) httptest server helpers
│   ├── config/                   # YAML 설정 로드
│   │   ├── config.go             # Config struct
│   │   ├── loader.go             # koanf + path resolver (XDG)
│   │   ├── profile.go
│   │   ├── keybinding.go
│   │   └── loader_test.go
│   ├── keys/                     # 키 바인딩 정의 (설정 커스터마이징 가능)
│   │   ├── keys.go               # KeyID 상수, 기본 맵
│   │   └── resolver.go           # 사용자 override 적용
│   ├── mask/                     # PII 마스킹 유틸
│   │   ├── mask.go
│   │   └── mask_test.go
│   ├── logger/                   # slog 설정
│   │   └── logger.go
│   └── version/                  # 빌드 시 ldflags 주입
│       └── version.go
├── pkg/                          # (당분간 비움 — 외부 재사용 필요 시 승격)
├── testdata/
│   └── oktaapi/                  # 실제 응답 기반 JSON 픽스처 (마스킹 완료)
│       ├── users/
│       ├── groups/
│       ├── rules/
│       ├── policies/
│       └── logs/
├── docs/                         # PRD, TUI_DESIGN, ARCHITECTURE 등
├── _workspace/                   # 중간 산출물
├── go.mod
├── go.sum
├── Makefile
├── .golangci.yml
├── .editorconfig
├── .gitignore
└── README.md
```

**의존성 방향:**
```
cmd → app → tui → service → domain
                           ↑
                         okta (implements ports defined near service)
cmd → config, keys, logger (config는 app이 주입)
```

**규칙:**
- `domain`은 표준 라이브러리 외 import 금지
- `tui/*`는 `service`만. Okta SDK 직접 import 금지.
- `okta/`는 SDK 타입을 외부에 노출 금지 (매퍼 필수)
- 순환 의존 금지 (컴파일러가 잡음)
- `internal/`로 외부 프로젝트의 import 차단

---

## 4. CONVENTIONS.md 아웃라인

**핵심 메시지:** Go 관용구 + PRD 보안 요구 + TUI 특이사항.

| 섹션 | 결정 초안 |
|------|---------|
| 포맷 | `gofumpt` 강제. soft 100자, hard 120자. |
| 린트 | `golangci-lint` + 활성 리너 열거 (errcheck, govet, staticcheck, gosec, gocritic, revive, ...) |
| 명명 | 패키지 소문자·짧게. 인터페이스는 소비자가 정의. 도메인 타입은 `domain.User`(단수). |
| 테스트 명명 | `Test_<Unit>_<Scenario>_<Expectation>` 또는 `TestXxx/subtest`. 테이블 드리븐 기본. |
| 에러 | `fmt.Errorf("doing X: %w", err)`. 도메인 sentinel은 `domain.ErrNotFound` 등. `errors.Is/As` 사용. |
| panic | 프로그래머 에러(초기화 필수 조건 실패) 외 금지. |
| Context | 모든 외부 호출 + 장기 작업 첫 인자. TUI에서 Esc → cancel() 전파. |
| 로그 | `slog` 사용. 필드명: `snake_case`. 상관ID `session_id`. 민감 필드(`token`, `authorization`) 마스킹. |
| PII | `mask.Phone`, `mask.Email` 유틸 경유. 로거에는 raw PII 금지. |
| 설정 키 | `snake_case`. 트리: `profiles.<name>.org_url`, `ui.pii_masking.enabled`, `keybindings.<key_id>` |
| Key ID | `<scope>.<verb>` 형식. 예: `nav.down`, `search.open`, `app.quit`. TUI_DESIGN §3과 일치. |
| 생성자 | `NewX(required, ...Option) *X`. Options 함수 패턴. |
| Goroutine | `tea.Cmd` 내부 외 금지. 내부도 context 취소 존중. |
| tea.Msg 타입 | 강타입 struct. suffix `Msg`. 예: `UsersLoadedMsg`, `ErrorMsg`, `RateLimitObservedMsg`. |
| tea.Cmd 팩토리 | `func fetchUsers(ctx, svc, q) tea.Cmd { return func() tea.Msg { ... } }`. 테스트 가능하게 서비스 주입. |
| 파일 헤더 | 라이선스 없음 (내부). |
| 커밋 메시지 | Conventional Commits. `feat(tui/users): add factors tab`. |
| PR 규칙 | 300 LOC 이하 권장. 테스트 포함. 문서 동기화. |
| 디렉토리 import 순서 | stdlib → 3rd party → internal (gofumpt 자동) |
| README 섹션 | (파일 최상단 짧게) |

**테스트 섹션 (test-engineer 공동)** — TESTING.md에 상세, CONVENTIONS.md에는 요약만:
- `t.Parallel()` default on
- Mock은 인터페이스 수준 (SDK mock 금지)
- 테이블 드리븐 표준 형식 예시

---

## 5. TESTING.md 아웃라인 (test-engineer 주도, developer 기여)

| 섹션 | 핵심 내용 |
|------|---------|
| 1. 피라미드 | unit(다수) / adapter integration (httptest) / tui component (teatest) / e2e (수동 Okta Sandbox) |
| 2. 실행 | `make test` = unit + adapter integration + tui. `make test-e2e` = 실 tenant. `make test-race`. |
| 3. Unit: domain | 순수 함수, 100% 도전. mock 없음. |
| 4. Unit: service | 도메인 port mock. testify/mock 또는 수동 fake. |
| 5. Adapter integration | Okta SDK + httptest.Server. 실제 JSON 픽스처(`testdata/oktaapi/`) 로드. Link 헤더, 429, errorCode 매핑, 페이지네이션 커버. |
| 6. TUI component | teatest. 키 입력 → 렌더 단정. 골든 파일 스냅샷(`testdata/golden/<screen>.txt`). Resize, Esc 취소 시나리오. |
| 7. Fixture 전략 | 실제 tenant 응답 캡처 → PII/토큰 스크럽 → `testdata/oktaapi/<resource>/<scenario>.json`. 재생 시 Adapter→SDK가 서명/헤더만 검증. Record/Replay: capture 툴은 `testdata_server.go`에 helper. |
| 8. Rate limit 테스트 | 429 응답 → Retry-After 준수 확인. Jitter 범위 검증. Monitor가 `RateLimitObservedMsg` 발행 확인. |
| 9. Tail 테스트 | `since` 재설정, 페이지 연속, 중단/재개, 429 후 hole-free 재개. |
| 10. Fail-First | 새 기능·버그 수정 전에 실패 테스트 제출. `tdd-fail-first` 스킬 참조. |
| 11. Coverage 목표 | domain 95%, service 85%, adapter 75% (integration 포함), tui 60%. |
| 12. Flaky 방지 | 시간: `clock.Clock` 주입. 랜덤: seed 고정. -race 필수. goroutine leak 탐지(`go.uber.org/goleak`). |
| 13. CI 매트릭스 | Go 1.23 linux/macOS. teatest는 리눅스 VT만. |
| 14. 변경 이력 | |

---

## 6. 주요 결정 포인트 (test-engineer와 합의 필요)

| # | 결정 사항 | developer 초안 | test-engineer 의견 대기 |
|---|----------|---------------|---------------------|
| D-A | Okta 도메인 인터페이스 추상화 수준 | **리소스별 Port 인터페이스**(UsersPort, GroupsPort 등)를 `internal/service/port.go`에 선언, `internal/okta/` 구현. Service는 Port만 참조. | 확인 필요: Service 테스트 시 mock 생성 편의성 |
| D-B | teatest 적용 범위 | 화면 **전환**·**키 흐름**·**tail 갱신**에 한정. 순수 렌더링 스냅샷은 골든 파일. | |
| D-C | Record/Replay 방식 | httptest.Server + JSON 픽스처. SDK mock 금지. | |
| D-D | Unit:Integration 비율 목표 | 테스트 수 기준 7:3. 실행 시간 기준 1:1 허용. | |
| D-E | Go 버전 하한선 | 1.23. stdlib `log/slog`, `slices`, `maps`. | |
| D-F | Config 라이브러리 | **koanf**. 의존성 경량. | |
| D-G | 로거 라이브러리 | **log/slog** (stdlib). zerolog 탈락(추가 의존성). | |
| D-H | Mock 생성 전략 | 수동 fake 또는 testify/mock. gomock/mockgen은 금지(코드생성 부담). | |
| D-I | 픽스처 Record 도구 | `scripts/record-fixture.go`로 실 tenant 응답 캡처 + 스크럽. 최초 작성은 Phase 5 시작 시 1회. | |
| D-J | clock 주입 | `internal/clock`에 `Clock` 인터페이스, 기본 `realClock` / 테스트 `FakeClock`. | |
| D-K | goleak 사용 | 테스트 main에서 `TestMain` 설정. | |
| D-L | Policies 7타입 중 3타입 raw-only 렌더 테스트 전략 | raw JSON diff만 검증, 구조화 렌더 테스트 생략(MVP). | |

---

## 7. 작업 분담

| 문서 | 주 작성자 | 기여자 |
|------|----------|------|
| ARCHITECTURE.md | developer | test-engineer: testability 관점 검토 |
| TECH_STACK.md | developer | test-engineer: 테스트 라이브러리 섹션 검토 |
| PROJECT_STRUCTURE.md | developer | test-engineer: testdata/ 구조 공동 정의 |
| CONVENTIONS.md | developer | test-engineer: §테스트 규칙 |
| TESTING.md | test-engineer | developer: adapter·TUI 테스트 섹션 |

---

## 8. REQ-ID 역추적 인덱스 (샘플)

각 문서에서 REQ 근거 명시:
- ARCH §11 Rate Limit → REQ-E01, REQ-R05 AC-2 adaptive polling
- ARCH §10 인증 흐름 → REQ-C04
- PROJECT_STRUCTURE `internal/mask` → PRD §6.2, REQ-R01 AC-6
- CONVENTIONS §로그 → REQ-O01, REQ-C05
- TESTING §tail → REQ-R05 AC-3 hole-free resume

---

**다음 단계:**
1. test-engineer에게 SendMessage로 본 아웃라인 공유 + 회의 제안
2. D-A~D-L 결정 합의
3. 각자 담당 초안 작성
4. 상호 리뷰 후 `docs/` 확정
