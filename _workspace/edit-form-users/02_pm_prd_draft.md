# PRD Addendum — Users Profile Edit Form (REQ-W*)

**상태:** DRAFT
**버전:** 1.1.0-draft (PRD v1.0.1 위에 addendum)
**작성일:** 2026-06-17
**작성자:** pm (ota-prd-team)
**도메인 레퍼런스:** `_workspace/edit-form-users/02_okta_domain_input.md` (okta-expert, 2026-06-17)
**상위 입력:** `_workspace/edit-form-users/00_input.md` (사용자 직접 요청)
**배경 맥락:** 본 addendum은 PRD v1.0.1에 **Users 프로필 편집(첫 mutation 표면)** 을 추가한다. 기존 REQ-R*/REQ-C*/REQ-E*/REQ-O*를 수정하지 않고, **W = Write/Workflow 네임스페이스**를 신설한다. 향후 lifecycle write (activate/deactivate/reset 등)도 동일 네임스페이스로 확장.

---

## 0. 통합 가이드 (기존 PRD에 어떻게 끼워 넣는가)

| 작업 | 위치 |
|------|------|
| §5에 새 하위 섹션 **5.6 Write 액션 (v0.2 Profile-Edit 선행)** 신설, REQ-W01 본문 삽입 | docs/PRD.md §5.5 직후 |
| §9 수용 기준 매트릭스에 REQ-W01 행 추가 | docs/PRD.md §9 테이블 끝 |
| §8 릴리즈 계획의 v0.2.0 라인을 "프로필 편집 선행 + 멤버십·라이프사이클 후속" 으로 정정 | docs/PRD.md §8 |
| §11.3 리더 결정 매트릭스에 D-7 (login MVP read-only) 추가 | docs/PRD.md §11.3 |
| 변경 이력 한 줄 추가 | docs/PRD.md 상단 |
| 4.1 MVP In-Scope에 "Users 프로필 편집 (REQ-W01)" 명시 / 4.2 OOS에서 "User 프로필 수정" 항목 제외 | docs/PRD.md §4.1, §4.2 |

> **버전 정책:** REQ-W01은 PRD v1.1.0의 신규 P0. 기존 R/C/E/O REQ는 변경 없음(하위 호환 100%). 따라서 이미 작성된 read-path 테스트·구현은 영향 받지 않는다.

---

## 1. 신규 기능 개요 (Why Now)

### 1.1. 문제 정의

PRD v1.0.x는 Read-Only 핵심 워크플로우만을 다뤘다. 그러나 운영 현장에서는 다음 루틴이 빈번하다:

- **헬프데스크 티켓 응답**: "alice의 부서가 바뀌었어요, Okta 프로필 업데이트해 주세요" → 현재는 Admin Console로 이탈
- **HR 통합 보정**: HR 시스템에서 이름이 잘못 들어온 경우의 즉시 보정
- **연락처 갱신**: 휴대폰/예비 이메일 갱신 (MFA 복구에 영향)

이러한 작업은 ota의 가장 큰 가치 명제 — **"키보드를 떠나지 않는다"** — 를 직접적으로 검증하는 실측 지표다.

### 1.2. 왜 Users 프로필이 첫 mutation 표면인가

| 후보 | MVP-첫-mutation 적합도 | 사유 |
|------|---------------------|------|
| **Users profile edit** | **★★★** | 단일 엔드포인트, 명확한 입력 폼 UI, 도메인 위험 중간 |
| Group 멤버십 변경 | ★★ | 동적 그룹 부작용, 두 리소스 cross-write |
| User lifecycle (activate 등) | ★★ | 각 액션이 비가역, 확인 다이얼로그 필요 |
| Policy Rule 편집 | ★ | 영향 범위 폭발, schema 복잡 |

→ Users profile edit이 **mutation 인프라(에러 매핑·dirty 추적·confirm 모달·optimistic vs server 보정)를 가장 적은 도메인 위험으로 검증**할 수 있는 후보. 이후 lifecycle write가 본 인프라를 재사용한다.

### 1.3. 해결 가치 (Measurable)

