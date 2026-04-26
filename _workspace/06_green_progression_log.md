# 06. Phase 6 — Green Progression Log

**작성:** test-engineer
**착수:** 2026-04-24
**역할:** developer가 Phase 5 Red 테스트를 Green으로 전환할 때마다 실제 실행을 모니터링하고 REQ별 진척을 추적.
**입력 문서:**
- `_workspace/05_test_fail_log_2026-04-24.txt` — Phase 5 Red 증거
- `_workspace/05_req_coverage.md` — REQ × 테스트 함수 매트릭스
- `docs/TESTING.md` v1.0.0 — 테스트 규약 + Coverage Gate

---

## Phase 6 시작 기준선 (2026-04-24)

### 컴파일/정적 검증
- `go build ./...` PASS (모든 stub 완료)
- `go vet ./...` PASS (logs_service_test.go의 unused `context` import 제거 완료)

### 테스트 실행 기준선 — `go test ./... -short`

| 패키지 | 초기 상태 | Phase 5 Red 원인 |
|-------|---------|----------------|
| `internal/domain` | **PASS** (Lock-in) | 타입 구조 검증 |
| `internal/security` | **PASS** (Lock-in) | peek PII 게이트 |
| `internal/app` | FAIL (build) | ResolveToken/ClassifyKey/QuitConfirmRequestMsg 등 미정의 |
| `internal/config` | FAIL (run) | config.Load panic |
| `internal/keys` | FAIL (run) | keys.Resolve panic |
| `internal/logger` | FAIL (run) | MaskAttr/New panic |
| `internal/mask` | FAIL (run) | Phone/Email panic |
| `internal/okta` | FAIL (run) | NewClient panic |
| `internal/okta/errormap` | FAIL (run) | FromResponse panic |
| `internal/okta/pagination` | FAIL (run) | NextCursor panic |
| `internal/okta/ratelimit` | FAIL (run) | CategoryFromPath/Observe panic |
| `internal/service` | FAIL (build) | LogsTail/LogsPresets/ErrPolicyTypeRequired/RulesService 미정의 |
| `internal/tui/users` | FAIL (run) | users.NewListModel 미구현 |

## Green 전환 추적 (REQ-ID별)

발생 시 아래 템플릿으로 추가:

```
### YYYY-MM-DD HH:MM — REQ-XXX [AC-Y] 패키지 Green
- 테스트: Test_Foo_Bar
- 실행: go test -race -count=1 -run Test_Foo_Bar ./internal/<package>
- 결과: PASS (Xms)
- 비고: flaky 여부, edge case 보완, coverage 수치
- 커밋/참조: <commit sha or task id>
```

### 2026-04-24 Phase 6 착수 이전 Green

#### 2026-04-24 09:19 — REQ-R01/R03/R04 [structure] `internal/domain` Lock-in Green
- 테스트: `Test_UserStatus_*`, `Test_GroupRuleStatus_*`, `Test_PolicyType_*`, `Test_DomainErrors_*`
- 실행: `go test -race ./internal/domain/...`
- 결과: ok (0.38s)
- 비고: Lock-in 테스트 (TESTING §1.3). 도메인 stub 최초 배치 시점부터 Green. 상수/타입 관계 검증용.
- 참조: Phase 4 domain stub + Phase 5 테스트 파일 작성

#### 2026-04-24 09:33 — REQ-C05/R01 [peek] `internal/security` Lock-in Green
- 테스트: `Test_Peek_Testdata_HasNoRawPII`
- 실행: `go test -race ./internal/security/...`
- 결과: ok (2.74s) — testdata 전체 스캔 통과
- 비고: Lock-in 보안 게이트. testdata/oktaapi/**/*.json + scenarios + config 전체에서 scrub되지 않은 이메일/전화/SSWS 토큰 없음 확인.
- 참조: Phase 5 peek_test.go 작성

### 2026-04-24 Phase 6 Layer 1 Green 전환 (첫 구현 릴레이)

#### 2026-04-24 10:18 — REQ-C05 / REQ-R01 AC-6 [mask] `internal/mask` Green
- 테스트: `Test_Mask_Phone_KeepsLastFourDigits`, `Test_Mask_Phone_UnknownFormatPassesThrough`, `Test_Mask_Email_FirstCharPlusAsterisks`, `Test_Mask_Email_InvalidReturnsInput`
- 실행: `go test -race -count=1 -v ./internal/mask/...`
- 결과: **ok (1.459s) — 4 테스트 + 13 서브테스트 전부 PASS**
- Coverage: **86.4%** (목표 대비: mask는 일반 utility이므로 별도 수치 미명시, 서비스 85% 기준 이미 초과)
- 비고: 전화·이메일 모두 정확한 포맷 마스킹. 알 수 없는 포맷(`1234`, `not-an-email`, `@starts-with-at.com` 등)은 원문 반환으로 오도 방지. `+82-10-1234-5678 → +82-**-****-5678` 국제 포맷도 처리.

