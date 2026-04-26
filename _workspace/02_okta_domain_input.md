# 02. Okta Domain Input — PRD/설계에 주입할 도메인 사실

**작성:** okta-expert (도메인 전문가)
**날짜:** 2026-04-24
**대상 프로젝트:** ota (Okta TUI, k9s 스타일)
**적용 기준:** Okta Identity Engine (OIE), Okta REST API v1, Okta Go SDK v5
**근거 수준 표기:**
- [확정] Okta 공식 문서/SDK 기준으로 확인된 사실
- [관례] 업계 운영 관례 또는 일반적 모범 사례
- [추정] 일반적 추정. 배포 전 확인 필요
- [확인필요] 테스트 tenant에서 검증 권장

> 이 문서는 PRD와 기술 설계의 도메인 입력이다. PM은 이 문서를 참조해 요구사항을, 개발자는 API 선택을, QA는 엣지케이스 점검을 한다. 장기적으로 유지보수하는 "사실 카탈로그"이며, 최신성 검증은 공식 문서로 재확인한다.

---

## 0. 공통 전제

### 0.1 Okta Edition: Identity Engine (OIE) 기준 [확정]
- 2024년 이후 Okta 조직은 대부분 OIE. Classic Engine은 EOL 진행 중.
- 본 문서는 OIE 기준으로 기술.
- OIE와 Classic은 정책 모델이 근본적으로 다름 (특히 Authentication Policy). Classic 호환 코드는 향후 제거 가능성.

### 0.2 Base URL [확정]
- Production: `https://<org>.okta.com/api/v1/...`
- Preview/Sandbox: `https://<org>.oktapreview.com/api/v1/...`
- Custom Domain: `https://id.example.com/api/v1/...`

> ota 설정 파일에서는 `<org>.okta.com` 혹은 custom domain 전체를 받아야 함. 관리자는 custom domain을 쓰는 경우가 많음.

### 0.3 인증 방식
| 방식 | 장점 | 단점 | ota MVP 권장 |
|-----|------|------|------------|
| API Token (SSWS) | 간단, 즉시 사용 | 발급자 권한 승계, 수동 회전 | **기본 (MVP)** [관례] |
| OAuth 2.0 Service App + Private Key JWT | 최소 권한, 스코프 제어 | 초기 설정 복잡 | 후속 버전 [관례] |
| DPoP | 토큰 탈취 방지 | 구현 복잡 | 범위 밖 |

> ota는 관리자/IAM 엔지니어의 로컬 도구이므로 API Token을 우선 지원하고, 향후 OAuth2 Private Key JWT로 확장. 환경변수 `OKTA_API_TOKEN`, `OKTA_ORG_URL`은 표준 관례.

### 0.4 System Log 불변식 [확정]
- ota가 호출하는 모든 mutative API는 System Log에 이벤트로 기록됨 (관리자 액터로).
- MVP는 read-only 지만, 향후 쓰기 액션 추가 시 UI에 "이 동작은 Okta System Log에 기록됩니다" 안내 권장.

---

## 1. 리소스 모델 요약

### 1.1 리소스 식별자 패턴 [확정]
| 리소스 | 식별자 | 형식 | 비고 |
|-------|-------|------|------|
| User | `id` | `00u...` | URL-safe. 이메일(`profile.login`)은 식별자 대체 가능 (일부 엔드포인트) |
| Group | `id` | `00g...` | |
| Group Rule | `id` | `0pr...` | |
| Application | `id` | `0oa...` | |
| Policy | `id` | `00p...` | |
| Policy Rule | `id` | `0pr...` 또는 유사 | 정책 타입별 prefix 다름 [확인필요] |
| Log Event | `uuid` | UUID | 단건 조회 API 없음 (검색만) |

> ARN 같은 글로벌 식별자 없음. `id`는 tenant-scoped. 여러 tenant 간 리소스 매칭은 name/email 기반으로 해야 함.

### 1.2 Users [확정]

**생명주기 상태:**
```
STAGED → PROVISIONED → ACTIVE ⇄ SUSPENDED → DEPROVISIONED → DELETED
                       ↑  ↓
                 PASSWORD_EXPIRED / LOCKED_OUT / RECOVERY
```

| 상태 | 의미 | 되돌림 가능 |
|-----|------|---------|
| STAGED | 활성화 미시작 | - |
| PROVISIONED | 활성화 메일 발송, 비밀번호 미설정 | - |
| ACTIVE | 로그인 가능 | - |
| SUSPENDED | 임시 차단 | ✅ unsuspend |
| LOCKED_OUT | 로그인 반복 실패 | ✅ unlock |
| PASSWORD_EXPIRED | 정책상 만료 | ✅ 재설정 |
| RECOVERY | 비밀번호 재설정 프로세스 중 | 자동 진행 |
| DEPROVISIONED | 비활성화 | ✅ reactivate |
| DELETED | 영구 삭제 | ❌ 불가 |

**핵심 엔드포인트:**
- `GET /api/v1/users?search=<SCIM>&filter=<SCIM>&q=<free>&limit=200&after=<cursor>` — 목록
- `GET /api/v1/users/{userIdOrLogin}` — 상세. `me`도 허용
- `GET /api/v1/users/{id}/groups` — 속한 그룹
- `GET /api/v1/users/{id}/factors` — 등록된 MFA factor (MVP 범위 여부는 §7 참고)
- `POST /api/v1/users/{id}/lifecycle/{activate|deactivate|suspend|unsuspend|unlock|reset_password}` — 상태 전이
- `DELETE /api/v1/users/{id}` — DEPROVISIONED 상태여야 삭제 가능 (2단계)

**응답 구조 요약:**
```json
{
  "id": "00u...",
  "status": "ACTIVE",
  "created": "2024-01-02T...",
  "activated": "2024-01-03T...",
  "lastLogin": "2026-04-23T...",
  "lastUpdated": "2026-04-01T...",
  "statusChanged": "2024-01-03T...",
  "passwordChanged": "2025-11-10T...",
  "type": { "id": "oty..." },
  "profile": {
    "login": "alice@acme.com",
    "email": "alice@acme.com",
    "firstName": "Alice",
    "lastName": "Smith",
    "displayName": "...",
    "mobilePhone": "...",
    "secondEmail": "...",
    "department": "Engineering",
    "title": "Senior SWE"
    // ... + 조직 커스텀 필드 (Profile Editor에서 정의됨)
  },
  "credentials": {
    "provider": { "type": "OKTA", "name": "OKTA" },
    "recovery_question": { "question": "..." }  // 설정 시에만
  }
}
```

