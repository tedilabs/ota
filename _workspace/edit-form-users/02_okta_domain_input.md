# Okta Domain Input — Users Edit Form

> **Audience**: `product-manager` for PRD addendum drafting (Phase 2).
> **Scope**: Okta domain facts/constraints that must shape the Users edit form behavior.
> **Confidence legend**: **[확실]** = Okta 공식 문서 직접 인용 또는 SDK 동작 검증.
> **[관례]** = Okta 운영 현장 일반 관례. **[추정]** = 문서 미확인, 합리적 추정. 모두 PRD 반영 전 재확인 필요는 [추정]에 한함.

---

## 0. TL;DR for PM

1. **엔드포인트**: `POST /api/v1/users/{userId}` (partial-merge). PUT은 strict update이므로 ota MVP에서는 사용 금지.
2. **편집 가능 필드 MVP 권장**: `firstName`, `lastName`, `displayName`, `nickName`, `title`, `department`, `division`, `employeeNumber`, `mobilePhone`, `secondEmail`, `email` — 총 11개.
3. **`login`은 MVP에서 read-only**로 잠그는 것을 강력 권장. 별도 명령(`:change-login`) 또는 v0.2에서 재논의.
4. **권한**: Super Admin / Org Admin은 풀 가능. User Admin은 가능. Help Desk Admin / Read-Only Admin은 불가 → 저장 시 403 → 토스트.
5. **Optimistic concurrency 미지원**: ETag/If-Match 없음. 마지막 쓰기가 이김. 폼 진입 시 latest GET 권장.
6. **PII 마스킹 유지**: `mobilePhone`, `secondEmail`은 form 표시 시 기본 마스킹 + `Tab`/`m` 토글.
7. **사이드 이펙트**: profile.email 변경 시 알림 메일 발송 가능(조직 설정 의존). login 변경 시 세션 무효화 + 토큰 폐기 + SSO 매핑 영향.

---

## 1. Okta API 엔드포인트

### 1.1 Partial vs Strict Update

**[확실]** Okta는 동일 path `/api/v1/users/{userId}`에 대해 두 메서드를 다르게 해석한다:

| Method | 시맨틱 | 누락 필드 동작 | ota 사용 권장 |
|--------|--------|---------------|--------------|
| `POST /api/v1/users/{userId}` | **Partial (merge)** | 요청 본문에 없는 필드는 그대로 유지 | **사용** |
| `PUT /api/v1/users/{userId}` | **Strict (replace)** | 요청 본문에 없는 필드는 `null`로 클리어 | **금지** |

> 출처: Okta Users API "Update User", Okta Workflows "Update User" connector docs (partial vs strict).
> 검색: `site:developer.okta.com update user partial strict`

### 1.2 Merge 시맨틱의 미묘한 차이 **[확실]**

`POST`(partial)는 다음과 같이 동작:

| 요청 본문 표현 | 결과 |
|--------------|------|
| 필드 **omit** (key 자체 없음) | 기존 값 유지 |
| 필드 = `null` | **필드 삭제** (값이 비워지고 schema에 따라 제거됨) |
| 필드 = `""` (빈 문자열) | 빈 문자열로 저장 (필드 schema가 nullable 여부에 따라 다름) |

**ota TUI 구현 권장:**

- 변경되지 않은 필드는 요청 body에서 **omit**. 화면에 표시되었다고 무조건 보내지 말 것.
- 사용자가 명시적으로 "지움" 액션을 취한 경우만 `null` 전송.
- 빈 문자열 전송은 회피 (서버 검증 변동성 큼).

> 근거: Okta Developer Community Q&A "Set profile attributes to NULL with partial update" — null = 삭제, 명시적 클리어 패턴 확립.
> 검색: `site:devforum.okta.com partial update null clear`

### 1.3 PUT을 쓰면 안 되는 이유 **[확실]**

PUT(strict)을 잘못 쓰면 ota 사용자가 폼에 띄운 필드만 살아남고 화면에 없는 custom profile 필드(Extras)가 전부 `null` 처리된다. 이는 조직의 다른 통합(SCIM, AD push 등)을 깨뜨릴 수 있는 **데이터 손실 사고**.