| 지표 | 현재 (Admin Console) | 목표 (ota) | 측정 |
|------|--------------------|----------|------|
| "사용자 부서 정보 수정 + 저장" 워크플로우 | 약 30~45초 (5~7 클릭, 페이지 2회 로드) | **10초 이내, 15 키 입력 이내** | 사용자 5명 수동 측정 |
| 변경사항 확인 후 취소 (실수 방지) | 별도 페이지 이동 필요 | **ESC 1회 + y/N 1회** | 키 입력 수 |
| 권한 없는 토큰의 실패 인지 | 저장 후 5xx/403 화면 전환 | **즉시 폼 위에 inline 안내, 입력 보존** | UX 검증 |

---

## 2. REQ-W 네임스페이스 정의

**규칙:**
- **W = Write/Workflow.** mutative API 호출이 발생하는 요구사항.
- 우선순위 표기 동일 (P0/P1/P2).
- 각 REQ-W는 다음 메타 필드를 명시한다:
  - **Mutation Endpoint**: 호출되는 Okta API
  - **Required Permission**: 최소 권한 요구
  - **Side Effects**: 호출 후 일어나는 다른 영향
  - **Rollback Strategy**: 실패 또는 사용자 후회 시 복구 경로

---

## 3. 신규 요구사항 — REQ-W01

### REQ-W01: Users 프로필 편집 폼

- **우선순위:** P0 (v0.2 출시 차단 항목)
- **Mutation Endpoint:** `POST /api/v1/users/{userId}` (partial-merge semantics, §7.1 참조)
- **Required Permission:** Super Admin / Org Admin / User Admin / Group Admin(자신 관리 그룹의 멤버 한정) / 또는 `okta.users.userprofile.manage` permission 보유 custom role. (도메인 §2.1)
- **Side Effects:**
  - `email` 변경 시 조직 설정에 따른 알림 메일 발송 가능 (사용자 사전 인지 필요)
  - `mobilePhone` 변경 시 SMS MFA factor 재인증 영향 가능
  - `department`/`division` 변경 시 Group Rule 재평가 → 그룹 멤버십 자동 변동 가능
  - SAML/OIDC claim 갱신은 대상 사용자 재로그인 시 반영
- **Rollback Strategy:** 저장 전 단계는 ESC + confirm 모달로 즉시 복귀 (drift 없음). 저장 후 후회는 사용자가 동일 폼에서 이전 값으로 재편집 (Okta는 ETag/undo 미지원 — last-write-wins).
- **설명:** Users 리스트 또는 User 상세에서 `e` 키로 진입하는 inline 편집 폼. Standard profile 11개 필드를 편집한다. `login`은 **표시하되 read-only**. 저장은 변경된 필드만 partial-merge로 전송. 모든 mutation 인프라(에러 매핑·dirty 추적·confirm 모달)의 모범 구현체.

#### 수용 기준 (AC)

##### AC-1: 진입점
- **AC-1.1**: Users 리스트 뷰의 선택된 행에서 `e` 키 입력 시 편집 폼 진입.
- **AC-1.2**: User 상세 뷰의 모든 탭(Profile/Credentials/Timestamps/Groups/Factors/Recent Logs)에서 `e` 키 입력 시 동일 폼 진입.
- **AC-1.3**: 진입 시 `GET /api/v1/users/{id}` 1회 호출로 **최신 스냅샷** 로드. 리스트 캐시는 신뢰하지 않는다 (도메인 §5.2-1, §9 D7).
- **AC-1.4**: 로딩 중 폼은 placeholder 입력칸 + 상단 `Loading…` indicator. 사용자는 `ESC`로 진입 취소 가능.
- **AC-1.5**: 권한 부족 등으로 GET이 4xx 실패하면 폼을 열지 않고 토스트로 사유 표시 + 직전 화면 유지. 5xx/네트워크 실패는 토스트 + 재시도 hint, 폼 진입은 차단.

##### AC-2: 편집 가능 필드 (Decision D-W2 확정)
폼이 표시하고 편집 가능한 필드는 다음 **11개**:

| TUI Label | API field | Required (default schema) | PII | 진입 시 마스킹 |
|----------|-----------|---------------------------|-----|---------------|
| First Name | `profile.firstName` | YES | - | - |
| Last Name | `profile.lastName` | YES | - | - |
| Display Name | `profile.displayName` | NO | - | - |
| Nickname | `profile.nickName` | NO | - | - |
| Email | `profile.email` | YES | △ | - |
| Title | `profile.title` | NO | - | - |
| Division | `profile.division` | NO | - | - |
| Department | `profile.department` | NO | - | - |
| Employee Number | `profile.employeeNumber` | NO | △ | - |
| Mobile Phone | `profile.mobilePhone` | NO | **YES** | **YES (§6.7 기존 정책)** |
| Secondary Email | `profile.secondEmail` | NO | **YES** | **YES** |

추가로 폼은 다음을 **read-only**로 표시한다:
- `profile.login` — read-only 입력칸 + inline hint "Login changes are blocked in MVP. Use `:change-login` (v0.2)." (도메인 §4.3 권고 수용, Decision D-W3)
- `status` 배지 — header에 read-only로 노출 + "Use `:activate`/`:deactivate` to change status" hint. (도메인 §7.2)
- `id`, `created`, `lastUpdated`, `passwordChanged` 등 메타데이터 — header 또는 footer에 read-only

**Custom Profile (Extras) 필드는 폼에 표시하지 않는다.** Detail 뷰에서만 read-only로 노출 (REQ-R01 AC-3 기존 동작 유지). 사유: §3.3 도메인 권고 (schema 다양성).

##### AC-3: 검증 규칙 (클라이언트 측 — 느슨, 도메인 §8.1)
- **AC-3.1 (필수)**: `firstName`, `lastName`, `email`이 빈 문자열이면 저장 버튼 disable + 해당 필드 inline 에러. (login은 read-only이므로 검증 대상 아님)
- **AC-3.2 (이메일 형식)**: `email`, `secondEmail`에는 느슨한 정규식 `*@*.*` 적용. 통과 못 하면 inline 에러 "Invalid email format". 엄격 검증은 서버 책임.
- **AC-3.3 (전화번호 hint)**: `mobilePhone`은 형식 강제하지 않음. focus-out 시 E.164 권장 hint inline 안내 ("Recommended: +<country><number>, e.g., +821012345678"). 저장은 가능.
- **AC-3.4 (길이)**: 클라이언트는 truncate 또는 차단하지 않는다. 서버 응답이 길이 위반(`E0000001`)이면 inline 에러로 표시 (도메인 §3.2, §8.1).
- **AC-3.5 (중복 사전 lookup 금지)**: 클라이언트가 `email` 등의 중복 여부를 별도 GET으로 미리 조회하지 않는다 (rate-limit 낭비, 도메인 §8.1). 서버 응답에서만 판단.

##### AC-4: 저장 동작
- **AC-4.1 (저장 키)**: `Ctrl+S` (전역) **또는** footer "Save" 버튼 포커스 상태에서 `Enter`. (Decision D-W5)
- **AC-4.2 (Partial-merge body)**: 진입 시 스냅샷 vs 현재 입력의 **diff에 포함된 필드만** body에 넣어 `POST /api/v1/users/{id}` 호출. 미변경 필드는 omit (도메인 §1.2). 빈 문자열 명시 전송은 회피(서버 검증 변동성).
- **AC-4.3 (저장 중 UI)**: footer에 spinner + "Saving…" 표시. 폼 전체 input disable. ESC도 비활성화 (race 방지). `Ctrl+C`만 강제 abort 허용 (요청 cancel + 폼 입력 보존).
- **AC-4.4 (연속 저장 가드)**: 200 응답 후 1초간 Save 버튼 disable. (도메인 §1.5 per-admin 40 req/user/10s 가드)
- **AC-4.5 (성공 처리)**: HTTP 200 응답
  - 응답 body의 `User` 객체로 detail/list 캐시 갱신 (다른 admin의 동시 변경 부분 반영, 도메인 §5.2-2)
  - 폼 닫고 진입 직전 화면(리스트 또는 detail)으로 복귀
  - 상태바 토스트 "Updated `<login>`" (3초)
  - 리스트 진입이었으면 해당 행을 selected 상태 유지

