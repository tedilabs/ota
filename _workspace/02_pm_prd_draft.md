# ota (Okta TUI) — Product Requirements Document

**상태:** FINAL (도메인 리뷰 반영 완료)
**버전:** 1.0.0
**작성일:** 2026-04-24
**작성자:** pm (ota-prd-team)
**도메인 레퍼런스:** `_workspace/02_okta_domain_input.md` (okta-expert, 2026-04-24)
**도메인 리뷰:** `_workspace/02_okta_prd_review.md` (okta-expert, 2026-04-24, APPROVE WITH CHANGES)

---

## 변경 이력

| 날짜 | 버전 | 변경점 | 작성자 |
|------|------|--------|--------|
| 2026-04-24 | 0.1.0-draft | 최초 초안 (Track 1: 도메인 비의존 섹션) | pm |
| 2026-04-24 | 0.2.0-draft | Track 2 통합: okta-expert 도메인 입력(§1~§12) 반영 — 리소스 필드·Policy 7타입·Rate Limit·페이지네이션·Logs 폴링·EL·검색 3종·권한 모델·MFA §7 포함 권고 수용 | pm |
| 2026-04-24 | 1.0.0 | okta-expert 도메인 리뷰 Must-fix 3건(M1/M2/M5) + Should-fix 3건(M3/M4/M6) + Minor 6건(m1/m3/m4/m5/m6/m7/m8) 전면 반영. Minor m2(User→Apps 탭)는 §11.3 결정 필요로 이관. v1.0으로 승격. | pm |
| 2026-04-24 | 1.0.0 | §11.3을 "결정 필요"에서 **"리더 결정 v1.0.0 확정"**으로 교체 (D-1~D-6). team-lead 승인 내역 반영: k9s+Vim 기본, 다크테마, tail 7초, Applications/User-Apps v0.2 연기, Write v0.2 리스크 오름차순. | pm |

---

## 1. 제품 비전 및 목표

### 1.1. 한 문장 비전

**ota는 IAM 운영자가 터미널을 떠나지 않고 Okta 조직을 "k9s처럼" 탐색·감사할 수 있게 하는 키보드 중심 TUI다.**

### 1.2. 왜 지금인가 (Why Now)

- Okta Admin Console은 조회/감사 워크플로우에 **지나치게 많은 클릭**을 요구한다. 사용자 한 명의 그룹 멤버십과 최근 로그인 이벤트를 상관시키려면 탭을 3~5번 오가야 한다.
- 현업에서는 Postman/curl + `jq`로 땜빵하고 있으나, **컨텍스트(어떤 tenant·어떤 리소스)를 잃기 쉽고 공유 가능한 뷰가 없다.**
- 반면 k9s가 증명했듯이, 키보드 중심 리소스 내비게이션 TUI는 매일 사용하는 운영자의 생산성을 수 배 향상시킨다. 같은 UX를 Okta에 적용하지 않을 이유가 없다.

### 1.3. 해결하지 않으면

- 조사·감사 시간이 줄어들지 않아 보안 인시던트 대응 시 **의사결정 병목**이 된다.
- 반복 조회 작업을 스크립트로 각자 만들며, **조직 차원의 표준 운영 절차(SOP)가 사람마다 다르다.**
- 신규 입사자는 Admin Console을 처음부터 배우면서 생산성을 잃는다.

### 1.4. 성공 정의 (What "Done" Looks Like)

- k9s 사용자가 ota를 처음 띄우고 "익숙하다"고 느낀다 (단축키/레이아웃/멘탈 모델이 일관).
- "사용자 Alice의 최근 로그인 실패 이벤트를 확인"하는 작업이 **10초 이내 10회 미만의 키 입력**으로 완료된다.
- 한 번 설정한 tenant 프로필을 재사용하여 복수 tenant 간 전환이 **2초 이내**.

---

## 2. 대상 사용자 (페르소나)

### P1. IAM 운영자 — "Dana"
- **역할:** IT/IAM 팀에서 Okta를 매일 운영. 사용자 프로비저닝 문의 대응, 그룹 멤버십 감사, 정책 변경의 영향 범위 파악.
- **기술 숙련도:** 터미널·curl·jq 편한 수준. Vim/Tmux 일상 사용.
- **일상 루틴:**
  - 매일 오전: 전날 System Log 훑기 (실패 로그인, 정책 거부 이벤트)
  - 수시: 헬프데스크 티켓 응대 → "이 사용자 그룹 상태/MFA 등록 현황"을 즉시 조회
  - 주간: Group Rules 동작 감사, 비활성 사용자 리스트 확인
- **고통점:** Admin Console의 페이지 전환 지연, 링크로 공유 시 동료가 컨텍스트 잃음, 대량 리스트의 필터가 느리거나 불편.
- **ota 가치:** "키보드만으로 모든 조회가 된다"는 것이 P1의 **단 하나의 핵심 가치**.

### P2. 보안 감사자 — "Sam"
- **역할:** 분기별 접근 권한 검토, 정책 위반 탐지, 인시던트 응답.
- **기술 숙련도:** Splunk/Sumo Logic 등 로그 도구 경험. Shell 친숙.
- **일상 루틴:**
  - 주간: System Log에서 `policy.evaluate_sign_on` outcome `FAILURE` 스크리닝
  - 인시던트 발생 시: 특정 User의 세션/Factor/Group을 **시간순으로** 재구성
- **고통점:** Okta 자체 로그 UI의 검색 옵션 한계, 필터 조합이 복잡. 외부 SIEM 연동은 시간차 있음.
- **ota 가치:** "검색 → 상세 → 관련 리소스 드릴다운"을 빠르게 반복. 로그를 1차 관문으로 삼고 User/Group/Policy 사이를 오가는 동선 단축.

### P3. SRE / 플랫폼 엔지니어 — "Kim"
- **역할:** SSO 통합 문제 분석(특정 앱으로 로그인 실패 급증 등), Terraform으로 관리하는 리소스의 현재 상태 검증.
- **기술 숙련도:** Go/Terraform/K8s 숙련. k9s 일상 사용자.
- **일상 루틴:**
  - 장애 응답: Apps 상태·Rate Limit 헤더·로그를 동시 관찰
  - 드리프트 체크: Terraform apply 후 실제 정책 평가 결과가 의도대로인지 확인
- **고통점:** 여러 창을 켜야 함. tfstate와 실제 상태의 불일치 디버깅.
- **ota 가치:** k9s와 동일 단축키로 진입 장벽 0. 스크립트로 엮은 Postman 컬렉션 대체.

### 2차 페르소나 (고려하되 MVP 목표 아님)
- **신규 팀원 "Jordan"**: 온보딩 시 ota를 참조 도구로 사용. MVP에서는 Help 화면/READ ME 잘 쓰면 커버 가능.
- **헬프데스크 티어1**: 단순 조회만 필요. ota는 약간 학습 곡선이 있어 초기 대상 아님 (v0.3 이후 고려).

---

## 3. 핵심 Use Cases (MVP 초점)

각 Use Case는 Dana(P1) 기준으로 서술. Sam/Kim은 파생.

### UC-1. "이 사용자가 어떤 그룹에 속해 있는가"
트리거: 헬프데스크 티켓 "alice@example.com이 특정 앱에 못 들어간다"
플로우:
1. ota 실행 → `:users` 또는 `:u` 엔터
2. `/alice` 로 필터
3. 엔터 → User 상세
4. `g` (groups) 키 또는 탭 전환 → 해당 사용자 소속 그룹
5. 그룹 선택 → 그룹의 `rules`/`apps` 확인
**목표:** 전체 10초, 15 키 입력 이내.

