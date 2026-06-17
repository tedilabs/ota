# Feature Input: Users Edit Form

## Date
2026-06-17 (UTC)

## Origin
사용자 직접 요청. 이전 작업 맥락: 홈 대시보드 폐기 후 boot screen이 Users 리스트로 환원된 상태 (commit `e2fbd51`).

## UX Specification (사용자 명시)

1. **트리거**:
   - Users 리스트 뷰에서 선택된 행에서 `e` 키
   - User Detail 뷰에서 `e` 키
2. **동작**: edit form 화면이 나타남
3. **결과**: 수정 후 저장 또는 취소 가능

## Scope (사용자 확정)

- **리소스**: Users 만 (단일 리소스로 풀하네스 진행)
- **워크플로우**: tui-product-orchestrator 풀세트 (Phase 2 PRD → Phase 7 QA)

## Initial Field Hypothesis

PM + okta-expert가 PRD에서 확정하되, 출발선:
- `profile.firstName`
- `profile.lastName`
- `profile.email`
- `profile.login`

추가 가능 후보 (Okta standard profile schema 전체):
- `displayName`, `nickName`, `title`, `division`, `department`,
  `employeeNumber`, `mobilePhone`, `secondEmail`

PII 필드 (`mobilePhone`, `secondEmail`)는 마스킹/언마스킹 정책과 통합 필요.
Custom Profile (`Extras`)는 MVP 제외 권장.

## Project State

- **이전 PRD/디자인/아키텍처/테스트/QA 산출물 존재**: `_workspace/`, `docs/`
- **써드 파티 라이브러리**:
  - Charm 생태계 (Bubble Tea, Lipgloss, Bubbles[textinput])
  - 사용 가능한 textinput 위젯이 이미 logs 필터 등에서 활용됨
- **첫 mutation 표면**:
  - 기존 mutation: lifecycle ops (ResetPassword, Unlock, ResetFactors,
    Activate, Deactivate, ExpirePassword, Delete)
  - **profile mutation은 처음** — `UsersPort.Update(ctx, userID, UserProfile) (User, error)` 추가 필요
  - Okta API: `POST /api/v1/users/{userId}` (partial profile patch, merge semantics)

## Constraints

1. **Conventional Commits**: feat/fix/docs prefix
2. **TDD Fail-First**: 테스트 먼저
3. **Charm 생태계**: bubbles/textinput 우선
4. **마스킹 정책 유지**: PII 필드는 기존 mask 정책 준수
5. **레이트리밋 안전**: 폼 저장은 단발성 mutation — list 화면 rate-limit 영향 없음
6. **에러 처리**: Okta 4xx (검증 실패 / 권한 없음 / 충돌) 명확한 사용자 피드백
7. **취소 안전**: 모든 변경 사항은 명시적 save 전까지 미반영, ESC 으로 cancel 가능
8. **권한**: API 토큰의 권한이 부족할 때 → 명확한 안내 (저장 시도 단계에서 발견 가능)

## Open Questions for PRD Phase

1. **편집 필드 최종 목록**: standard profile 전체? 핵심 4개? 단계별?
2. **Login 변경 가능 여부**: Okta는 login 변경을 지원하지만 인증 영향이 크다 → PRD에서 결정
3. **검증 규칙**:
   - email 형식
   - login 형식 (email-like)
   - 빈 값 허용 vs 필수
4. **충돌 처리**: 다른 세션에서 동시 편집 시 (Okta는 If-Match 헤더 지원?)
5. **취소 확인**: 변경 사항이 있을 때 ESC 누르면 한 번 더 확인할지?
6. **저장 중 상태**: 저장 진행 중 UI / 실패 시 폼 유지 vs 닫기

## Deliverables (예상)

| Phase | 산출물 |
|------|--------|
| 2 | `_workspace/edit-form-users/02_pm_prd_addendum.md`, `docs/PRD.md` 패치 |
| 3 | `_workspace/edit-form-users/03_tui_design_addendum.md`, `docs/TUI_DESIGN.md` 패치 |
| 4 | `docs/ARCHITECTURE.md` form 위젯 섹션, `docs/CONVENTIONS.md` form 패턴, `docs/TESTING.md` form-screen 테스트 패턴 |
| 5 | `internal/tui/users/edit_*_test.go`, `internal/service/*_test.go` (Update 관련) |
| 6 | `internal/domain/ports.go` (Update 메서드), `internal/okta/users.go` (HTTP), `internal/service/users.go`, `internal/tui/users/edit.go` + 통합 |
| 7 | `_workspace/edit-form-users/07_qa_findings.md`, 회귀 테스트 추가 |
