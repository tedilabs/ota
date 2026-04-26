# 04. TESTING.md 아웃라인 (테스트 엔지니어 초안)

**작성:** test-engineer
**날짜:** 2026-04-24
**상태:** Draft — developer 공동 설계 회의 회신 대기 중. 일부 섹션은 합의 후 구체화.
**대상 산출물:** `docs/TESTING.md` (v1.0.0)

> 이 문서는 `docs/TESTING.md`의 공개 전 아웃라인이다. PRD v1.0.0과 TUI_DESIGN v1.0.0의 모든 REQ-ID·AC에 대해 테스트 전략을 수립한다. 내부 설계 논의 흔적을 포함하고, 최종 공개본은 간결하게 정리한다.

---

## 0. 테스트 철학 (TESTING.md §1)

### 0.1. 세 가지 핵심 원칙
1. **Fail-First TDD** — 구현 전 실패 테스트. 실패 로그를 `_workspace/05_test_fail_log_<date>.txt`에 증거로 남김.
2. **경계면 정합성 (Three-Way Triangulation)** — PRD REQ ↔ 구현 ↔ 테스트가 삼각 일치. QA는 이를 검증 (PRD §10.4).
3. **회귀 방지 자동화** — QA/사용자 발견 버그는 먼저 실패 테스트로 재현 → 수정.

### 0.2. 비목표
- 100% 커버리지 추구 — 핵심 도메인 95%, 서비스 85%, 어댑터 70%, TUI 60% 목표 (PRD §6.3 대비 상향)
- SDK 자체 동작 테스트 — SDK는 Okta 책임, 우리는 **Adapter 경계**만 검증
- UI 픽셀 단위 비교 — ANSI 토큰 수준 골든 비교로 충분

---

## 1. 테스트 피라미드 (TESTING.md §2)

```
        ┌──────────────┐
        │   E2E (실)    │  ← manual/opt-in integration tag, 실 Okta dev tenant
        │  소수         │    PRD §10.1 테스트 피라미드 4단
        ├──────────────┤
        │ TUI (teatest)│  ← 화면 전체 흐름, 골든 파일 diff
        │              │    PRD §10.1 단 2 "Component"
        ├──────────────┤
        │ Integration  │  ← httptest.Server + testdata JSON fixture
        │   중간        │    Adapter · Ratelimit · Pagination 합성
        ├──────────────┤
        │    Unit      │  ← domain 파서/매퍼/필터/에러맵
        │   많음        │    PRD §10.1 단 1
        └──────────────┘
```

### 1.1. 레이어별 책임
| 레이어 | 대상 패키지 | 외부 의존 | 예시 |
|-------|-----------|---------|------|
| Unit | `internal/domain/*` | 없음 | UserStatus.String, errormap.FromCode, ratelimit.ShouldWarn |
| Interface | `internal/domain/ports.go` 구현 계약 | interface only | UserRepository contract: ErrNotFound on missing |
| Integration | `internal/okta/*` | httptest.Server | OktaUserRepo.List parses Link header |
| TUI | `internal/tui/*` | 주입된 fake Repository | UsersListModel filter → detail flow |
| E2E (manual) | `cmd/ota` | 실 Okta tenant | smoke test via integration build tag |

---

## 2. 테스트 도구 스택 (TESTING.md §3)

| 용도 | 선정 | 버전 | 비고 |
|------|-----|------|------|
| Assertion | `github.com/stretchr/testify/assert`, `require` | latest | `assert` 기본, 치명 실패 시 `require` |
| Interface mock | 수동 fake 기본 + (복잡도 증가 시) `github.com/matryer/moq` codegen | - | testify/mock 지양 (stringly-typed) |
| HTTP mock | `net/http/httptest.Server` (표준) | - | `jarcoal/httpmock`은 SDK 내부 클라 교체 불가 시 후보 |
| TUI E2E | `github.com/charmbracelet/x/exp/teatest` | latest | 골든 파일 기본 |
| Deep diff | `github.com/google/go-cmp/cmp` | latest | JSON 비교, 구조체 diff |
| Fixture | `testdata/oktaapi/` JSON | - | Record/Replay 전략 (§5) |
| JSON schema | Okta OpenAPI를 ref로 수동 표본 검증 (MVP); `github.com/xeipuuv/gojsonschema`는 v0.2 고려 | - | MVP는 골든 JSON 고정 비교로 충분 |
| Race detector | `go test -race` | - | CI 필수 |
| Coverage | `go test -coverprofile`, `go tool cover` | - | 목표치 CI 체크 |
| Lint | `golangci-lint` | latest | testfx 패키지 포함 대상 |
| Vuln | `govulncheck` | latest | CI 주 1회 |