### UC-2. "이 사용자의 최근 로그인 실패 사유"
트리거: Sam(보안 감사자)의 주간 감사 또는 Dana의 티켓 대응
플로우:
1. `:logs`
2. 필터 `actor.alternateId eq "alice@example.com" and outcome.result eq "FAILURE"`
3. 최근 N건 상세 확인 (severity, reason)
**목표:** 필터 문법이 자연스럽게 떠오르는 단축 문법 제공. 빈번한 필터는 **저장 가능**해야 함.

### UC-3. "정책 변경의 영향 범위 파악"
트리거: Kim(SRE)이 Authentication Policy에 룰을 추가하려 할 때
플로우:
1. `:policies` → 타입 선택 → 해당 정책 상세
2. 현재 Rules 순서/조건 확인
3. 관련 그룹 탭으로 전환하여 어떤 그룹들이 이 룰에 매칭되는지 드릴다운
**목표:** 정책 → 룰 → 조건 → 타겟 그룹까지 **끊김 없이** 이동 (각 전환 < 300ms).

### UC-4. "Group Rule이 의도대로 동작하는가"
트리거: Dana가 동적 그룹 규칙을 확인할 때
플로우:
1. `:grouprules`
2. 규칙 선택 → 상세 (expression, allGroupsValid, 적용 그룹)
3. `m` (members) → 규칙 매칭으로 자동 추가된 사용자 샘플
**목표:** Expression을 **읽기 쉽게 포맷**. 검증 오류 상태(`INVALID`) 시 즉시 눈에 띔.

### UC-5. "로그 스트림 모니터링"
트리거: Dana가 정책 변경 후 실시간 영향 관찰
플로우:
1. `:logs`
2. 필터 설정 후 `s` (stream) 토글
3. 새 이벤트가 상단에 누적, 자동 스크롤 토글 가능
**목표:** Okta API Rate Limit을 존중하는 자동 폴링. 사용자는 주기를 의식하지 않음.

---

## 4. 범위 (Scope)

### 4.1. MVP In-Scope (v0.1)

**리소스 (Read-Only):**
- Users: 리스트, 상세, 검색, **등록된 MFA Factors 읽기(상세 탭)** — §7 도메인 권고에 따라 MVP 포함. 운영자의 가장 빈번한 요청 중 하나.
- Groups: 리스트, 상세, 멤버 조회, 할당 앱 카운트 표시
- Group Rules: 리스트, 상세 (Expression 원문 monospace 표시)
- Policies: **타입을 네임스페이스처럼 선택하는 UX** (OKTA_SIGN_ON / ACCESS_POLICY / PASSWORD / MFA_ENROLL / PROFILE_ENROLLMENT / POST_AUTH_SESSION / IDP_DISCOVERY), 상세, Rules 조회
- System Logs: 필터 검색, 상세, 폴링 기반 "tail" 모드 (5~10초 간격)

**UX 공통:**
- Vim 스타일 단축키 (`hjkl`, `gg`, `G`)
- 커맨드 프롬프트 (`:`)
- 인크리멘털 검색 (`/`)
- 리스트 ↔ 상세 ↔ 연관 리소스 드릴다운
- Help 화면 (`?`)

**설정 및 인증:**
- 설정 파일: `~/.config/ota/config.yaml` (XDG 기준)
- 복수 tenant 프로필
- 인증 우선순위: 환경변수 > 설정 파일 프로필 > 대화식 프롬프트
- 단축키 커스터마이징

**운영:**
- Rate Limit 감지 및 백오프
- Pagination (무한 스크롤 느낌)
- 에러/상태 토스트
- 컬러 테마 (기본 + high-contrast)

### 4.2. Explicit Out-of-Scope (MVP)

**Write 작업 전체 (모든 mutative 액션):**
- User/Group/Rule/Policy의 생성·수정·삭제
- User lifecycle 전이 (activate/deactivate/suspend/unsuspend/unlock/reset_password)
- 그룹 멤버 추가/제거
- Group Rule 활성화/비활성화 (비활성화가 멤버십 제거 부작용이 있어 위험)
- MFA Factor 리셋/제거/활성화
- Policy Rule 추가/순서 변경/활성화

> MVP는 **Read-Only Administrator** 발급 토큰으로 동작함을 가정. 모든 쓰기 호출은 도메인 제약상 403이 반환되며, UX는 이를 명확히 표시한다 (REQ-C04 참조).

**고급/대규모 기능:**
- 조직 간 Bulk 작업 (CSV 임포트 등)
- Application 관리 (SSO 설정, 할당)
- 대시보드/메트릭 집계 뷰
- SAML/OIDC 설정 에디터
- 웹훅(Event Hook/Inline Hook) 관리
- API Token 자체 발급/관리 (ota 내에서)

**인증:**
- OAuth 2.0 (서비스 앱 기반) — v0.2
- SSO 로그인 플로우 (브라우저 콜백) — 미정

**리소스:**
- Applications 리스트/상세 — v0.2 후보
- Authorization Servers — out of scope
- Identity Providers (IdPs) — out of scope
- Zones, Network Zones — out of scope
- Trusted Origins — out of scope
- Sessions API — out of scope

**플랫폼:**
- Windows 네이티브 지원 (WSL만) — v0.3 이후

### 4.3. Nice-to-Have (시간 허용 시 MVP 포함)

- 북마크 (`m`) — 자주 보는 사용자/그룹을 즐겨찾기
- 최근 리소스 목록 (`r`)
- YAML/JSON으로 원본 복사 (`y`) — 예: `yy`로 선택 row 전체, `yf`로 선택 필드
- 브라우저 열기 (`o`) — Admin Console 해당 리소스로 딥링크
- 필터 프리셋 저장 (`:save-filter`)

---

## 5. 기능 요구사항 (REQ-ID)

> **원칙:** 각 REQ는 "완료의 관찰 가능한 조건"을 AC로 가진다. 도메인 제약은 okta-expert 답변 수령 후 추가.

### 5.1. 공통 UX

#### REQ-U01: Vim 스타일 내비게이션
- **우선순위:** P0
- **설명:** 리스트/상세 화면에서 `h/j/k/l`, `gg`, `G`, `Ctrl-d`/`Ctrl-u`, `Ctrl-f`/`Ctrl-b`로 탐색 가능.
- **수용 기준:**
  - AC-1: 화살표 키도 동등하게 동작 (보조 키)
  - AC-2: 설정 파일로 키 매핑 오버라이드 가능 (REQ-C03 참조)
  - AC-3: 모든 주요 화면(리스트/상세/프롬프트)에서 일관
  - AC-4: Vim 모드가 아닌 에디터(예: 텍스트 필터 입력)는 표준 readline 키 (`Ctrl-a`/`Ctrl-e`)

#### REQ-U02: 커맨드 프롬프트 `:`
- **우선순위:** P0
- **설명:** 어느 화면에서든 `:`로 명령 팔레트 진입. 리소스 전환, 설정 변경, 도움말 접근.
- **수용 기준:**
  - AC-1: `:users`, `:groups`, `:grouprules`, `:policies`, `:logs`, `:help`, `:quit` 지원
  - AC-2: 탭 자동완성
  - AC-3: 부분 매칭 (`:u` → `:users` 후보)
  - AC-4: 히스토리 저장 (세션 간 보존, 최근 50개)

#### REQ-U03: 인크리멘털 검색 `/`
- **우선순위:** P0
- **설명:** 리스트 화면에서 `/`로 현재 보이는 컬럼 기준 즉시 필터. 서버 쿼리와는 별개(클라이언트측).
- **수용 기준:**
  - AC-1: 키 입력마다 < 50ms 갱신 (1,000행 기준)
  - AC-2: `Enter` 확정 / `Esc` 취소
  - AC-3: 대소문자 무시 기본, `\C`로 대소문자 구분 토글
  - AC-4: `n`/`N`으로 다음/이전 매치

