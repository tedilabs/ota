# 05. Phase 5 Dev Stub Log

**Author:** developer
**Scope:** Phase 5 — Interface Stubs Only (no business logic)
**Status:** Stubs complete, all production code compiles + vets.

---

## 1. 환경

- Go 1.24.2 (go.mod `go 1.23` 이상 허용, CI matrix는 최소 1.23부터)
- Module: `github.com/tedilabs/ota`

## 2. 최종 디렉토리 구조

```
ota/
├── cmd/ota/                (main.go, wire.go)
├── internal/
│   ├── app/                (app.go, auth.go, keymap.go, msg.go)
│   ├── cache/              (ttl.go)
│   ├── clock/              (clock.go, fake.go, jitter.go)
│   ├── config/             (config.go, loader.go, paths.go, profile.go)
│   ├── domain/             (ports.go + 전 엔티티)
│   ├── keys/               (keys.go, defaults.go, resolver.go)
│   ├── logger/             (logger.go, mask_attr.go)
│   ├── mask/               (mask.go)
│   ├── okta/               (client.go + 각 adapter, pagination/ratelimit/errormap/testfx 서브패키지)
│   ├── service/            (users.go, groups.go, rules.go, policies.go, logs.go, logs_tail.go, logs_presets.go, bundle.go, options.go)
│   ├── tui/                (shared/, users/, groups/, rules/, policies/, logs/, overlay/)
│   └── version/            (version.go)
├── go.mod / go.sum
├── Makefile
└── .golangci.yml
```

## 3. 핵심 설계 결정 (PRD/ARCHITECTURE/CONVENTIONS 대비 변경 없음)

- **Port 위치**: `internal/domain/ports.go` (합의 D-A). 모든 adapter는 컴파일 타임 체크 포함 (`var _ domain.UsersPort = (*UsersAdapter)(nil)` 등).
- **Service Query aliases**: `service.UsersQuery = domain.UsersQuery` 등 — 호출부 표기 안정성 + 향후 domain 재편 여지 유지.
- **Screen Model 생성자**: `Deps` 구조체 주입 (SetXxx 금지 원칙, CONVENTIONS §8.1). 초기 상태는 `Deps.InitialUsers` 같은 필드로 주입.

## 4. test-engineer 테스트 요구로 **추가된** 스텁 (시그니처 확정)

Phase 4 문서에는 없었지만, test-engineer가 Red 테스트에서 기대하는 API를 유추하여 스텁으로 선제 반영. 모두 설계 취지에 부합하여 수용.

| 위치 | API | 근거 |
|------|------|------|
| `internal/app/auth.go` | `ResolveToken(ResolveTokenInput) (token, source, err)` | REQ-C04 AC-1 — 토큰 결정은 앱 부팅 책임 (config → app으로 이동) |
| `internal/app/keymap.go` | `ClassifyKey(msg, resolved)`, `ClassifyKeyInContext(msg, resolved, ctx)`, `KeyContextDefault/InputActive/OverlayModal` | REQ-U01/U03 — 입력 컨텍스트에 따른 키 분류 |
| `internal/app/msg.go` | `NetworkErrorMsg`, `NetworkRestoredMsg`, `OfflineStateMsg`, `RefreshActiveScreenMsg` | REQ-E03 |
| `internal/service/logs_tail.go` | `LogsTail` 전용 객체 + `InitialQuery`, `NextSinceAfter`, `ObserveRateLimit`, `PollInterval`, `Pause/Resume/Paused`, `Poll`, `WithLogsTailNow` | REQ-R05 AC-2/AC-3 — tail을 별도 state-ful 객체로 분리 (LogsService.PollOnce 대체). 더 깔끔. |
| `internal/service/logs.go` | `LogsService.HistoryQuery()` | REQ-R05 AC-4 |
| `internal/service/logs_presets.go` | `LogsPreset` 구조체 + `LogsPresets()` | REQ-R05 AC-5 |
| `internal/service/policies.go` | `ErrPolicyTypeRequired`, `PoliciesService.ListAll` | REQ-R04 AC-2/AC-3 |
| `internal/service/rules.go` | `NewRulesService` 별칭, `RuleWithTargetNames`, `ListWithTargetNames` | REQ-R03 AC-4 |
| `internal/service/groups.go` | `GroupsService`가 `GroupRulesPort`도 주입받음 (DynamicTargeted 계산용), 메서드명 `List` → `Search` | REQ-R02 AC-1 |