→ ota 어댑터는 PUT 경로를 노출조차 하지 말 것 (코드 레벨에서 차단 권장).

### 1.4 Credentials는 별도 키 경로

```json
POST /api/v1/users/{userId}
{
  "profile": { "firstName": "...", ... },
  "credentials": {
    "password": { "value": "..." },          // 보통 별도 lifecycle API 사용
    "recovery_question": { "question": "...", "answer": "..." }
  }
}
```

**[관례]** `credentials.*` 변경은 별도 영역으로 분리. profile edit form에서는 `profile.*`만 다루는 것이 깔끔하다. recovery question은 MVP 제외 권장.

### 1.5 Rate-limit 카테고리 **[확실]**

- `POST /api/v1/users/{userId}`는 **management API rate-limit 버킷**(주로 `/api/v1/users` 그룹)에 카운트.
- 폼 저장은 단발성 (1 요청 / 1 save) → list/log 화면의 polling이 잡아먹지 않는 한 거의 영향 없음.
- 단, 동일 사용자에 대한 빠른 연속 저장(예: 사용자가 save 후 즉시 다시 save)은 `40 req/user/10s/endpoint` per-admin limit에 걸릴 수 있음 → 저장 직후 1초 disable 또는 명시적 가드 권장.

> 출처: developer.okta.com/docs/reference/rl-additional-limits/. 정확한 버킷 ID는 테넌트 플랜별로 다름.

### 1.6 SDK 매핑

```go
// okta-sdk-golang/v5
client.UserAPI.UpdateUser(ctx, userID).
    User(okta.UpdateUserRequest{
        Profile: &okta.UserProfile{
            FirstName: okta.PtrString("Jane"),
            // ...
        },
    }).
    Execute()
```

> 주의: SDK는 PUT을 노출하는 메서드명을 `ReplaceUser`로 분리하거나 `Strict(true)` 옵션으로 제공. v5 정확한 시그니처는 sdk-go.md 참고 후 확정.

### 1.7 Strict 모드 옵션? **[확실]**

`POST`에 `?strict=true` 같은 글로벌 옵션은 **없다**. 일부 인접 엔드포인트(예: lifecycle change-password)에는 `strict` 쿼리가 있지만 profile update에는 적용되지 않음.

---

## 2. 권한 모델 (Admin Role Matrix)

### 2.1 Standard Role × Action **[확실]**

| Role | Profile 수정 (firstName 등) | Login 변경 | 비고 |
|------|---------------------------|----------|------|
| Super Admin | YES | YES | 풀 권한 |
| Org Admin | YES | YES | 사실상 동등 |
| **User Admin** | **YES** | **YES** | ota 권장 최소 권한 |
| Group Admin | YES* | YES* | * 자신이 관리하는 그룹의 멤버에 한정 |
| **Help Desk Admin** | **NO** (표준 권한 기준) | NO | 비밀번호 리셋/잠금 해제만 |
| Read-Only Admin | NO | NO | 조회만 |
| Mobile Admin | NO | NO | |
| API Access Mgmt Admin | NO | NO | |

**Custom Role**의 경우 다음 permission을 부여하면 Help Desk 등급도 profile 편집 가능:
- `okta.users.userprofile.manage` — profile 속성 변경(hidden/sensitive 포함)
- `okta.users.manage` — 더 광범위, create + update 포함

> 출처: help.okta.com "Standard administrator roles and permissions",
>       developer.okta.com/docs/api/openapi/okta-management/guides/permissions

### 2.2 ota에서의 처리 권장 **[관례]**

- ota는 토큰 발급자의 권한을 **사전 검증하지 않는다**. 그 이유:
  - 권한 매트릭스가 custom role로 매우 다양화됨 (사전 매핑 어려움)
  - 토큰 자기 분석 API는 있지만 (`GET /api/v1/users/me`) 모든 permission이 반환되지 않음
