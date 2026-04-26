# 02. Okta 도메인 관점 PRD 리뷰 — v0.2.0-draft

**리뷰어:** okta-expert
**날짜:** 2026-04-24
**대상:** `/Users/austin/workspace/tedilabs/ota/_workspace/02_pm_prd_draft.md` (v0.2.0-draft, 728 lines)
**기준 문서:** `_workspace/02_okta_domain_input.md` (§1~§12)
**리뷰 관점:** Okta Workforce Identity / OIE 도메인 사실 정합성, 운영 현실성, 보안/감사 리스크

## 총평

**결론: APPROVE WITH MINOR CHANGES.** 도메인 입력을 체계적으로 흡수했고 근거 수준([확정]/[관례]/[확인필요]) 트래킹 메커니즘이 제대로 작동합니다. CRITICAL 도메인 사실 오류는 **0건**. 정합성은 매우 높습니다. 다만 Group Rule Expression validation의 네거티브 확정(§5.5 보강본)이 아직 PRD에 전이되지 않았고, 몇 가지 MAJOR 수준의 구현 난이도 과소평가·모호성이 있어 PM에게 반영을 요청합니다.

| 수준 | 건수 |
|------|------|
| CRITICAL | 0 |
| MAJOR | 6 |
| MINOR | 8 |

---

## 1. 필수 체크 포인트 (team-lead 지정 8건)

### 체크 1. REQ-R03 `status=INVALID` 배지/경고색 요구사항
**상태: ✅ PASS.** AC-2에 명시 — "INVALID=red (경고색, 운영자가 즉시 인지해야 함)". AC-5에 경고 배너 요구사항 포함. 추가 요구사항 없음.

다만 추가 권고 (MAJOR): 네거티브 확정(§5.5)이 아직 PRD §11.2 "잔존 불확실성"에 "EL Validate 엔드포인트 경로 [재확인]"으로 남아있는데, **네거티브 확정**됐으므로 "해소 — 공식 validate endpoint 없음. Write v0.2 설계 시 'create+delete dry-run' 또는 클라이언트 사전 파싱으로 대응"으로 업데이트 필요.

### 체크 2. §7 도메인 제약에 rate limit/pagination/검색 3종/에러 8종 반영
**상태: ✅ PASS.** 모두 반영됨.
- §7.2 Rate Limit: 헤더 기반 동적 + 참고 한도 (Enterprise 기준 추정치)
- §7.3 Pagination: Link 헤더 커서, 병렬 불가, 엔드포인트별 limit 표
- §7.5 검색 3종: q/search/filter 구분 + eventually consistent 경고
- §7.7 에러 매핑 테이블: 8종(E0000001, 04, 06, 07, 11, 22, 38, 47) 전부 포함

MINOR 개선 포인트: `E0000038`을 "기능 비활성화"로만 설명했는데, 실제로는 ota 컨텍스트에서 MFA/Factors 조회 시 조직이 해당 factor를 비활성화한 경우 발생 가능. Help 혹은 AC에서 "Feature disabled for your organization. Contact Okta admin." 문구 권장.

### 체크 3. REQ-R04 Policies 7 타입 전부 MVP 포함 적절성
**[권장] 7 타입 모두 유지.** 단 **AC 조정 권고(MAJOR)** — 아래 PM 질문 #1 상세 답변 참조.

### 체크 4. REQ-R05 Logs tail 기본 7초
**[권장] 7초 유지.** AC-2 + AC-5의 안전 마진 계산도 정확합니다 (120 RPM 한도 대비 ~8 RPM = ~7% 사용). 다만 **운영 현실 반영 필요(MAJOR)** — PM 질문 #2 상세 답변 참조.

### 체크 5. REQ-R01 Users 상세 MFA Factors 읽기 필드 완전성
**[권장] 유지, 단 MAJOR 보완 필요.** PM 질문 #4 상세 답변 참조. AC-6의 필드 리스트(`factorType`, `provider`, `status`, `created`, `lastUpdated`)는 Okta API 응답 필드와 일치하지만 `device`, `profile.credentialId`, `vendorName` 등 판별 유용한 필드가 빠져 있습니다.

