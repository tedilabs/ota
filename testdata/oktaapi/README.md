# testdata/oktaapi/

Okta Management API v1 응답 fixture.

## 규칙

- **합성 데이터만.** 실 tenant 기록 금지. 모든 이메일은 `@redacted.example.com`, 전화는 `+1-555-000-NNNN`.
- 파일명: `<endpoint>_<scenario>.json`
- 메타 데이터(상태 코드, 헤더)는 `<file>.meta.json`에 분리.
- 캡처 근거: `/Users/austin/workspace/tedilabs/ota/_workspace/02_okta_domain_input.md` §1.2~§1.7 응답 스키마.

## 디렉토리

- `users/` — GET /api/v1/users, /users/{id}, /users/{id}/factors, /users/{id}/groups
- `groups/` — GET /api/v1/groups, /groups/{id}/users
- `grouprules/` — GET /api/v1/groups/rules
- `policies/` — GET /api/v1/policies?type=<TYPE>, /policies/{id}/rules (7종)
- `logs/` — GET /api/v1/logs
- `errors/` — 8종 errorCode (E0000001 ~ E0000047). PRD §7.7 에러 매핑 테이블 기준.

## 시드 규약 (TESTING §5.4)

사용자 ID prefix `00u`, 그룹 `00g`, 그룹 규칙 `0pr`, 정책 `00p` — 모두 허구.

| 식별자 | 상태/타입 | 비고 |
|-------|---------|------|
| `00u_active_alice` | ACTIVE, factors=[push,sms] | 주요 사용자 예시 |
| `00u_active_bob` | ACTIVE, factors=[webauthn] | 키 인증 예시 |
| `00u_suspended` | SUSPENDED | 상태 색상 |
| `00u_locked` | LOCKED_OUT | 경고 테스트 |
| `00u_password_expired` | PASSWORD_EXPIRED | magenta 테스트 |
| `00u_deprovisioned` | DEPROVISIONED | DELETED 혼동 방지 |
| `00g_engineering` | OKTA_GROUP, 동적 | Group Rule target |
| `00g_everyone` | BUILT_IN | 대용량 배너 |
| `0pr_active` | ACTIVE | expression 정상 |
| `0pr_invalid` | INVALID | 경고색 |
| `00p_signon_default` | OKTA_SIGN_ON, system=true | 기본 정책 |