#### 2026-04-24 10:18 — REQ-U01 / REQ-C03 [keys] `internal/keys` Green
- 테스트: `Test_KeysResolve_Defaults_IncludeVimAndArrows`, `Test_KeysResolve_UserOverride_WinsOnConflict`, `Test_KeysResolve_UnknownID_ProducesWarningNotError`
- 실행: `go test -race -count=1 -v ./internal/keys/...`
- 결과: **ok (1.718s) — 3 테스트 전부 PASS**
- Coverage: **100.0%** ✓
- 비고: Vim 기본 매핑(j/k/:/?/q 등) + 화살표 동등성 + Reverse lookup 동작. 사용자 override가 빌트인을 우선 (REQ-C03 AC-2). Unknown ID는 warning만 반환 — fatal 아님 (REQ-C03 AC-3).

#### 2026-04-24 10:18 — REQ-R01 AC-4 / REQ-R02 AC-3 / PRD §7.3 [pagination] `internal/okta/pagination` Green
- 테스트: `Test_LinkHeader_ParsesNextCursor` (4 서브테스트)
- 실행: `go test -race -count=1 -v ./internal/okta/pagination/...`
- 결과: **ok (2.087s) — 1 테스트 + 4 서브테스트 전부 PASS**
- Coverage: **93.5%** (초기 Green 직후)
- 비고: next+self / self only / empty / opaque cursor 4 시나리오 통과. 경계 유닛 목표 95%에서 -1.5%p → 아래 엣지 케이스 4종 보강으로 100% 도달.

#### 2026-04-24 10:22 — pagination coverage 95% 목표 충족 엣지 보강
- 추가 서브테스트: `rel_without_quotes`, `malformed_missing_rel`, `next_link_without_after_param`, `next_after_prev`
- 실행: `go test -race -cover -count=1 ./internal/okta/pagination/...`
- 결과: **ok (1.483s) coverage: 100.0% of statements** ✓
- 비고: RFC 5988 따옴표 없는 rel, rel 파트 부재, after 파라미터 누락 next, rel="prev" 뒤의 rel="next" 순서 전부 커버. Phase 6 Layer 1 경계 유닛 목표치(95%) 초과 달성.

### Phase 6 누적 Green (2026-04-24 10:18 기준)
- Lock-in: `internal/domain`, `internal/security`
- **Layer 1 신규 Green:** `internal/mask`, `internal/keys`, `internal/okta/pagination`
- 여전히 Red: `internal/app`, `internal/config`, `internal/logger`, `internal/okta`, `internal/okta/errormap`, `internal/okta/ratelimit`, `internal/service`, `internal/tui/users`

### 2026-04-24 대규모 Green 전환 (Layer 2~5 거의 전부)

developer가 Layer 2~5를 빠르게 Green 전환. `go test -race -count=1 ./...` 실행 결과 12/13 패키지 Green.

| 패키지 | 상태 | Coverage | 목표 | 갭 |
|-------|------|---------|------|----|
| `internal/app` | Green | **79.6%** | (unspecified) | OK |
| `internal/config` | Green | **53.3%** | 85% | **-31.7%p** |
| `internal/domain` | Green | **42.9%** | 95% | **-52.1%p ⚠️** |
| `internal/keys` | Green | 100.0% ✓ | - | OK |
| `internal/logger` | Green | 75.0% | - | OK |
| `internal/mask` | Green | 86.4% | - | OK |
| `internal/okta` | Green | **43.7%** | 75% | **-31.3%p** |
| `internal/okta/errormap` | Green | **62.7%** | 95% | **-32.3%p ⚠️** |
| `internal/okta/pagination` | Green | **100.0%** ✓ | 95% | OK |
| `internal/okta/ratelimit` | Green | 87.9% | 95% | -7.1%p |
| `internal/security` | Green | (peek, no stmts) | - | OK |
| `internal/service` | Green | **62.0%** | 85% | **-23.0%p** |
| `internal/tui/users` | **FAIL** | - | 60% | 여전히 Red (teatest) |

**유일한 Red:** `internal/tui/users` Screen Model 구현 대기. `Test_UsersListFlow_FilterAlice_OpensDetail` (2s timeout).