#### REQ-U04: 서버측 검색/필터
- **우선순위:** P0
- **설명:** 클라이언트 필터로 커버 못 하는 대규모 결과는 Okta API 쿼리 파라미터 3종 (`q` / `search` / `filter`)을 사용. 각 리소스가 지원하는 문법 차이를 UX로 흡수한다.
- **수용 기준:**
  - AC-1: `/` 키는 항상 `q` (자유 텍스트) 기반 간단 검색. 모든 리소스에서 작동.
  - AC-2: `:search <SCIM-expr>` 커맨드로 고급 검색. Users/Groups는 SCIM-like `search`, Groups/Apps/Logs는 `filter` 사용. Help에 치트시트 포함 (연산자: `eq ne gt ge lt le sw ew co pr and or ()`)
  - AC-3: 잘못된 문법은 Okta API의 `E0000001` 에러를 파싱하여 "필드 X: 이유 Y" 형식으로 표시
  - AC-4: Logs 필터는 preset 제공 (예: "Failed Sign-ins Last 24h", "Group Rule Changes")
  - AC-5: Users `search`가 **eventually consistent**임을 Help에 명시 (방금 생성한 사용자 검색 지연 가능)

#### REQ-U05: 리스트 → 상세 → 연관 리소스 드릴다운
- **우선순위:** P0
- **설명:** 상세 뷰에서 탭(`Shift-Tab`/`Tab`)으로 연관 리소스 전환. 예: User → Groups → Apps(v0.2)
- **수용 기준:**
  - AC-1: 각 전환 < 300ms (캐시된 경우)
  - AC-2: 네트워크 호출 중이면 "loading" 인디케이터, 취소 가능 (`Esc`)
  - AC-3: Breadcrumb으로 현재 위치 표시

#### REQ-U06: 도움말 (`?`)
- **우선순위:** P0
- **설명:** 현재 화면의 단축키/커맨드 목록을 모달로 표시.
- **수용 기준:**
  - AC-1: 화면 컨텍스트별로 다른 도움말
  - AC-2: 검색 가능 (`?` 내부에서 `/`)
  - AC-3: 사용자 커스텀 키 바인딩도 반영

#### REQ-U07: 종료 보호
- **우선순위:** P1
- **설명:** `:q`, `Ctrl-c`로 종료. 로그 스트리밍 중이거나 미완료 요청이 있으면 확인.
- **수용 기준:**
  - AC-1: `Ctrl-c` 연타 시 즉시 종료 (보호 해제)
  - AC-2: 설정으로 보호 기능 비활성화 가능

### 5.2. 리소스별 요구사항 (도메인 의존)

> 필드·컬럼·검색 문법의 구체는 okta-expert 답변 수령 후 확정. 아래는 뼈대.

#### REQ-R01: Users 리스트/상세/검색
- **우선순위:** P0
- **설명:** 활성/비활성 사용자 리스트와 상세, 서버측 검색. id prefix는 `00u`, 식별자는 불변 `id`, 사람 친화 식별자는 `profile.login`(이메일).
- **수용 기준:**
  - AC-1 (리스트 기본 컬럼): `status`(색상 배지) · `profile.login` · `profile.displayName`(또는 firstName+lastName) · `lastLogin`(relative) · `statusChanged`
  - AC-2 (상태 색상): ACTIVE=green · PROVISIONED/STAGED=cyan · SUSPENDED=yellow · LOCKED_OUT=red · PASSWORD_EXPIRED=magenta · DEPROVISIONED=gray. **SUSPENDED와 DEPROVISIONED는 시각적으로 뚜렷이 구분** (혼동 잦음)
  - AC-3 (상세 탭): (i) Profile — 고정 필드 + 커스텀 필드 섹션 분리, (ii) Credentials — provider/type, (iii) Timestamps — created/activated/lastLogin/lastUpdated/statusChanged/passwordChanged, (iv) Groups — 별도 API (`/users/{id}/groups`), (v) **Factors** — 등록된 MFA factor (`/users/{id}/factors`), (vi) Recent Logs — actor.id로 로그 점프
  - AC-4 (성능): 1,000명 이상 조직에서 초기 페이지 렌더링 < 1s (`limit=200`, Link 헤더 기반 무한 스크롤)
  - AC-5 (검색): `/`는 `q` (prefix/substring), `:search`는 SCIM `search` (예: `profile.department eq "Engineering" and status eq "ACTIVE"`)
  - AC-6 (Factors 섹션): 각 등록된 factor에 대해 다음 필드 표시:
    - `factorType` — 내부 매핑으로 **사람 친화 라벨** 변환 (예: `push`→"Okta Verify (Push)", `token:software:totp`→"TOTP", `sms`→"SMS", `webauthn`→"WebAuthn (Security Key)", `token:hardware`→"Hardware Token", `email`→"Email", `question`→"Security Question")
    - `provider` + `vendorName` (third-party 구분 — OKTA/FIDO/DUO/GOOGLE 등)
    - `status` — `NOT_SETUP` · `PENDING_ACTIVATION` · `ACTIVE` · `EXPIRED` · `DISABLED`. **EXPIRED/DISABLED는 경고색(yellow/red)**
    - `profile` 내 판별 필드 (factor 타입별 다름):
      - WebAuthn: `credentialId` (키 별칭)
      - Okta Verify: `deviceType`, `name` (디바이스 모델)
      - SMS/Voice: `phoneNumber` — **기본 마스킹** (`+1-***-***-1234` 형태, 뒷 4자리만). `:unmask` 커맨드로 전체 표시 요청 시에만 해제
      - Email factor: `email`
    - `created`, `lastUpdated` (relative time)
    - `id` (factor id) — 기본 숨김, 상세 펼침(`e`) 시 표시, `y`로 복사 가능
    - 읽기만. reset/suspend/delete는 Out-of-Scope (v0.2 Write)
  - AC-7 (엣지): `status=DELETED`는 결과에 포함 안 됨(API 기본). Help 문구: "Users with status=DELETED are excluded by default. Deactivated (DEPROVISIONED) users ARE included unless filtered out with `status ne DEPROVISIONED`." — DELETED와 DEPROVISIONED 혼동 방지.

#### REQ-R02: Groups 리스트/상세/멤버
- **우선순위:** P0
- **설명:** 그룹 리스트, 상세, 멤버 탭, 할당 앱 카운트. id prefix는 `00g`. 이름 중복 가능하므로 id로 식별.
- **수용 기준:**
  - AC-1 (리스트 컬럼): `type` 아이콘(OKTA_GROUP/APP_GROUP/BUILT_IN) · `profile.name` · `profile.description` · `lastMembershipUpdated` · **동적 그룹 마커**(`RULE` 배지) — Group Rule 타겟팅 여부로 판단 (type으로 판별 불가, §1.3)
  - AC-2 (필터): `:filter type eq "OKTA_GROUP"` 등 SCIM `filter` 지원
  - AC-3 (멤버 탭): `/groups/{id}/users?limit=200&after=` 페이지네이션. 무한 스크롤. **대용량 그룹 경고 정책:**
    - `type == "BUILT_IN"`인 모든 그룹은 large-membership 배너 항상 표시 ("This is a system-wide group with potentially tens of thousands of members.")
    - 추가로 `profile.name == "Everyone"`이면 "all organization members" 명시적 라벨 추가
    - 기타 그룹은 진입 시 예상 크기를 알 수 없으므로 "Loading: N members so far…"로 progressive. 페이지 소진 중 사용자가 `q`/`Esc`로 중단 가능.
  - AC-4 (앱 카운트): 상세에서 `/groups/{id}/apps` 지연 호출(탭 진입 시). 권한 부족(403)이면 "-" 표시
  - AC-5 (멤버 수): 전체 카운트 API 없으므로 "페이지 소진 시 합산"으로 표시, 중단 가능