> **Clock/Jitter 주입:** 표준 라이브러리 wrapper 자작. 외부 의존 없음.

---

## 3. 디렉토리 구조 (TESTING.md §4)

```
ota/
├── internal/
│   ├── domain/
│   │   ├── user.go
│   │   ├── user_test.go               ← 내부 테스트 (package domain)
│   │   ├── ports.go                   ← Repository interface 집합
│   │   └── ports_contract_test.go     ← 공통 계약 테스트 (external: package domain_test)
│   ├── service/
│   │   ├── users_service.go
│   │   └── users_service_test.go
│   ├── okta/
│   │   ├── users_adapter.go
│   │   ├── users_adapter_test.go
│   │   ├── ratelimit/
│   │   │   ├── monitor.go
│   │   │   └── monitor_test.go
│   │   ├── pagination/
│   │   │   ├── link.go
│   │   │   └── link_test.go
│   │   ├── errormap/
│   │   │   ├── map.go
│   │   │   └── map_test.go
│   │   ├── testfx/                    ← 테스트 헬퍼 (NewFakeOktaServer 등)
│   │   │   ├── fake_server.go
│   │   │   └── fixtures.go
│   │   └── integration/
│   │       └── live_test.go           ← //go:build integration
│   └── tui/
│       ├── users/
│       │   ├── list_model.go
│       │   ├── list_model_test.go
│       │   └── testdata/
│       │       └── Test_UsersList_Filter.golden
│       └── ...
├── testdata/
│   └── oktaapi/                        ← 공유 fixture
│       ├── users/
│       │   ├── list_page1.json
│       │   ├── list_page2.json
│       │   ├── list_link_header.txt
│       │   ├── detail.json
│       │   ├── factors.json
│       │   └── user_groups.json
│       ├── groups/
│       ├── grouprules/
│       ├── policies/
│       │   ├── okta_sign_on.json
│       │   ├── access_policy.json
│       │   └── ...
│       ├── logs/
│       │   ├── tail_initial.json
│       │   ├── tail_poll_2.json
│       │   ├── empty.json
│       │   └── rate_limit_429.json
│       └── errors/
│           ├── E0000004.json
│           ├── E0000006.json
│           └── ...
└── Makefile
```

### 3.1. 내부 vs 외부 테스트 패키지 규약
- **내부 (`package foo`):** unexported 심볼 직접 검증이 필요할 때만
- **외부 (`package foo_test`):** 기본. 인터페이스 사용자 관점
- 원칙: **외부 우선**. 내부 테스트는 예외 (정당화 주석)

---

## 4. 픽스처 관리 (TESTING.md §5)

### 4.1. testdata 규약
- 파일명: `<endpoint>_<scenario>.json`
- 응답 헤더는 `*_link_header.txt`, `*_headers.txt`에 분리 저장
- 파일 크기: 5KB 이상이면 편집 주의 문구 상단 주석 (JSON은 주석 없으므로 `_` key)
- **버전 고정:** fixture 상단 주석으로 기록 시각/tenant 버전 정보 (JSON 외부 README)

### 4.2. 시드 데이터 세트 (PRD §10.2 대응)
```
users/
  - 00u_active_alice (profile.login=alice@acme.com, ACTIVE, factors=[push, sms])
  - 00u_active_bob (ACTIVE, factors=[webauthn])
  - 00u_suspended (SUSPENDED)
  - 00u_locked (LOCKED_OUT)
  - 00u_password_expired (PASSWORD_EXPIRED)
  - 00u_deprovisioned (DEPROVISIONED)
  - 00u_staged (STAGED)
  - 00u_provisioned (PROVISIONED)

groups/
  - 00g_engineering (OKTA_GROUP, 동적 — Group Rule target)
  - 00g_sales (OKTA_GROUP, 정적)
  - 00g_everyone (BUILT_IN, 대용량 배너 대상)
  - 00g_app_synced (APP_GROUP)

grouprules/
  - 0pr_active (ACTIVE, expression="user.department == \"Engineering\"")
  - 0pr_inactive (INACTIVE)
  - 0pr_invalid (INVALID, 경고색 테스트)

policies/
  - OKTA_SIGN_ON × 1 (system=true default + 1 custom)
  - ACCESS_POLICY × 2
  - PASSWORD × 1
  - MFA_ENROLL × 1
  - PROFILE_ENROLLMENT, POST_AUTH_SESSION, IDP_DISCOVERY 각 1 (raw 렌더러용)

logs/
  - user.session.start success × 3
  - user.session.start failure × 2
  - group.rule.deactivate × 1 (경고 프리셋)
  - system.api_token.create × 1 (토큰 수명 추정용)
```