- 대신: **저장 시도 → 403 응답 → 친화적 메시지** 패턴을 채택.
- 403 메시지 예: `"이 동작에는 'Manage user profiles' 권한이 필요합니다. 토큰 발급 관리자를 확인하세요."`
- 폼은 닫지 말고 유지 → 사용자가 토큰 교체 후 재시도 가능하게.

---

## 3. 편집 가능 필드 명세 (Standard User Profile)

### 3.1 MVP 권장 필드 (11개)

| TUI Label | API field | Type | Validation | PII | 변경 시 사이드 이펙트 |
|----------|-----------|------|-----------|-----|---------------------|
| First Name | `profile.firstName` | string | 1-50자 권장 (조직 schema 의존) | - | SAML/OIDC claim 갱신 (재로그인 시 반영) |
| Last Name | `profile.lastName` | string | 1-50자 | - | 위와 동일 |
| Display Name | `profile.displayName` | string, optional | 100자 이내 | - | UI 표시명 변경 |
| Nickname | `profile.nickName` | string, optional | 100자 이내 | - | claim 영향 |
| Email | `profile.email` | string (RFC 5322) | 이메일 형식 | △ (전사 노출) | **조직 설정에 따라 알림 메일 발송 가능 / 일부 정책에서 재인증 트리거 / login이 email인 경우 §4 참조** |
| Title | `profile.title` | string, optional | 100자 이내 | - | claim 영향 |
| Division | `profile.division` | string, optional | 100자 이내 | - | claim 영향, Group Rule 재평가 가능 |
| Department | `profile.department` | string, optional | 100자 이내 | - | **Group Rule 재평가 → 그룹 멤버십 자동 변경 가능** |
| Employee Number | `profile.employeeNumber` | string, optional | 50자 이내 | △ | HR 통합 영향 |
| **Mobile Phone** | `profile.mobilePhone` | string (E.164 권장) | 전화번호 형식 (느슨) | **YES** | **MFA factor enroll에 영향 가능 (SMS factor) — 변경 후 재인증 필요할 수 있음** |
| **Secondary Email** | `profile.secondEmail` | string, optional | 이메일 형식 | **YES** | 복구 채널 변경 |

### 3.2 검증 규칙 상세

- **이메일 형식** [확실]: Okta 서버 검증 정규식이 RFC 5322보다 엄격할 수 있음. 클라이언트 측은 느슨하게(`*@*.*`) 두고 서버 검증 결과를 반영하는 것이 안전.
- **전화번호** [추정]: Okta는 형식을 엄격히 강제하지 않지만 E.164 (`+821012345678`)를 강력 권장. SMS factor 작동에 영향.
- **길이 제한** [추정]: 100자/50자는 일반적인 default schema 값. Profile Editor에서 조직별로 다를 수 있음 → **클라이언트 검증은 최소화하고 서버 검증 의존** 권장.
- **필수 vs 선택** [확실]: `firstName`, `lastName`, `login`, `email`은 default schema에서 **required**. 그 외는 optional.
  - 단, 조직이 schema를 커스터마이즈하면 다를 수 있음 — ota는 default schema 기준으로 가드 후 서버 응답으로 보정.

### 3.3 Custom Profile (Extras) MVP 제외 근거 **[관례]**

조직별로 schema가 모두 다름:
- `customField_xxx`, `urn:scim:schemas:extension:enterprise:2.0:User:*` 등
- 데이터 타입이 string, integer, boolean, enum, array 등 다양 → form 위젯 일반화 어려움
- enum의 가능 값은 별도 API(`GET /api/v1/meta/schemas/user/default`)로 조회 필요
- v0.2 이상에서 schema-driven form으로 확장 권장

→ MVP는 default standard 필드에 집중. ota는 custom 필드를 form에 띄우지 않고 detail에서 read-only로 노출.

---

## 4. `login` (Username) 변경의 위험성

### 4.1 사실 정리 **[확실]**

Okta `profile.login`은:
- 사용자가 로그인 시 입력하는 **username** 그 자체
- 대부분의 SAML/OIDC 통합에서 `nameID` 또는 `sub` claim의 소스
- 일부 조직은 `login == email`, 일부는 분리