**리스트/상세 표시 권장 필드 (k9s 스타일):**

리스트 (width 제약):
- `status` (색상 구분 표지, ACTIVE=녹색/SUSPENDED=노랑/DEPROV=회색/LOCKED=빨강)
- `profile.login` 또는 `email`
- `profile.displayName` 또는 firstName+lastName
- `lastLogin` (relative time: "2h ago")
- `created` 또는 `statusChanged`

상세:
- 위 전체 + `id`, `credentials.provider`, `type`, 커스텀 profile 필드
- 속한 그룹 (별도 API 호출)
- 최근 로그인 이벤트 (System Log 링크)

**중요 UX 주의점:**
- **SUSPENDED vs DEPROVISIONED** — 사용자가 혼동함. 시각적으로 뚜렷이 구분.
- **DELETE는 DEPROVISIONED를 거쳐야 함** — UI는 두 단계를 강제하거나 안내.
- `profile`에는 조직별 커스텀 필드가 있을 수 있음. 표시 시 고정 + 동적 필드 분리.

### 1.3 Groups [확정]

**그룹 타입:**
| `type` | 설명 | 멤버 수동 편집 |
|-------|------|--------|
| `OKTA_GROUP` | 일반 그룹 (정적 또는 Group Rule 기반 동적) | ✅ (정적만) |
| `APP_GROUP` | AD/LDAP/앱 동기화 | ❌ |
| `BUILT_IN` | `Everyone` 등 | ❌ |

> **주의:** Group Rule로 멤버십이 결정되는 "동적 그룹"도 type은 `OKTA_GROUP`. 별도 플래그 없음. UI에서는 Group Rule이 해당 그룹을 타겟팅하는지 조회해 동적/정적 판단. [확정]

**핵심 엔드포인트:**
- `GET /api/v1/groups?filter=<SCIM>&search=<SCIM>&q=<free>&limit=200&after=<cursor>`
- `GET /api/v1/groups/{id}` — 상세
- `GET /api/v1/groups/{id}/users?limit=200&after=<cursor>` — 멤버 (페이지네이션 필수)
- `GET /api/v1/groups/{id}/apps` — 할당된 앱
- `PUT /api/v1/groups/{id}/users/{userId}` — 멤버 추가 (정적만)
- `DELETE /api/v1/groups/{id}/users/{userId}` — 멤버 제거
- `DELETE /api/v1/groups/{id}` — 삭제

**응답 구조:**
```json
{
  "id": "00g...",
  "created": "...",
  "lastUpdated": "...",
  "lastMembershipUpdated": "...",
  "type": "OKTA_GROUP",
  "objectClass": ["okta:user_group"],
  "profile": {
    "name": "Engineering",
    "description": "All engineers"
  }
}
```

> `objectClass`는 배열. 대부분 `okta:user_group` 하나.

**리스트 권장 필드:**
- `type` (아이콘으로 구분: OKTA/APP/BUILT_IN)
- `profile.name`
- `profile.description`
- 멤버 수 (별도 호출 필요 — 비싸니 요청 시 표시)
- `lastMembershipUpdated`
- (동적이면 마커) "RULE" 배지

**UX 주의점:**
- 멤버 수는 별도 호출 필요. 대용량 그룹(수만 명)은 표시 지연 가능. 지연 로딩 권장. [관례]
- `Everyone` 그룹 클릭 → 멤버 수천~수만 명 가능. 페이지네이션 경고 필수.
- 앱 할당 조회는 권한 이슈 있을 수 있음 (Read-Only Admin이라도 일부 제한).

### 1.4 Group Rules [확정]

**상태:**
- `INVALID` — expression 오류. 활성화 불가.
- `ACTIVE` — 활성. 사용자 profile 변경 시 재평가.
- `INACTIVE` — 비활성. **이 상태로 전환하면 해당 규칙으로 생긴 멤버십이 제거됨.** (!!!)

**핵심 엔드포인트:**
- `GET /api/v1/groups/rules?limit=50&after=<cursor>` — 기본 limit 50 [확정]
- `GET /api/v1/groups/rules/{ruleId}` — 상세
- `POST /api/v1/groups/rules/{id}/lifecycle/{activate|deactivate}` — 상태 전이
- `DELETE /api/v1/groups/rules/{id}` — INACTIVE여야 삭제 가능

**응답 구조:**
```json
{
  "id": "0pr...",
  "type": "group_rule",
  "status": "ACTIVE",
  "name": "Engineers to Eng group",
  "created": "...",
  "lastUpdated": "...",
  "conditions": {
    "people": {
      "users": { "exclude": [] },
      "groups": { "exclude": [] }
    },
    "expression": {
      "value": "user.department == \"Engineering\"",
      "type": "urn:okta:expression:1.0"
    }
  },
  "actions": {
    "assignUserToGroups": {
      "groupIds": ["00g..."]
    }
  }
}
```

**리스트 권장 필드:**
- `status` (ACTIVE/INACTIVE/INVALID — INVALID는 경고색)
- `name`
- 타겟 그룹 이름 (`actions.assignUserToGroups.groupIds` 중 첫 1~2개)
- expression 요약 (긴 경우 truncate)
- `lastUpdated`

**UX 주의점:**
- **비활성화 시 경고 필수**: "이 규칙으로 멤버십이 있는 N명이 그룹에서 제거됩니다." (정확한 N은 API로 구할 수 없음 — 추정치나 모른다고 명시)
- expression 표시는 monospace + syntax highlight 권장 (Okta Expression Language, §5 참고)
- ruleId prefix는 `0pr`이지만 Policy Rule과 혼동 주의 [확인필요]