### 체크 6. REQ-E01 Rate Limit 동적 판단 전략 현실성
**상태: 대체로 PASS, 단 MAJOR 보완 1건.** 헤더 기반 동적 + 카테고리별 버킷 인지는 올바른 방향. 그러나 **AC-4의 "카테고리별 Remaining 구분 표시"가 구현상 과소평가**되어 있음 — Okta는 응답 헤더로 "현재 요청이 속한 카테고리"만 반환하지, 전체 카테고리 Remaining을 한 번에 주지 않습니다. 구현 시 각 카테고리별로 최근 요청 시 관찰된 Remaining을 "last-seen" 스냅샷으로 유지해야 하며, 이것이 항상 최신이 아님을 UI에서 표기해야 합니다. AC-4에 "last observed" 문구 추가 권장.

### 체크 7. 잔존 §12 불확실성 항목의 PRD 전이 정확성
**상태: 대체로 PASS, 단 1건 업데이트 필요(MAJOR).** §11.2 테이블에 7개 항목 전이됨. 그러나:
- **EL Validate 엔드포인트** 행은 제가 2026-04-24 재확인으로 네거티브 확정(§5.5)했으므로 "MVP 읽기만이므로 영향 없음" → "해소 — 공식 validate endpoint 없음. Write v0.2 설계 시 대안 전략 §5.5 참조"로 업데이트.
- **`ENTITY_RISK`** 항목의 "GA 확정 시 v0.2에서 편입"은 OK. 단 Okta가 2024년 말경 공개한 것으로 보이는 `CONTINUOUS_ACCESS`와 함께 Phase 3 기술 조사 시 재확인.

### 체크 8. §1 리소스 식별자 prefix가 어디에 하드코딩/잘못된 예시로 박혀있는지
**상태: ✅ PASS.** 전체 스캔 결과:
- REQ-R01 "id prefix는 `00u`" ✓ 정확
- REQ-R02 "id prefix는 `00g`" ✓ 정확
- REQ-R03 "id prefix는 `0pr`" ✓ 정확 (Group Rule용)
- REQ-R04 "id prefix는 `00p`" ✓ 정확
- `alice@example.com` 등 페르소나/use case 예시는 모두 `example.com` 사용 — ✓ 실제 tenant 노출 없음
- tenant URL 예시 `dev-NNNNNN.okta.com` — ✓ placeholder

단 MINOR 주의: REQ-R03에서 Group Rule id prefix `0pr`과 Policy Rule id prefix(잠정 `0pr`)가 같은 것처럼 보일 수 있음. 도메인 §12 "Policy Rule id prefix 일관성 [확인필요]" 항목이 잔존. PRD에서는 Group Rule 문맥이므로 문제 없지만, 개발자가 두 리소스를 같은 파서로 처리하지 않도록 Phase 3 기술 문서에 경고 필요.

---

## 2. PM 검토 질문 7건에 대한 답변

> 형식: **[권장/반대/유보]** + 근거 + 대안

### Q1. REQ-R04 Policies 7 타입 전부를 MVP에 포함하는 것이 적절한가?

**[유보 → 권장으로 조건부 승격]**

**근거:**
- Okta 조직 대부분에서 **`OKTA_SIGN_ON`(Global Session)**, **`ACCESS_POLICY`(앱별 인증)**, **`PASSWORD`**, **`MFA_ENROLL`** 4 타입이 일상 운영 대상이며 Dana/Sam 페르소나의 일차 수요입니다. 이 4개 없이 "Policies 화면"은 의미 없습니다.
- 나머지 3 타입(`PROFILE_ENROLLMENT`, `POST_AUTH_SESSION`, `IDP_DISCOVERY`)은 **존재는 하지만 일상 조회 빈도가 낮고**, 응답 구조가 앞 4개와 크게 다릅니다. 특히 `IDP_DISCOVERY`는 `conditions`/`actions` 스키마가 독특합니다.
- 문제는 AC-5 "액션 요약 — 타입별 가변 스키마"입니다. 7 타입의 `actions` 렌더러를 MVP에 전부 작성하면 구현 공수가 비대칭적으로 커집니다 (타입별 200~400줄 매핑 예상). MVP 품질 저하 위험.

