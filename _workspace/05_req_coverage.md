# 05. Phase 5 REQ-ID → Red 테스트 매핑

**작성:** test-engineer
**날짜:** 2026-04-24
**근거:** TESTING.md §12 + Phase 5 실행 결과
**목적:** PRD 21개 REQ 각각이 최소 1개 Red 테스트로 커버됨을 단일 문서로 추적.

Phase 6 green 전환 시 각 행의 "현재 상태"를 업데이트한다.

---

## 공통 UX (7개)

| REQ-ID | 설명 | 테스트 파일 / 함수 | 현재 상태 |
|--------|------|------------------|----------|
| REQ-U01 | Vim 내비게이션 | `internal/keys/resolver_test.go::Test_KeysResolve_Defaults_IncludeVimAndArrows`, `internal/app/keymap_test.go::Test_Keymap_ClassifyKey_ArrowsEquivalentToVim` | Red (panic / undefined) |
| REQ-U02 | 커맨드 프롬프트 `:` | `internal/app/app_test.go::Test_AppModel_ColonKey_OpensCommandPalette` | Red (Update no-op) |
| REQ-U03 | 인크리멘털 검색 `/` | `internal/app/keymap_test.go::Test_Keymap_ClassifyKey_DoesNotDispatchWhenInputCapturesKeys`, `internal/tui/users/list_flow_test.go::Test_UsersListFlow_FilterAlice_OpensDetail` | Red (package missing) |
| REQ-U04 | 서버측 검색/필터 | `internal/service/users_service_test.go::Test_UsersService_Search_QIsPassedThroughAsFreeText`, `Test_UsersService_Search_SearchExpressionIsForwarded`, `internal/okta/errormap/map_test.go::Test_ErrorMap_BadRequest_PreservesFieldCauses` | Red (build failed / panic) |
| REQ-U05 | 리스트 → 상세 → 드릴다운 | `internal/tui/users/list_flow_test.go::Test_UsersListFlow_FilterAlice_OpensDetail` | Red (package missing) |
| REQ-U06 | 도움말 `?` | `internal/app/app_test.go::Test_AppModel_QuestionMarkKey_OpensHelp` | Red (Update no-op) |
| REQ-U07 | 종료 보호 | `internal/app/app_test.go::Test_AppModel_CtrlC_SingleTriggersQuitConfirm` | Red (QuitConfirmRequestMsg never emitted) |

## 리소스별 (5개)

| REQ-ID | 설명 | 테스트 파일 / 함수 | 현재 상태 |
|--------|------|------------------|----------|
| REQ-R01 | Users list/detail/factors | `internal/service/users_service_test.go` (전체), `internal/okta/users_adapter_test.go::Test_UsersAdapter_List_IteratesAllPagesViaLinkHeader`, `internal/domain/user_test.go::Test_UserStatus_*`, `internal/mask/mask_test.go::Test_Mask_Phone_KeepsLastFourDigits` | Red (service/okta) + Lock-in (domain) |
| REQ-R02 | Groups list + members + RULE 배지 | `internal/service/groups_service_test.go::Test_GroupsService_Search_FilterIsPassedThrough`, `Test_GroupsService_Search_FlagsDynamicTargetedGroupsViaRules` | Red (build failed) |
| REQ-R03 | Group rules + INVALID 경고 | `internal/domain/rule_test.go::Test_GroupRuleStatus_*`, `internal/service/rules_service_test.go::Test_RulesService_List_ResolvesTargetGroupNames`, `Test_RulesService_List_FallsBackToIDWhenGroupLookupFails` | Red (build failed) + Lock-in (domain) |
| REQ-R04 | Policies 7 타입 (rich 4 / raw 3) | `internal/domain/policy_test.go::Test_PolicyType_AllSevenTypesAreDefined`, `Test_PolicyType_RichRenderedTypesAreExactlyFour`, `Test_PolicyType_RawOnlyTypesExcludedFromRich`, `internal/service/policies_service_test.go::Test_PoliciesService_List_RequiresType`, `Test_PoliciesService_List_OrdersByPriorityAscending` | Red (service) + Lock-in (domain) |
| REQ-R05 | Logs tail + 프리셋 | `internal/service/logs_service_test.go` (전체 7개 테스트), `internal/okta/logs_adapter_test.go::Test_LogsAdapter_Tail_HoleFreeResumeAfterRateLimit` | Red (build failed + package stub panic) |

## 설정 · 인증 (5개)