### 1.5 Applications (MVP 선택적) [확정]
- MVP 스코프에서 명시되지 않았지만 Groups와 긴밀히 연결됨. 적어도 "이 그룹이 몇 개 앱에 할당됨" 정도는 가치 있음.
- `signOnMode`: `SAML_2_0`, `OPENID_CONNECT`, `SECURE_PASSWORD_STORE`, `BOOKMARK`, `AUTO_LOGIN` 등
- 엔드포인트: `GET /api/v1/apps?filter=status eq "ACTIVE"&limit=200`

> ota MVP에서는 Applications를 독립 뷰로 넣지 않더라도, Group 상세 화면에서 "이 그룹에 할당된 앱 N개" 정보는 안전하게 표시 가능. [추정]

### 1.6 Policies [확정]

OIE Policy 타입:
| `type` | 통칭 | 대상 | 핵심 결정 | ota 표시 우선순위 |
|-------|------|------|---------|---------------|
| `OKTA_SIGN_ON` | Global Session Policy | 조직 전체 로그인 세션 | 세션 수명, MFA 요구, 네트워크 | 높음 |
| `ACCESS_POLICY` | Authentication Policy (앱별) | 개별 앱의 인증 | MFA 필요성, 디바이스, 재인증 | 높음 (앱 많음) |
| `PASSWORD` | Password Policy | 사용자 그룹 | 복잡도, 만료, 리커버리 | 중간 |
| `MFA_ENROLL` | MFA Enrollment Policy | 사용자 그룹 | 어떤 authenticator 등록 | 중간 |
| `PROFILE_ENROLLMENT` | Profile Enrollment Policy | 셀프 등록 | 신규 사용자 스키마 | 낮음 |
| `POST_AUTH_SESSION` | Post-Login Session Policy | 인증 후 세션 | 재인증 조건 | 낮음 |
| `IDP_DISCOVERY` | IdP Discovery Policy | 라우팅 | 어느 IdP로 보낼지 | 낮음 |
| `ENTITY_RISK` (최신) | Entity Risk Policy | 위험 감지 | 세션 종료 등 | 최신/확인필요 |

**엔드포인트:**
- `GET /api/v1/policies?type=<POLICY_TYPE>&limit=20` — 타입 **지정 필수** [확정]
- `GET /api/v1/policies/{policyId}` — 상세
- `GET /api/v1/policies/{policyId}/rules` — 규칙 (priority 순)
- `POST /api/v1/policies/{policyId}/rules/{ruleId}/lifecycle/{activate|deactivate}`

**Policy 응답 구조 (공통):**
```json
{
  "id": "00p...",
  "name": "Default Policy",
  "type": "OKTA_SIGN_ON",
  "priority": 1,
  "status": "ACTIVE",
  "system": true,                // 기본 정책이면 true
  "created": "...",
  "lastUpdated": "...",
  "conditions": { ... },         // 적용 대상 (그룹 등)
  "settings": { ... }            // 타입별 상세 설정
}
```

**Policy Rule 응답 구조 (타입별 상이하지만 공통 형태):**
```json
{
  "id": "...",
  "name": "Require MFA for admins",
  "priority": 1,
  "status": "ACTIVE",
  "system": false,
  "conditions": {
    "people": { "users": { "include": [], "exclude": [] }, "groups": { "include": ["00g..."] } },
    "network": { "connection": "ANYWHERE" },
    "authContext": { "authType": "ANY" },
    "platform": { "include": [{"type":"ANY"}], "exclude": [] },
    "riskScore": { "level": "ANY" },
    "device": { "migrated": false, "registered": false, "managed": false }
  },
  "actions": {
    "signon": {                           // OKTA_SIGN_ON 타입
      "access": "ALLOW",
      "requireFactor": true,
      "factorLifetime": 15,
      "session": { "maxSessionIdleMinutes": 120, "maxSessionLifetimeMinutes": 0, "usePersistentCookie": false }
    }
    // ACCESS_POLICY(앱별)는 "appSignOn" 아래 verificationMethod, constraints
    // PASSWORD는 "passwordSettings" 아래 complexity/lifecycle
    // MFA_ENROLL은 "enroll"
  }
}
```

> `conditions`/`actions` 스키마는 정책 타입별로 다르다. ota MVP는 **읽기 전용 표시**이므로 구조를 이해해 렌더링하면 됨. JSON 원문 토글 표시가 단기 대안. [관례]

**리스트 권장 필드 (Policy 목록, 타입 선택 후):**
- `priority` (숫자, 낮을수록 먼저)
- `status` (ACTIVE/INACTIVE)
- `name`
- `system` (배지: 기본 정책 표시)
- `lastUpdated`

**Rule 리스트 (특정 policy 내):**
- `priority`
- `status`
- `name`
- action 요약 (예: "Require MFA" or "Deny" or "Allow w/o MFA")
- `lastUpdated`

**UX 주의점:**
- 정책은 **타입 단위**로 탐색. k9s의 namespace처럼 "Policy Type" selector가 자연.
- 규칙 평가 순서는 priority 오름차순. UI에서 "이 user가 이 정책의 어떤 규칙에 걸릴지" 시뮬레이션 UX는 MVP 이후 가치 큼. [관례]
- ACCESS_POLICY는 **앱과 매핑**되어 있어 수백 개일 수 있음. 페이지네이션 + 앱 연결 표시 필요. [확정]
- `system=true`인 Default Policy/Rule은 삭제/비활성화 불가. UI 표지.

### 1.7 System Logs [확정]

**엔드포인트:**
```
GET /api/v1/logs?since=<ISO8601>&until=<ISO8601>&filter=<SCIM>&q=<free>&limit=1000&sortOrder=ASCENDING|DESCENDING
```

**쿼리 파라미터:**
| 파라미터 | 설명 | 주의 |
|---------|------|------|
| `since` | 시작 시각 (ISO8601) | 기본 보관 90~180일 (플랜 의존) [확인필요] |
| `until` | 끝 시각 | 없으면 최신까지 |
| `filter` | SCIM 필터 | §6 참조 |
| `q` | 자유 텍스트 | actor/target 필드 매치 |
| `sortOrder` | ASCENDING(기본)/DESCENDING | 최신순이면 DESCENDING 명시 |
| `limit` | 최대 1000 | Rate Limit별로 |