**권고 (MAJOR):**
- MVP는 **4 타입 풀 렌더링 + 3 타입 "raw JSON only" 모드**로 타협:
  - `OKTA_SIGN_ON`, `ACCESS_POLICY`, `PASSWORD`, `MFA_ENROLL` → AC-5 액션 요약 매퍼 전부 구현
  - `PROFILE_ENROLLMENT`, `POST_AUTH_SESSION`, `IDP_DISCOVERY` → 리스트는 공통 컬럼(priority/status/name/system/lastUpdated), 상세는 JSON raw + "Rich view not yet available — press `r` for raw JSON" 배지
- 이렇게 하면 타입 선택 UX(AC-2)는 유지되고 후속 AC-5 완성도 약속할 수 있으며, 운영자가 "이 타입이 지원된다"는 신호는 받습니다.
- AC-1 문구를 "7종 모두 조회 가능. 액션 렌더러는 우선 4종 완비, 나머지 3종은 raw-only"로 수정.

**대안:** 시간 여유가 있으면 7 타입 풀 지원 유지. 다만 Phase 5 구현 중 일정 압박 시 먼저 컷 가능한 곳이 여기입니다.

### Q2. REQ-R05 Logs tail 기본 7초 주기가 적절한가?

**[권장]**

**근거:**
- 도메인 §1.7의 5~10초 권장 중앙값. `/logs` 엔드포인트 분당 120 추정 한도 대비 7초 주기 = 분당 ~8.6회 = **~7% 사용률**. 다른 화면(Users/Groups/Policies)도 동시에 `/logs`를 쓰지 않으므로 안전 마진 충분.
- 7초는 사람이 "tail off/on" 토글 시 체감 지연도 거의 없고, 5초보다 배터리·네트워크 친화적.
- 다른 TUI 도구(k9s의 `--refresh 5`)와 유사 범위.

**추가 권고 (MAJOR):**
- **AC-2에 "플랜 독립 안전 하한"을 명시 필요.** 무료 Developer tenant는 `/logs`가 더 제약적일 수 있음 (확인필요 — 제 지식으로는 동일하지만 테스트 권장). Developer tenant에서 처음 실행 시 `X-Rate-Limit-Limit`가 120 미만이면 폴링 주기 자동 증가(예: 15초)하는 **adaptive polling** 요구사항을 추가하면 운영 견고성 상승.
- **AC-3의 "429 자동 일시정지"는 OK, 단 복구 후 재개 시 `since`가 "정지 시점의 마지막 published"로 유지되어 데이터 구멍이 없어야 함**. 현재 AC-3에는 "paused"만 명시, 복구 동작은 AC-5 REQ-E01 AC-3에 있음. 두 섹션 간 cross-reference를 명시적으로 달면 구현자가 혼동하지 않음.

**대안:** 5초로 줄이면 체감 즉시성 ↑, 안전 마진 ↓ (10% 사용). 권하지 않음.

### Q3. (team-lead가 이 번호는 생략 — 8번 체크 포인트로 대체)

### Q4. REQ-R01 User 상세 MFA Factors 읽기 필드 완전성

**[권장 + MAJOR 보완]**

**근거 — 필드 보강 필요:**
현재 AC-6은 `factorType`, `provider`, `status`, `created`, `lastUpdated` 5개 필드. 실제 Okta `GET /users/{id}/factors` 응답은 훨씬 풍부하며, 운영자가 **판별에 필요한 필드가 빠져 있습니다:**

- `id` (factor id — reset 등 후속 동작에 필수. MVP 읽기만이어도 표시 필요)
- `vendorName` (예: OKTA, FIDO, DUO — `provider`와 별개로 Duo 같은 3rd party 구분)
- `factorType`의 구체 값: `push`, `sms`, `call`, `question`, `token:software:totp`, `token:hardware`, `webauthn`, `email`, `password` 등. **UI에서 사람 친화적 라벨로 변환 권고** (예: "Okta Verify (Push)", "SMS", "WebAuthn (Security Key)")
- `profile` 객체 내 판별 필드:
  - `credentialId` (WebAuthn의 경우 키 별칭)
  - `deviceType`, `name` (Okta Verify의 경우 디바이스 모델)
  - `phoneNumber` (SMS/Voice — 마스킹 필요! 뒷 4자리만 표시)
  - `email` (email factor)
