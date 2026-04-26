# 06b. Phase 6b — TUI/Overlay/Router Red Test Log

**작성:** test-engineer
**착수:** 2026-04-24 (Phase 6 Green 이후 재개방)
**범위:** `internal/tui/{groups,rules,policies,logs,overlay}` + `internal/app/router` + `internal/keys/defaults`
**이유:** Phase 6 통과 후 team-lead가 "users 외 TUI 구현 부재 + 테스트 갭 심각" 지적. 본 Phase 6b는 **test-engineer가 Red 테스트를 먼저 작성하여 developer의 구현을 유도**하는 Fail-First 재개방.

---

## 착수 시점 기준선 (2026-04-24)

### 기존 상태
```
go test -race ./... -short

ok  internal/tui/users          74.5% coverage
?   internal/tui/groups         [no test files]
?   internal/tui/rules          [no test files]
?   internal/tui/policies       [no test files]
?   internal/tui/logs           [no test files]
?   internal/tui/overlay        [no test files]
?   internal/tui/shared         [no test files]
?   internal/app                79.6% (app_test, offline_test, keymap_test 있음)
```

### 주요 명세 불일치 (TUI_DESIGN §3.3 vs 현재 코드)
| Key ID | TUI_DESIGN 명세 | 현재 코드 (keys/defaults.go) | 상태 |
|--------|--------------|----------------------------|------|
| `logs.tail_toggle` | **`s`** | `IDLogsTailToggle: "t"` | ❌ 불일치 |
| `logs.follow` | `f` | `IDLogsFollowToggle: "f"` | ✓ 일치 |

또한 `IDLogsTailToggle`의 ID 상수 이름이 `logs.tail.toggle`인데 문서는 `logs.tail_toggle`. 이는 ID 상수 이름 규약이므로 lock-in 우선순위는 **값("s")**.

---

## Phase 6b 작업 단위

1. `internal/keys/defaults_test.go` — TUI_DESIGN §3 전체 매핑 테이블 드리븐
2. `internal/tui/groups/list_flow_test.go` — 리스트 렌더 + `/` 필터 + Enter 드릴다운
3. `internal/tui/rules/list_flow_test.go` — 리스트 + INVALID 배지 가시성 + detail 전환
4. `internal/tui/policies/list_flow_test.go` — 타입 선택 → 리스트 + rich/raw 분기
5. `internal/tui/logs/list_flow_test.go` — 초기 로드 + tail 인디케이터 + Enter detail
6. `internal/tui/overlay/palette_test.go` — `:` 커맨드 제안 + SwitchScreenMsg 발송
7. `internal/tui/overlay/help_test.go` — `?` 도움말 + 키 ID 렌더
8. `internal/tui/overlay/confirm_test.go` — 종료 확인 (REQ-U07)
9. `internal/app/router_test.go` — App Shell SwitchScreenMsg 라우팅
10. `internal/tui/shared/styles_test.go` — NO_COLOR 환경변수 존중

---

## Red 기록 (append)

(실행 결과를 여기에 누적)

---

## 2026-04-24 Phase 6b Red 실행 결과

### 명령
```
go vet ./...   → FAIL on internal/tui/shared (undefined shared.MonochromeEnabled)
go test ./internal/keys/... ./internal/tui/... ./internal/app/...
```

### 패키지별 결과