##### AC-5: 취소 동작
- **AC-5.1 (clean 상태)**: 변경 사항 없음(dirty=false) 상태에서 `ESC` → 즉시 폼 닫고 진입 직전 화면 복귀.
- **AC-5.2 (dirty 상태)**: dirty=true에서 `ESC` → **1단계 확인 모달** "Discard N changes? `y/N`". 기본 No(폼 유지). `y`/`Y`/`Enter`(on Yes 포커스)로 확정 시 변경 폐기 + 폼 닫기. (Decision D-W4)
- **AC-5.3 (저장 중 ESC)**: 비활성. footer hint "Saving… use Ctrl+C to abort" 표시.

##### AC-6: 에러 처리 (도메인 §6 매핑 통합)
저장 실패 시 폼은 **닫지 않는다** (변경값 보존, 도메인 §6.2). 예외는 AC-6.4의 404만.

| HTTP | Okta errorCode | 처리 |
|------|----------------|------|
| **400** | `E0000001` (validation) | `errorCauses` 배열을 파싱. `<field>: <message>` 패턴 매칭 → 해당 입력칸 위에 inline error. 매칭 실패한 cause는 footer "Other errors: …" 영역에 누적. |
| **400** | `E0000038` (schema 위반) | footer에 "Schema constraint failed: <errorSummary>" 표시. MVP는 standard 필드만 다루므로 발생 가능성 낮음. |
| **401** | `E0000011` / `E0000004` | 폼 유지 + 변경 사항 draft 보존. 토스트 "Token invalid/expired. Rotate and retry." (REQ-C04 AC-4와 일관). |
| **403** | `E0000006` | 폼 유지 + 토스트 "Insufficient permissions: 'Manage user profiles' required." 변경값 보존. 사용자가 토큰 교체 후 재시도 가능. |
| **404** | `E0000007` | 폼 **닫고** 진입 직전 화면 복귀 + 리스트 refresh 트리거. 토스트 "User no longer exists. Refreshing list." (도메인 §6 표 외 처리). |
| **409** | — | 발생하지 않음 (Okta `/users` profile update는 409를 반환하지 않음, 도메인 §5.3). UI는 409 코드 분기 미보유. 향후 다른 mutation에서 재사용 시 별도 처리. |
| **429** | `E0000047` | REQ-E01의 공통 백오프 적용. `Retry-After` 카운트다운 footer 표시 "Rate limited. Retrying in Ns…". 카운트 0이 되면 자동 1회 재시도 (수동 재시도 사용자 트리거 가능). 변경값 보존. |
| **5xx** | 다양 | 폼 유지. footer "Okta service error. Retry?" + 변경값 보존. |

- **AC-6.1**: `errorCauses` 파싱은 `<fieldName>:` prefix 정확 매칭. 미매칭은 footer로. (도메인 §6.1)
- **AC-6.2**: 동일 필드에 대한 에러가 사용자가 해당 필드를 수정하면 즉시 클리어된다(낙관적 UX).
- **AC-6.3**: 에러 토스트는 REQ-E02 정책 준수 (3초 자동 해제, `Esc` 즉시 제거). 단 inline error는 사용자가 필드를 수정할 때까지 유지.

##### AC-7: PII 필드 마스킹 (기존 §6.2 정책 통합)
- **AC-7.1 (진입 시)**: `mobilePhone`, `secondEmail`은 기존 PRD §6.2 PII 정책에 따라 **기본 마스킹** 상태로 표시 (`+1-***-***-1234`, `a***@example.com`).
- **AC-7.2 (편집 진입)**: 사용자가 해당 필드에 포커스(Tab으로 이동 또는 클릭/숏컷)하면 **자동 언마스킹** → 전체 값으로 편집 가능 (도메인 §8.4).
- **AC-7.3 (포커스 아웃 + 미수정)**: focus out 시 변경이 없으면 다시 마스킹.
- **AC-7.4 (포커스 아웃 + 수정)**: 변경된 값은 마스킹 없이 계속 표시 (사용자가 수정한 값 확인 가능). dirty 마커 점등.
- **AC-7.5 (전체 토글)**: `m` 키 (form-wide). 모든 PII 필드 일괄 mask/unmask. 기존 REQ-R01 AC-6 / §6.2 토글 키와 일관.
- **AC-7.6 (로깅)**: debug.log (REQ-O01)에는 PII 필드는 마스킹된 값만 기록.