- `status` 값 분기: `NOT_SETUP`, `PENDING_ACTIVATION`, `ACTIVE`, `EXPIRED`, `DISABLED` 등. **EXPIRED / DISABLED 는 경고색**.

**권고 변경안 (REQ-R01 AC-6 재작성):**
```
AC-6 (Factors 섹션): 각 등록된 factor에 대해 다음 필드 표시:
  - factorType (사람 친화 라벨로 변환, 내부 매핑 테이블)
  - provider + vendorName (third-party 구분)
  - status (NOT_SETUP/PENDING_ACTIVATION/ACTIVE/EXPIRED/DISABLED, EXPIRED/DISABLED는 경고색)
  - profile 내 판별 필드: credentialId(WebAuthn), deviceType+name(Okta Verify), phoneNumber 뒷 4자리만(SMS/Voice), email(email factor)
  - created, lastUpdated (relative time)
  - factor id (상세 펼침에서만, 복사용)
읽기 전용. reset/suspend/delete는 Out-of-Scope (v0.2 Write).
```

**추가 보안 권고 (MAJOR):**
- `phoneNumber` 전체 표시는 개인정보 노출. UI에서 **기본은 마스킹**(`+1-***-***-1234` 형태), `y`로 복사 또는 `:unmask`로 전체 표시를 요청해야 함. 이 정책은 §6.2 보안에도 추가할 가치 있음.

### Q5. (team-lead 지시에 포함 안됨 — 7 체크포인트로 대체)

### Q6. 잔존 §12 불확실성 항목이 PRD에 올바르게 전이되었는가

**[유보 — 1건 업데이트 필요]**

위 "체크 7" 참조. EL Validate 엔드포인트는 네거티브 확정됐으므로 §11.2 테이블 업데이트 필요. 나머지는 OK.

### Q7. (자유 질문으로 해석 — 일반 품질 관찰 종합)

**[권장 전체 승인]**

PRD는 prd-authoring 방법론을 모범적으로 따랐습니다. 특히:
- 페르소나 3종이 구체적이고 ota 가치 제안이 분명
- Use Case 5개가 측정 가능한 목표(키 입력 수, 시간) 포함
- REQ-ID + AC 구조가 일관되고 테스트 가능
- §7 도메인 제약이 체계적으로 통합됨
- §11.2 잔존 불확실성 + §11.3 결정 필요 분리가 훌륭

단 Phase 3(TUI 디자인)에 넘기기 전에 아래 MAJOR 6건은 반영 권고.

---

## 3. CRITICAL

**없음.** 도메인 사실 오류나 보안 리스크 수준의 이슈는 발견하지 못했습니다.

---

## 4. MAJOR (반영 권고)

### M1. REQ-R04 Policies — 7 타입 풀 렌더링 AC 현실화
- **위치:** §5.2 REQ-R04 AC-5
- **문제:** 7 타입 모두 AC-5의 "액션 요약" 매퍼를 MVP에 완비하는 것은 구현 공수 과소평가. 타입별 스키마가 크게 달라 실질 8-12주 추정.
- **권고:** 4 타입(`OKTA_SIGN_ON`, `ACCESS_POLICY`, `PASSWORD`, `MFA_ENROLL`) 풀 렌더 + 3 타입(`PROFILE_ENROLLMENT`, `POST_AUTH_SESSION`, `IDP_DISCOVERY`) raw-JSON 모드. AC-1 문구 수정.

### M2. REQ-R01 MFA Factors 필드 보강 + 개인정보 마스킹
- **위치:** §5.2 REQ-R01 AC-6 + §6.2 보안
- **문제:** `phoneNumber` 전체 표시는 PII 노출. 판별에 유용한 `vendorName`, `profile.credentialId/deviceType/name`, factor `id` 누락.
- **권고:** 위 Q4 답변의 AC-6 재작성안 적용. §6.2에 "PII 필드 기본 마스킹 정책" 추가.