#### REQ-R03: Group Rules 리스트/상세
- **우선순위:** P0
- **설명:** 동적 그룹 규칙 목록, 상세. id prefix는 `0pr`. 상태 3종: ACTIVE / INACTIVE / INVALID. **비활성화는 해당 규칙으로 부여된 멤버십을 제거하는 부작용**이 있음 (쓰기 MVP 아님이지만 상세 페이지에 경고 표시 필요).
- **수용 기준:**
  - AC-1 (리스트 컬럼): `status` · `name` · 타겟 그룹 이름(`actions.assignUserToGroups.groupIds` 중 첫 1~2개, id→name 해소 필요, 캐시) · expression 요약(truncate) · `lastUpdated`
  - AC-2 (상태 컬러): ACTIVE=green · INACTIVE=gray · **INVALID=red (경고색, 운영자가 즉시 인지해야 함)**
  - AC-3 (상세 뷰): Expression 원문 monospace 표시. 개행 없으므로 가로 스크롤 또는 soft-wrap 토글. 대부분 한두 줄.
  - AC-4 (id→name 해소): 타겟 그룹 id는 리스트에서 name으로 표시. 조회 실패 시 id를 그대로 노출하고 "(name unavailable)" 표시.
  - AC-5 (경고 배너): 상세 뷰 상단에 "Deactivating this rule will remove all memberships it created. This action is disabled in read-only mode." — Write MVP에서 재사용 가능하도록 배너 컴포넌트 분리
  - AC-6 (기본 limit): API 기본 `limit=50`. ota는 `limit=200`까지 허용

#### REQ-R04: Policies 리스트/상세
- **우선순위:** P0
- **설명:** **Policy 타입은 네임스페이스처럼 취급.** `GET /policies?type=<TYPE>` 호출에 type 파라미터 필수이므로 UI는 타입 선택이 항상 선행. id prefix는 `00p`.
- **수용 기준:**
  - AC-1 (지원 타입 MVP): **7종 전체 조회 가능. 액션 렌더러는 4종만 MVP 완비** — `OKTA_SIGN_ON` · `ACCESS_POLICY` · `PASSWORD` · `MFA_ENROLL`의 상세는 사람 친화 액션 요약 렌더링. 나머지 3종(`PROFILE_ENROLLMENT` · `POST_AUTH_SESSION` · `IDP_DISCOVERY`)은 **raw-JSON 모드만 지원**, 리스트는 공통 컬럼으로 표시. `ENTITY_RISK`는 도메인 §12.5 확인필요로 MVP 제외.
  - AC-2 (타입 선택 UX): `:policies`는 타입 선택 메뉴(모달) 먼저 노출. `:policies OKTA_SIGN_ON` 직행 허용. 화면 상단 breadcrumb에 현재 타입 고정 표시. 렌더러 미완비 타입은 메뉴에 `(raw view)` 배지 표기로 사용자 기대 관리.
  - AC-3 (리스트 컬럼): `priority` · `status` · `name` · **`system` 배지**(기본 정책은 삭제/비활성 불가임을 명시) · `lastUpdated`. priority 오름차순 기본 정렬. 모든 7 타입에 동일 적용.
  - AC-4 (Rules 탭): policy → Rules (`/policies/{id}/rules`) priority 순. Rule 컬럼: `priority` · `status` · `name` · 액션 요약(렌더러 완비 4종만, 나머지는 "Rich view not yet available — press `r` for raw JSON") · `lastUpdated`. `system=true` 기본 Rule도 배지.
  - AC-5 (액션 요약 — 4 타입 풀 렌더): `ACCESS_POLICY`→"Require MFA" / "Deny" / "Allow w/o MFA"; `OKTA_SIGN_ON`→세션 속성(maxIdle/maxLifetime/requireFactor); `PASSWORD`→complexity(min length/age/history); `MFA_ENROLL`→enroll 정책(required authenticators). 내부 매퍼로 구현.
  - AC-6 (JSON 원본 토글): 상세 뷰에서 `r` 또는 `:raw`로 원본 JSON pretty-print. raw-only 3 타입은 기본 뷰가 raw (`r` 토글로 구조화 섹션 제공 안 함을 명시).
  - AC-7 (페이지네이션): `/policies`는 한도 엄격 (플랜별 상이, 도메인 §2.2 확인필요). ota는 `limit=20` 기본.
  - AC-8 (확장성): 새 Policy 타입(예: 향후 `CONTINUOUS_ACCESS`) 추가 시 리스트 컬럼/타입 메뉴에 코드 변경 최소로 추가할 수 있도록 내부 타입 카탈로그를 설정 가능 구조로 둔다.

#### REQ-R05: System Logs 검색/tail
- **우선순위:** P0
- **설명:** `/api/v1/logs` 기반 검색 + **`since` 재설정 폴링**을 통한 "tail" 모드. 실시간 스트리밍 API 없음.
- **수용 기준:**
  - AC-1 (리스트 컬럼): `published`(절대↔상대 토글, 로컬 TZ 옵션) · `severity`(DEBUG회색/INFO녹색/WARN노랑/ERROR빨강) · `eventType` · `actor.displayName` + `actor.alternateId` · `target[0].displayName`(있으면) · `outcome.result`(SUCCESS/FAILURE/CHALLENGE) · `client.ipAddress` 또는 geo
  - AC-2 (tail 알고리즘): 초기 `since=now-5m`, `sortOrder=ASCENDING`, `limit=1000`. 폴링마다 마지막 `published` 기준으로 `since` 갱신 (+1ms로 중복 방지). **기본 간격 7초** (§2.2의 5~10초 권장 중앙값), 설정으로 조정 가능. **Adaptive polling:** 첫 호출에서 관찰된 `X-Rate-Limit-Limit`가 60 미만이면 (Developer Free tenant 등 저한도 환경) 폴링 간격을 자동으로 15초로 상향. 이 조정은 `:about`에 표시.
  - AC-3 (tail UX): 토글 on 시 우상단 표시 "▶ tail". 새 이벤트 도착 시 상단 카운터 깜빡임. 자동 스크롤 토글(`f`). 429 수신 시 자동 일시정지 + "Paused (rate limited, retrying in Ns)". **복구 시 `since` 유지로 데이터 구멍 없이 재개** (백오프/재개 공통 메커니즘은 REQ-E01 AC-3 참조).
  - AC-4 (히스토리 모드): tail off 기본. `sortOrder=DESCENDING`로 최신순 표시. 무한 스크롤로 과거 탐색 (보관 기간 90~180일, 플랜 의존, §12.2 확인필요).
  - AC-5 (프리셋 필터): Help에 치트시트. 내장 프리셋:
    (i) "Failed Sign-ins 24h" = `eventType eq "user.session.start" and outcome.result eq "FAILURE"` + `since=24h ago`
    (ii) "Group Rule Changes" = `eventType sw "group.rule"`
    (iii) "**Group Rule Deactivations (may remove memberships)**" = `eventType eq "group.rule.deactivate"` — 멤버십 제거 유발 가능성을 경고색으로 표시
    (iv) "API Token Activity" = `eventType sw "system.api_token"`
    (v) "MFA Challenges" = `eventType sw "user.authentication.auth_via_mfa"`
  - AC-6 (상세): JSON pretty-print + 구조화 섹션(Actor/Target/Client/Outcome/Debug). `y`로 JSON 복사. `actor.id`/`target[].id`에서 해당 리소스 화면 점프 가능 (예: User로).
  - AC-7 (시간대): 기본 UTC. `:set tz=local` 또는 설정으로 로컬 변환. 표시에 항상 "Z" 또는 "+HH:MM" 명시.
  - AC-8 (`actor.type`): `User`가 아닌 `SystemPrincipal`(자동화/API 토큰)도 존재. 아이콘으로 구분.
  - AC-9 (지연): Log는 이벤트 후 수 초~수십 초 지연. Help에 "Logs may lag a few seconds behind real-time events" 명시.

### 5.3. 설정 및 인증