| REQ-ID | 설명 | 테스트 파일 / 함수 | 현재 상태 |
|--------|------|------------------|----------|
| REQ-C01 | 설정 파일 로더 | `internal/config/loader_test.go::Test_ConfigLoad_ExplicitPathOverrideSucceeds`, `Test_ConfigLoad_FullConfigAllFourSections`, `Test_ConfigLoad_SyntaxErrorReportedWithLocation`, `Test_ConfigValidate_RejectsHTTPInProfileURL` | Red (panic) |
| REQ-C02 | 프로필 | `internal/config/loader_test.go::Test_ConfigProfile_StoresOnlyEnvVarName_NotTokenValue` | Red (panic) |
| REQ-C03 | 키 커스터마이징 | `internal/keys/resolver_test.go::Test_KeysResolve_UserOverride_WinsOnConflict`, `Test_KeysResolve_UnknownID_ProducesWarningNotError` | Red (panic) |
| REQ-C04 | 인증 우선순위 + 에러 매핑 | `internal/app/auth_test.go::Test_Auth_Resolve_CLIFlagWinsOverEnv`, `Test_Auth_Resolve_ProfileEnvWinsWhenCLIMissing`, `Test_Auth_Resolve_NoTokenReturnsError`, `internal/okta/errormap/map_test.go::Test_ErrorMap_FromResponse_MapsAllKnownCodes` | Red (build failed / panic) |
| REQ-C05 | 시크릿 유출 방지 | `internal/logger/mask_attr_test.go::Test_Logger_MaskAttr_ReplacesAuthorizationValueWithAsterisks`, `Test_Logger_MaskAttr_MasksAllSensitiveKeys`, `internal/app/auth_test.go::Test_Auth_Resolve_ErrorDoesNotLeakToken`, `internal/mask/mask_test.go` 전체, `internal/security/peek_test.go::Test_Peek_Testdata_HasNoRawPII` | Red (panic) + Lock-in (peek) |

## 에러 · Rate Limit · 관측성 (4개)

| REQ-ID | 설명 | 테스트 파일 / 함수 | 현재 상태 |
|--------|------|------------------|----------|
| REQ-E01 | Rate limit 동적 대응 | `internal/okta/ratelimit/monitor_test.go::Test_Monitor_Observe_StoresPerCategory`, `Test_CategoryFromPath_ClassifiesCorrectly`, `Test_Monitor_Observe_KeepsLastObservedPerCategory`, `internal/okta/errormap/map_test.go::Test_ErrorMap_RateLimit_ExposesRetryAfter`, `internal/okta/users_adapter_test.go::Test_UsersAdapter_List_RetriesOn429AndRecovers`, `internal/service/users_service_test.go::Test_UsersService_Search_CachesResultsByQueryKey` | Red (panic) |
| REQ-E02 | 에러 일관성 + `:errors` | `internal/app/app_test.go::Test_AppModel_ErrorMsg_EmitsToastWithAutoDismiss` | Red (Update no-op) |
| REQ-E03 | 오프라인 대응 | **TODO: Phase 6에 추가 테스트 필요** (statusbar.offline 전이, 자동 리프레시) | 미커버 (Phase 6 첫 task) |
| REQ-O01 | 디버그 로그 | `internal/logger/mask_attr_test.go::Test_Logger_New_WithDiscardSinkSucceeds` | Red (panic) |

---

## 통계

- **총 REQ**: 21개 (U 7, R 5, C 5, E 3, O 1)
- **Red 테스트 존재**: 20개 (REQ-E03만 잔여 — Phase 6에 보강)
- **Lock-in 테스트**: 3개 그룹 (domain, security peek, 일부 enum 구조)
- **Phase 5 완료 기준 충족 여부**: 20/21 — REQ-E03 추가 후 100%

---

## Phase 6 접근 순서 (test-engineer 추천)

1. **무의존 유틸 우선** (mask, logger, keys, pagination, errormap) → 빠른 Red → Green 승리감
2. **config / clock.FakeClock.NewTimer/Advance 구현** → 시간·IO 의존 테스트들 unblock
3. **ratelimit.Monitor** → okta client 통합에 필요
4. **okta.NewClient + 각 Adapter** → service 테스트 상당수 unblock
5. **service.*Service + LogsTail + RulesService** → tui 테스트 unblock
6. **app.ClassifyKey/ResolveToken/Update 로직** → E2E teatest 접근
7. **internal/tui/users, groups, rules, policies, logs/* (신규 패키지)**

각 단계에서 Fail-First 루프: 테스트 확인 → Red → 최소 구현 → Green → 리팩터.