### 4.3. Record / Replay 전략
- **v0.1.x 초기:** 수동 `scripts/record_fixtures.sh` — curl로 실 dev tenant 응답 저장
- **재사용:** fixture가 Okta 응답 스키마의 단일 출처 (ground truth)
- **CI:** `go test` 기본은 오프라인. `-tags=integration`만 실제 호출

---

## 5. teatest 패턴 (TESTING.md §6)

### 5.1. 기본 패턴 — 키 입력 → 출력 스냅샷
```go
func Test_UsersList_FilterThenOpenDetail(t *testing.T) {
    t.Parallel()

    fakeRepo := &fakes.UsersRepo{
        List: []domain.User{
            {ID: "00u1", Profile: domain.UserProfile{Login: "alice@acme.com"}, Status: domain.StatusActive},
            {ID: "00u2", Profile: domain.UserProfile{Login: "bob@acme.com"}, Status: domain.StatusActive},
        },
    }
    clock := testfx.FixedClock(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))

    model := users.NewListModel(fakeRepo, clock)
    tm := teatest.NewTestModel(t, model,
        teatest.WithInitialTermSize(100, 30),
    )

    // 초기 fetch Cmd 결과 수신 대기
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte("alice@acme.com"))
    }, teatest.WithDuration(2*time.Second))

    // '/alice' 필터 입력
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alice")})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // 최종 출력 골든 비교
    out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
    require.NoError(t, err)
    teatest.RequireEqualOutput(t, out)
    // golden: testdata/Test_UsersList_FilterThenOpenDetail.golden
    // 업데이트: go test -update ./internal/tui/users/
}
```

### 5.2. 골든 파일 업데이트 규약
- `-update` 플래그로만 갱신. CI에서는 금지.
- 업데이트 시 diff를 PR 본문에 첨부 (리뷰어 확인)
- ANSI 코드 포함 그대로 저장. 터미널 폭은 `teatest.WithInitialTermSize`로 고정

### 5.3. 비동기 Cmd 대기 전략
- `teatest.WaitFor(output, predicate, WithDuration)` — 특정 문자열/패턴 등장 대기
- 타임아웃: 기본 2s, 네트워크 없으므로 충분
- Flaky 방지: `time.Sleep` 금지, 항상 예측가능 조건으로 대기

---

## 6. 시나리오별 테스트 매트릭스

### 6.1. Rate Limit (REQ-E01)
| AC | 테스트 | 레이어 |
|----|-------|-------|
| AC-1 Remaining ≤ 10% 노란 경고 | ratelimit.Monitor_Warns_WhenRemainingBelowThreshold | Unit |
| AC-2 429 Retry-After 준수 + jitter | Transport_Retries429_WithRetryAfterAndJitter | Integration (httptest) |
| AC-2 3회 실패 시 빨간 에러 | Transport_GivesUp_After3Retries | Integration |
| AC-3 tail 일시정지 + since 유지 | LogsTail_PausesOnRateLimit_ResumesWithSameSince | Unit + Integration |
| AC-4 카테고리별 last-observed | ratelimit.Monitor_RecordsLastObservedPerCategory | Unit |
| AC-5 `/logs` 분당 ~120 준수 | LogsTail_DefaultInterval_StaysUnder120PerMin | Unit (clock fake) |
| AC-6 30초 TTL 캐시 + 강제 무효화 | Cache_TTL_Expires · Cache_ForceInvalidate | Unit |

### 6.2. Pagination (PRD §7.3, REQ-R01 AC-4, REQ-R02 AC-3)
| 시나리오 | 테스트 |
|---------|-------|
| Link 헤더 next 커서 파싱 | pagination.LinkHeader_ParsesNextCursor |
| 2페이지 이상 순차 fetch | UsersRepo_ListAll_IteratesAllPages |
| 병렬 fetch 금지 (순차 보장) | UsersRepo_ListAll_SequentiallyFetches |
| 중간 취소 (ctx cancel) | UsersRepo_ListAll_CancelsGracefully |
| 멤버 페이지 소진 중 중단 | GroupMembers_UserCanAbort |