#### REQ-C01: 설정 파일 (XDG 준수)
- **우선순위:** P0
- **설명:** `~/.config/ota/config.yaml` (또는 `$XDG_CONFIG_HOME/ota/config.yaml`) 로드. 파일이 없으면 기본값으로 동작.
- **수용 기준:**
  - AC-1: 파싱 실패 시 친절한 에러 (행/열 표기)
  - AC-2: `--config <path>` CLI 플래그로 경로 오버라이드
  - AC-3: 최소 섹션: `profiles`, `ui`, `keybindings`, `logs`
  - AC-4: 주석(`#`) 보존 (읽기 전용이므로 실제 파일 수정 없음)

#### REQ-C02: 복수 Tenant 프로필
- **우선순위:** P0
- **설명:** 설정 파일에 복수 Okta tenant를 등록하고 `-p <name>` 또는 `:profile <name>`로 전환.
- **수용 기준:**
  - AC-1: 프로필별 `org_url`, `api_token_env` (토큰 환경변수 이름 지정), `default_log_filter` 등
  - AC-2: 실행 시 `--profile prod`로 지정
  - AC-3: TUI 중 `:profile` 전환 시 모든 상태 리셋 (< 2s)
  - AC-4: **실제 토큰은 설정 파일에 직접 쓰지 않음** (환경변수 참조 또는 OS keychain — MVP는 환경변수만)

#### REQ-C03: 단축키 커스터마이징
- **우선순위:** P1
- **설명:** 설정 파일에서 커맨드-단축키 매핑 오버라이드.
- **수용 기준:**
  - AC-1: 빌트인 매핑은 문서화 (기본 테이블 Help에 포함)
  - AC-2: 사용자 매핑이 빌트인과 충돌 시 사용자 매핑 우선
  - AC-3: 잘못된 키 이름(예: `Ctrl-∞`)은 부팅 시 경고
  - AC-4: 매핑 리로드는 MVP에서 재실행 필요 (런타임 리로드 v0.2)

#### REQ-C04: 인증 우선순위 및 에러 구분
- **우선순위:** P0
- **설명:** SSWS API Token 기반 인증. 토큰 결정 순서: (1) CLI `--token-env=<VAR>` 또는 프로필 `api_token_env` → (2) 표준 환경변수 `OKTA_API_TOKEN` + `OKTA_ORG_URL` → (3) 대화식 프롬프트(마스킹). Org URL은 `<org>.okta.com` · `<org>.oktapreview.com` · custom domain 모두 허용.
- **수용 기준:**
  - AC-1: 각 단계 실패 시 다음 단계로 폴백. 최종 결정 소스를 `:about`에 노출 ("token: env OKTA_API_TOKEN")
  - AC-2: 대화식 입력한 토큰은 메모리에만, 파일/히스토리/프로세스 환경 기록 없음
  - AC-3: 토큰 없이 TUI가 뜨지 않음. 명확한 에러 메시지 + 환경변수 예시 + 가이드 URL 출력 후 정상 종료(exit 1)
  - AC-4 (에러 매핑, 도메인 §2.3): `E0000004/401`="API token invalid or revoked. Rotate and retry." · `E0000011/401`="Token expired or revoked" · `E0000006/403`="Insufficient permissions for <resource> (token may be Read-Only; write actions blocked)" · `E0000047/429`=자동 재시도(REQ-E01) · `E0000007`="Resource not found (may have been deleted). Refreshing list…"
  - AC-5: 토큰 수명 힌트 (선택, best-effort) — `system.api_token.create` 이벤트 기반 추정치를 `:about`에 표시. 로그에는 토큰 id가 아닌 이름만 기록되므로 동일 이름 토큰이 여러 개면 정확도 낮음. "Token may be ~N days old (best-effort, based on token-name match)"로 명시. 실패 시 조용히 생략.
  - AC-6: **OAuth 2.0 Service App (Private Key JWT)은 MVP 범위 밖, v0.2.**

#### REQ-C05: 시크릿 유출 방지
- **우선순위:** P0
- **설명:** 로그, 에러 메시지, 프로필 덤프 어디에도 API Token이 노출되지 않음.
- **수용 기준:**
  - AC-1: 토큰 값은 내부 표현에서 zero-copy로 사용 후 메모리에서 가능한 조기 소거
  - AC-2: HTTP 디버그 로그에서 `Authorization` 헤더는 `***` 마스킹
  - AC-3: 크래시 스택트레이스에도 토큰 포함 금지 (구조체 필드에 `sensitive` 태깅 또는 `Stringer` 재정의)
  - AC-4: 설정 파일 예제는 placeholder만 (`"env": "OKTA_API_TOKEN"`, 실제 값 기재 금지)

### 5.4. 에러 / Rate Limit UX

#### REQ-E01: Rate Limit 감지 및 자동 백오프
- **우선순위:** P0
- **설명:** Okta API의 `X-Rate-Limit-Limit`, `X-Rate-Limit-Remaining`, `X-Rate-Limit-Reset` 헤더를 **동적으로** 해석. 수치 하드코딩 금지(플랜별 상이). 429 발생 시 자동 백오프 + 재시도.
- **수용 기준:**
  - AC-1 (경고 임계): `Remaining / Limit <= 10%` 감지 시 상단 상태바에 노란 경고 "Rate: <remaining>/<limit> (resets in Ns)"
  - AC-2 (429 처리): `Retry-After` 헤더 값(초 또는 HTTP-date) 준수 + ±20% jitter. 최대 3회 재시도. 3회 실패 시 빨간 에러.
  - AC-3 (폴링 자동 제어): tail 폴링·backfill은 백오프 기간 중 자동 중단. 복구 시 같은 `since`로 재개 (데이터 구멍 없이). UI 표시: "⏸ Paused · resuming in Ns"
  - AC-4 (엔드포인트 카테고리 인지): 관리 API(`/users`,`/groups`) · 로그 API(`/logs`) · 정책 API(`/policies`, 엄격)는 각각 다른 한도 버킷임을 모니터링. 카테고리별 Remaining을 구분 표시 (`:about` 또는 `:ratelimit` 화면). **중요 한계:** Okta 응답 헤더는 "현재 요청이 속한 카테고리"의 Remaining만 반환하므로, 각 카테고리 값은 **last-observed**(해당 카테고리 최근 호출 시점의 관찰값). UI에는 관찰 시각을 함께 표시("logs: 112/120 · 18s ago"). 관찰 시각이 오래됐으면 회색 처리.
  - AC-5 (로그 API 특별 고려): `/logs` 한도가 가장 엄격(Enterprise 기준 분당 120 추정). tail 기본 주기 7초는 분당 ~8회로 안전 마진 충분히 확보.
  - AC-6 (캐시): 사용자/그룹/정책 리스트 결과는 30초 TTL 메모리 캐시. `R` 또는 `:refresh`로 강제 무효화.

#### REQ-E02: 에러 상태 표시 일관성
- **우선순위:** P0
- **설명:** 네트워크/API 에러는 화면 하단 상태바 토스트 + 상세는 `:errors` 로그.
- **수용 기준:**
  - AC-1: 토스트는 3초 자동 해제, `Esc`로 즉시 제거
  - AC-2: 동일 에러 반복 시 카운터 표시 (스팸 방지)
  - AC-3: `:errors`로 세션 내 에러 히스토리 조회

#### REQ-E03: 오프라인/네트워크 단절 대응
- **우선순위:** P1
- **설명:** 네트워크 단절 감지 시 폴링 일시 중지, 복구 시 자동 재개.
- **수용 기준:**
  - AC-1: 상태바에 "offline" 표시
  - AC-2: 현재 캐시된 데이터는 계속 볼 수 있음 (쓰기 작업 없으므로)
  - AC-3: 복구 시 자동 리프레시 또는 사용자 확인

### 5.5. 관측성