### 4.2 변경 시 발생하는 일 **[확실]**

| 영역 | 동작 |
|------|------|
| 사용자 세션 | **모두 무효화** — 다음 동작 시 재인증 강제 |
| OAuth/OIDC 토큰 | refresh + access token 모두 **폐기** |
| SAML assertion | 다음 로그인부터 새 login 기반 nameID 발급 — **다운스트림 앱이 nameID 변경을 어떻게 받아들이는지 조직마다 다름** |
| 사용자 알림 | 조직 설정에 따라 변경 안내 메일 발송 가능 |
| 감사 로그 | `user.account.update_profile` + `user.identity.update` 이벤트 |

> 출처: developer.okta.com/docs/release-notes/2025-okta-identity-engine/ (session/token revocation 메커니즘 확인).
> SAML nameID 변경의 다운스트림 영향은 조직/앱에 의존하므로 **운영자 사전 계획 필요**.

### 4.3 권고: MVP에서는 read-only로 잠금 **[강한 권장]**

이유:
1. 가장 위험한 변경 — 단일 실수가 전 직원 SSO 차단 가능
2. ota는 lifecycle 명령 + profile edit이 첫 mutation 표면. 신뢰 구축 단계에서 비가역적 위험 노출 회피.
3. 화면 비좁음 — 적절한 경고/확인 다이얼로그 + 영향 범위 미리보기를 form 안에 우겨넣기 어려움.
4. login 변경이 필요한 케이스는 빈도가 매우 낮음 — 별도 전용 명령(`:change-login` palette)으로 충분.

**구체 권고:**
- form에 login 필드는 **표시하되 read-only**.
- "Login 변경은 `:change-login <id>` 명령 사용" 안내 inline hint.
- v0.2에서 dedicated 워크플로(영향 범위 preflight + 2단계 확인) 설계 후 enable.

### 4.4 만약 PM이 MVP에서 enable하기로 결정한다면 **[관례]**

다음 가드를 PRD에 명시:
- 별도 확인 다이얼로그 ("`y/N` 으로 확인. 이 동작은 모든 활성 세션을 종료하고 토큰을 폐기합니다.")
- 변경 전후 값 diff 명시 표시
- 저장 직후 user detail로 리다이렉트 + status banner ("Login 변경됨. 이 사용자의 SSO 통합 작동을 모니터링하세요.")
- TUI 자체 토큰이 해당 사용자의 토큰이면 — 매우 드물지만 — ota 세션도 무효화될 수 있음 (운영자 계정 ≠ 편집 대상 일반 사용자이므로 통상 무관).

---

## 5. 동시 편집 / Optimistic Concurrency

### 5.1 사실 **[확실]**

- Okta `/users` 엔드포인트는 **ETag / If-Match 미지원**.
- 따라서 **last-write-wins**. 두 admin이 동시에 같은 사용자를 편집하고 둘 다 저장하면 나중 요청이 이김.
- 손실된 변경에 대한 알림 없음. 감사 로그에서만 추적 가능.

> 근거: okta/okta-sdk-golang Issue #302 ("Partial update user race condition losing updates") — Okta 측 응답이 ETag 미지원 + 동시 부분 업데이트 race condition 인정.

### 5.2 ota 처리 권장 **[관례]**

**Defensive 패턴 (MVP 수준):**

1. **폼 진입 시 latest GET**: list 캐시가 아닌 `GET /api/v1/users/{id}`로 가장 최신 상태 로드.
2. **저장 후 응답을 재사용**: `POST` 응답 본문은 업데이트 후 전체 user. 이를 detail/list에 reflect → 다른 변경자의 동시 수정도 부분적으로 인지 가능.
3. **diff 표시**: 진입 시 스냅샷 vs 저장 시점 변경된 필드만 표시 ("변경됨" 마커) → 자기 변경분 추적.