**응답 요약:**
```json
{
  "uuid": "...",
  "published": "2026-04-24T12:34:56.789Z",
  "eventType": "user.session.start",
  "version": "0",
  "severity": "INFO",
  "legacyEventType": "core.user_auth.login_success",
  "displayMessage": "User login to Okta",
  "actor": { "id": "00u...", "type": "User", "alternateId": "alice@acme.com", "displayName": "Alice" },
  "client": {
    "userAgent": { "browser": "CHROME", "os": "Mac OS X", "rawUserAgent": "..." },
    "zone": "OFF_NETWORK",
    "device": "Computer",
    "ipAddress": "1.2.3.4",
    "geographicalContext": { "country": "US", "state": "California", "city": "San Francisco" }
  },
  "outcome": { "result": "SUCCESS", "reason": null },
  "target": [ { "id": "...", "type": "AppInstance", "displayName": "Salesforce" } ],
  "authenticationContext": { "authenticationProvider": "OKTA_AUTHENTICATION_PROVIDER", ... },
  "securityContext": { "asNumber": 12345, "asOrg": "...", "isp": "...", "isProxy": false },
  "transaction": { "type": "WEB", "id": "...", "detail": {} },
  "request": { "ipChain": [ { "ip": "1.2.3.4", "geographicalContext": { ... } } ] },
  "debugContext": { "debugData": { ... } }
}
```

**리스트 권장 필드:**
- `published` (절대/상대 토글)
- `severity` (색상: DEBUG 회색/INFO 녹색/WARN 노랑/ERROR 빨강)
- `eventType` (또는 displayMessage — eventType 쪽이 기계친화)
- `actor.displayName` + `actor.alternateId` (이메일)
- `target[0].displayName` (있으면)
- `outcome.result` (SUCCESS/FAILURE/CHALLENGE)
- `client.ipAddress` 또는 geo

**상세:**
- JSON 원문 Pretty-print + 구조화된 섹션 (Actor/Target/Client/Outcome/Debug)
- actor/target id로 해당 화면 점프 (강력한 UX)

**폴링 전략 (실시간 스트리밍 대용):** [확정 + 관례]
- Okta는 **실시간 스트림 API 없음**. 폴링 or Event Hook.
- ota는 로컬 TUI이므로 **폴링**.
- 권장 알고리즘 (tail mode):
  ```
  1. 초기: since = now-5min, sortOrder=ASCENDING, limit=1000
  2. 응답 받으면 각 이벤트 처리, 마지막 이벤트의 published 기록
  3. 다음 폴링: since = 마지막 published (또는 + 1ms로 중복 방지)
  4. 간격: 5~10초 (관례). 너무 짧으면 rate limit
  5. 429 시 backoff, UI에 "paused" 표시
  ```
- 한 번에 많은 과거를 땡기면 rate limit 위험. 초기 로드는 최근 범위만.
- `DESCENDING`으로 리스트 모드 시 페이지네이션으로 과거 탐색.

**Event Type 카테고리 (UI 필터 프리셋 소스):**
- 인증: `user.session.*`, `user.authentication.*`
- 사용자 생명주기: `user.lifecycle.*`
- 그룹: `group.*`, `group.user_membership.*`, `group.rule.*`
- 앱: `application.*`, `application.user_membership.*`
- 정책: `policy.*`
- 보안: `user.account.lock`, `security.threat.*`
- API: `system.api_token.*`

전체 카탈로그: https://developer.okta.com/docs/reference/api/event-types/

**UX 주의점:**
- 시간대: Okta는 UTC. 로컬 변환 옵션 제공.
- `outcome.result=FAILURE`는 강조.
- `actor` 중 `User` 아닌 `SystemPrincipal`도 있음 (자동화/API 토큰 호출).

---

## 2. API 관례 (공통)

### 2.1 페이지네이션 — Link 헤더 커서 방식 [확정]
- `Link: <...?after=<cursor>&limit=...>; rel="next"` 헤더
- 병렬 페이지 요청 **불가** (순차 fetch)
- `after` 커서는 불투명. 디코드/조작 금지.
- `limit` 기본값이 엔드포인트별로 다름:

| 엔드포인트 | 최대 limit | 권장 |
|----------|---------|-----|
| `/users` | 200 | 200 |
| `/groups` | 200 | 200 |
| `/groups/{id}/users` | 200 | 200 |
| `/groups/rules` | 200 (기본 50) | 50~200 |
| `/apps` | 200 | 200 |
| `/policies` | 제한 엄격 [확인필요] | 20 |
| `/policies/{id}/rules` | N/A (보통 적음) | 전부 |
| `/logs` | 1000 | 1000 |

### 2.2 Rate Limit [확정]

**응답 헤더:**
```
X-Rate-Limit-Limit: 600
X-Rate-Limit-Remaining: 598
X-Rate-Limit-Reset: 1713620000   # unix epoch (초)
```

**429 응답:**
```
HTTP 429 Too Many Requests
Retry-After: 10   # 초 또는 HTTP-date
X-Okta-Request-Id: ...
```

**카테고리별 한도 (플랜별로 다름):** [확정 + 확인필요]
| 카테고리 | Enterprise 기본 한도 (분당) | 비고 |
|---------|----------------------------|------|
| 관리 API (`/users`, `/groups`) | 600~1200 | 플랜 의존 |
| 로그 API (`/logs`) | 120 | 엄격 |
| `/apps` | 500 | |
| `/policies` | 100 | 엄격 |
| `/users/{id}/lifecycle/*` | 600 | |

> 정확한 수치는 tenant별로 다름. ota는 헤더 기반 **동적 판단**으로 대응하는 것이 안전. 고정 수치 하드코딩 비추천.

**대응 전략 [관례]:**
1. `X-Rate-Limit-Remaining < 10` 감지 시 자동 속도 조절 (concurrency 1로 낮추기, delay 추가)
2. 429 수신 시 `Retry-After` + jitter 대기 후 1회 재시도 (최대 3회)
3. 사용자에게 상태 표시: "Rate limited — waiting 10s (Remaining: 5)"
4. 캐시: 사용자/그룹/정책 리스트 결과는 짧은 TTL(예: 30초) 메모리 캐시로 중복 호출 감소