### 6.3. 에러 매핑 (REQ-U04 AC-3, REQ-C04 AC-4, PRD §7.7)
8종 errorCode 전부 테스트:
| errorCode | 테스트 | 기대 UX 메시지 |
|-----------|-------|---------------|
| E0000001 | errormap_ValidationError_ParsesErrorCauses | 필드별 표시 |
| E0000004 | errormap_AuthError_MapsToUnauthorized | "API token invalid or revoked..." |
| E0000006 | errormap_ForbiddenError_MapsToForbidden | "Insufficient permissions..." |
| E0000007 | errormap_NotFoundError_MapsToNotFound | "Resource not found..." |
| E0000011 | errormap_TokenExpired_MapsToUnauthorized | "Token expired or revoked" |
| E0000022 | errormap_DeleteBlocked_MapsToValidation | "Deactivate before deleting" |
| E0000038 | errormap_FeatureDisabled_MapsToValidation | "Feature is disabled..." |
| E0000047 | errormap_RateLimit_MapsToRateLimited | 자동 재시도 |

### 6.4. PII 마스킹 (REQ-C05, REQ-R01 AC-6, TUI_DESIGN §7)
| 시나리오 | 테스트 | 레이어 |
|---------|-------|-------|
| SMS phoneNumber 기본 마스킹 | UserFactor_SMS_MasksPhoneNumberByDefault | Unit |
| secondEmail 마스킹 포맷 (`a***@example.com`) | UserProfile_MasksSecondEmail | Unit |
| `:unmask` 후 [M!] 배지 표시 | UsersDetail_Unmask_ShowsBadge | TUI (teatest golden) |
| `y` 복사 (마스킹 상태에서 마스킹 값) | UsersDetail_CopyMasked_CopiesMaskedValue | Unit (clipboard fake) |
| 화면 전환 시 자동 재마스킹 | UsersDetail_Navigate_AutoRemasks | TUI |
| 60초 inactivity 자동 재마스킹 | UsersDetail_Inactivity_AutoRemasks | Unit (clock fake) |
| **peek test**: 로그·스택트레이스·크래시덤프에 원본 PII 부재 | Secrets_NeverLeakToLogs (grep guard) | Integration |
| Debug log Authorization 헤더 마스킹 | DebugLog_Authorization_Masked | Unit |

### 6.5. Logs Tail (REQ-R05)
| AC | 테스트 |
|----|-------|
| AC-2 since 재설정 (마지막 published + 1ms) | LogsTail_Since_AdvancesByMinPlusOneMs |
| AC-2 기본 7초 간격 | LogsTail_DefaultPollInterval_Is7s |
| AC-2 adaptive polling `X-Rate-Limit-Limit < 60` → 15초 | LogsTail_AdaptivePolling_UpgradesTo15sOnLowLimit |
| AC-3 새 이벤트 도착 시 상단 카운터 | LogsTail_NewEventsBadge_BlinksOnArrival (TUI) |
| AC-3 `f` 자동 스크롤 토글 | LogsTail_FollowToggle_PausesAutoScroll (TUI) |
| AC-3 429 자동 일시정지 | LogsTail_Paused_WhenRateLimited |
| AC-4 DESCENDING 최신순 히스토리 | LogsList_Descending_LatestFirst |
| AC-5 프리셋 5종 | LogsPresets_ApplyFilter (table-driven × 5) |
| AC-6 actor.id → User 점프 | LogEvent_ActorJump_OpensUserDetail (TUI) |
| AC-7 UTC/로컬 토글 | LogTime_Format_RespectsTzSetting |

### 6.6. 수용 기준 매핑 매트릭스 (REQ-ID → 테스트 파일/함수)
TESTING.md §13에 전면 테이블. **모든 21개 REQ**에 대해 매핑 (상세는 §13 완성 시).

---

## 7. Fail-First 프로세스 (TESTING.md §10)

### 7.1. 표준 루프
```
1. 새 요구사항 선정 (REQ-ID 또는 AC 단위)
2. 실패할 테스트 작성 (테스트 파일만, 구현은 stub 또는 부재)
3. `go test ./...` 실행 → 실패 확인
4. 실패 로그를 _workspace/05_test_fail_log_<date>.txt에 append
5. 구현 작성 (최소)
6. 재실행 → 통과 (Green)
7. 리팩터링 (녹색 유지)
8. 커밋
```