**Advanced (v0.2 이상):**
- 저장 직전 한 번 더 GET → 진입 시 스냅샷과 diff → 다른 admin이 같은 필드 변경했으면 "Conflict — 새 값 확인하시겠어요?" 모달.
- 이는 race 자체를 막지는 못함 (GET과 POST 사이도 race 존재) 단 사용자 인지도 향상.

### 5.3 409 응답? **[확실]**

ETag가 없으므로 **profile update에서 409 Conflict는 발생하지 않는다.** 409는 주로 다른 시나리오 (e.g., duplicate user create)에 사용됨. ota는 409를 profile update의 응답 시나리오에 포함시키지 말 것.

---

## 6. 에러 시나리오 매핑

| HTTP | Okta errorCode | 의미 | TUI 처리 권장 |
|------|---------------|------|--------------|
| 400 | `E0000001` | Validation 실패 — `errorCauses`에 필드별 사유 | 폼 유지. `errorCauses`에서 필드명 추출 → 해당 input 위에 inline error. 추출 실패 시 form footer에 summary. |
| 400 | `E0000038` | Schema 위반 (e.g., custom field 누락) | 폼 유지 + footer error. MVP는 standard 필드만 다루므로 발생 가능성 낮음. |
| 401 | `E0000011` | API 토큰 무효/만료 | 폼 닫지 말고 토스트 "토큰 무효. 재인증 필요" + 변경 사항 보존 (드래프트). |
| 403 | `E0000006` | 권한 부족 | 폼 유지 + 토스트 "권한 부족 — Manage user profiles 권한 필요". 변경값 보존. |
| 404 | `E0000007` | 사용자가 deleted/disappeared | 폼 닫고 list로. 토스트 "사용자가 더 이상 존재하지 않습니다. 목록을 새로고침합니다." → list refetch. |
| 409 | (해당 없음, §5.3 참조) | — | — |
| 429 | `E0000047` | Rate limit | 폼 유지. `Retry-After` 초 카운트다운 표시. "X초 후 자동 재시도" or 사용자가 수동 재시도. |
| 5xx | 다양 | Okta 서비스 이슈 | 폼 유지. "Okta 측 일시 오류. 잠시 후 다시 시도해주세요." + 변경값 보존. |

### 6.1 `errorCauses` 파싱 패턴 **[확실]**

```json
{
  "errorCode": "E0000001",
  "errorSummary": "Api validation failed: profile",
  "errorCauses": [
    { "errorSummary": "login: An object with this field already exists in the current organization" },
    { "errorSummary": "email: Email is not valid" }
  ]
}
```

권장: prefix 매칭 (`<fieldName>: <msg>`)으로 필드 추출. prefix 미매칭 시 footer에 표시. 다국어화는 MVP 제외.

### 6.2 폼 유지 정책 **[관례]**

- **변경 사항 보존 우선**: 4xx (except 404) / 5xx / 429 → 폼 닫지 말고 사용자 입력 유지. drafted state.
- **명시적 닫기 사유**: 200 OK (저장 성공) / 404 (대상 소멸) / 사용자가 ESC.

---

## 7. profile.* 외 변경 가능 영역

### 7.1 credentials.recovery_question **[확실]**

`POST /api/v1/users/{id}`의 본문에 `credentials.recovery_question`을 함께 보낼 수 있음. 하지만:

- security 영역 (답이 평문 검증) → form에서 다루기 부적절
- 사용자 본인이 self-service로 설정하는 게 일반적
- **MVP 제외 권장**

### 7.2 status 변경 **[확실]**

이미 ota에 lifecycle 명령들이 있음 (`:activate`, `:deactivate`, `:expire-password`, `:reset-mfa`, `:reset-password`, `:unlock`, `:delete`).

→ **편집 폼에 status를 절대 포함시키지 말 것.** 이유:
- status는 단일 API 호출이 아닌 lifecycle action endpoint 호출이 필요
- 각 status 전이는 확인/되돌릴 수 없음 경고가 필요 — form 안에 우겨넣기 부적절
- 기존 명령 UX와 충돌

권장 처리: form 헤더에 현재 status 배지를 **read-only**로 표시 + 변경 방법 inline hint ("status 변경: `:deactivate` 등").