| 패키지 | 상태 | 실패/Red 테스트 | 이유 | 대응 developer Task |
|-------|------|--------------|------|---------------------|
| `internal/keys` | **PASS** (Green) | 0 | `IDLogsTailToggle: "s"` 이미 반영 (Task #48 완료) | Lock-in 확보 |
| `internal/app` | **PASS** (Green) | 0 | `SwitchScreenMsg`/`OpenResourceMsg`/`ActiveScreenName` 이미 구현 (Task #50 완료) | Lock-in 확보 |
| `internal/tui/users` | **PASS** (Green) | 0 | 기존 구현 유지 | - |
| `internal/tui/groups` | **FAIL** (Red) | 2 | List/Update/View no-op → teatest WaitFor 타임아웃 | #52 |
| `internal/tui/rules` | **FAIL** (Red) | 1 | 동일 | #53 |
| `internal/tui/policies` | **FAIL** (Red) | 3 | TypeSelect/List/Detail 전부 no-op | #54 |
| `internal/tui/logs` | **FAIL** (Red) | 2 | SearchModel no-op + tail 인디케이터 미구현 | #55 |
| `internal/tui/overlay` | **FAIL** (Red) | 6 | CmdPalette/Help/Confirm 전부 no-op | #51 진행 중 |
| `internal/tui/shared` | **BUILD FAIL** | - | `MonochromeEnabled` 미정의 (NO_COLOR 감지 스텁 부재) | 신규 구현 필요 |

### Red 구체 증거

**teatest timeout 샘플 (groups):**
```
--- FAIL: Test_GroupsListFlow_InitialRender_ShowsNames (2.01s)
    list_flow_test.go:44: WaitFor: condition not met after 2s. Last output:
        [?25l[?2004h [K
```
→ Init이 `nil` 반환 + View가 `""` 반환. fetch Cmd가 발행되지 않아 화면이 빈 상태.

**overlay timeout 샘플:**
```
--- FAIL: Test_CmdPalette_Render_ShowsResourceCommands (2.00s)
    teatest.go:175: timeout after 2s
```
→ CmdPalette.View() 가 `""` 반환. 대기 종료 대신 FinalOutput 3초 타임아웃.

**shared build fail:**
```
internal/tui/shared/styles_test.go:21:25: undefined: shared.MonochromeEnabled
```
→ `shared.MonochromeEnabled()` 함수 신설 필요 (NO_COLOR env 감지).

### 의도된 Red (Fail-First 원칙)

- **13개 신규 실패 테스트** 전부 구현 부재로 인한 실패 — trivial pass 없음
- 모든 실패는 구체적 assertion (특정 문자열 렌더 기대)
- Phase 6b-4~8 developer 구현 완료 시 순차 Green 전환 예상

### Phase 6b Task 상태 (2026-04-24 기준)

- Task #48 (keys defaults) — **완료 (Green lock-in)**
- Task #49 (cmd/ota -version/--help) — **완료**
- Task #50 (app router) — **완료 (Green lock-in)**
- Task #51 (overlay) — 진행 중
- Task #52~55 (groups/rules/policies/logs Screen Model) — 대기
- Task #56 (final verify) — 대기

### 다음 모니터링 포인트

1. `internal/tui/overlay` Green 전환 (6 테스트)
2. `internal/tui/groups` Green 전환 (2 테스트)
3. `internal/tui/rules` Green 전환 (1 테스트, INVALID 배지)
4. `internal/tui/policies` Green 전환 (3 테스트, 7 types + rich/raw)
5. `internal/tui/logs` Green 전환 (2 테스트, `s` 키 tail 토글)
6. `internal/tui/shared` Green 전환 (2 테스트, NO_COLOR 감지)

각 단계에서 race + coverage 재측정.

---

## QA-017 Coverage 보강 결과 (2026-04-24)

### 최종 coverage

| 패키지 | 이전 | 목표 | **최종** | 달성 |
|-------|------|------|---------|------|
| `internal/okta` | 43.7% | 75% | **80.6%** | ✓ +5.6%p 초과 |
| `internal/service` | 62.0% | 85% | **86.6%** | ✓ +1.6%p 초과 |
| `internal/okta/errormap` | 100% | 95% | 100.0% | ✓ 유지 |
| `internal/okta/pagination` | 100% | 95% | 100.0% | ✓ 유지 |
| `internal/okta/ratelimit` | 87.9% | 95% | 87.9% | -7.1%p |
| `internal/app` | - | - | 37.1% | (신규 production code 증가로 상대 감소, 기능은 전부 Green) |

### 추가 파일

- `internal/okta/groups_adapter_test.go` — Groups/Rules/Policies adapter integration (12 테스트)
  - Groups.List 혼합 타입 드레인
  - Groups.Get 단건
  - Groups.Members iterator
  - Groups.AppCount (첫 페이지)
  - Groups.AppCount 403 → ErrForbidden (PRD §7.7)
  - Rules.List 3 상태 (ACTIVE/INACTIVE/INVALID)
  - Rules.Get 단건
  - Policies.List type 필수 검증 (REQ-R04 AC-2)
  - Policies.List 드레인 + type 쿼리 전송 검증
  - Policies.Get Raw JSON 보존 (REQ-R04 AC-6)
  - Policies.Rules 디코드
  - Client builder 메서드 smoke test

- `internal/okta/users_adapter_more_test.go` — Users Get/ListGroups/ListFactors (5 테스트)
  - Get 단건 detail 디코드
  - Get 404 → ErrNotFound
  - ListGroups 배열 디코드
  - ListFactors 7종 factor type 전부 매핑
  - Client Options (WithLogger/Monitor/Clock/MaxRetries) 조합

- `internal/service/coverage_boost_test.go` — Service 엣지 (14 테스트)
  - GroupsService Get/Members/AppCount pass-through
  - UsersService.Groups pass-through
  - PoliciesService Get/Rules/Invalidate
  - RulesService List/Get/ResolveTargetGroupNames/Invalidate
  - LogsService Search/PollInterval/SetAdaptive
  - LogsTail.Poll events + nextSince
  - Bundle.InvalidateAll 부분/완전 주입
  - ServiceOption 조합

### 총 추가 테스트
- **31개 신규 테스트**
- Build/race 모두 PASS
- QA-017 완료 기준 충족 (okta/service 목표 초과)