### Coverage 갭 분석 (test-engineer 추가 작업 요구)

심각한 갭이 3건 (-30%p 이상):
- `internal/domain` (42.9% vs 95%): 구조 Lock-in 외 엔티티 사용·iterator·에러 타입 추가 유닛 필요
- `internal/okta/errormap` (62.7% vs 95%): 경계 유닛 목표 95% 미달 — malformed body, 5xx, non-E code, 빈 body branch 등 추가 필요
- `internal/config` (53.3% vs 85%): Validate 분기·XDG path resolve 추가 필요

### 2026-04-24 Coverage 보강 작업 (test-engineer)

#### domain 42.9% → **100.0%** ✓ (목표 95% 초과)
- 추가: `Test_DomainErrors_RateLimitedError_ErrorMessageFormat`, `Test_DomainErrors_BadRequestError_ErrorMessageFormat` (0/1/N cause 분기)
- 파일: `internal/domain/errors_test.go`
- 근거: 기존 테스트가 `.Error()` 메서드 미호출로 0% 집계된 두 함수를 명시적으로 검증.

#### errormap 62.7% → **100.0%** ✓ (목표 95% 초과)
- 추가 10종 엣지:
  - `Test_ErrorMap_NilResponse_ReturnsErrNetwork`
  - `Test_ErrorMap_5xx_MapsToErrOktaServer`
  - `Test_ErrorMap_UnknownErrorCode_FallsBackToHTTPStatus` (5 서브테스트: 401/403/404/400/418)
  - `Test_ErrorMap_RateLimit_RetryAfterHTTPDate_Parsed`
  - `Test_ErrorMap_RateLimit_NoRetryAfterHeader_ZeroDuration`
  - `Test_ErrorMap_RateLimit_RetryAfterPastDate_ClampedToZero`
  - `Test_ErrorMap_RateLimit_RetryAfterUnparseable_ZeroDuration`
  - `Test_ErrorMap_MalformedBody_FallsBackToHTTPStatus`
  - `Test_ErrorMap_EmptyBody_UsesStatusTextAsSummary`
  - `Test_ErrorMap_BadRequest_CauseWithoutColon_FieldEmpty`
  - `Test_ErrorMap_UnknownStatus_EmptyBody_SentinelOnly`
  - `Test_ErrorMap_ErrorsIsCompatibleAcrossWrappers` (회귀 lock-in)
- 파일: `internal/okta/errormap/map_test.go`
- 근거: 모든 branch 커버 — nil resp / 5xx fallback / Retry-After 3 포맷 / malformed JSON / empty body / splitCause 콜론 없음 / 999 status fall-through.

#### tui/users Red → Green (자동 전환)
- `Test_UsersListFlow_FilterAlice_OpensDetail` — PASS (0.02s)
- Coverage: **74.5%** (목표 60% 초과)
- developer가 Layer 6 Screen Model 구현 완료 시점 (상세한 `v` 토글, `/` 필터, Enter 드릴다운 전부 동작)

### 최종 Coverage 스냅샷 (2026-04-24 — 모든 13 패키지 Green)

| 패키지 | 목표 | 최종 | 상태 |
|-------|------|------|------|
| `internal/domain` | 95% | **100.0%** ✓ | 달성 |
| `internal/keys` | - | **100.0%** ✓ | 달성 |
| `internal/okta/errormap` | 95% | **100.0%** ✓ | 달성 |
| `internal/okta/pagination` | 95% | **100.0%** ✓ | 달성 |
| `internal/okta/ratelimit` | 95% | 87.9% | -7.1%p |
| `internal/mask` | - | 86.4% | OK |
| `internal/app` | - | 79.6% | OK |
| `internal/logger` | - | 75.0% | OK |
| `internal/tui/users` | 60% | **74.5%** ✓ | 달성 |
| `internal/service` | 85% | 62.0% | -23.0%p |
| `internal/config` | 85% | 53.3% | -31.7%p |
| `internal/okta` (overall) | 75% | 43.7% | -31.3%p |

**고위험 시나리오 전부 Green + 13/13 패키지 Green.** 남은 coverage 갭은 service/config/okta(overall)이지만 기능적 Red는 0개.

---

## REQ-ID별 진행표 (Phase 5 Red → Phase 6 Green)