**레이트 리밋 경고 임계값 (ota UX):**
- Remaining 10% 이하 → 노란 경고
- 429 → 빨간 경고 + 자동 대기

### 2.3 에러 응답 [확정]
```json
{
  "errorCode": "E0000001",
  "errorSummary": "Api validation failed: login",
  "errorLink": "E0000001",
  "errorId": "oaeXXXXXXX",
  "errorCauses": [
    { "errorSummary": "login: An object with this field already exists in the current organization" }
  ]
}
```

**주요 errorCode → 사용자 메시지 매핑:**
| Code | 상황 | ota UX |
|------|------|-----|
| E0000001 | 유효성 검사 실패 | errorCauses 필드별 표시 |
| E0000004 | 인증 실패 | "API 토큰 확인 필요" |
| E0000006 | 권한 없음 | "권한 부족: {scope}" |
| E0000007 | Not found | "리소스 없음 (이미 삭제?)" + 목록 새로고침 유도 |
| E0000011 | 토큰 무효 | "토큰 재발급 필요" |
| E0000022 | 삭제 불가 | "먼저 비활성화하세요" |
| E0000038 | 기능 비활성화 | "조직 설정에서 비활성화됨" |
| E0000047 | Rate limit | 자동 재시도 |

### 2.4 멱등성 [확정]
- 대부분의 `lifecycle/*` 액션은 **멱등 아님** (상태 전이가 유효하지 않으면 에러)
- 예: 이미 SUSPENDED 사용자에게 `suspend` 재호출 → 409 또는 에러
- ota는 읽기 MVP이므로 영향 적음. 향후 쓰기 기능 추가 시 현재 상태 선조회 권장.

### 2.5 날짜 형식 [확정]
- 모든 입출력 ISO8601 UTC: `2026-04-24T12:34:56.789Z`
- 클라이언트에서 로컬 타임존 변환.

---

## 3. 검색/필터 문법

Okta는 3종의 쿼리 파라미터가 혼재. 각각 의미 다름. [확정]

### 3.1 `q` — 자유 텍스트 [확정]
- 간단한 prefix/substring 검색
- Users: `firstName`, `lastName`, `email`, `login` 등 주요 필드에 매치
- Groups: `name` 매치
- Logs: actor/target/message 매치
- 장점: 단순. 단점: 정확도 낮음, 필드 지정 불가.

### 3.2 `search` — SCIM-like 고급 검색 [확정]
Users/Groups에서 권장. 모든 필드 지원 + 정렬 + 연산자 다양.
```
search=profile.department eq "Engineering" and status eq "ACTIVE"
search=profile.lastName sw "S"                 # starts with
search=lastUpdated gt "2026-01-01T00:00:00.000Z"
search=profile.email ew "@acme.com"            # ends with
```
**연산자:** `eq`, `ne`, `gt`, `ge`, `lt`, `le`, `sw`, `ew`, `co`(contains), `pr`(present), `and`, `or`, `()`.

**Users에서만:** eventually consistent (인덱스 업데이트 지연 가능). 실시간 직후 작성된 사용자는 검색에 지연될 수 있음. [확정]

### 3.3 `filter` — 엄격 필터 (SCIM) [확정]
- 특정 엔드포인트에서만 지원. search보다 덜 유연.
- Groups: `filter=type eq "OKTA_GROUP"`
- Apps: `filter=status eq "ACTIVE"`
- Logs: `filter=eventType eq "user.session.start" and outcome.result eq "FAILURE"`

### 3.4 Users 검색 표기법 예시 [확정]
```
# 활성 사용자 중 Engineering 부서
search=status eq "ACTIVE" and profile.department eq "Engineering"

# 이메일 접미어
search=profile.email ew "@acme.com"

# 최근 30일 내 마지막 로그인
search=lastLogin gt "2026-03-25T00:00:00Z"

# 이름으로 시작
search=profile.firstName sw "Al"
```

### 3.5 Logs 필터 예시 [확정]
```
# 로그인 실패
filter=eventType eq "user.session.start" and outcome.result eq "FAILURE"

# 특정 사용자 활동
filter=actor.id eq "00u..."

# 앱 할당 변경
filter=eventType sw "application.user_membership" and target.id eq "0oa..."

# MFA 인증
filter=eventType sw "user.authentication.auth_via_mfa"
```

### 3.6 ota UI 권장 [관례]
- **Users/Groups**: `search` (SCIM-like) 기반 필터 빌더 또는 raw 입력
- **Logs**: preset 필터(지난 24h 로그인 실패 등) + advanced로 raw filter 입력
- `/` 키로 간단 텍스트(`q`) 검색
- `:` 커맨드로 고급 모드 (`:search status=ACTIVE`)
- **주의:** SCIM 쿼리는 이스케이프 주의. 따옴표는 ASCII 큰따옴표만.

---

## 4. 권한 모델 (Read-Only Admin)

### 4.1 Okta Administrator Role 분류 [확정]
| Role | 읽기 | 쓰기 | ota MVP 적합 |
|------|-----|------|-----|
| Super Administrator | 전체 | 전체 | 너무 강함 |
| Organization Administrator | 대부분 | 대부분 | 과함 |
| Read-Only Administrator | 대부분 | 없음 | ✅ **권장** |
| Group Administrator | 특정 그룹 | 제한적 | 제한적 |
| Application Administrator | 앱 | 앱 | 부족 |
| API Access Management Admin | OAuth | OAuth | 부족 |
| Help Desk Administrator | 사용자 일부 | 일부 lifecycle | 부족 |
| Custom Role (OIE) | 스코프 지정 | 스코프 지정 | ✅ 향후 |

### 4.2 Read-Only Admin 접근 가능 리소스 [확정]
- Users (읽기)
- Groups (읽기, 멤버 읽기)
- Group Rules (읽기)
- Applications (읽기)
- Policies (읽기)
- System Logs (전체 읽기)
- Admin dashboard 메트릭 (읽기)