##### AC-8: 접근성 (기존 §6.4 정책 통합)
- **AC-8.1 (키보드 only)**: 폼 내 모든 동작은 키보드만으로 가능. `Tab`/`Shift+Tab` 필드 이동, `Ctrl+S` 저장, `ESC` 취소, `m` 마스킹 토글.
- **AC-8.2 (NO_COLOR)**: `NO_COLOR` 환경변수 존중. 색 없이도 다음이 식별 가능:
  - dirty 마커: 필드 라벨 좌측 `*` 기호
  - required 필드: 라벨 좌측 `[required]` 텍스트 또는 `!` 기호
  - inline error: 필드 아래 `! <message>` 텍스트
  - read-only 필드: 라벨 우측 `(read-only)` 텍스트
- **AC-8.3 (최소 터미널 크기)**: 80×24에서 폼이 정상 표시. 더 좁으면 필드를 세로로만 배치하고 truncate 경고를 footer에 노출.
- **AC-8.4 (포커스 표시)**: 색 + 굵은 테두리 + 라벨 prefix `▸` 셋 모두 사용. 색맹/모노크롬 모드에서도 식별 가능 (§6.4).

##### AC-9: Dirty 추적 / Diff 표시 (도메인 §8.3, Decision D-W10)
- **AC-9.1**: 진입 시 snapshot 저장. 매 keystroke마다 snapshot vs current 비교.
- **AC-9.2**: 변경된 필드는 라벨에 `*` 마커 (또는 색).
- **AC-9.3**: footer에 `N changes` 카운터 표시 (0이면 표기 생략).
- **AC-9.4**: 저장 body 구성 시 dirty 필드만 포함 (AC-4.2).

##### AC-10: 폼 외 상태 미오염
- **AC-10.1**: 저장 성공 시에만 list/detail 캐시를 갱신한다. 진행 중인 다른 폴링/리스트는 영향 없음.
- **AC-10.2**: 폼이 열려 있는 동안에도 백그라운드 `since` 폴링(logs tail), rate-limit 헤더 갱신은 계속 동작 (사용자 인지 없음).
- **AC-10.3**: 폼 진입 직전 화면의 스크롤 위치/선택 행은 종료 시 복원.

#### Out of Scope (REQ-W01에서 명시 제외)

폼에 **포함하지 않는다** (도메인 §3.3/§4/§7 권고):

1. **`login` 편집** — read-only 표시만. 별도 `:change-login` 명령은 v0.2.
2. **Custom profile fields (Extras)** — schema 다양성. v0.2 schema-driven form.
3. **Credentials**:
   - `password` 직접 변경 — 별도 `:reset-password` lifecycle (v0.2).
   - `recovery_question` — 보안 영역, MVP 제외 (도메인 §7.1).
4. **Status 변경** — 기존 lifecycle 명령(`:activate`/`:deactivate`/`:suspend`/`:unlock` v0.2)이 담당. 폼은 read-only badge.
5. **MFA Factor reset/delete** — v0.2 Write 백로그.
6. **Group 멤버십 변경** — 별도 워크플로 (v0.2).
7. **User type 변경** (`profile.userType`) — schema 마이그레이션 효과, MVP/v0.2 제외 (도메인 §7.3).
8. **PUT (strict replace) 경로** — 코드 레벨에서 차단. ota가 노출하지 않는다 (도메인 §1.3).
9. **Optimistic concurrency (If-Match)** — Okta 미지원. last-write-wins (AC-6 409 미발생). v0.2에서 "사전 GET → diff 비교" UX 검토 (Decision D-W11).

---

## 4. Decision Matrix — REQ-W01 확정 결정 (PM)