| REQ | 테스트 개수 | Red | Green | Refactor | 메모 |
|-----|----------|-----|-------|---------|------|
| REQ-U01 | 3 | ✅ | ✅ (Green) | ☐ | keys + app.ClassifyKey 전부 통과 |
| REQ-U02 | 1 | ✅ | ✅ (Green) | ☐ | app Update `:` 팔레트 통과 |
| REQ-U03 | 2 | ✅ | 부분 (app) | ☐ | keymap context ✓, TUI users flow 대기 |
| REQ-U04 | 3 | ✅ | ✅ (Green) | ☐ | service + errormap BadRequest 통과 |
| REQ-U05 | 1 | ✅ | ☐ | ☐ | TUI users drill-down (Red 잔여) |
| REQ-U06 | 1 | ✅ | ✅ (Green) | ☐ | app `?` 도움말 통과 |
| REQ-U07 | 1 | ✅ | ✅ (Green) | ☐ | app Ctrl-c 종료 보호 통과 |
| REQ-R01 | 7+ | ✅ | 부분 (service+okta+mask) | ☐ | TUI users list/detail teatest 대기 |
| REQ-R02 | 3 | ✅ | ✅ (Green) | ☐ | service Groups + RULE 배지 통과 |
| REQ-R03 | 4 | ✅ | ✅ (Green) | ☐ | RulesService id→name 해소 통과 |
| REQ-R04 | 5+ | ✅ | ✅ (Green) | ☐ | Policies 7종 전부 통과 (service 레벨) |
| REQ-R05 | 7+ | ✅ | ✅ (Green) | ☐ | LogsTail + hole-free + presets 통과 |
| REQ-C01 | 4 | ✅ | ✅ (Green) | ☐ | config.Load + Validate 통과 |
| REQ-C02 | 1 | ✅ | ✅ (Green) | ☐ | 프로필 env 이름 저장 통과 |
| REQ-C03 | 2 | ✅ | ✅ (Green) | ☐ | keys override + unknown warning 통과 |
| REQ-C04 | 4 | ✅ | ✅ (Green) | ☐ | ResolveToken 우선순위 + errormap 통과 |
| REQ-C05 | 5+ | ✅ | ✅ (Green) | ☐ | mask + logger.MaskAttr + peek 통과 |
| REQ-E01 | 7 | ✅ | ✅ (Green) | ☐ | ratelimit.Monitor + Retry-After + 캐시 통과 |
| REQ-E02 | 1 | ✅ | ✅ (Green) | ☐ | app.ErrorMsg → ToastMsg 통과 |
| REQ-E03 | 2 | ✅ | ✅ (Green) | ☐ | NetworkError/Restored Msg 통과 |
| REQ-O01 | 1 | ✅ | ✅ (Green) | ☐ | logger.New sink 주입 통과 |

**Green: 19/21 REQ 완전 통과.** 잔여 Red: REQ-U05 (드릴다운) + REQ-R01 (TUI 부분) = TUI users 패키지만 Red.

**우선 순위 추천 (TESTING §12 + 의존 순서 기반, 05_req_coverage.md 하단 복습):**
1. mask → keys → pagination → errormap (빠른 릴레이)
2. config → FakeClock.NewTimer/Advance
3. ratelimit → okta.NewClient
4. adapter들 → service (caches/iterator)
5. app Msg 계약 → tui/users (teatest 최종 green)

---

## 고위험 시나리오 추적 (team-lead 지시 체크리스트)

각 시나리오가 Green이 되면 커밋 SHA/타임스탬프 기록:

- [x] **Rate Limit 7 AC** (REQ-E01 AC-1~6 + 429 자동 재시도) — **전부 Green (2026-04-24)**
  - [x] AC-1 Remaining ≤ 10% 노란 경고 (`Test_Monitor_Observe_*`)
  - [x] AC-2 Retry-After + jitter (`Transport_Retries429*`)
  - [x] AC-3 tail 일시정지 + since 유지
  - [x] AC-4 카테고리별 last-observed
  - [x] AC-5 `/logs` 분당 ~120 안전 마진
  - [x] AC-6 30s TTL 캐시 + 강제 무효화
  - [x] 종합: 3회 재시도 후 빨간 에러

- [x] **Pagination Link 헤더 + multi-page** (PRD §7.3, REQ-R01 AC-4) — 2/2 Green
  - [x] `LinkHeader_ParsesNextCursor` 4+4 케이스 — **Green (2026-04-24 10:22)**, coverage 100.0%
  - [x] `UsersAdapter_ListAll_IteratesAllPagesViaLinkHeader` 3페이지 순차 — Green (Layer 3)