#### REQ-O01: 디버그 로그 파일
- **우선순위:** P1
- **설명:** `--debug` 또는 설정 `debug: true`로 `~/.cache/ota/debug.log` 작성.
- **수용 기준:**
  - AC-1: 기본 비활성
  - AC-2: HTTP 요청/응답 헤더 (토큰 마스킹), 타이밍, 스테이트 전이
  - AC-3: 로그 로테이션 (10MB × 3) — 표준 라이브러리 수준으로 충분
  - AC-4: TUI 내 `:debug open`으로 tail 가능 (별도 창 대신 설명 메시지 OK)

---

## 6. 비기능 요구사항

### 6.1. 성능

| 항목 | 목표 |
|------|------|
| 초기 실행 → 리스트 렌더 | < 500ms (토큰 유효 시) |
| 리스트 키 입력 응답 | < 16ms (60fps 체감) |
| 리스트 → 상세 전환 | < 300ms (캐시 적중 시) |
| 리스트 → 상세 전환 | < 1s (API 호출 필요 시) |
| 클라이언트 필터 1,000행 | < 50ms |
| 페이지 prefetch | 백그라운드, 사용자 체감 없음 |
| 메모리 | 세션당 < 200MB (로그 버퍼 포함) |

### 6.2. 보안

**시크릿/토큰:**
- API Token은 파일에 저장하지 않음 (환경변수 또는 대화식 입력)
- 모든 HTTP는 TLS (Okta는 TLS 전용이므로 강제)
- 디버그 로그에서 토큰 필드 마스킹 (`Authorization` 헤더는 `***`)
- 크래시 덤프·코어 덤프·디버그 로그 모두 토큰·PII 스크럽 적용
- 설치 가이드에 `ulimit -c 0` 권장 (코어 덤프 디스크 기록 차단)

**PII (개인정보) 마스킹 정책:**
- 기본 마스킹 대상 필드: `phoneNumber`(Factors/SMS/Voice), `secondEmail`(User profile), `mobilePhone`(User profile)
- 마스킹 표기: `+1-***-***-1234` / `a***@example.com` 형태 (가시 프리픽스·접미사만 표시)
- 사용자 명시적 요청(`:unmask` 커맨드 또는 `y` 복사)에만 전체 값 노출
- 마스킹 해제 시 세션 범위만 유효, 로그에는 마스킹된 값만 기록

**공급망:**
- 의존성은 최소화, 주요 의존은 pinning (go.sum 검증)
- **CVE 모니터링**: dependabot 활성화

### 6.3. 신뢰성

- API 호출 재시도: idempotent GET만 자동 재시도 (최대 3회, 지수 백오프)
- `context.Context` 전파로 사용자 `Esc` 즉시 취소
- 패닉 시 친절한 크래시 메시지와 로그 경로 안내
- 테스트 커버리지: 핵심 도메인 패키지 ≥ 70% (MVP 기준)

### 6.4. 접근성 / 국제화

- **컬러 모드**: 기본(256-color) + high-contrast + monochrome
- **색맹 대응**: 상태 표시는 색 + 기호 모두 사용 (✓/✗/⚠ 등)
- **터미널 호환**: `xterm-256color`, `tmux`, `kitty`, `alacritty`, `wezterm` 테스트
- **국제화**: MVP는 영어만. 로케일별 포맷(`en-US` 날짜)은 OS 기본 사용
- **화면 크기**: 최소 80×24 지원 (작은 터미널에서는 컬럼 우선순위에 따라 생략)

### 6.5. 사용성

- **첫 실행 UX**: 토큰 없으면 가이드 메시지 + 환경변수 예제 출력 후 종료
- **오류 메시지**: 원인 + 권장 조치 포함 ("Rate limit hit. Retrying in 42s. Ctrl-c to abort.")
- **학습 곡선**: k9s 사용자는 "단축키 배울 필요 없음" 수준. 비사용자는 Help(`?`)로 1분 내 핵심 단축키 학습

### 6.6. 관측성 (운영용)

- 버전·커밋 해시·빌드 시각을 `:about`에 노출
- `:healthcheck` 커맨드: 현재 tenant 연결성·rate limit 상태 확인
- 로그 파일에 상관 ID (세션 시작마다 UUID) 포함

---

## 7. 도메인 제약 및 외부 의존

> 출처: `_workspace/02_okta_domain_input.md` (okta-expert, 2026-04-24). 아래 항목은 해당 문서의 근거 수준([확정]/[관례]/[확인필요])을 그대로 승계한다.

### 7.1. 기반 전제

- **Identity Engine (OIE) 기준** [확정]. Classic Engine 호환은 MVP 범위 밖.
- **API**: Management Core API (`/api/v1/...`) 전용. **SCIM (`/scim/v2/`)는 사용하지 않음** (외부 프로비저닝 용도).
- **Base URL**: `<org>.okta.com` / `<org>.oktapreview.com` / custom domain 세 가지 모두 허용.
- **시간대**: 모든 입출력 ISO8601 UTC. 로컬 변환은 클라이언트(ota)에서.
- **글로벌 식별자 없음**: 리소스 id는 tenant-scoped. tenant 간 매칭은 name/email 기반.

### 7.2. Rate Limit (동적 대응) [확정 + 관례]

- **헤더 기반 동적 판단**. 수치 하드코딩 금지 (플랜별 상이).
- 참고 한도 (Enterprise 기준 추정): 관리 API 600~1200/분, `/logs` 120/분, `/apps` 500/분, `/policies` ~100/분.
- 대응: REQ-E01 참조.

### 7.3. Pagination [확정]

- **Link 헤더 커서** 방식. `Link: <...?after=<cursor>&limit=...>; rel="next"`.
- **병렬 페이지 요청 불가** — 순차 fetch 필수.
- 엔드포인트별 최대 `limit`: `/users` 200 / `/groups` 200 / `/groups/{id}/users` 200 / `/groups/rules` 200 (기본 50) / `/apps` 200 / `/policies` ~20 / `/logs` 1000.
- `after` 커서는 불투명. 디코드/조작 금지.

### 7.4. System Logs [확정 + 관례]

- **실시간 스트림 API 없음.** polling with `since` 재설정이 유일한 tail 방식.
- tail 권장 주기 5~10초. ota 기본 7초 (REQ-R05 AC-2).
- 90~180일 보관 (플랜 의존, §12.2 확인필요).
- 이벤트 지연: 수 초~수십 초. 실시간 보장 없음.

### 7.5. 검색/필터 3종 [확정]

- `q`: 자유 텍스트, prefix/substring
- `search`: SCIM-like 고급 (Users/Groups 권장). eventually consistent (Users)
- `filter`: 엄격 (Groups/Apps/Logs). SCIM 연산자 서브셋

### 7.6. 권한 모델 [확정 + 관례]

- **MVP 권장 토큰 발급자: Read-Only Administrator**.
- 읽기 가능: Users, Groups, Group Rules, Applications, Policies, System Logs, Admin dashboard 메트릭.
- 불가: 모든 mutative, API Token 발급, Custom Profile Editor 변경, OAuth 앱 생성.

### 7.7. 에러 매핑 테이블 [확정]

| `errorCode` | HTTP | 상황 | ota 메시지 전략 |
|-------------|------|------|-----------------|
| E0000001 | 400 | 유효성 검사 실패 | `errorCauses` 파싱해 필드별 표시 |
| E0000004 | 401 | 인증 실패 | "API token invalid or revoked. Rotate and retry." |
| E0000006 | 403 | 권한 없음 | "Insufficient permissions for <resource>" |
| E0000007 | 404 | Not found | "Resource not found. Refreshing list…" |
| E0000011 | 401 | 토큰 무효/만료 | "Token expired or revoked" |
| E0000022 | 400 | 삭제 불가 | "Deactivate before deleting" (MVP에선 안내만) |
| E0000038 | 400 | 기능 비활성화 (예: 조직이 특정 MFA factor 비활성) | "This feature is disabled for your organization. Contact your Okta administrator." |
| E0000047 | 429 | Rate limit | 자동 재시도 (REQ-E01) |