### 4.3 Read-Only Admin이 **못 하는** 것 [확정]
- 모든 mutative 액션 (lifecycle 변경, 멤버 편집, 정책 변경)
- API Token 발급 (본인도 발급받을 수 없음, Super Admin 필요)
- Custom Profile Editor 변경
- OAuth 앱 생성

### 4.4 ota MVP 권장 [관례]
- 토큰 발급자는 **Read-Only Administrator**로 설정할 것을 설정 가이드에 명시
- PRD에 "MVP는 read-only. 쓰기 액션 시 403 처리" 명시
- 향후 쓰기 MVP에서는 Group Admin 등 단계별 권한 안내

---

## 5. Group Rule — Okta Expression Language 개요 [확정]

### 5.1 언어 특성
- Apache Commons EL 기반 변형
- `urn:okta:expression:1.0` type 식별자
- **튜링 비완전** — 조건식만 가능 (while/for 없음)
- 반환값은 boolean

### 5.2 주요 변수
- `user.{field}` — profile 필드 접근 (예: `user.department`, `user.email`)
- `user.profile.{customField}` — 일부 컨텍스트
- `user.mfaFactors` — 등록된 factor
- `user.statusChanged` — 날짜

### 5.3 주요 함수
```
String.stringContains(user.email, "@acme.com")
String.startsWith(user.firstName, "A")
String.substring(user.login, 0, 3)
String.toUpperCase(user.department)

isMemberOfGroup("00g...")
isMemberOfAnyGroup("00g1...", "00g2...")
isMemberOfGroupName("Engineering")
isMemberOfGroupNameContains("Eng")
isMemberOfGroupNameStartsWith("Eng")

Arrays.contains(user.roles, "admin")

Time.now()                       # 현재 시각 (UTC)
```

### 5.4 예시
```
user.department == "Engineering"
user.department == "Engineering" && String.stringContains(user.email, "@acme.com")
isMemberOfGroupNameStartsWith("Eng-") && user.title != "Intern"
```

### 5.5 Expression Validation — 공식 REST endpoint 없음 [확정 + 확인필요]

**결론:** Okta **Management API에는 Group Rule Expression만 별도로 검증하는 독립 엔드포인트(`/validate`, `/dry-run`, `/test`)가 공식 문서상 존재하지 않는다.** 검색·공식 OpenAPI 레퍼런스(`/docs/api/openapi/okta-management/management/tag/GroupRule/`)·Okta Support KB에서 모두 해당 엔드포인트를 찾지 못했다. [확정 — 네거티브 확정. 2026-04 재검증]

**실제 검증 흐름:**
1. 클라이언트가 `POST /api/v1/groups/rules`로 규칙을 생성 (기본 `status=INACTIVE`).
2. Okta가 서버 측에서 expression을 파싱·평가 가능성 검사.
3. 결과:
   - **파싱 불가 / 문법 오류 / 지원되지 않는 함수 사용 (`Convert`, `Time` 등)** → HTTP 400, `errorCode=E0000001` + `errorCauses`에 파서 메시지. [관례 + Okta Support KB 확정]
   - **파싱은 되지만 런타임 참조가 불안정 (예: 커스텀 user type 전용 속성을 default user에 적용)** → 규칙은 저장되지만 `status=INVALID`로 남고 활성화(`/lifecycle/activate`) 시 실패. [관례 + Okta Support KB "the rule cannot be saved or previewed" 문구 기반]
4. Admin Console UI는 rule 작성 시 **"Preview"** 기능을 제공(현재 매치 대상 수 미리보기) — 이는 별도 공개 API가 아니라 콘솔 전용. [관례, Okta Support KB]

**결과적으로 ota가 취할 전략:**
- **MVP(읽기 전용):** `status=INVALID` 상태의 규칙을 리스트/상세에서 명확히 표시 (경고 아이콘 + 원인 설명 불가능함을 안내).
- **향후 쓰기 지원 시:**
  - 사용자 입력 표현식 검증은 **서버 응답에 전적으로 의존**. 400 응답의 `errorCauses`를 파싱해 UI에 필드별 에러 표시.
  - dry-run 대안: (1) 임시로 INACTIVE 상태로 `POST` → 성공 시 바로 `DELETE`. 단 System Log에 `group.rule.create`/`delete` 이벤트가 남으므로 감사 잡음 발생 → UI에 "이 검증은 로그에 남습니다" 안내 필요.
  - 또는 **클라이언트 측 사전 파서**: OEL 문법의 주요 패턴(비교 연산자, 지원 함수 목록, 중괄호 매칭)을 로컬에서 미리 체크해 빈번한 오류는 네트워크 전에 차단. 단, Okta 측 문법 변경에 노출됨 → 보완적 용도.
- **expression 변경 워크플로:** 공식 권장 플로우는 **deactivate → edit → reactivate**. PUT으로 바로 수정해도 동작하지만 deactivate 경유가 안전. [Okta Support KB 확정]

**PRD에 요구사항으로 반영할 점:**
- REQ: "Group Rule Expression 입력을 받는 UI가 있다면, 서버 400 응답의 `errorCauses`를 필드별 에러로 표시해야 한다."
- REQ(MVP, 읽기): "Group Rule 리스트는 `status=INVALID` 규칙을 경고색으로 표시하고, 상세 화면에 'Okta가 이 표현식을 평가할 수 없어 비활성 상태'임을 안내한다."
- REQ(후속, 쓰기): "dry-run 검증은 공식 API 부재로 'create+delete' 임시 규칙 방식을 쓸 수 있으나, System Log 잡음을 수반하므로 사용자 확인 단계를 둔다." 또는 "클라이언트 측 기초 문법 검증만 제공하고 최종 검증은 서버에 위임한다."
- Open Issue: "Okta가 향후 공식 validate 엔드포인트를 추가할 가능성 지속 모니터링."

### 5.6 ota UX 함의 [관례]
- 표시는 monospace + 줄바꿈 (긴 표현식).
- MVP는 읽기 전용이므로 syntax highlight는 선택. 단 `INVALID` 상태 배지는 필수.