도메인 §9 권고를 우선 채택. PM 추가 결정사항은 사유 명시.

| # | 결정 사항 | 확정 결과 | 근거 / 도메인 ref |
|---|----------|----------|------------------|
| **D-W1** | 편집 가능 필드 (final) | §3.1의 **11개 필드** 채택 (AC-2 표) | 도메인 §3.1, §9 D1 권고 그대로 |
| **D-W2** | `login` 편집 허용 여부 | **No — read-only로 표시.** 별도 `:change-login` 명령 v0.2 | 도메인 §4.3 강한 권고. 전사 SSO 단절 위험. §9 D2 |
| **D-W3** | `email` 변경 시 추가 확인 모달? | **No.** inline hint만 ("Changing email may trigger user notification per org settings") | 도메인 §9 D3. confirm 피로 회피. Okta 자체 알림이 발송됨 |
| **D-W4** | dirty 상태 ESC | **1단계 confirm 모달 (`y/N`, default N)** | 도메인 §9 D4 |
| **D-W5** | 저장 키 | **`Ctrl+S`** (전역) **OR** Save 버튼 포커스 시 `Enter` | 도메인 §9 D5. Vim ergonomics + bubbletea 관례 일치 |
| **D-W6** | 저장 실패 시 폼 동작 | **닫지 않음, 변경값 보존** (404 예외) | 도메인 §6.2, §9 D6 |
| **D-W7** | 진입 시 latest GET | **1회 (폼 진입 시점)** | 도메인 §5.2-1, §9 D7. v0.1에서는 "저장 직전 한 번 더 GET → diff conflict 모달"은 미도입 (v0.2 검토) |
| **D-W8** | Custom fields, recovery question | **MVP 제외** | §9 D8 |
| **D-W9** | Status 표시 | **read-only header badge + lifecycle 명령 hint** | §9 D9, REQ-R01 AC-3 (Profile 탭과 일관) |
| **D-W10** | dirty diff 표시 | footer `N changes` + 필드별 `*` 마커 | §9 D10 |
| **D-W11** | 동시 편집 (race) 대응 | **last-write-wins + 진입 시 latest GET + 저장 응답으로 캐시 보정**. 사전 conflict 모달은 v0.2 검토. | 도메인 §5. ETag/If-Match 미지원이 근본 제약 |
| **D-W12** | 권한 사전 검증 | **No.** 저장 시도 → 403 응답 → 친화적 메시지 + 폼 유지 | 도메인 §2.2 권고. permission 매트릭스가 custom role로 다양화 |
| **D-W13** | 빈 패치 저장 (변경 0) | Save 버튼 disable + footer "No changes to save". API 호출 자체를 보내지 않는다 | 도메인 §12 어댑터 테스트 케이스 5 일치. UX 무의미 호출 차단 |
| **D-W14** | `email` 변경 시 login 자동 동기화 | **No.** login은 read-only이고 email 변경이 login에 영향을 주지 않는다 | login == email 조직이라도 ota는 login을 잠그므로 동기화 부담 없음. v0.2 dedicated 명령에서 처리 |
| **D-W15** | PUT/strict-mode 표면화 | **금지** (ota 어댑터 자체에서 PUT 경로를 노출하지 않음) | 도메인 §1.3 데이터 손실 위험 |
| **D-W16** | 폼 mount 모드 | **modal/overlay (full-screen take-over)**. push view onto navigation stack (commit a68426b의 nav stack 활용) | TUI 일관성 — list/detail와 같은 stack semantics. ESC로 pop |

---

## 5. 비기능 요구사항 (기존 §6 통합 확인)

REQ-W01은 기존 §6 비기능 정책을 **그대로 따른다**. 신규 정책 도입 없음.