### 7.8. 리소스별 핵심 엔드포인트 요약

| 리소스 | 엔드포인트 | 비고 |
|-------|-----------|------|
| Users | `GET /api/v1/users?search=…&q=…&limit=200&after=…` | 검색 eventually consistent |
| User 상세 | `GET /api/v1/users/{idOrLogin}` | login도 허용 |
| User groups | `GET /api/v1/users/{id}/groups` | |
| User factors | `GET /api/v1/users/{id}/factors` | MFA 읽기 |
| Groups | `GET /api/v1/groups?filter=…&search=…&limit=200` | |
| Group members | `GET /api/v1/groups/{id}/users?limit=200` | 대용량 경고 |
| Group Rules | `GET /api/v1/groups/rules?limit=200` | 기본 50 |
| Policies | `GET /api/v1/policies?type=<TYPE>&limit=20` | type 필수 |
| Policy Rules | `GET /api/v1/policies/{id}/rules` | priority 순 |
| Logs | `GET /api/v1/logs?since=…&filter=…&sortOrder=…&limit=1000` | tail 모드 |

### 7.9. 외부 의존 (기술 선정은 Phase 3에서 확정)

- **Go SDK**: `github.com/okta/okta-sdk-golang/v5` 권장 (도메인 §8). **얇은 Adapter 레이어**로 감싸 TUI 레이어에 SDK 타입이 누출되지 않도록 한다.
- **TUI 프레임워크**: `github.com/charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- **설정 파서**: YAML (`gopkg.in/yaml.v3` 또는 `knadh/koanf`) — Phase 3에서 결정
- **테스트**: `stretchr/testify`, `charmbracelet/x/exp/teatest`, `net/http/httptest`(Okta SDK 통합 테스트), `jarcoal/httpmock` 보조

### 7.10. 운영 함정 (도메인 §9 요약)

PRD에 반영된 완화책:
- **Everyone 그룹 대용량**: REQ-R02 AC-3 배너
- **Custom Profile 필드**: REQ-R01 AC-3 (고정 + 동적 섹션 분리)
- **Users search 인덱싱 지연**: REQ-U04 AC-5 Help 명시
- **로그 지연**: REQ-R05 AC-9 Help 명시
- **Timezone confusion**: REQ-R05 AC-7 UTC 기본, 로컬 토글
- **Token rotation 누락**: REQ-C04 AC-5 수명 힌트 (선택)
- **Preview vs Production 차이**: 설정 가이드에 명시 (테스트는 Preview, 프로덕션 재확인)

---

## 8. 릴리즈 계획 (초안)

### v0.1.0 (MVP) — 목표: Read-Only 핵심 워크플로우
- 모든 P0 REQ 완료
- 리소스 5종 리스트/상세/검색
- 설정 파일 + 프로필 + 인증 우선순위
- Vim 단축키 기본
- Rate Limit 대응
- 문서: README, 설정 예제, 단축키 치트시트

### v0.2.0 — 목표: 운영 편의 및 Write 초기
- Applications 리소스 추가
- 북마크·최근 목록
- OAuth 2.0 서비스 앱 인증 추가
- Write 초기: 그룹 멤버 추가/제거
- Windows (WSL 외) 테스트

### v0.3.0 — 목표: 고급 감사/분석
- 필터 프리셋·저장된 뷰
- 로그 집계 뷰 (eventType별 카운트 등)
- 플러그인 훅 (커스텀 뷰)

---

## 9. 수용 기준 매트릭스 (요약)

> 각 REQ의 AC는 각 섹션 내에 명시됨. 이 표는 추적용 요약.

| REQ-ID | 제목 | P | AC 개수 | 도메인 확인 필요 |
|--------|------|---|---------|------------------|
| REQ-U01 | Vim 내비게이션 | P0 | 4 | N |
| REQ-U02 | 커맨드 프롬프트 | P0 | 4 | N |
| REQ-U03 | 인크리멘털 검색 | P0 | 4 | N |
| REQ-U04 | 서버측 검색 | P0 | 5 | 해소됨 |
| REQ-U05 | 드릴다운 | P0 | 3 | N |
| REQ-U06 | 도움말 | P0 | 3 | N |
| REQ-U07 | 종료 보호 | P1 | 2 | N |
| REQ-R01 | Users (+ Factors 섹션, PII 마스킹) | P0 | 7 | 해소됨 |
| REQ-R02 | Groups (BUILT_IN 배너 정책) | P0 | 5 | 해소됨 |
| REQ-R03 | Group Rules | P0 | 6 | 해소됨 |
| REQ-R04 | Policies (7 타입: 4 풀 렌더 + 3 raw-only) | P0 | 8 | 대부분 해소 (ENTITY_RISK/CONTINUOUS_ACCESS는 §11.2 잔존) |
| REQ-R05 | Logs tail (+ adaptive polling) | P0 | 9 | 해소됨 (보관기간은 §11.2 잔존) |
| REQ-C01 | 설정 파일 | P0 | 4 | N |
| REQ-C02 | 프로필 | P0 | 4 | N |
| REQ-C03 | 키 커스터마이징 | P1 | 4 | N |
| REQ-C04 | 인증 우선순위 + 에러 매핑 | P0 | 6 | 해소됨 |
| REQ-C05 | 시크릿 유출 방지 | P0 | 4 | N |
| REQ-E01 | Rate Limit | P0 | 6 | 해소됨 (헤더 기반 동적) |
| REQ-E02 | 에러 UX | P0 | 3 | N |
| REQ-E03 | 오프라인 | P1 | 3 | N |
| REQ-O01 | 디버그 로그 | P1 | 4 | N |

---

## 10. 테스트/QA 전략 개요

### 10.1. 테스트 피라미드

1. **Unit (많음)**: 도메인 모델(User/Group/Policy) 파싱/변환, 필터 매처, 설정 로더, 키 바인딩 매처, 에러 매핑 테이블(§7.7)
2. **Component**: 각 TUI 화면 모델(`teatest`로 메시지 주입 → 출력 비교), Adapter 레이어(도메인 §8.3)
3. **Integration (중간)**: `net/http/httptest.Server`로 Okta API 응답 고정 → 엔드-투-엔드 워크플로우 (리스트→상세→검색, tail 폴링의 since 재설정, Link 헤더 페이지네이션, 429 백오프)
4. **E2E (소수)**: 실제 Okta Developer tenant 사용 (수동 또는 선택적 CI). 요구 사전 조건 아래.

### 10.2. 테스트 Tenant 사전 조건 (도메인 §6)

로컬/CI 검증에 필요한 최소 시드:
- Okta Developer Free Tenant (`https://dev-NNNNNN.okta.com`)
- Read-Only Administrator 역할로 발급한 API Token 1개 (placeholder: `OKTA_API_TOKEN`)
- 사용자 5~10명 (ACTIVE, SUSPENDED, DEPROVISIONED 각 1명 이상 포함)
- OKTA_GROUP 2~3개 (예: Engineering, Sales, Interns)
- Group Rule 1~2개 (expression: `user.department == "Engineering"`)
- Policy Rule 몇 개를 기본 정책에 추가
- 테스트 로그인/로그아웃 몇 번 (로그 이벤트 생성)

> 설정 가이드는 `docs/`의 별도 README에서 다룸. PRD는 요구사항만 명시.

### 10.3. 회귀 방지

- 모든 버그 수정은 **실패 테스트 먼저** 작성 (Fail-First)
- TUI 스냅샷 테스트: 주요 화면의 "정상 렌더링" 골든 파일
- 단축키 매핑: 키 → 액션 매트릭스 테이블 테스트

### 10.4. QA 기준 (Phase 7에서 상세)