### 7.2. 로그 규약
`_workspace/05_test_fail_log_YYYY-MM-DD.txt`에 append:
```
## REQ-R01 AC-1 리스트 컬럼
### 2026-05-01 10:23 (Red)
$ go test ./internal/tui/users/ -run Test_UsersList_DefaultColumns
--- FAIL: Test_UsersList_DefaultColumns (0.01s)
    list_model_test.go:42: expected column 'status' at index 0, got ""
FAIL

### 2026-05-01 10:45 (Green)
$ go test ./internal/tui/users/ -run Test_UsersList_DefaultColumns
ok      github.com/tedilabs/ota/internal/tui/users  0.123s
```

### 7.3. Green 상태로 추가되는 테스트 (예외)
- 허용 조건: 기존 구현이 우연히 충족하지만 **명시적 lock-in이 필요한** 경우
- 파일 상단 주석 필수: `// Lock-in test (not Fail-First derived): prevents regression of behavior X`

---

## 8. CI 요구사항 (TESTING.md §11)

### 8.1. GitHub Actions 잡 목록
```
1. test-unit          → make test (race + cover)
2. test-integration   → httptest-based, 기본 go test (실제 tenant 없음)
3. lint              → golangci-lint run
4. vuln              → govulncheck ./...
5. cover-gate        → coverage < thresholds이면 실패
6. (manual) e2e-live → workflow_dispatch, OKTA_* secrets 필요
```

### 8.2. Coverage Gate (Phase 5 말 기준)
| 패키지 prefix | 최소 |
|--------------|------|
| `internal/domain/...` | 95% |
| `internal/service/...` | 85% |
| `internal/okta/...` | 70% |
| `internal/tui/...` | 60% |

### 8.3. Flaky Test 정책
- 재시도 금지 (근본 원인 찾기)
- 1회라도 flaky로 관찰되면 `t.Skip(...)` + GitHub issue + `//go:build flaky` 태그로 격리
- 원인 수정 후 복원

---

## 9. 로컬 개발자 워크플로 (TESTING.md §12)

### 9.1. Makefile 타겟
```makefile
.PHONY: test test-short test-race test-integration test-cover test-update-golden

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
	go tool cover -html=coverage.out -o coverage.html
	@echo "coverage.html generated"

test-update-golden:
	go test -update ./internal/tui/...
```

### 9.2. 일반 개발 사이클
1. `make test-short` — 작업 중 빠른 피드백 (`-short`로 무거운 테스트 스킵)
2. 커밋 전 `make test` — race 포함 전체
3. 골든 파일 변경 시 `make test-update-golden` + diff 검토

---

## 10. 수용 기준 매핑 매트릭스 (TESTING.md §13)

### 10.1. 포맷
| REQ-ID | AC | 테스트 파일 | 테스트 함수 | 레이어 |
|--------|----|------------|-----------|-------|
| REQ-U01 | AC-1 | `internal/tui/keys_test.go` | `Test_Navigation_ArrowKeysEquivalentToVim` | Unit |
| REQ-U01 | AC-2 | `internal/config/keybindings_test.go` | `Test_Keybindings_OverrideFromConfig` | Unit |
| REQ-U01 | AC-3 | `internal/tui/users/list_test.go` 외 | `Test_*_NavigationConsistent` | TUI |
| ... | ... | ... | ... | ... |

**목표:** 21개 REQ-ID 전부 최소 1개 테스트 함수에 매핑. AC-level로 매핑 가능하면 그 수준으로.

### 10.2. 매트릭스 완성 시점
- Phase 5 시작 전 REQ → 테스트 이름 **예약** (구현 없이 test stub + `t.Skip("not implemented")`)
- Phase 5 진행 중 하나씩 Fail-First로 풀어감
- Phase 6 구현과 함께 Green 전환

---

## 11. peek test (보안 QA) (TESTING.md §7.4)

### 11.1. 목적
PRD §6.2 + TUI_DESIGN §7에 정의된 PII/secret 마스킹이 실제로 **모든 출력 경로**에서 작동함을 기계적으로 검증.

### 11.2. 검증 경로
- 디버그 로그 파일 (`~/.cache/ota/debug.log`)
- panic/crash 스택트레이스
- 세션 에러 히스토리 (`:errors`)
- 골든 파일 (teatest)
- 클립보드 복사 (마스킹 상태)