### 5.7 레퍼런스
- Okta Expression Language: https://developer.okta.com/docs/reference/okta-expression-language/
- OEL in Identity Engine: https://developer.okta.com/docs/reference/okta-expression-language-in-identity-engine/
- Group Rule Restrictions (Okta Support KB): https://support.okta.com/help/s/article/Group-Rule-Restrictions
- Group Rules OpenAPI: https://developer.okta.com/docs/api/openapi/okta-management/management/tag/GroupRule/

---

## 6. 테스트 Tenant 준비 가이드

### 6.1 테스트 Tenant 획득 [확정]
- Okta Developer Program: https://developer.okta.com/signup/
- 무료 developer account (Preview/Sandbox tenant)
- URL 형식: `https://dev-NNNNNN.okta.com`
- 기능 제한 있음 (사용자 수, 일부 엔터프라이즈 feature 없음). 대부분의 CRUD API는 동일.

### 6.2 즉시 존재하는 기본 리소스 [확정]
| 리소스 | 기본 |
|-------|-----|
| Groups | `Everyone` (전원 자동 포함, 편집 불가) |
| Policies | Default Global Session Policy (`OKTA_SIGN_ON`, system=true) |
| Policies | Default Authentication Policy (앱별) |
| Policies | Default Password Policy |
| Policies | Default MFA Enrollment Policy |
| Authenticators | Password (기본 활성) |
| Apps | `okta_browser_plugin`, `okta_enduser`, `okta_admin_console` 등 내장 앱 |
| User | 생성자 Super Admin 1명 |

### 6.3 ota 개발 시 시드 데이터 권장 [관례]
최소:
- 사용자 5~10명 (다양한 상태: ACTIVE, SUSPENDED, DEPROVISIONED 각 1명씩)
- OKTA_GROUP 2~3개 (예: Engineering, Sales, Interns)
- Group Rule 1~2개 (expression 예시용)
- Policy Rule 추가 (기본 정책에 규칙 몇 개)
- 로그 이벤트: 테스트 로그인/로그아웃 몇 번

### 6.4 API Token 발급 [확정]
- Admin Console → Security → API → Tokens → Create Token
- 이름 예: `ota-dev-readonly`
- 발급자 권한이 토큰에 승계됨 → **Read-Only Admin 계정으로 발급 권장**
- 토큰 1회 표시, 재조회 불가. Secret manager나 1Password 저장.
- 만료 없음. 회전 주기 권장: 90~180일 [관례]

### 6.5 로컬 테스트 환경변수 [관례]
```
export OKTA_ORG_URL=https://dev-123456.okta.com
export OKTA_API_TOKEN=<토큰 placeholder>
```

> ota는 위 두 변수를 읽고 설정 파일의 profile과 병합. 우선순위는 일반적으로 CLI flag > env > config > default.

---

## 7. MFA Factors — MVP 포함 여부 권고

### 7.1 엔드포인트 [확정]
- `GET /api/v1/users/{id}/factors` — 해당 사용자 등록된 factor 목록
- `GET /api/v1/users/{id}/factors/catalog` — 등록 가능 factor
- `POST /api/v1/users/{id}/factors/{factorId}/lifecycle/activate|reset|suspend`
- `DELETE /api/v1/users/{id}/factors/{factorId}` — factor 제거

### 7.2 Factor 타입 [확정]
- `okta_verify` (push/totp)
- `okta_email`
- `okta_password`
- `phone` (SMS/Voice)
- `webauthn` (FIDO2)
- `security_key`
- `security_question`
- `google_otp`
- `duo` (통합)

### 7.3 MVP 포함 권고 [관례]
- **MVP 포함 권장**: User 상세 페이지에서 "등록된 factor 리스트" 표시
  - 이유: Help Desk/IAM 엔지니어의 가장 빈번한 요청. k9s의 pod→container 관계와 유사한 필수 컨텍스트.
  - 표시 필드: factor type, status, provider, 등록 시각
- **MVP 제외 권고**: factor reset/suspend/delete 같은 mutative 액션
  - 이유: 고위험 (사용자가 로그인 못 하게 됨). 향후 쓰기 버전에서 강한 확인 + 감사 로그 배너와 함께.
- **별도 뷰 제외**: "조직 전체 factor 등록 현황" 같은 분석은 MVP 범위 밖

**권장 REQ:** User 상세에 "Factors" 탭/섹션 추가 (읽기만).

---

## 8. Go SDK vs 직접 HTTP

### 8.1 권장: Okta Go SDK v5 [관례]
- `github.com/okta/okta-sdk-golang/v5/okta`
- 장점:
  - OpenAPI 기반. 전 엔드포인트 커버.
  - 페이지네이션 헬퍼(`HasNextPage`/`Next`)
  - 인증(API Token, OAuth2) 내장
  - rate limit 자동 재시도 옵션
  - User-Agent, Accept 헤더 자동
- 단점:
  - 생성 코드 스타일 (builder pattern): `client.UserAPI.ListUsers(ctx).Limit(200).Execute()`
  - 최신 기능 반영 지연 가능성
  - 거대한 의존성 트리

### 8.2 직접 HTTP [관례]
- `net/http` + `encoding/json`
- 장점: 가벼움, 최신 API 즉시 대응
- 단점: 페이지네이션/에러/인증 직접 구현, 유지보수 부담

### 8.3 ota 권장 아키텍처 [관례]
```
┌──────────────────────────────────────────────┐
│ Bubbletea UI (cmd/, internal/tui)            │
└────────────────┬─────────────────────────────┘
                 │ domain.UserRepository 등 인터페이스
┌────────────────▼─────────────────────────────┐
│ Adapter (internal/okta/*_adapter.go)         │
│  - SDK 호출 + 도메인 매핑                      │
│  - 페이지네이션/rate limit/에러 → 도메인 에러    │
└────────────────┬─────────────────────────────┘
                 │ okta-sdk-golang/v5
┌────────────────▼─────────────────────────────┐
│ Okta REST API v1                              │
└───────────────────────────────────────────────┘
```

- **원칙:** TUI 레이어는 `domain.User` 같은 순수 타입만. SDK 타입(`oktaSDK.User`)이 TUI로 누출 금지.
- **인터페이스 테스트:** Adapter를 `domain.UserRepository` 인터페이스로 정의 → 테스트 시 mock 간편.
- **SDK 통합 테스트:** `httptest.Server`로 Okta 응답 흉내 → 실제 tenant 없이도 CI에서 검증.