### 7.3 type 필드 **[확실]**

`profile.userType` (또는 `type.id`)은 user type 변경. 매우 드물고 위험 — 다른 schema로 마이그레이션 효과. **MVP 제외**.

### 7.4 그룹 멤버십 **[관례]**

폼에서 그룹 추가/제거를 동시에 제공? **No**.
- 그룹은 별도 큰 워크플로 (동적 그룹 vs 정적 그룹 구분, 멤버십 영향)
- form 화면 비좁음
- 별도 detail tab(이미 user → groups 뷰 있음 추정) 또는 별도 명령으로 분리

---

## 8. 검증/저장의 표준 패턴 권장

### 8.1 클라이언트 vs 서버 검증의 책임 분리 **[관례]**

| 검증 종류 | 위치 | 예 |
|---------|------|---|
| 형식 (이메일·전화번호 정규식) | **클라이언트** (느슨) + 서버 (엄격) | 빈 값 차단, `@` 포함 여부 |
| 길이 (max chars) | 서버 의존 | UI는 너무 긴 입력만 truncate |
| 중복 (login/email unique) | **서버만** | 클라이언트가 미리 lookup하지 말 것 (rate-limit 낭비) |
| 필수 여부 | 클라이언트 (default schema 기준) + 서버 보정 | firstName/lastName 빈 값 차단 |
| 권한 | **서버만** | 사전 검증 불가, §2.2 |
| 충돌/race | 서버 (last-write-wins) | §5 참조 |

### 8.2 추천 저장 흐름

```
[사용자 'e' 키] → enter edit form
      ↓
[GET /users/{id}] (최신 상태 로드, latest snapshot 기록)
      ↓
[사용자 편집 ... ESC = cancel(변경 있으면 확인) / 'Ctrl+S' or 'Enter' = save]
      ↓
[로컬 검증] (필수/형식)
      ↓ (passes)
[변경된 필드만 추출 → POST /users/{id} body 구성]
      ↓
[저장 중 indicator]
      ↓
  200 → form 닫기 + detail/list refresh + 토스트 "Updated"
  400 → §6 처리, 폼 유지
  403 → §6 처리, 폼 유지
  404 → 폼 닫고 list 복귀 + refresh
  429 → 카운트다운, 자동 재시도 or 수동
  5xx → 폼 유지 + 재시도 hint
```

### 8.3 변경 사항 dirty 추적

- 진입 시 snapshot (loaded user profile) 저장
- 매 input 변경 시 snapshot vs current diff → footer에 "N changes" 표시
- save 시 diff에 있는 필드만 body에 포함 (§1.2 권장 — omit unchanged)
- ESC + dirty → "변경사항 버리시겠습니까? `y/N`" 모달
- ESC + clean → 즉시 닫기

### 8.4 마스킹 / 언마스킹 통합 **[기존 ota 정책]**

- `mobilePhone`, `secondEmail`은 form 진입 시 기본 마스킹 (`010-****-1234` 형태)
- 편집 진입 시(input focus) 자동 언마스킹 → 편집 가능
- focus out + 변경 없음 → 다시 마스킹
- form 전체 토글 키: `m` (기존 패턴 유지)
- 저장 후 detail로 복귀 시 다시 기본 마스킹 정책 적용

---

## 9. PRD 작성 시 PM이 확정해야 할 결정 사항

| # | 결정 사항 | okta-expert 권고 |
|---|----------|----------------|
| D1 | 편집 필드 목록 (final) | §3.1의 11개 필드 |
| D2 | `login` 편집 가능? | **MVP read-only**, v0.2 dedicated 워크플로 |
| D3 | `email` 변경 시 추가 confirm? | NO — 단순 inline 안내 hint면 충분 (Okta 자체 알림이 발송됨) |
| D4 | 변경 사항 있는 상태에서 ESC | 1단계 확인 모달 (`y/N`) |
| D5 | 저장 키 | `Ctrl+S` 또는 `Enter`(footer "Save" 포커스 시) |
| D6 | 저장 실패 시 폼 동작 | 닫지 말고 유지, 변경값 보존 (§6.2) |
| D7 | latest GET timing | 폼 진입 시 1회 |
| D8 | recovery question, custom fields | **MVP 제외** (§3.3, §7.1) |
| D9 | status 표시 | header read-only 배지, 변경은 기존 lifecycle 명령 |
| D10 | dirty diff 표시 | footer "N changes" + 필드별 "modified" 마커 |