- [x] **에러 매핑 8종** (REQ-U04 AC-3, PRD §7.7) — 전부 Green
  - [x] E0000001 (BadRequest + causes)
  - [x] E0000004 (TokenInvalid, 401)
  - [x] E0000006 (Forbidden, 403)
  - [x] E0000007 (NotFound, 404)
  - [x] E0000011 (TokenInvalid, 401)
  - [x] E0000022 (BadRequest "Deactivate before")
  - [x] E0000038 (FeatureDisabled)
  - [x] E0000047 (RateLimited + Retry-After=10s)

- [x] **PII 마스킹 peek test** — 3/3 Green
  - [x] testdata scan: `Test_Peek_Testdata_HasNoRawPII` PASS (Lock-in, 2026-04-24 09:33)
  - [x] `mask.Phone`, `mask.Email` 단위 — **Green (2026-04-24 10:18)**, coverage 86.4%
  - [x] `logger.MaskAttr` 민감 키 치환 (Authorization/api_token/mobile_phone/…) — **Green (Layer 5)**

- [x] **Logs tail hole-free 복구** (REQ-R05 AC-3, REQ-E01 AC-3) — 2/2 Green
  - [x] `LogsAdapter_Tail_HoleFreeResumeAfterRateLimit` — Green (Layer 3)
  - [x] `LogsService_TailPauseResume_PreservesSince` — Green (Layer 4)

- [x] **Policies 7종** (REQ-R04) — 4/4 Green
  - [x] Rich 4: OKTA_SIGN_ON, ACCESS_POLICY, PASSWORD, MFA_ENROLL action summary (service 레벨)
  - [x] Raw 3 raw JSON 타입 검증 (PROFILE_ENROLLMENT, POST_AUTH_SESSION, IDP_DISCOVERY)
  - [x] `service.PoliciesService.List_RequiresType` sentinel
  - [x] `service.PoliciesService.List_OrdersByPriorityAscending`

---

## Coverage 추적 (TESTING §9.2 / CONVENTIONS §13)

마지막 측정: 2026-04-24 10:18 (Layer 1 부분 Green 시점)

| 패키지 | 목표 | 현재 | 상태 |
|-------|------|------|------|
| `internal/domain/...` | 95% | (미측정) | Lock-in Green — Phase 6 Layer 2에서 측정 |
| `internal/service/...` | 85% | - | 전체 Red (build fail) |
| `internal/okta/...` | 75% | - | 대부분 Red (adapter panic) |
| `internal/okta/pagination` | 95% | **100.0%** ✓ | Green (엣지 4종 추가 후 100% 도달) |
| `internal/okta/errormap` | 95% | - | Red (FromResponse panic) |
| `internal/okta/ratelimit` | 95% | - | Red (Observe/CategoryFromPath panic) |
| `internal/mask` | (unclassified) | **86.4%** ✓ | Green, 서비스 기준 초과 |
| `internal/keys` | (unclassified) | **100.0%** ✓ | Green |
| `internal/security` | - | (미측정) | Lock-in peek Green |
| `internal/tui/...` | 60% | - | Red (users flow panic) |

Phase 6에서 각 계층이 first Green 도달하면 `go test -cover` 값을 이 표에 기록.

---

## Flaky 관측 (있을 시 append)

| 날짜 | 테스트 | 증상 | 조치 |
|------|-------|------|------|
| - | - | - | - |

Flaky 발견 시:
1. 즉시 `//go:build flaky` 태그로 격리 or `t.Skip("flaky: <issue>")`
2. GitHub issue 생성 (REQ-ID 포함)
3. CI 재시도로 덮지 않는다 (CONVENTIONS §13.9)
4. 근본 원인 파악 후 복원

---

## TESTING Appendix A 이월 항목 (Phase 6에서 확정)

- [ ] **§6.6 teatest 실측 결과** — 첫 teatest Green 전환 시 채움:
  - ANSI 처리 / stripping 필요성
  - 초기 `tea.Cmd` 실행 순서 · 타이밍
  - `goleak` allowlist 항목 (§9.3)
  - Resize 이벤트 전파 신뢰성
- [ ] **§9.3 goleak allowlist** — 첫 실행 시 관찰되는 teatest/bubbletea 내부 goroutine 등록

---

## 회귀/버그 추적 (Phase 7 QA 대비)

QA나 사용자 버그 발견 시:
1. 즉시 Red 재현 테스트 작성 (TESTING §7 Fail-First)
2. 테스트 파일 상단 주석 `// regression: <issue/PR>` (CONVENTIONS §13.2)
3. 본 로그에 "버그-N" 섹션으로 추적
4. Green 확인 후 회귀 방지 lock-in

---

**END of Phase 6 Green Progression Log.** developer의 각 전환 메시지 수령 시 append.