### 8.4 제안: SDK + 얇은 Wrapper [관례]
```go
type oktaClient struct {
    sdk   *oktaSDK.APIClient
    cache *cache.TTL
    rlMon *RateLimitMonitor
}

func (c *oktaClient) ListUsers(ctx context.Context, q UserQuery) ([]domain.User, PageInfo, error) {
    // SDK 호출 + Link 헤더 파싱 + 에러 매핑
}
```

---

## 9. 리스크/운영 함정

### 9.1 Token Rotation [관례]
- SSWS 토큰은 만료 없음 → 회전 누락 쉬움.
- 조직 정책: 90~180일 회전. ota는 회전을 강제하지 않지만 **"토큰 사용 기간 N일"** UI 힌트 제공 가능 (`system.api_token.create` 이벤트로 추정).

### 9.2 관리자 lockout 방지 [확정]
- Policy의 "Deny all" 규칙은 관리자 본인까지 차단 가능. ota가 정책 편집을 지원할 때 dry-run 필수.
- MVP는 읽기만이므로 직접 리스크 낮음.

### 9.3 대용량 조직에서 성능 [관례]
- Everyone 그룹 멤버 조회 = 전사 인원 (수만~수십만). 페이지네이션 + 스트림 UI 필수.
- 사용자 리스트 전체 로드 금지. 검색/필터 유도.

### 9.4 Custom Profile 필드 [확정]
- `profile.*`는 조직마다 스키마 다름. 고정 컬럼 + 동적 컬럼 분리.
- Schema 조회: `GET /api/v1/meta/schemas/user/default`

### 9.5 OAuth Scope 불일치 [확정]
- OAuth Service App 사용 시 scope 부족하면 403. ota 초기 버전에서는 SSWS만 지원하는 게 단순.
- 필요한 스코프 예시 (향후): `okta.users.read`, `okta.groups.read`, `okta.policies.read`, `okta.logs.read`, `okta.apps.read`.

### 9.6 Log 지연 [관례]
- System Log는 이벤트 발생 후 수 초~수십 초 지연 가능. 실시간처럼 보여도 polling 간격은 이 지연을 고려.

### 9.7 Eventual Consistency [확정]
- Users `search`는 인덱싱 지연 있음 (수 초~수 분). 방금 생성한 사용자는 `/users/{id}` 직접 조회는 즉시 가능하나 `search`에서 누락될 수 있음.

### 9.8 Preview vs Production 차이 [확정]
- 일부 feature flag가 다름. ota 개발은 Preview에서 하고 Production 배포 전 동작 재확인.

### 9.9 Timezone Confusion [관례]
- Okta 응답은 UTC. 관리자는 보통 로컬 타임. ota는 설정으로 토글 제공.

---

## 10. 레퍼런스 URL 패턴

- API Reference: https://developer.okta.com/docs/reference/core-okta-api/
- Users: https://developer.okta.com/docs/reference/api/users/
- Groups: https://developer.okta.com/docs/reference/api/groups/
- Group Rules: https://developer.okta.com/docs/reference/api/groups/#group-rule-operations
- Apps: https://developer.okta.com/docs/reference/api/apps/
- Policies: https://developer.okta.com/docs/reference/api/policy/
- System Log: https://developer.okta.com/docs/reference/api/system-log/
- Event Types: https://developer.okta.com/docs/reference/api/event-types/
- Rate Limits: https://developer.okta.com/docs/reference/rate-limits/
- Okta Expression Language: https://developer.okta.com/docs/reference/okta-expression-language/
- Go SDK: https://github.com/okta/okta-sdk-golang
- Terraform Provider: https://registry.terraform.io/providers/okta/okta/latest/docs

---

## 11. PRD 반영 체크리스트 (PM용)

- [ ] MVP 리소스: Users/Groups/Group Rules/Policies/Logs 각각의 리스트/상세 컬럼이 본 문서 §1 필드 매핑과 일치하는가
- [ ] Users 상세에 MFA Factors 섹션 포함 여부 결정 (§7 권장: 포함)
- [ ] Policy 타입별 탐색 UX (네임스페이스처럼 타입 선택) 반영
- [ ] Group Rule 비활성화/삭제 경고 문구 요구사항
- [ ] Rate Limit UI 힌트 요구사항 (Remaining 경고, 429 waiting 표시)
- [ ] Log tail 모드 + 프리셋 필터 요구사항
- [ ] SSWS 토큰 / OAuth2 Service App 지원 범위 (MVP는 SSWS 권장)
- [ ] 권한 모델: Read-Only Admin 권장 명시, 쓰기 시 403 핸들링 (MVP는 읽기)
- [ ] 테스트 tenant 설정 가이드 (§6) docs에 포함
- [ ] SCIM search/filter 문법 설명 — 사용자용 짧은 치트시트 포함 여부
- [ ] 에러 매핑 테이블 (§2.3) — ota 에러 메시지 표준에 반영
- [ ] Out of Scope에 명시: mutative 액션, custom policy 편집, MFA reset, OAuth 앱 관리, SAML 설정, Directory 통합 설정 등

---

## 12. 알려진 불확실성/확인 필요 항목

1. Policy Rule의 id prefix 일관성 [확인필요]
2. 로그 보관 기간 (플랜별 90/180일) [확인필요]
3. `/policies` 엔드포인트 rate limit 정확 수치 [확인필요]
4. ~~Okta Expression Language의 "Validate" 엔드포인트 현재 경로 [확인필요]~~ → **[해결] 공식 독립 validate endpoint 없음. §5.5 참고. 2026-04-24 재확인.**
5. `ENTITY_RISK` 정책 타입 정식 출시 여부 [확인필요]
6. 최신 OIE Policy 타입 추가 (예: `CONTINUOUS_ACCESS`) [확인필요]
7. Event Hook을 통한 유사 실시간 스트림 — ota에서 활용 가치 [확인필요, 복잡도 높음]

---

**작성 완료.** PRD 작성 시 본 문서를 참조할 것. 질문/추가 필요는 SendMessage로 okta-expert에게.