| §6 항목 | REQ-W01 통합 위치 |
|---------|------------------|
| §6.1 성능 — "리스트 키 입력 응답 < 16ms" | 폼 keystroke도 동일 목표 (60fps). AC-9 dirty 추적 알고리즘은 O(N) where N = 11 필드 → 무시 가능 |
| §6.1 성능 — "리스트 → 상세 < 1s (API)" | 폼 진입 시 GET 1회. AC-1.4의 placeholder 폼은 < 100ms 표시 |
| §6.2 보안 — 시크릿 유출 방지 | 폼은 API token을 다루지 않음. PII 마스킹은 AC-7 |
| §6.2 보안 — PII 마스킹 | AC-7 전체. 기존 §6.2 정책의 mobilePhone/secondEmail 토글 키 `m`을 form-wide로 재사용 |
| §6.3 신뢰성 — `context.Context` 전파 | AC-4.3 `Ctrl+C` abort = ctx cancel |
| §6.3 신뢰성 — idempotent GET 재시도 | AC-1.5 진입 시 GET은 3회 자동 재시도 (기존 정책). 저장 POST는 **재시도하지 않음** (mutative → 의도하지 않은 중복 적용 방지). 단 429만 REQ-E01 AC-2의 자동 1회 재시도 적용 |
| §6.4 접근성 — NO_COLOR / 80×24 | AC-8 |
| §6.5 사용성 — 오류 메시지 형식 | AC-6 |
| §6.7 (가정: 기존 PII 마스킹 §6.2 내) | AC-7. 토글 키 `m` 일관 |

---

## 6. 보안 추가 고려사항

기존 §6.2를 보강하는 mutation-specific 항목:

- **감사 로그**: ota는 사용자 변경의 actor가 토큰 발급자. ota 자체는 별도 client-side audit log를 남기지 않으며, Okta system log(`user.account.update_profile`, `user.identity.update`)에 의존한다 (REQ-R05로 조회 가능).
- **변경 의도성 보장**: AC-13 (빈 패치 거부), AC-4.4 (1초 가드) 외 추가 confirm 다이얼로그는 두지 않는다 (운영자 피로 회피). PII 필드 변경 시에도 별도 confirm 없음 — dirty 마커와 `N changes` footer로 변경 인지 충분으로 판단.
- **저장 직후 로그 추적 hint**: 저장 성공 토스트에서 `l` 키로 `:logs eventType eq "user.account.update_profile" and target.id eq "<userId>"` 점프 가능 (선택 구현, v0.1.x 패치 후보 — Open Issue OI-W3 참조).

---

## 7. 측정 지표 (REQ-W01 출시 후 검증)

| 지표 | 목표 | 측정 |
|------|------|------|
| 진입 → 저장 완료 (성공 케이스) | ≤ 10초 (운영자 1명 단순 1필드 수정 기준) | 수동 측정 5회 평균 |
| 키 입력 수 (1필드 수정 + 저장) | ≤ 15 keystroke | 수동 측정 |
| 저장 실패 후 변경값 보존 확인 | 100% (4xx/5xx/429 모두) | 통합 테스트 |
| 권한 부족(403) 발견 → 토큰 교체 → 재저장 성공 | 폼 닫지 않고 가능 (변경값 보존) | 수동 검증 |
| PII 마스킹/언마스킹 동작 일관성 | REQ-R01 AC-6 / §6.2 정책 100% 부합 | 회귀 테스트 |
| 빈 패치 호출 0건 | API 호출 자체 없음 (D-W13) | 통합 테스트 (HTTP mock 호출 카운트 = 0) |

---

## 8. 릴리즈 계획 (PRD §8 정정안)

기존 §8 v0.2.0 라인을 다음으로 교체:

### v0.2.0 — 목표: 운영 편의 및 Write 1차 (Profile-Edit 선행)
- **REQ-W01: Users 프로필 편집 (P0, 본 addendum)** — Write 인프라(에러 매핑·dirty·confirm·partial-merge·PII 통합) 모범 구현
- (이후) Group 멤버 추가/제거 (Write 인프라 재사용)
- (이후) User lifecycle: `unlock`/`activate`/`deactivate`/`reset-password`/`reset-factors` (도메인 §11.3 D-6 순서 유지)
- (이후) `:change-login` dedicated 워크플로 (v0.2 후반)
- Applications 리소스 추가 (read)
- 북마크·최근 목록
- OAuth 2.0 서비스 앱 인증 추가