### 11.3. 구현 아이디어
```go
func Test_Secrets_NeverLeakToDebugLog(t *testing.T) {
    token := "00a-SECRET-TOKEN-VALUE-xyz"
    phone := "+1-555-123-4567"
    email := "alice@secret.com"

    var buf bytes.Buffer
    logger := slog.New(maskingHandler(&buf))

    // 실제 HTTP 요청/에러 로깅 시뮬레이션
    logger.Error("request failed",
        "authorization", "SSWS "+token,
        "phone", phone,
        "email", email)

    output := buf.String()
    assert.NotContains(t, output, token, "raw token must not appear")
    assert.NotContains(t, output, phone, "raw phone must not appear")
    assert.NotContains(t, output, email, "raw email must not appear")
    assert.Contains(t, output, "***", "masked marker must appear")
}
```

---

## 12. 계약 테스트 (Contract Test)

### 12.1. 개념
Repository interface(예: `domain.UserRepository`)의 구현체가 여러 개 생길 때(실 Okta / in-memory fake / record-replay) 모두 **동일 계약**을 만족하는지.

```go
// internal/domain/ports_contract_test.go
package domain_test

func RunUserRepositoryContract(t *testing.T, factory func(t *testing.T) domain.UserRepository) {
    t.Run("Get nonexistent returns ErrNotFound", func(t *testing.T) {
        repo := factory(t)
        _, err := repo.Get(context.Background(), "00u-does-not-exist")
        assert.ErrorIs(t, err, domain.ErrNotFound)
    })
    t.Run("List empty tenant returns empty slice (not nil)", func(t *testing.T) {
        repo := factory(t)
        users, err := repo.List(context.Background(), domain.UserFilter{})
        require.NoError(t, err)
        assert.Equal(t, []domain.User{}, users)
    })
    // ... 기타 계약
}

// 각 구현체 테스트에서 호출
func Test_InMemoryUserRepo_Contract(t *testing.T) {
    RunUserRepositoryContract(t, func(t *testing.T) domain.UserRepository {
        return fakes.NewInMemoryUserRepo()
    })
}

func Test_OktaUserRepo_Contract(t *testing.T) {
    RunUserRepositoryContract(t, func(t *testing.T) domain.UserRepository {
        srv := testfx.NewFakeOktaServer(t, testfx.ContractScenario)
        return okta.NewUserRepo(srv.Client(), srv.URL)
    })
}
```

### 12.2. 커버 포인트
- `Get(id)` — 존재/부재/비활성화된 사용자
- `List(filter)` — 빈/필터매치/페이지네이션
- `context.Cancelled` → `errors.Is(err, context.Canceled)` 전파

---

## 13. 미해결 / developer 합의 대기 항목

### 13.1. 공동 설계 회의에서 합의 필요
- [ ] Mock 생성 방식 (수동 fake vs moq codegen) — 제안: 수동 기본
- [ ] httpmock vs httptest.Server — 제안: httptest 기본, SDK 제약 시 httpmock
- [ ] Logger (slog) 합의 + 마스킹 handler 위치
- [ ] Domain error 타입 (sentinel vs typed) — 제안: typed + errors.Is/As
- [ ] 설정 파서 라이브러리 확정 (yaml.v3 vs koanf)

### 13.2. teatest 실전 데모 (Phase 4 말)
- Users List filter → detail flow 1개 작성 → 골든 생성
- 이를 기준 패턴으로 TESTING.md §5에 인라인 예시로 포함

### 13.3. Phase 5 착수 전 준비물
- [ ] `testdata/oktaapi/` 초기 fixture 세트 (수동 curl 또는 SDK example)
- [ ] `internal/okta/testfx/` 스켈레톤
- [ ] Makefile 타겟
- [ ] golangci-lint config (depguard 규칙 포함)
- [ ] REQ-ID → 테스트 함수명 예약 목록 (Phase 5 가이드)

---

## 14. 작성 순서 (TESTING.md 생성)
1. 본 아웃라인 developer 검토 수령 (§13.1 합의)
2. `docs/TESTING.md` 초안 — §1~13 순서로
3. 수용 기준 매핑 매트릭스 (§10)는 PRD 21 REQ 전부 채움
4. teatest 데모 코드 삽입 (§5)
5. developer 리뷰 → 반영 → finalize

---

**END OF OUTLINE.** 회신 대기 중. 합의 완료 후 `docs/TESTING.md`로 전환.