- **Critical/High 0개**: 릴리즈 차단
- **Medium**: v0.1.x 패치로 해결
- **Low/Cosmetic**: v0.2 백로그
- 경계면 검증: PRD REQ ↔ 구현 ↔ 테스트가 삼각 일치

---

## 11. 오픈 이슈 및 후속

### 11.1. 도메인 Q&A 해소 현황 (Q1~Q12)

| 번호 | 주제 | 상태 | 반영 위치 |
|------|------|------|-----------|
| Q1 | 리소스별 기본 컬럼/필드 | ✅ 해소 | REQ-R01~R05 |
| Q2 | Policy 타입 범위 | ✅ 해소 | REQ-R04 (7종 채택, ENTITY_RISK 제외) |
| Q3 | Rate Limit 수치 | ✅ 해소 (헤더 기반 동적) | REQ-E01, §7.2 |
| Q4 | Pagination 패턴 | ✅ 해소 (Link 헤더 커서 통일) | §7.3 |
| Q5 | Logs 폴링 주기 | ✅ 해소 (기본 7초) | REQ-R05 AC-2 |
| Q6 | Read-Only Admin 범위 | ✅ 해소 | §7.6 |
| Q7 | Core API 결정 | ✅ 확정 (SCIM 미사용) | §7.1 |
| Q8 | Group Rule EL | ✅ 읽기만 (Validate API는 §12.4 잔존) | REQ-R03 |
| Q9 | 식별자 패턴 | ✅ 해소 (id prefix 문서화) | §7.8 + REQ-R* |
| Q10 | 검색 문법 3종 | ✅ 해소 | REQ-U04, §7.5 |
| Q11 | MFA Factors MVP | ✅ 해소 (읽기 포함, §7 권고 수용) | REQ-R01 AC-6 |
| Q12 | 테스트 tenant | ✅ 해소 (Developer Free + Read-Only Admin) | 10.1 |

### 11.2. 잔존 도메인 불확실성 (도메인 §12, PRD 수용 가능)

| 항목 | 영향 | 대응 방침 |
|------|------|-----------|
| Policy Rule id prefix 일관성 | 문서화 수준 | 아이콘 분기 단순화, 실제 API 응답 관찰로 Phase 4에서 확정. Phase 3 기술 문서에 Group Rule(`0pr`)과 Policy Rule 파싱을 분리하도록 경고 |
| 로그 보관 기간 (90 vs 180일) | REQ-R05 히스토리 모드 상한 | Help에 "plan-dependent" 표기, 실제 조회 실패는 404로 처리 |
| `/policies` rate limit 정확 수치 | REQ-E01 카테고리 경고 임계 | 헤더 기반 동적이라 영향 최소 |
| EL Validate 엔드포인트 경로 | v0.2 Write 설계만 영향 | **해소 — 공식 validate endpoint 부재 확정** (도메인 §5.5). Write v0.2 설계 시 'create+delete dry-run' 또는 클라이언트 사전 파싱 대안 사용. |
| `ENTITY_RISK` Policy 정식 여부 | REQ-R04 | MVP 제외. Phase 3 기술 조사 시 `CONTINUOUS_ACCESS`와 함께 재확인 후 GA 확정 시 v0.2 편입 |
| 최신 OIE Policy 타입 추가 (`CONTINUOUS_ACCESS` 등) | REQ-R04 확장성 | 타입 리스트를 설정 가능하게 (REQ-R04 AC-8). Phase 3 기술 조사 포함 |
| Event Hook 기반 유사 실시간 스트림 | REQ-R05 | 복잡도 높음, v0.3+ 고려 |

### 11.3. 리더 결정 (v1.0.0 확정, 2026-04-24)

| # | 결정 항목 | 확정 결과 | 근거 / 구현 시 유의 |
|---|-----------|-----------|---------------------|
| D-1 | 키 바인딩 철학 | **k9s 호환 + Vim 친화 기본값** | 사용자 요구 "k9s와 같은 컨셉". 기본 맵은 k9s 스타일(`:`, `/`, `gg`, `G`, `hjkl`, `q`). 단축키 충돌 시 Vim 우선. 설정 파일(REQ-C03)로 전부 override 가능. |
| D-2 | 컬러 테마 기본값 | **다크 테마 (k9s 기본 팔레트 유사)** | 상태 색상: 정상=초록 · 경고=노랑 · 에러=빨강 · 비활성=회색 · 하이라이트=시안. `NO_COLOR` env 존중 (REQ 6.4 접근성 섹션과 일관). REQ-R01 AC-2 상태 색상 매핑도 본 팔레트에 정렬. |
| D-3 | 로그 tail 기본 주기 | **7초** | okta-expert 권고 수용 (REQ-R05 AC-2). 사용자 `--poll-interval` 플래그 또는 설정 파일로 override. `X-Rate-Limit-Limit < 60` 관찰 시 15초로 adaptive 상향(기존 AC 유지). |
| D-4 | Applications 독립 뷰 v0.2 승격 | **No — v0.2 유지** | 초기 사용자 요구에 Apps 미포함. MVP 집중. Group 상세의 "할당 앱 카운트"(REQ-R02)만 MVP 유지. |
| D-5 | User 상세 "Apps" 탭 MVP 포함 | **No — v0.2 연기** | Applications 리소스와 함께 묶어 제공. MVP는 User→Groups→(Group의) 앱 카운트 경로로 대체. |
| D-6 | Write v0.2 로드맵 순서 | **도메인 리스크 오름차순 채택** | (1) Group 멤버 추가/삭제 (MVP는 읽기만; v0.2에서 쓰기 확장) → (2) User lifecycle: `unlock`/`activate`/`deactivate` → (3) Group Rule 생성/수정/삭제. **각 단계 명시적 확인 다이얼로그 필수** (비활성화 시 멤버십 제거 부작용 등 도메인 §1.4 경고 포함). |

> 본 표가 v1.0.0 확정 결정이다. 이후 변경은 변경 이력 + 영향받는 REQ 명시 후 진행.

### 11.4. 기술 검증 필요 (Phase 3~4 — developer + test-engineer)

- `teatest`로 TUI 메시지 주입·스냅샷 테스트 실전 적용 난이도
- Okta SDK v5의 Rate Limit 응답 노출 방식 (SDK 구조체 필드 vs 에러 메시지 파싱)
- Adapter 레이어에서 SDK 타입 차단 수준 (cyclic import·숨김 비용)
- 터미널 resize 중 tail 폴링 안정성 (cancel + 재구독)
- Link 헤더 커서 파싱의 SDK 내장 헬퍼(`HasNextPage`/`Next`) 커버리지

### 11.5. Out-of-Scope 재확인 (도메인 §11 체크리스트 기반)

- Custom Policy 편집, MFA reset, OAuth 앱 관리, SAML/OIDC 설정 에디터, Directory 통합 설정, API Token 발급 — 모두 MVP 제외 유지.
- Applications 독립 뷰는 v0.2.
- Write 액션 일괄은 v0.2+.

---

## 12. 향후 확장 포인트 (아키텍처에 고려)

- **도메인 플러그인**: Okta 외 IdP (Azure AD/Entra, JumpCloud, Google Workspace)를 동일 TUI에 투입하려면 리소스 모델과 클라이언트 어댑터를 분리 가능해야 함
- **Write 확장**: 쓰기 작업은 승인 프롬프트·감사 로그·dry-run 모드를 전제로 설계
- **커스텀 뷰**: 사용자가 YAML로 리소스 타입·컬럼·필터를 정의하는 DSL (v0.3)
- **공유 링크**: `ota://profile/users/<id>` 같은 URI 스키마로 팀 간 공유

---

**END OF PRD v1.0.0 (FINAL, 도메인 리뷰 반영 완료)**

*다음 단계: 본 문서를 `docs/PRD.md`로 복사하여 확정. Phase 3 (TUI Design)로 이관.*