### v0.2.0 — Profile-Edit 출시 게이트 (REQ-W01 한정)
- AC-1~AC-10 모두 통과 (회귀 테스트 포함)
- 도메인 권고 위반 0건 (특히 D-W2 login 잠금)
- HTTP mock 통합 테스트: 200/400(`E0000001`)/400(`E0000038`)/401/403/404/429/5xx 모두 케이스 통과
- 수동 QA: 측정 지표 §7 모두 충족

---

## 9. Open Questions / Deferred

| OI-ID | 항목 | 처리 | 결정 시점 |
|-------|------|------|----------|
| OI-W1 | Custom Profile (Extras) 편집 — schema-driven form | **v0.2 후반 또는 v0.3.** schema introspection (`/api/v1/meta/schemas/user/default`) 기반 동적 form generator 검토 | v0.2 종료 후 |
| OI-W2 | `:change-login` dedicated 워크플로 (영향 범위 preflight + 2단계 확인) | **v0.2.** 도메인 §4.4의 가드 사양 반영 | v0.2 진입 |
| OI-W3 | 저장 성공 토스트의 "`l` 키로 audit log 점프" | **v0.1.x 패치 후보** (선택). REQ-R05 검색 문법 재사용으로 구현 단순 | TUI Design Phase 3 |
| OI-W4 | 사전 conflict 모달 (저장 직전 GET → diff) | **v0.2 검토.** 도메인 §5.2 Advanced 패턴. race 자체는 막지 못함 — UX 만족도 vs 추가 호출 trade-off | v0.2 후반 |
| OI-W5 | 다른 mutation 표면(lifecycle 등)에서 본 폼 인프라 재사용 패턴 | **TUI Design Phase 3에서 component 추상화**. confirm 모달·error 매핑·toast는 도메인-agnostic | Phase 3 산출물 |
| OI-W6 | SDK v5의 `UpdateUser` strict 옵션 노출 차단 (lint 또는 wrapper) | **개발자에게 위임** (도메인 §1.3, §12). 코드 리뷰 가드 |  Phase 4 Architecture |
| OI-W7 | `email` 변경 + `login == email` 조직에서의 분기 | **D-W14로 일단 분리.** v0.2 `:change-login`에서 통합 처리 검토 | v0.2 |
| OI-W8 | 폼 진입 권한 사전 힌트 (best-effort) | **MVP 미도입** (D-W12). v0.2에서 `GET /api/v1/iam/me/...` 같은 가능성 재조사 | v0.2 |

---

## 10. Change Log (PRD 상단에 추가될 한 줄)

```
| 2026-06-17 | 1.1.0 | REQ-W01(Users 프로필 편집 폼) addendum 추가. 첫 mutation 표면 도입. Write/Workflow 네임스페이스 신설. login은 MVP read-only(D-W2). 도메인 권고 §9 D1~D10 전건 채택. 기존 REQ-R/C/E/O 무변경(하위 호환 100%). 영향 문서: §4.1/4.2, §5(신설 5.6), §8 v0.2, §9, §11.3(D-7). | pm |
```

---

## 11. 수용 기준 매트릭스 추가행 (PRD §9 테이블에 삽입)

```
| REQ-W01 | Users 프로필 편집 폼 | P0 | 10 (AC-1~AC-10) | 해소됨 |
```

---

## 12. §11.3 리더 결정 매트릭스 추가행 (D-7)

```
| D-7 | Users 프로필 편집 MVP 포함? | **Yes — REQ-W01로 v0.2 P0 채택.** Write 1차로 진입. login은 read-only(MVP 잠금). | 사용자 요청(2026-06-17). 도메인 위험 가장 낮은 mutation 표면. Write 인프라(에러 매핑·dirty·confirm·PII 통합) 모범 구현체로 후속 lifecycle write가 재사용. login 잠금은 SSO 단절 위험 회피(도메인 §4.3). |
```

---

**END OF REQ-W01 ADDENDUM (DRAFT)**

*다음 단계: 본 문서를 docs/PRD.md에 패치 적용 → Phase 3 TUI Design으로 이관.*