### 시그니처 변경 통지
- `NewGroupsService` 시그니처: `(port, opts...)` → `(port, rulesPort, opts...)` — Phase 4 ARCHITECTURE.md §6.2에서 `GroupsPort`만 언급했으나 구현상 RULE 배지 계산에 GroupRulesPort 필요. ARCHITECTURE.md 차기 개정 시 보강 예정.

## 5. 빌드 / vet 상태

- `go build ./...` → **PASS**
- `go vet ./cmd/... ./internal/app/... ./internal/service/... ./internal/okta/... ./internal/tui/... ...` (프로덕션 코드) → **PASS**
- `go vet ./...` (test 파일 포함) → 실패 1건: `internal/service/logs_service_test.go:6:2: "context" imported and not used` — test-engineer 수정 대기.

## 6. 스텁 동작

모든 함수는 아래 중 하나:
- `panic("<Pkg>.<Fn>: not implemented yet")` — 호출 시 즉시 패닉 → 테스트 Red 확인
- zero-value return (tea.Model Update는 `return m, nil`, View는 `""`) — 테스트가 출력 검증에서 Red
- 생성자는 최소한의 struct 초기화 (필드 세팅)만. 로직 없음.

## 7. depguard 린트

`.golangci.yml`에 3개 depguard 룰:
- `sdk-isolation` — app/tui/service/domain 등은 `github.com/okta/okta-sdk-golang/**` import 금지
- `domain-purity` — `internal/domain/**`는 제3자(`github.com/`, `gopkg.in/`) import 금지
- `testfx-exclusion` — `cmd/ota/**`는 `internal/okta/testfx` import 금지

## 8. Phase 6에서 반드시 채워야 할 영역

- `internal/okta/mapping.go`: SDK struct ↔ domain struct 매핑 (User/Group/Rule/Policy/Logs/Factor)
- `internal/okta/pagination/link.go`: Link 헤더 파서
- `internal/okta/ratelimit/monitor.go`: 헤더 파싱 + 카테고리 분류 + 스냅샷 저장
- `internal/okta/errormap/map.go`: 8개 errorCode → domain.Err* 매핑
- `internal/okta/*_adapter.go`: SDK 호출 + mapping + rate limit observe
- `internal/service/*.go`: 쿼리 정규화, 캐시, priority 정렬 (Policies), id→name 해소 (Rules)
- `internal/service/logs_tail.go`: since+1ms, adaptive 전환, pause/resume
- `internal/app/app.go`: Update 라우팅, 전역 키, overlay 합성, NetworkErrorMsg → OfflineStateMsg 변환
- `internal/app/keymap.go`: 키 분류 알고리즘
- `internal/app/auth.go`: 토큰 우선순위 (CLI env > profile env > prompt)
- `internal/tui/**`: Model 상태머신 + View 렌더
- `internal/config/loader.go`: koanf 로드 + 검증
- `internal/keys/resolver.go`: 사용자 override merge + 경고
- `internal/mask/mask.go`: Phone / Email 마스킹 로직
- `internal/logger/logger.go` + `mask_attr.go`: slog 설정 + 민감 키 치환
- `internal/clock/fake.go`: Advance + timer 시뮬레이션
- `internal/cache/ttl.go`: TTL map
- `cmd/ota/main.go` + `wire.go`: flag 파싱, wire-up, tea.Program 실행

## 9. 의존성 요약 (`go.mod` direct)

- Charm: `bubbletea`, `bubbles`, `lipgloss`, `huh`, `glamour`, `x/exp/teatest`
- Okta: `okta-sdk-golang/v5`
- Config: `knadh/koanf/v2` + `providers/file`, `providers/env`, `parsers/yaml`
- Logs: `log/slog` (stdlib), `lumberjack.v2`
- Utils: `google/uuid`
- Test: `stretchr/testify`, `go.uber.org/goleak`, `google/go-cmp`, `jarcoal/httpmock`

## 10. 변경 이력

| 날짜 | 변경 |
|------|------|
| 2026-04-24 | 초안 (Phase 5 stub 완료) |