### M3. REQ-E01 AC-4 Rate Limit 카테고리별 표시의 "last-seen" 한계
- **위치:** §5.4 REQ-E01 AC-4
- **문제:** Okta는 응답 헤더에 "현재 요청이 속한 카테고리"만 반환. 카테고리별 Remaining을 "항상 최신으로" 표시 불가.
- **권고:** AC-4에 "each category's Remaining is 'last observed' from that category's most recent call" 문구 추가. `:ratelimit` 화면에 마지막 관찰 시각 표시.

### M4. REQ-R05 AC-2 Adaptive Polling 요구사항 추가
- **위치:** §5.2 REQ-R05 AC-2, AC-3
- **문제:** 고정 7초 주기는 일반 Enterprise tenant에서 안전하지만 Developer Free tenant나 제약된 트라이얼 환경에서 한도가 다를 수 있음.
- **권고:** AC-2에 "If observed X-Rate-Limit-Limit for /logs < 60, automatically increase poll interval to 15s." AC-3와 REQ-E01 AC-3 간 cross-reference 명시.

### M5. §11.2 EL Validate 엔드포인트 행 업데이트
- **위치:** §11.2 테이블 4행
- **문제:** 2026-04-24 재확인으로 네거티브 확정됨. 현재 "MVP 읽기만이므로 영향 없음"은 맞지만 v0.2 설계자가 혼동할 수 있음.
- **권고:** "해소 — 공식 validate endpoint 부재 확정 (도메인 §5.5). Write v0.2 설계 시 'create+delete dry-run' 또는 클라이언트 사전 파싱 대안 사용" 으로 변경.

### M6. REQ-R02 AC-3 — Everyone 그룹 특별 처리를 "type 체크"로 명시
- **위치:** §5.2 REQ-R02 AC-3
- **문제:** 현재 "Everyone(BUILT_IN) 같은 조직 전원 그룹은 특별 배너"라고 쓰여있는데, **판별 기준**이 애매. `type == "BUILT_IN"` 만으로 Everyone인지 확정되지 않을 수 있음(다른 built-in도 있음).
- **권고:** AC-3 수정: "Groups with `type=BUILT_IN` always display the large-membership banner; additionally, if `profile.name == 'Everyone'` show an explicit 'all organization members' label."

---

## 5. MINOR (개선 권고)

### m1. §7.7 에러 매핑 테이블 — `E0000038` 구체화
"기능 비활성화" → "조직 설정에서 이 기능이 비활성화되어 있음. 관리자에게 문의." 문구로 확장.

### m2. §7.8 엔드포인트 요약 테이블 — `GET /users/{id}/apps` 누락
Users 상세에 "할당된 앱" 탭이 REQ-R01에는 안 쓰여있지만 페르소나 Dana의 UC-1 "특정 앱에 못 들어간다" 플로우에 자연스럽게 필요. MVP 범위에 포함할지 결정 필요. (Groups→앱 카운트는 있으나 User→앱은 없음.) 권고: MVP에 User 상세 "Apps" 탭 추가, `GET /users/{id}/appLinks` 사용 (정확한 경로는 확인필요).

### m3. REQ-R05 AC-5 프리셋 — `group.rule.deactivate` 명시 누락
"Group Rule Changes" 프리셋은 `eventType sw "group.rule"`이라 `activate`/`deactivate` 모두 포함됨 — OK. 다만 특히 중요한 `deactivate`(멤버십 제거 유발)를 별도 프리셋으로 추가 권고: "Group Rule Deactivations (may remove memberships)" = `eventType eq "group.rule.deactivate"`.

### m4. REQ-R01 AC-7 DELETED 상태 Help 문구
"`status=DELETED`는 결과에 포함 안 됨(API 기본)"은 정확. 추가로 "Deactivated(DEPROVISIONED) users ARE included unless filtered out" 문구도 Help에 추가 권고 — 이 점이 실무에서 혼동 유발.