---

## 10. 위험 요약 (PRD 위험 섹션에 반영)

1. **Login 변경 enable 시**: 전사 SSO 단절 가능. MVP에서는 잠금 권장.
2. **동시 편집 race condition**: ETag 미지원, last-write-wins, 손실 알림 없음. 운영 모드에서 두 admin이 동시 편집할 가능성 안내 필요.
3. **PUT 오용**: 코드 레벨에서 PUT 경로 차단. custom field 손실 방지.
4. **Custom schema 의존성**: 조직별 schema 차이로 default 필드라도 required/optional이 다를 수 있음. 서버 검증 결과 graceful 처리 필수.
5. **PII 노출 사고**: form 진입 시 자동 언마스킹 정책에 운영자 share-screen 시 노출 위험. 토글 키 명시.
6. **Group Rule 자동 재평가**: `department`, `division` 변경 시 사용자의 그룹 멤버십이 자동 변동될 수 있음. PRD에서 "변경 영향은 즉시 비동기로 반영될 수 있습니다" 명시 권장.

---

## 11. 참고 URL / 검색 쿼리

| 주제 | URL or 쿼리 |
|-----|------------|
| Users API "Update User" | https://developer.okta.com/docs/api/openapi/okta-management/management/tag/User/ |
| Partial vs Strict update | `site:developer.okta.com update user partial strict` |
| Null clear semantics | https://devforum.okta.com/t/set-profile-attributes-to-null-with-partial-update/9376 |
| Race condition / no ETag | https://github.com/okta/okta-sdk-golang/issues/302 |
| Standard admin roles | https://help.okta.com/en-us/content/topics/security/administrators-admin-comparison.htm |
| Permissions catalog | https://developer.okta.com/docs/api/openapi/okta-management/guides/permissions |
| Session/token revocation on identity update | https://developer.okta.com/docs/release-notes/2025-okta-identity-engine/ |
| Rate limits | https://developer.okta.com/docs/reference/rl-additional-limits/ |
| SDK v5 | https://github.com/okta/okta-sdk-golang/tree/master/okta |

---

## 12. ota 어댑터 추가 권고 (개발자에게 전달)

`internal/domain/ports.go`에 추가 (TDD test interface 우선):

```go
type UsersPort interface {
    // ... 기존 메서드 ...
    UpdateProfile(ctx context.Context, userID string, patch UserProfilePatch) (User, error)
}

// UserProfilePatch는 nil 포인터 = "변경 없음", non-nil = "값으로 설정"
// 명시적 클리어가 필요하면 별도 표현 (예: 값 = nil ptr to empty 또는 ClearFields []string)
type UserProfilePatch struct {
    FirstName       *string
    LastName        *string
    DisplayName     *string
    NickName        *string
    Email           *string
    Title           *string
    Division        *string
    Department      *string
    EmployeeNumber  *string
    MobilePhone     *string
    SecondEmail     *string
    // MVP는 login 제외 (§4.3)
}
```

어댑터 구현 핵심:
- 모든 nil 필드는 JSON body에서 omit (`omitempty`)
- 서버 응답으로 `domain.User`를 재구성 → 호출자에 반환 (race 보정용)
- 에러는 기존 `errors.go` 매핑 사용 + 신규 errorCode 추가 (`E0000038` 등)

**TDD test 작성 시 케이스:**
1. 단일 필드 변경 → body에 해당 필드만 포함되는지
2. 미변경 필드는 omit
3. 400/403/404/429 응답 → 적절한 에러 타입 매핑
4. 응답 본문 → domain.User 역매핑
5. 빈 패치(모두 nil) 호출 → API 호출 없이 no-op 또는 명시적 에러