### m5. REQ-C04 AC-5 토큰 수명 힌트 — 오탐 위험
`system.api_token.create` 이벤트 기반 추정은 **현재 사용 중인 토큰이 정확히 어떤 이벤트에 해당하는지** 식별할 수 없습니다 (로그에는 토큰 id가 아닌 이름이 들어감). 토큰 이름이 여러 개면 오추정 가능. AC-5에 "best-effort, may be imprecise" 문구 추가 또는 v0.2로 연기.

### m6. §10.2 중복된 섹션 번호
§10.2가 두 번 등장 (Tenant 사전조건과 회귀 방지). 회귀 방지 섹션을 §10.3으로 번호 수정. §10.3 "QA 기준"은 §10.4로.

### m7. §6.2 보안 — 디스크 쓰기 금지 강화
"API Token은 파일에 저장하지 않음"만 명시. 추가로 "Crash dumps, core dumps, debug logs 모두 토큰 마스킹 적용. 설치 시 `ulimit -c 0` 권고" 문구 추가 권고.

### m8. §11.3 "Write MVP 확장 순서" — Group Rule activate/deactivate 위험성 강조
(b) Group Rule activate/deactivate는 제가 도메인 §5.5에 명시한대로 **비활성화가 멤버십 제거를 유발**합니다. (a) Group 정적 멤버 < (c) User unlock/unsuspend < (b) Group Rule 순으로 위험. 이 순서 정보를 §11.3에 추가해 v0.2 로드맵 결정자가 인지하도록.

---

## 6. 정합성 체크 (Cross-Reference)

도메인 문서 §1~§12의 모든 항목이 PRD에 어떻게 전이됐는지:

| 도메인 § | PRD 반영 위치 | 상태 |
|----------|-------------|------|
| §0 공통 전제 (OIE, Base URL, 인증) | §7.1, REQ-C04 | ✅ |
| §1.2 Users 상태 머신 | REQ-R01 AC-2 | ✅ 6 상태 + 색상 |
| §1.3 Groups 3 타입 | REQ-R02 AC-1 | ✅ |
| §1.4 Group Rules 3 상태 + 비활성화 부작용 | REQ-R03 AC-2, AC-5 | ✅ |
| §1.6 Policies 7 타입 | REQ-R04 AC-1~7 | ✅ (M1 조정 권고) |
| §1.7 Logs 폴링 알고리즘 | REQ-R05 AC-2~3 | ✅ (M4 adaptive 권고) |
| §2.1 Pagination | §7.3 | ✅ |
| §2.2 Rate Limit | REQ-E01, §7.2 | ✅ (M3 보완) |
| §2.3 에러 8종 | §7.7 | ✅ (m1 보완) |
| §3 검색 3종 | REQ-U04, §7.5 | ✅ |
| §4 Read-Only Admin | §7.6, §4.2 | ✅ |
| §5.5 EL validation 네거티브 확정 | §11.2 | ⚠️ 업데이트 필요 (M5) |
| §6 테스트 tenant | §10.1 | ✅ |
| §7 MFA Factors MVP 포함 | REQ-R01 AC-6 | ✅ (M2 필드 보강) |
| §8 SDK + Adapter | §7.9 | ✅ |
| §9 운영 함정 9개 | §7.10 | ✅ |

**핵심 발견:** §5.5 네거티브 확정이 PRD에 아직 부분 반영 (§11.2에만 오래된 문구). 나머지 반영은 훌륭.

---

## 7. PM 수정 요청 우선순위

**Must-fix (Phase 3 이관 전):**
- M1 (Policies 7 타입 AC 현실화)
- M2 (MFA Factors 필드 + PII 마스킹)
- M5 (§11.2 EL validation 업데이트)

**Should-fix:**
- M3, M4, M6
- m4, m6, m8

**Nice-to-have:**
- 나머지 minor 전부

---

## 8. 결론

PRD v0.2.0-draft는 도메인 정합성이 매우 높은 수준입니다. CRITICAL 이슈 없음. MAJOR 6건·MINOR 8건을 반영하면 Phase 3(TUI 디자인)로 깔끔하게 이관 가능합니다. 특히 Must-fix 3건(M1, M2, M5)만 반영해도 도메인 관점에서는 승인 수준입니다.

**승인: APPROVE WITH CHANGES (Must-fix 3건 반영 조건).**

— okta-expert, 2026-04-24
