# ota TUI Design

**상태:** FINAL (pm + okta-expert 리뷰 반영 완료)
**버전:** 1.0.0
**작성일:** 2026-04-24
**작성자:** tui-designer (ota-prd-team)
**근거 PRD:** `docs/PRD.md` v1.0.0 (2026-04-24, FINAL)
**도메인 레퍼런스:** `_workspace/02_okta_domain_input.md` (2026-04-24)
**검수 문서:**
- `_workspace/03_pm_design_review.md` (pm, 2026-04-24, APPROVE WITH MINOR CHANGES)
- `_workspace/03_okta_design_review.md` (okta-expert, 2026-04-24, APPROVE WITH MINOR CHANGES)

---

## 변경 이력

| 날짜       | 버전         | 변경점 | 작성자       |
|------------|--------------|--------|--------------|
| 2026-04-24 | 0.1.0-draft  | 최초 초안 | tui-designer |
| 2026-04-24 | 1.0.0        | pm MAJOR 4건 + okta MAJOR 2건 + MINOR 11건 전면 반영. team-lead M5 결정 (PRD §7.7이 에러 매핑 소스 오브 트루스) 반영. v1.0으로 승격. | tui-designer |
| 2026-04-24 | 1.1.0        | **Phase 6d 시각 충실도 사이클.** §15 Renderable Reference Specs(Lip Gloss 토큰/컴포넌트/보더/컬럼 매핑) · §16 Golden Snapshots 12개(NO_COLOR 모드, 5 list + 1 detail + 3 overlay + 3 상태) · §17 Error Surfacing 명세(PRD §7.7 8종 errorCode 표시 매트릭스) · §18 Testability Guide 추가. 와이어프레임/단축키 §0~§14는 변경 없음 — 시각 사양만 보강. | tui-designer-2 |

**v1.0 주요 반영 사항:**
- [pm M-1] SCR-011 탭 "Logs" → "Recent" (전역 Logs 화면과 네이밍 분리)
- [pm M-2] §2.4에 Policy 타입 카탈로그 외부화 규약 명시 (REQ-R04 AC-8)
- [pm M-3] Users `search` eventually consistent 경고 이중 노출 (SCR-010 Empty + SCR-902 Help)
- [pm M-4 + okta 보강] Preset "Group Rule Deactivations" warning 색상 토큰 적용
- [okta M-2] SCR-031 Group Rule Deactivate 배너 5포인트 불릿으로 강화, `ⓘ` → `⚠`
- [okta M-3] Large group 판정을 런타임 200명 초과 관찰로 확장 (OKTA_GROUP/APP_GROUP 포함)
- [okta M-4] Adaptive polling 발동 타이밍·토스트 명시
- [okta M-1, m1, m2, m3, m4, m5] 그 외 MINOR 전부 반영
- [pm MINOR 1~6, MINOR-7 "모달" 확정 승격] 전부 반영

---

## 0. 디자인 원칙 (ota 특화 규칙)

PRD와 도메인 입력으로부터 도출한 **강행 규칙**. 이후 모든 화면·단축키·상태 UI는 이 원칙에 부합해야 한다.

### 0.1. 운영자의 근육 기억 재활용
- **k9s 호환 우선, Vim 기본값**. 두 관례가 충돌하면 **Vim 우선** (리더 결정, PRD §11.3).
  - 예: `j/k`는 리스트 이동 (k9s와 동일). `gg/G`는 맨 위/아래 (Vim).
  - 리소스 전환은 k9s식 **`:` 커맨드**가 주 경로. lazygit식 `1/2/3` 숫자 탭은 **Nice-to-Have (MVP 제외)**.
- Vim이 아닌 에디터 컨텍스트(텍스트 입력, 검색 버퍼)는 **표준 readline 키** (`Ctrl-a/e/w/u`).

### 0.2. 읽기 전용 MVP — 모든 mutation은 "없는 것처럼"
- MVP는 Read-Only Administrator 토큰 가정 (PRD §4.2, §7.6).
- 삭제/비활성화/리셋 키는 **의도적으로 미배정**. v0.2 예약 (PRD §11.3).
- Help 화면에 "Write actions are not available in MVP" 배너 상시 노출.
- 단, **경고 배너는 읽기에서도 표시**: Group Rule 상세 화면 상단에 멤버십 제거 함정 경고 (PRD REQ-R03 AC-5, §4 SCR-031).

### 0.3. 80x24에서 살아남기
- 모든 리스트는 최소 크기(80x24)에서 **의도적 컬럼 드롭 순서**를 가진다.
- 드롭 순서는 각 화면 섹션에서 명시. 핵심 식별자는 절대 드롭 안 함 (User: login, Group: name).
- 상태 바는 1줄 고정, 헤더는 1~2줄 고정.

### 0.4. 상태는 숨기지 않는다
모든 화면은 다음 4가지 상태를 **반드시** 시각적으로 구분한다:
- **Loading** — 스피너 + `Esc` 취소 가능 안내
- **Empty** — 빈 상태 + 다음 액션 힌트
- **Error** — errorCode별 메시지 + 재시도 힌트 (`R`)
- **Rate-limited** — 카테고리별 배지 + "Paused / resuming in Ns"

### 0.5. PII는 기본 가림
- PRD §6.2 "PII 마스킹 정책"에 따라 `phoneNumber`, `secondEmail`, `mobilePhone`은 **기본 마스킹**.
- 해제는 `:unmask <field>` 커맨드로 **세션 한정**.
- Logs의 `actor.alternateId`(login email)는 **감사 가독성 우선으로 기본 평문** (설정 키 예약은 §7.3 참고).

### 0.6. Rate-Limit 인지는 있되 숫자는 낮게 노출
- `X-Rate-Limit-Remaining` 비율만 배지화 (정상/경고/위험 3단계).
- **절대 수치는 `:about` 또는 `:ratelimit`에서만 노출**하여 초심자가 "숫자에 집착"하지 않도록 한다.
- Adaptive polling 중에는 `[TAIL 15s · ADAPTIVE]` 단일 인디케이터. 기본 상태는 `[TAIL 7s]`만 (§4 SCR-050).

### 0.7. 색맹 친화 (듀얼 채널)
- 모든 상태는 **색상 + 기호** 두 채널로 표시.
- 예: `● ACTIVE` (green), `○ STAGED` (cyan), `⚠ LOCKED_OUT` (red), `✗ DEPROVISIONED` (gray).
- `NO_COLOR` 환경변수 존중 → monochrome + 기호만.

### 0.8. 확인은 타이핑, 경고는 배너, 토스트는 3초
- 위험 동작은 [확인 타이핑] 방식 (MVP에서는 `:unmask`만 해당, v0.2 Write 설계는 §10 참고).
- 영구적 경고(정책·규칙 비활성화 등 파급성)는 **배너** (화면 상단 고정).
- 일회성 알림(복사 완료, 필터 적용, 프로필 전환)은 **토스트** (상태바, 3초).

---

## 1. 레이아웃 시스템

### 1.1. 표준 영역 (80x24 최소)

```
┌─ Header ───────────────────────────────────────────────────────────────────┐
│ ota · <tenant-name> · <env-badge>         [RL: ok]  UTC  v0.1.0           │   <- Row 0
│ Users                                    42 of 1,205 · search: q="al"     │   <- Row 1 (context)
├─ Body ─────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   <main content: list / detail / form>                                     │   <- Row 2 .. N-2
│                                                                            │
├─ Status Bar ───────────────────────────────────────────────────────────────┤
│ <↑↓> nav  </> search  <:> cmd  <?> help  <q> back                         │   <- Row N-1
└────────────────────────────────────────────────────────────────────────────┘
```

**영역 설명:**

| 영역      | 높이  | 내용                                                                        |
|-----------|-------|-----------------------------------------------------------------------------|
| Header L1 | 1줄   | 제품명 · 테넌트 · 환경 배지 · Rate-Limit 배지 · TZ · 버전                   |
| Header L2 | 1줄   | 현재 리소스명 · 카운트/페이지 진행 · 현재 필터/검색 인수                    |
| Body      | 가변  | 리스트 / 상세 / 폼 / 모달 오버레이                                          |
| Status    | 1줄   | 주요 단축키 힌트 (화면별), 토스트 메시지, tail 인디케이터                   |

오버레이(커맨드 프롬프트, 도움말, 확인 다이얼로그)는 Body 위에 **모달**로 중첩. 뒷 배경은 어둡게 처리.

### 1.2. 반응형 규칙

| 터미널 폭 | 레이아웃 전략                                                                  |
|-----------|--------------------------------------------------------------------------------|
| < 80      | **지원 안 함**. 진입 시 "ota requires minimum 80x24 terminal" 안내 후 block. |
| 80~99     | 최소 모드. 컬럼 드롭 최대. Header L1이 다음 줄 wrap 금지(절단).                |
| 100~139   | 표준 모드. 대부분 화면 최적 렌더.                                              |
| 140+      | 확장 모드. 추가 컬럼 표시 (예: Users 리스트에 `department`).                   |
| 180+      | Wide 모드. **사이드 패널** 활용 (리스트 + 상세 프리뷰 동시). *v0.2 추가, MVP는 단일 패널.* |

**높이 반응형:**

| 터미널 높이 | 전략                                                                      |
|-------------|---------------------------------------------------------------------------|
| < 24        | 지원 안 함. 같은 안내.                                                   |
| 24~29       | Header L2 생략(컨텍스트는 L1으로 흡수). Status 그대로.                   |
| 30+         | 표준.                                                                    |

### 1.3. 컬럼 드롭 우선순위 (공통 규칙)

각 리스트 화면은 컬럼을 `[필수 | 중요 | 보조 | 선택]` 네 등급으로 분류하고, 폭 부족 시 `선택 → 보조 → 중요` 순으로 드롭한다. **필수는 절대 드롭 불가.**

- 드롭된 컬럼은 상세(`Enter`)에서 확인.
- 드롭 상태는 Header L2 우측에 `[-2 cols]` 표기로 투명하게.

---

## 2. 네비게이션 모델

### 2.1. 정보 아키텍처 (IA)

```
                    ┌─ Help (?)────┐
                    │  Global      │
                    │  Screen-spec │
                    └──────────────┘
                           ▲
                           │ ? 모달
                           │
(App Boot) ──▶ [Profile Select] ──▶ [Home / Users]
                  │ --profile 시 skip
                  │
                  ▼
             ┌─────────────────────────────────────────┐
             │ Resource Views (k9s-style, : commands)  │
             │                                         │
             │   :users       → Users list             │
             │                    └▶ User Detail       │
             │                         ├▶ Groups tab   │
             │                         ├▶ Factors tab  │
             │                         └▶ Recent tab   │
             │                                         │
             │   :groups      → Groups list            │
             │                    └▶ Group Detail      │
             │                         ├▶ Members tab  │
             │                         ├▶ Apps tab     │
             │                         └▶ Rules tab    │
             │                                         │
             │   :grouprules  → Group Rules list       │
             │                    └▶ Rule Detail       │
             │                         └▶ Targets tab  │
             │                                         │
             │   :policies    → Policy Type Select     │
             │                    └▶ Policies list     │
             │                         └▶ Policy Det.  │
             │                              └▶ Rules   │
             │                                         │
             │   :logs        → Logs search/tail       │
             │                    └▶ Log Event Detail  │
             │                                         │
             └─────────────────────────────────────────┘
                           ▲
                           │ :<resource>
                           │
              ┌─────────── any screen ───────────┐
              │  :about       :healthcheck       │
              │  :profile     :ratelimit         │
              │  :errors      :quit              │
              └──────────────────────────────────┘
```

### 2.2. 화면 ID 카탈로그

| ID        | 이름                         | 진입                                                  | 종결                 |
|-----------|------------------------------|-------------------------------------------------------|----------------------|
| SCR-000   | Profile Select (boot)        | 앱 기동 시 프로필 미지정 + multi-profile 설정 시      | 선택 후 SCR-010      |
| SCR-001   | Error Boot Screen            | 토큰 없음 / 잘못된 org_url / 네트워크 실패            | 종료                 |
| SCR-010   | Users List                   | `:users` / `:u` / `:` 팔레트에서                     | `q` → 앱 종료        |
| SCR-011   | User Detail                  | Users List에서 `Enter`                                | `Esc` → SCR-010      |
| SCR-020   | Groups List                  | `:groups` / `:g`                                      | `q`                  |
| SCR-021   | Group Detail                 | Groups List에서 `Enter`                               | `Esc`                |
| SCR-030   | Group Rules List             | `:grouprules` / `:gr`                                 | `q`                  |
| SCR-031   | Group Rule Detail            | Group Rules List에서 `Enter`                          | `Esc`                |
| SCR-040   | Policy Type Select (modal)   | `:policies`                                           | `Esc` / 선택         |
| SCR-041   | Policies List (within type)  | Type 선택 후, 또는 `:policies <TYPE>`                 | `Esc` → SCR-040      |
| SCR-042   | Policy Detail                | Policies List에서 `Enter`                             | `Esc`                |
| SCR-050   | Logs Search/Tail             | `:logs` / `:l`                                        | `q`                  |
| SCR-051   | Log Event Detail             | Logs에서 `Enter`                                      | `Esc`                |
| SCR-900   | Command Palette (overlay)    | `:` on any screen                                     | `Esc` / `Enter`      |
| SCR-901   | Search Prompt (overlay)      | `/` on any list                                       | `Esc` / `Enter`      |
| SCR-902   | Help (modal)                 | `?` on any screen                                     | `?` / `Esc` / `q`    |
| SCR-903   | Confirm Dialog (modal)       | 위험 액션 시 (MVP: `:unmask`)                        | `Esc` / typed confirm|
| SCR-904   | Error Detail (overlay)       | 에러 토스트 클릭 또는 `:errors`                       | `Esc`                |
| SCR-905   | About / RateLimit / Healthcheck | `:about` / `:ratelimit` / `:healthcheck`          | `Esc` / `q`          |
| SCR-910   | Quit Confirm                 | `:q` 또는 `Ctrl-c` (단발), tail 중                    | `y` → exit / `n`     |

### 2.3. Breadcrumb 표기

상세 화면에서 Header L2에 breadcrumb:

```
Users › alice@acme.com › Groups                              [-1 col]
```

- 구분자: ` › ` (U+203A)
- 탭 전환 시 마지막 조각만 변경.
- 너비 부족 시 중간 조각부터 `…`로 축약.

### 2.4. Policy 타입 카탈로그 외부화 (REQ-R04 AC-8)

**규약 (v1.0 추가, pm MAJOR-2 반영):**

- SCR-040 Policy Type Select 메뉴는 **내부 타입 카탈로그를 순회 렌더링**한다. 하드코딩 금지.
- 카탈로그 위치: Phase 4에서 `internal/domain/policies/catalog.go` 코드 상수 또는 `configs/policy_types.yaml` 로 외부화 (개발자 판단).
- 카탈로그 entry 스키마(개념):
  ```
  id:          "OKTA_SIGN_ON"       # API type string
  label:       "Global Session Policies"
  rendererMode: "rich" | "raw"       # rich=풀 렌더러 구현 / raw=JSON only
  rendererKey:  "okta_sign_on"       # rich 모드일 때 action summary 매퍼 참조
  enabled:     true                  # MVP 포함 여부
  ```
- **새 Policy 타입 추가 절차 (예: `CONTINUOUS_ACCESS` GA 시):**
  1. 카탈로그 entry 추가 (`enabled: true`, 초기 `rendererMode: "raw"`)
  2. 필요 시 rich 렌더러 함수 작성 후 `rendererMode: "rich"` + `rendererKey` 설정
  3. 테스트 tenant에서 응답 스키마 확인
  - TUI 설계 문서는 **수정 불요**. 타입 목록·메뉴·상세 라우팅 모두 카탈로그 구동.
- MVP 기본 카탈로그 (7종): `OKTA_SIGN_ON`, `ACCESS_POLICY`, `PASSWORD`, `MFA_ENROLL` (rich) + `PROFILE_ENROLLMENT`, `POST_AUTH_SESSION`, `IDP_DISCOVERY` (raw).

### 2.5. 뒤로가기 정책

- `Esc` — 가장 최근 진입 경로의 역방향 1단계 (Detail → List, Tab → 첫 Tab).
- `q` — 해당 화면 종결 (List에서는 앱 종료 확인, Detail에서는 상위 List).
- **예외:** 검색/프롬프트 모드에서는 `Esc` = 모드 종료만.

---

## 3. 글로벌 단축키 맵

### 3.1. 전역 활성화 (Context-Free)

모든 화면에서 동작. 단, 텍스트 입력 포커스 중에는 **숫자 아닌 키를 입력**하면 해당 키가 입력 버퍼로 흐른다.

| 키                        | 동작                                          | 설정 ID                    | 관례     |
|---------------------------|-----------------------------------------------|----------------------------|----------|
| `:`                       | 커맨드 팔레트 열기                            | `global.cmd_palette`       | k9s/Vim  |
| `/`                       | 인크리멘털 검색 (리스트만 활성)               | `global.search`            | Vim      |
| `?`                       | 도움말 모달                                   | `global.help`              | k9s/Vim  |
| `Esc`                     | 현재 모드/모달 취소 (1단계 뒤로)              | `global.cancel`            | 공통     |
| `q`                       | 현재 화면 닫기 (List→앱 종료 확인)            | `global.close`             | k9s/Vim  |
| `Ctrl-c`                  | 1회: 소프트 종료 (tail 중이면 확인). 연타: 즉시 종료 | `global.hard_quit`   | Unix     |
| `Ctrl-l`                  | 화면 강제 재렌더 (tmux resize 등 복구)        | `global.redraw`            | Unix     |
| `?` 내부 `/`              | 도움말 내 검색                                | `help.search`              | 관례     |

### 3.2. 전역 네비게이션 (List/Detail 공통)

| 키               | 동작                          | 설정 ID             | 관례 |
|------------------|-------------------------------|---------------------|------|
| `j` / `↓`        | 아래로 1행                    | `nav.down`          | Vim  |
| `k` / `↑`        | 위로 1행                      | `nav.up`            | Vim  |
| `h` / `←`        | 왼쪽 탭/컬럼                  | `nav.left`          | Vim  |
| `l` / `→`        | 오른쪽 탭/컬럼                | `nav.right`         | Vim  |
| `gg`             | 맨 위                         | `nav.top`           | Vim  |
| `G`              | 맨 아래                       | `nav.bottom`        | Vim  |
| `Ctrl-d`         | 반 페이지 아래                | `nav.half_down`     | Vim  |
| `Ctrl-u`         | 반 페이지 위                  | `nav.half_up`       | Vim  |
| `Ctrl-f`         | 한 페이지 아래                | `nav.page_down`     | Vim  |
| `Ctrl-b`         | 한 페이지 위                  | `nav.page_up`       | Vim  |
| `Enter`          | 선택 (List→Detail, Item 펼침) | `nav.select`        | 공통 |
| `Tab`            | 다음 탭                       | `nav.tab_next`      | 공통 |
| `Shift-Tab`      | 이전 탭                       | `nav.tab_prev`      | 공통 |
| `Home` / `0`     | 줄 처음 (wrap 시)             | `nav.line_home`     | 관례 |
| `End` / `$`      | 줄 끝                         | `nav.line_end`      | Vim  |

### 3.3. 전역 액션 (Observe 계열)

| 키          | 동작                                          | 설정 ID              | 관례/근거        |
|-------------|-----------------------------------------------|----------------------|------------------|
| `R`         | 현재 리소스 새로고침 (캐시 무효화)            | `action.refresh`     | k9s              |
| `r`         | 상세에서 raw JSON 토글 (Policies/Logs 전용)   | `action.toggle_raw`  | PRD REQ-R04 AC-6 |
| `y`         | 선택 항목 YAML/JSON 복사 (clipboard)          | `action.yank`        | Vim yank         |
| `yf`        | 현재 커서 필드만 복사                          | `action.yank_field`  | Vim 계열         |
| `yy`        | 전체 row 복사                                  | `action.yank_row`    | Vim 계열         |
| `o`         | Admin Console 링크 열기 (브라우저)            | `action.open_web`    | 일상 관례        |
| `e`         | 상세 항목 펼침/접힘 (Factors의 id 등)         | `action.expand`      | 관례             |
| `f`         | tail 자동 스크롤 on/off (Logs에서)            | `logs.follow`        | PRD REQ-R05 AC-3 |
| `s`         | tail 토글 (Logs)                              | `logs.tail_toggle`   | PRD UC-5         |
| `n` / `N`   | 검색 다음/이전 매치                           | `search.next/prev`   | Vim              |

### 3.4. 커맨드 팔레트 명령

`:` 프롬프트에서 입력. 탭 자동완성, 부분 매칭 (PRD REQ-U02 AC-2, AC-3).

| 명령                       | 동작                                                        | 근거 REQ               |
|----------------------------|-------------------------------------------------------------|------------------------|
| `:users` / `:u`            | Users 리스트로 전환                                          | REQ-U02 AC-1           |
| `:groups` / `:g`           | Groups 리스트로 전환                                         | REQ-U02 AC-1           |
| `:grouprules` / `:gr`      | Group Rules 리스트로 전환                                    | REQ-U02 AC-1           |
| `:policies [TYPE]`         | Policy Type 선택 or 지정 타입 직진 (예: `:policies OKTA_SIGN_ON`) | REQ-R04 AC-2           |
| `:logs` / `:l`             | Logs 검색/tail 화면                                          | REQ-U02 AC-1           |
| `:profile [name]`          | 프로필 리스트 조회 / 전환 (인자 있으면 즉시). 전환 시 "Switching to <name>… (invalidating cache)" 토스트 | REQ-C02 AC-3           |
| `:search <expr>`           | 현재 리소스 서버측 고급 검색 (SCIM). **Users: eventually consistent — 방금 만든 사용자는 분 단위 지연 가능** | REQ-U04 AC-2/AC-5      |
| `:filter <expr>`           | SCIM filter (Groups/Apps/Logs)                               | REQ-U04 AC-2           |
| `:unmask <field>`          | 세션 내 PII 필드 마스킹 해제                                 | PRD §6.2               |
| `:mask`                    | 현재 세션 unmask 전부 되돌림                                | §7.2                   |
| `:raw`                     | 상세 뷰에서 raw JSON 토글                                    | REQ-R04 AC-6           |
| `:refresh`                 | 현재 리소스 캐시 무효화 후 재로드                            | REQ-E01 AC-6           |
| `:about`                   | 앱/토큰/Rate Limit 현황 모달                                 | REQ-C04 AC-1           |
| `:ratelimit`               | Rate Limit 카테고리별 상세                                   | REQ-E01 AC-4           |
| `:errors`                  | 세션 에러 히스토리                                           | REQ-E02 AC-3           |
| `:healthcheck`             | tenant 연결성·토큰·rate limit 종합 모달 (토스트 아님)       | PRD §6.6 / v1.0 확정   |
| `:debug open`              | 디버그 로그 경로 안내 (파일 tail 대체). Help: "prints log path; use `tail -f` in another terminal" | REQ-O01 AC-4           |
| `:help` / `:?`             | Help 모달                                                    | REQ-U02 AC-1           |
| `:quit` / `:q`             | 종료 (tail 중이면 확인)                                      | REQ-U07                |

**히스토리:** 최근 50개 유지 (REQ-U02 AC-4). `↑/↓`로 커서. `Ctrl-r` reverse-search.

### 3.5. 충돌 검사

- `q` (전역 close) ↔ `q` 쿼리 파라미터와 혼동? — `q`는 **텍스트 검색 버퍼 안**에서만 문자. 외부에서는 닫기. 충돌 없음.
- `r` (raw toggle) ↔ `R` (refresh) — 대소문자 구분. 양쪽 다 관례 준수 (k9s의 R=refresh).
- `/` (검색) ↔ `?` (도움말) — Vim 관례. 혼동 없음.
- `s` (tail toggle) ↔ search 키? — Logs 전용. 다른 화면에서 `s`는 No-op(경고 토스트 "no action for `s` here").

### 3.6. 학습 부담 관리

- 각 화면에서 **Status Bar에 동시 노출되는 키는 최대 6개**.
- 나머지는 `?` 도움말에서 조회.
- Status Bar 노출 우선순위: `nav / select / search / cmd / help / close`.

---

## 4. 화면 카탈로그

각 화면은 다음 구조로 정의한다:
- 목적·진입 경로
- 와이어프레임 (80x24 기준 + wider variant)
- 화면 전용 단축키
- 상태별 표현 (Loading / Empty / Error / RateLimited / Offline)
- 전이 매트릭스
- Bubble 컴포넌트 매핑
- 근거 REQ-ID

---

### SCR-000: Profile Select (Boot)

**목적:** 여러 Okta 테넌트를 등록한 경우, 기동 시 어떤 프로필로 동작할지 선택 (PRD REQ-C02).

**진입 경로:**
- `ota` 실행 + 설정 파일에 2+ 프로필 존재 + `--profile` 미지정
- 단일 프로필은 건너뛰고 SCR-010 직행

**와이어프레임:**
```
┌─ ota · select profile ─────────────────────────────────────────────────────┐
│                                                                            │
│   Select a tenant profile to continue                                      │
│                                                                            │
│   > prod          acme.okta.com              env: prod    token: env OKTA │
│     preview       acme.oktapreview.com       env: test    token: env OKTA │
│     dev           dev-123456.okta.com        env: dev     token: prompt   │
│                                                                            │
│                                                                            │
│                                                                            │
│   No token configured for `dev` — will prompt on select.                   │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <↑↓> select  <Enter> connect  <e> edit config  <q> quit                    │
└────────────────────────────────────────────────────────────────────────────┘
```

**단축키:**

| 키        | 동작                                                         |
|-----------|--------------------------------------------------------------|
| `↑↓ j k`  | 프로필 커서 이동                                             |
| `Enter`   | 선택한 프로필로 연결 시도 (토큰 없으면 마스킹 프롬프트)      |
| `e`       | 설정 파일 경로 안내 (편집은 외부 에디터 — MVP는 경로 표시만) |
| `q Esc`   | 종료                                                         |

**상태별:**
- Empty (프로필 0개): "No profiles configured. Set `OKTA_ORG_URL` + `OKTA_API_TOKEN` or edit `~/.config/ota/config.yaml`." + 종료
- 토큰 프롬프트: 마스킹 입력 (`*` 표시), 메모리 한정 (REQ-C04 AC-2)

**전이:** `Enter` → SCR-010 (Users 홈). 인증 실패 시 SCR-001 (에러 부트 화면).

**Bubble 매핑:** `bubbles/list` (간단 선택) + `huh.Input` (토큰 프롬프트).

**근거:** REQ-C02 AC-2, AC-3; REQ-C04 AC-1, AC-2.

---

### SCR-001: Error Boot Screen

**목적:** 연결/토큰 실패 시 명확한 종료 + 가이드 (PRD REQ-C04 AC-3).

**와이어프레임:**
```
┌─ ota · connection error ───────────────────────────────────────────────────┐
│                                                                            │
│   ✗ Cannot connect to Okta                                                 │
│                                                                            │
│   profile:   prod (acme.okta.com)                                          │
│   cause:     E0000004 / 401 — API token invalid or revoked                 │
│                                                                            │
│   How to fix:                                                              │
│     1. Rotate your token in Admin Console (Security › API › Tokens)        │
│     2. Set the new value:                                                  │
│        export OKTA_API_TOKEN=<new-token>                                   │
│     3. Retry:   ota --profile prod                                         │
│                                                                            │
│   Docs:       https://developer.okta.com/docs/reference/api/…              │
│   Debug log:  ~/.cache/ota/debug.log (enabled with --debug)                │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ Press any key to exit                                                      │
└────────────────────────────────────────────────────────────────────────────┘
```

**상태별 (errorCode별 메시지, PRD §7.7 8종 전부 커버):**

| errorCode  | HTTP | 헤더 문구                             | 추가 안내                                         |
|------------|------|----------------------------------------|---------------------------------------------------|
| E0000001   | 400  | Validation failed                      | errorCauses 파싱 표시                              |
| E0000004   | 401  | API token invalid or revoked           | Rotate → retry (위 예시)                           |
| E0000006   | 403  | Insufficient permissions               | "Token may be Read-Only. Check `:about`."         |
| E0000007   | 404  | Resource not found                     | "Org URL incorrect? Check `OKTA_ORG_URL`."        |
| E0000011   | 401  | Token expired or revoked               | 위와 동일, "may be older than org retention"      |
| E0000022   | 400  | Resource in invalid state              | "Deactivate before deleting" (boot에서 드묾)      |
| E0000038   | 400  | Feature disabled for org               | "Contact Okta admin."                              |
| E0000047   | 429  | Rate limited on startup                | "Retry in Ns. Rare on boot."                      |
| NETWORK    | -    | Cannot reach Okta                      | "Check connectivity / proxy / firewall."          |
| DNS        | -    | DNS resolution failed                  | "Org URL may be incorrect."                       |

> **v0.2 재검토 예정 (오픈 이슈 §14):** `E0000054` (invalid attribute value), `E0000068` (invalid passcode/answer)는 Write 스코프 진입 시 별도 매핑 추가 검토.

**전이:** 키 입력 → exit 1.

**Bubble 매핑:** `bubbles/viewport` (정적 텍스트).

**근거:** REQ-C04 AC-3, AC-4; REQ-E02.

---

### SCR-010: Users List

**목적:** 조직의 사용자 탐색, 검색, 상세 진입 (PRD UC-1, REQ-R01).

**진입 경로:**
- 앱 부팅 (기본 홈)
- `:users` / `:u` / `:` 팔레트에서

**와이어프레임 (120x30, 표준 모드):**
```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.1.0 ─┐
│ Users                                        42 of 1,205  · q="al"         │
├────────────────────────────────────────────────────────────────────────────┤
│ STATUS          LOGIN                    DISPLAY NAME    LASTLOGIN  CHANGE│
│                                                                            │
│ > ● ACTIVE      alice@acme.com           Alice Smith     2h ago    14d    │
│   ● ACTIVE      alan.turing@acme.com     Alan Turing     1d ago    60d    │
│   ⚠ LOCKED_OUT  alex.lee@acme.com        Alex Lee        —         3m     │
│   ○ STAGED      amy.wong@acme.com        Amy Wong        —         1d     │
│   ✗ SUSPENDED   aaron.k@acme.com         Aaron K.        5d ago    5d     │
│   ⊘ DEPROV      alicia.old@acme.com      Alicia Old      45d ago   30d    │
│   ● ACTIVE      ang.m@acme.com           Ang Mei         17h ago   100d   │
│   …                                                                       │
│                                                                            │
│                                                                            │
│                                                                            │
│   Loading next page…                                                       │
│                                                                            │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <j k> nav  </> search  <:search> SCIM  <Enter> detail  <?> help  <q> back │
└────────────────────────────────────────────────────────────────────────────┘
```

**컬럼 (우선순위):**

| 컬럼          | 등급 | 폭 권장 | 드롭 조건   | 근거              |
|---------------|------|---------|-------------|-------------------|
| STATUS        | 필수 | 13      | 드롭 불가   | REQ-R01 AC-1/AC-2 |
| LOGIN         | 필수 | 28      | 드롭 불가   | REQ-R01 AC-1      |
| DISPLAY NAME  | 중요 | 18      | 폭 < 90 드롭| REQ-R01 AC-1      |
| LASTLOGIN     | 중요 | 10      | 폭 < 80 드롭| REQ-R01 AC-1      |
| STATUSCHANGED | 보조 | 8       | 폭 < 100 드롭| REQ-R01 AC-1     |
| DEPARTMENT    | 선택 | 12      | 폭 < 140 드롭 (확장 모드만) | PRD §10   |

**상태 아이콘 (색+기호 듀얼):**

| 상태              | 기호 | 색     | 근거                  |
|-------------------|------|--------|-----------------------|
| ACTIVE            | `●`  | green  | REQ-R01 AC-2          |
| PROVISIONED/STAGED| `○`  | cyan   | REQ-R01 AC-2          |
| SUSPENDED         | `✗`  | yellow | REQ-R01 AC-2, 혼동방지|
| LOCKED_OUT        | `⚠`  | red    | REQ-R01 AC-2          |
| PASSWORD_EXPIRED  | `◒`  | magenta| REQ-R01 AC-2          |
| DEPROVISIONED     | `⊘`  | gray   | REQ-R01 AC-2, 혼동방지|

> **중요:** SUSPENDED(`✗`/yellow)와 DEPROVISIONED(`⊘`/gray)는 기호도 색도 다름. 사용자 혼동의 가장 큰 원인이므로 Help에 1:1 비교표 포함 (§SCR-902, PRD §1.2, REQ-R01 AC-2).

**단축키 (화면 전용):**

| 키          | 동작                                                         | 근거              |
|-------------|--------------------------------------------------------------|-------------------|
| `Enter`     | 선택 사용자 상세 (SCR-011)                                  | REQ-R01           |
| `/`         | 클라이언트 인크리멘털 필터 (현재 페이지만)                  | REQ-U03           |
| `:search`   | 서버측 SCIM `search` (예: `status eq "ACTIVE"`)             | REQ-R01 AC-5      |
| `g`         | 선택 사용자의 Groups 탭 바로 (상세 생략)                    | PRD UC-1 플로우    |
| `L`         | 선택 사용자의 Recent 탭 바로 (recent events)                | PRD UC-2 플로우    |
| `R`         | 새로고침                                                    | REQ-E01 AC-6      |
| `y / yy`    | 선택 사용자 JSON 복사                                       | Nice-to-Have      |
| `o`         | Admin Console → 해당 user deep link                         | Nice-to-Have      |

**상태별 UI:**

**Loading (초기):**
```
│                                                                            │
│                                                                            │
│                   ⠋ Fetching users…                                        │
│                     GET /api/v1/users?limit=200                            │
│                     Press <Esc> to cancel                                  │
│                                                                            │
```

**Empty (필터 결과 0):**
```
│   No users match your filter.                                              │
│                                                                            │
│   Try:                                                                     │
│     /                    clear filter                                      │
│     :search status eq "SUSPENDED"    switch to SCIM search                 │
│     Note: `/` uses Okta `q` (free text). Use `:search` for fields.         │
│     Note: recently created users may take minutes to appear in search      │
│           (indexing lag — eventually consistent).                          │
```

**Error:**
```
│   ✗ Failed to load users                                                   │
│     E0000006 · 403 · Insufficient permissions for /users                   │
│     Token may be Read-Only + Admin role may lack user read scope.          │
│                                                                            │
│   <R> retry     <:about> token info     <:errors> history                  │
```

**Rate-limited:**
```
│   ⏸ Paused · Rate limited on /users                                        │
│     Resuming in 8s…                                                        │
│     Cached results shown below (age: 42s)                                  │
│                                                                            │
│   [ ... existing cached list ... ]                                         │
```
- 상태바: `[RL: ⚠ warn]` 또는 `[RL: ✗ limited]`

**Offline:**
```
│   ✗ offline — network unreachable                                          │
│     Cached data from 2m ago shown.                                         │
│   <R> retry when online                                                    │
```

**페이지네이션:**
- 하단에 "Loading next page…" 스피너 (Link 헤더 `rel="next"` 있을 때)
- 더 없으면 "End of list (1,205 total)" 표기 (카운트 가능한 경우)
- 사용자가 스크롤 하단에 도달 시 prefetch (백그라운드, PRD 비기능 §6.1)

**컬럼 드롭 시연 (80x24, 최소 모드):**
```
│ STATUS          LOGIN                         LASTLOG                      │
│ > ● ACTIVE      alice@acme.com                2h ago                       │
│   ⚠ LOCKED_OUT  alex.lee@acme.com             —                            │
│                                              [-3 cols · Enter for detail] │
```

**전이 매트릭스:**

| 시작 상태   | 입력             | 다음                                          |
|-------------|------------------|-----------------------------------------------|
| 일반        | `Enter`          | SCR-011 User Detail (Profile 탭)              |
| 일반        | `g`              | SCR-011 + Groups 탭 활성                      |
| 일반        | `L`              | SCR-050 Logs, 필터 `actor.id eq "{userId}"`   |
| 일반        | `/`              | 필터 모드 진입 (SCR-901)                      |
| 일반        | `:search ...`    | 서버측 SCIM 재쿼리, 결과 교체                 |
| Rate lim.   | 자동 복구        | 일반 상태                                     |
| 일반        | `q`              | Quit 확인 (SCR-910)                           |

**Bubble 매핑:**
- `bubbles/table` (정렬·스크롤 가능한 컬럼 리스트)
- `bubbles/textinput` (인크리멘털 `/` 필터)
- `bubbles/spinner` (로딩/rate-limit)
- 커스텀 delegate: status 아이콘+색상 렌더링

**근거 REQ:** REQ-R01 전부, REQ-U01, REQ-U03, REQ-U04 AC-1/AC-2/AC-5, REQ-E01, REQ-E02, REQ-E03.

---

### SCR-011: User Detail

**목적:** 사용자의 Profile/Credentials/Timestamps/Groups/Factors/Recent 탭 탐색.

**진입 경로:**
- SCR-010 Users List에서 `Enter`, `g`, `L` (탭 포커스 변경)

**탭 카운트 로딩 규약 (pm MINOR-3):**
- 진입 직후: `[ Groups … ]`, `[ Factors … ]` (데이터 대기)
- 실패 시: `[ Groups ? ]`, `[ Factors ? ]` (403 등)
- 로드 완료: `[ Groups 4 ]`, `[ Factors 2 ]` (실제 카운트)
- 0건: `[ Groups 0 ]`, `[ Factors 0 ]`

**와이어프레임 (120x30, Profile 탭):**
```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.1.0 ─┐
│ Users › alice@acme.com                                          id: 00u…x8 │
├────────────────────────────────────────────────────────────────────────────┤
│ [ Profile ] [ Credentials ] [ Timestamps ] [ Groups 4 ] [ Factors 2 ] [ Recent ] │
├────────────────────────────────────────────────────────────────────────────┤
│   login             alice@acme.com                                         │
│   email             alice@acme.com                                         │
│   firstName         Alice                                                  │
│   lastName          Smith                                                  │
│   displayName       Alice Smith                                            │
│   status            ● ACTIVE                                               │
│   mobilePhone       +1-***-***-1234       <- masked · `:unmask mobilePhone`│
│   secondEmail       a***@personal.com     <- masked                        │
│                                                                            │
│   — Custom fields ──────────────────────────                               │
│   department        Engineering                                            │
│   title             Senior SWE                                             │
│   costCenter        ENG-42                                                 │
│                                                                            │
│                                                                            │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <Tab> next tab  <y> copy  <o> admin console  <L> recent  <Esc> back        │
└────────────────────────────────────────────────────────────────────────────┘
```

> **탭 라벨 "Recent" (pm MAJOR-1 반영):** 전역 `:logs` 화면(SCR-050)과 네이밍 충돌을 피하기 위해 "Logs" → **"Recent"**. 본문에는 "Recent events for Alice (last 100 within 30d)" 유지. Help에서 "User → Recent 탭은 해당 사용자의 System Log 부분 조회; 전체 Logs는 `:logs`로"로 대비 안내.

**Factors 탭 (REQ-R01 AC-6):**

okta-expert m4 반영: vendorName 차이가 보이는 DUO 예시 추가.

```
│ [ Profile ] [ Credentials ] [ Timestamps ] [ Groups 4 ] [ Factors 3 ] [ Recent ] │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   ● Okta Verify (Push)                          ACTIVE    registered 180d  │
│     provider         OKTA / OKTA                                           │
│     deviceType       iPhone 14 Pro                                         │
│     name             Alice's iPhone                                        │
│     created          2025-10-30  lastUpdated  2026-04-02                   │
│     id               (e) expand                                            │
│                                                                            │
│   ● Duo Mobile (Push)                           ACTIVE    registered 90d   │
│     provider         OKTA / DUO         <- 3rd party (vendorName)          │
│     factorType       push                                                  │
│     created          2026-01-24  lastUpdated  2026-04-10                   │
│     id               (e) expand                                            │
│                                                                            │
│   ⚠ SMS                                         EXPIRED   registered 402d  │
│     provider         OKTA / OKTA                                           │
│     phoneNumber      +1-***-***-1234   <- masked · `:unmask phoneNumber`   │
│     created          2024-10-18  lastUpdated  2025-05-10                   │
│     id               (e) expand                                            │
│                                                                            │
│   No WebAuthn / TOTP / Email / Security Question factors.                  │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <e> expand/collapse  <y> copy factor  <Tab> next tab  <Esc> back           │
└────────────────────────────────────────────────────────────────────────────┘
```

**factor 상태 색상:** `ACTIVE` green · `PENDING_ACTIVATION` cyan · `EXPIRED` yellow · `DISABLED`/`NOT_SETUP` gray (REQ-R01 AC-6).

**factor type 렌더 힌트:**
- `push` / `token:software:totp` / `sms` / `call` / `webauthn` / `token:hardware` / `email` / `question`
- WebAuthn: `profile.credentialId`를 키 별칭으로 표시
- TOTP: profile 필드 거의 없음. factorType + status + created만.
- SMS/Voice: `profile.phoneNumber` 기본 마스킹 (§7.2).

**Groups 탭:**
```
│   Alice is a member of 4 groups.                                           │
│                                                                            │
│ > ◆ OKTA_GROUP   Engineering             (Rule: "Engineers to Eng")        │
│   ◆ OKTA_GROUP   All Employees                                             │
│   ◈ BUILT_IN     Everyone                    (system-wide, ~all users)     │
│   ▣ APP_GROUP    Jira Users              (synced from Jira)                │
│                                                                            │
│                                                                            │
│   <Enter> open group                                                       │
```

**Recent 탭:**
```
│   Recent events for Alice (last 100 within 30d)                            │
│                                                                            │
│ > 2h ago    INFO   user.session.start             SUCCESS    203.0.113.5   │
│   14h ago   INFO   user.session.end               SUCCESS    203.0.113.5   │
│   1d ago    WARN   user.session.start             FAILURE    198.51.100.2  │
│   …                                                                        │
│   <Enter> open log event · <:search actor.id eq "00u…x8" and …> refine    │
```

**단축키 (화면 전용):**

| 키            | 동작                                          | 근거                 |
|---------------|-----------------------------------------------|----------------------|
| `Tab / Shift-Tab` | 탭 순환 (좌→우, 우→좌)                        | REQ-U05              |
| `1~6`         | 탭 직접 이동 (1=Profile, 2=Credentials, ...)  | 관례                 |
| `e`           | (Factors) id/상세 펼침 토글                   | REQ-R01 AC-6         |
| `:unmask <f>` | PII 필드 세션 마스킹 해제                     | PRD §6.2             |
| `y`           | JSON 전체 복사                                | Nice-to-Have         |
| `yf`          | 커서 필드만 복사                              | Nice-to-Have         |
| `o`           | Admin Console → 해당 user                     | Nice-to-Have         |
| `L`           | Recent 탭으로 점프 (actor.id 필터 preset)     | PRD UC-2             |
| `Esc / q`     | Users 리스트로 복귀                           | 공통                 |

**상태별:**
- Loading (탭 전환 시): 탭 영역 중앙 스피너 "Loading groups…" (GET `/users/{id}/groups`)
- Empty Factors: "No MFA factors registered. User may be unable to satisfy MFA policies."
- Empty Groups: "Member of no groups (except possibly Everyone)."
- 403 on Groups: "Cannot read groups for this user (insufficient permissions)."
- 404 on User: "User not found or deleted. <R> refresh list"

**전이 매트릭스:**

| 시작     | 입력        | 다음                                             |
|----------|-------------|--------------------------------------------------|
| Profile  | `Tab`       | Credentials                                      |
| Any Tab  | `Esc / q`   | SCR-010                                          |
| Groups   | `Enter`     | SCR-021 Group Detail                             |
| Recent   | `Enter`     | SCR-051 Log Event Detail                         |
| Factors  | `e`         | 해당 factor id/profile 펼침                      |

**Bubble 매핑:**
- `bubbles/viewport` (탭 내용 스크롤)
- 커스텀 탭 바 (`lipgloss.JoinHorizontal` + 포커스 스타일)
- Groups 탭: `bubbles/list`
- Factors 탭: `bubbles/list` with custom delegate (펼침 상태 표현)

**근거:** REQ-R01 AC-3/AC-6/AC-7, REQ-U05, PRD §6.2, UC-2.

---

### SCR-020: Groups List

**목적:** 그룹 탐색, 타입 구분, 멤버/앱 드릴다운 (PRD REQ-R02).

**와이어프레임:**
```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.1.0 ─┐
│ Groups                                         18 of 18 · filter type=OKTA│
├────────────────────────────────────────────────────────────────────────────┤
│ TYPE    NAME                    DESCRIPTION              UPDATED   TAGS   │
│                                                                            │
│ > ◆     Engineering             All engineers            2h ago    RULE   │
│   ◆     Sales                   Sales team               1d ago           │
│   ◆     Finance                 Finance team             5d ago    RULE   │
│   ▣     Jira Users              Synced from Atlassian    3h ago           │
│   ▣     GSuite Admins           Synced from Google       1d ago           │
│   ◈     Everyone                All organization members 1m ago    SYS    │
│   ◆     Contractors             External contractors     7d ago           │
│   …                                                                       │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <j k> nav  </> search  <Enter> detail  <m> members  <R> refresh  <?>      │
└────────────────────────────────────────────────────────────────────────────┘
```

**타입 아이콘:**

| type         | 아이콘 | 설명                                           |
|--------------|--------|------------------------------------------------|
| `OKTA_GROUP` | `◆`    | 일반 그룹 (RULE 배지 있으면 동적)              |
| `APP_GROUP`  | `▣`    | 앱 동기화 그룹 (AD/LDAP/Jira 등)               |
| `BUILT_IN`   | `◈`    | 시스템 그룹 (Everyone 등), `SYS` 배지 같이 표시 |

**배지 (okta-expert M-3 반영):**

- `RULE` — 이 그룹을 타겟팅하는 Group Rule이 1개 이상 존재 (PRD REQ-R02 AC-1)
- `SYS` — BUILT_IN 타입
- `LARGE` — **런타임 멤버 로딩 중 200명 초과 관찰 시 자동 부착**. 초기 진입 시에는 없을 수 있으며, Members 탭을 열거나 상세 진입 시 점진적으로 감지. `BUILT_IN` 조건에 한정되지 않고 OKTA_GROUP/APP_GROUP에도 적용된다. (okta-expert M-3: 수만 명 그룹은 BUILT_IN이 아닌 OKTA_GROUP/APP_GROUP에 더 흔함)

**단축키:**

| 키       | 동작                                                    | 근거          |
|----------|---------------------------------------------------------|---------------|
| `Enter`  | Group Detail (SCR-021, Info 탭)                         | REQ-R02       |
| `m`      | Members 탭으로 바로 진입                                | REQ-R02 AC-3  |
| `a`      | Apps 탭으로 바로 진입                                   | REQ-R02 AC-4  |
| `/`      | 클라이언트 필터 (현재 페이지)                           | REQ-U03       |
| `:filter type eq "OKTA_GROUP"` | SCIM filter                               | REQ-R02 AC-2  |
| `R`      | 새로고침                                                | REQ-E01 AC-6  |

**컬럼 드롭 우선순위:** TAGS(선택) → UPDATED(보조) → DESCRIPTION(중요). TYPE/NAME은 필수.

**상태별 (대용량 그룹 경고, REQ-R02 AC-3 + okta-expert M-3):**

**판정 기준 (v1.0 명확화):**

| 조건 | 라벨 / 배너 |
|------|-------------|
| `type == "BUILT_IN"` | `◈` + `SYS` 배지, 항상 large-membership 배너 |
| `type == "BUILT_IN" && profile.name == "Everyone"` | 위 + "all organization members" 추가 라벨 |
| 기타 (OKTA_GROUP / APP_GROUP), 멤버 로딩 중 200명 초과 관찰 시 | 런타임 `LARGE` 배지 자동 부착 + "Large group — may contain thousands" 배너로 업그레이드 |
| 나머지 | 배너 없이 일반 progressive loading |

**Everyone 선택 후 `m`:**
```
│ Groups › Everyone › Members                                                │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   ⚠ This is a system-wide group (BUILT_IN).                                │
│     All organization members — membership count may be tens of thousands.  │
│     Paginated load will take time. Press <Esc> to stop at any point.       │
│                                                                            │
│   Loading: 400 members so far…                                             │
│                                                                            │
│   > alice@acme.com                ACTIVE                                   │
│     bob@acme.com                  ACTIVE                                   │
│     …                                                                      │
```

**OKTA_GROUP "All Employees" members 탭 (런타임 LARGE 감지):**
```
│ Groups › All Employees › Members                                           │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   Loading: 205 members so far… <LARGE detected>                            │
│                                                                            │
│   ⚠ Large group — may contain thousands. Press <Esc> to stop.              │
│                                                                            │
│   > alice@acme.com      ACTIVE                                             │
│     bob@acme.com        ACTIVE                                             │
│     …                                                                      │
```

**기타 그룹 members 탭 (200명 미만, 일반):**
```
│   Loading: 42 members so far…        <Esc> stop                            │
│                                                                            │
│   > alice@acme.com      ACTIVE                                             │
│     …                                                                      │
│                                                                            │
│   End of members (87).                                                     │
```

**Apps 탭 (403 권한 부족):**
```
│   Apps assigned to Engineering                                             │
│                                                                            │
│   ✗ Cannot read app assignments for this group.                            │
│     E0000006 · 403 · Insufficient permissions for /groups/{id}/apps        │
│     (Read-Only Admin may lack app read scope for your org.)                │
│                                                                            │
│   App count: —                                                             │
```

**Bubble 매핑:** `bubbles/table` + custom delegate (아이콘/배지, LARGE 동적 추가).

**근거:** REQ-R02 전부, REQ-U03, REQ-U04.

---

### SCR-021: Group Detail

**목적:** 그룹의 Info/Members/Apps/Rules 탭 탐색.

**와이어프레임 (Info):**
```
│ Groups › Engineering                                          id: 00g…x3  │
├────────────────────────────────────────────────────────────────────────────┤
│ [ Info ] [ Members … ] [ Apps … ] [ Rules … ]                              │
├────────────────────────────────────────────────────────────────────────────┤
│   name                 Engineering                                         │
│   description          All engineers                                       │
│   type                 OKTA_GROUP                                          │
│   objectClass          okta:user_group                                     │
│   created              2024-03-01 09:22:15 UTC                             │
│   lastUpdated          2h ago                                              │
│   lastMembershipUpdated 1h ago                                             │
│                                                                            │
│   Targeted by Group Rules:                                                 │
│     • "Engineers to Eng group" (0pr…a1)  ACTIVE                            │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <Tab> next tab  <m> members  <y> copy  <o> admin  <Esc> back               │
└────────────────────────────────────────────────────────────────────────────┘
```

> 탭 카운트는 진입 후 로드 완료 시 `… → 숫자`로 채움 (pm MINOR-3).

**Members 탭:**
```
│   42 members (loaded)                                                      │
│                                                                            │
│ > alice@acme.com       ACTIVE     Senior SWE              3h ago           │
│   bob@acme.com         ACTIVE     Staff Engineer          1d ago           │
│   …                                                                        │
│                                                                            │
│   <Enter> open user · <L> recent events of selection                       │
```

**Rules 탭 (이 그룹을 타겟으로 하는 룰):**
```
│   Group Rules targeting this group: 1                                      │
│                                                                            │
│ > ● ACTIVE   Engineers to Eng group                                        │
│     expression:  user.department == "Engineering"                          │
│     <Enter> open rule                                                      │
```

**단축키:**

| 키     | 동작                                |
|--------|-------------------------------------|
| `Tab`  | 탭 순환                             |
| `1~4`  | 탭 직접 이동                        |
| `Enter`| 선택 항목 진입 (user, rule, app)   |
| `Esc`  | SCR-020 복귀                        |
| `y`    | Group JSON 복사                     |

**근거:** REQ-R02 AC-3/AC-4.

---

### SCR-030: Group Rules List

**목적:** 동적 그룹 규칙 탐색. **INVALID 상태가 즉시 눈에 띄어야 함** (PRD REQ-R03 AC-2).

**와이어프레임:**
```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.1.0 ─┐
│ Group Rules                                    5 of 5                      │
├────────────────────────────────────────────────────────────────────────────┤
│ STATUS          NAME                      TARGETS            UPDATED      │
│                                                                            │
│ > ● ACTIVE      Engineers to Eng          Engineering        2h ago       │
│   ● ACTIVE      Managers to Managers      Managers           1d ago       │
│   ○ INACTIVE    Legacy Eng Mapping        Engineering        30d ago      │
│   ⚠ INVALID     Broken Dept Rule          Sales              3h ago       │
│   ● ACTIVE      Contractors Gate          Contractors,Extern 7d ago       │
│                                                                            │
│   ⚠ 1 rule in INVALID state — expression cannot be evaluated by Okta.      │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <j k> nav  </> search  <Enter> detail  <i> filter INVALID  <R> refresh    │
└────────────────────────────────────────────────────────────────────────────┘
```

**상태 아이콘:**

| status   | 아이콘 | 색상     |
|----------|--------|----------|
| ACTIVE   | `●`    | green    |
| INACTIVE | `○`    | gray     |
| INVALID  | `⚠`    | **red**  |

> PRD REQ-R03 AC-2: INVALID는 빨간색 + 경고 기호. 리스트 하단에 INVALID 카운터 배너.

**단축키:**

| 키      | 동작                                                       |
|---------|------------------------------------------------------------|
| `Enter` | Rule Detail (SCR-031)                                     |
| `i`     | INVALID만 필터 (토글)                                      |
| `a`     | ACTIVE만 필터 (토글)                                       |
| `/`     | 클라이언트 필터                                           |
| `R`     | 새로고침                                                  |

**컬럼 드롭:** UPDATED(보조) → TARGETS(중요). STATUS/NAME은 필수.

**Bubble 매핑:** `bubbles/table`.

**근거:** REQ-R03 AC-1/AC-2/AC-6.

---

### SCR-031: Group Rule Detail

**목적:** 룰 상세 (expression, 타겟, 조건). **비활성화 경고 배너 필수** (REQ-R03 AC-5).

**와이어프레임 (ACTIVE rule):**

배너 강화 적용 (okta-expert M-2 반영). `ⓘ` → `⚠`, 1줄 → 5포인트 불릿.

```
│ Group Rules › Engineers to Eng group                          id: 0pr…a1  │
├────────────────────────────────────────────────────────────────────────────┤
│ [ Overview ] [ Expression ] [ Targets ] [ Raw JSON ]                       │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   ⚠ Deactivating this rule removes group memberships it granted.           │
│                                                                            │
│     • Rule-based members of the target group(s) will lose membership.      │
│     • Users with another rule producing the same membership are unaffected.│
│     • Re-activation is NOT instant: Okta re-evaluates (may take minutes).  │
│     • Downstream policies / app assignments depending on group membership  │
│       will also change immediately. Verify access impact first.            │
│     • This action is disabled in read-only mode (MVP).                     │
│                                                                            │
│   status                ● ACTIVE                                           │
│   name                  Engineers to Eng group                             │
│   type                  group_rule                                         │
│   created               2024-05-12 10:00 UTC                               │
│   lastUpdated           2h ago                                             │
│                                                                            │
│   Expression                                                               │
│   ┌──────────────────────────────────────────────────────────────────────┐ │
│   │ user.department == "Engineering"                                     │ │
│   └──────────────────────────────────────────────────────────────────────┘ │
│                                                                            │
│   Target groups (1):                                                       │
│     → Engineering (00g…x3)                                                 │
│                                                                            │
│   Excluded users: none                                                     │
│   Excluded groups: none                                                    │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <Tab> next  <r> raw  <w> soft-wrap  <Enter> open target  <Esc> back        │
└────────────────────────────────────────────────────────────────────────────┘
```

> **배너 색상:** 아이콘 `⚠` yellow (warning), 테두리/강조 영역은 `styleWarning` 토큰 사용. 읽기 모드에서도 동일 배너 유지 — v0.2 Write 도입 시 배너 컴포넌트를 그대로 재사용하고 이중 확인 UX (§10 L3) 추가. (PRD REQ-R03 AC-5 "경고 배너 재사용")

**INVALID rule 상태 (PRD REQ-R03 AC-2):**
```
│                                                                            │
│   ⚠ INVALID — Okta cannot evaluate this expression.                        │
│     The rule is saved but cannot activate. Cause details are not           │
│     available via API. Check Admin Console or rewrite the expression.      │
│                                                                            │
│   Expression                                                               │
│   ┌──────────────────────────────────────────────────────────────────────┐ │
│   │ user.department == "Engineering" && Convert.unknown(user.x)         │ │
│   └──────────────────────────────────────────────────────────────────────┘ │
│   (highlighted in red — full expression shown in monospace)               │
```

**단축키:**

| 키    | 동작                                  |
|-------|---------------------------------------|
| `Tab` | 탭 순환                               |
| `r`   | Raw JSON 토글                         |
| `w`   | Expression soft-wrap 토글 (긴 표현식) |
| `y`   | JSON 복사                             |
| `Enter`| 타겟 그룹 열기 (포커스가 target일 때) |
| `Esc` | SCR-030 복귀                          |

**Id 해소 (REQ-R03 AC-4):**
- `actions.assignUserToGroups.groupIds`의 id는 **백그라운드 조회**로 name 치환 후 표시.
- 조회 실패 시: `→ (name unavailable) 00g…x9`
- name 캐시 TTL 30초.

**Bubble 매핑:** `bubbles/viewport` (긴 expression scroll), 커스텀 탭 바.

**근거:** REQ-R03 AC-3/AC-4/AC-5.

---

### SCR-040: Policy Type Select (modal)

**목적:** `:policies`는 타입 선택 필수 (PRD REQ-R04 AC-2). 카탈로그 외부화 기반 (§2.4).

**와이어프레임 (모달 오버레이):**
```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.1.0 ─┐
│ <dimmed background>                                                        │
│                                                                            │
│         ╔═══════════════════════════════════════════════════════╗          │
│         ║  Select Policy Type                                   ║          │
│         ╠═══════════════════════════════════════════════════════╣          │
│         ║                                                       ║          │
│         ║  > OKTA_SIGN_ON         Global Session Policies       ║          │
│         ║    ACCESS_POLICY        Authentication Policies (app) ║          │
│         ║    PASSWORD             Password Policies             ║          │
│         ║    MFA_ENROLL           MFA Enrollment Policies       ║          │
│         ║    PROFILE_ENROLLMENT   (raw view)                    ║          │
│         ║    POST_AUTH_SESSION    (raw view)                    ║          │
│         ║    IDP_DISCOVERY        (raw view)                    ║          │
│         ║                                                       ║          │
│         ║  ⓘ `(raw view)` types show JSON only; no rich render. ║         │
│         ║    Basic fields (name/priority/status/system/lastUpd) ║         │
│         ║    are still shown — only conditions/actions require  ║         │
│         ║    raw JSON mode.                                     ║         │
│         ║                                                       ║          │
│         ║  <↑↓> select  <Enter> load  <Esc> cancel              ║          │
│         ╚═══════════════════════════════════════════════════════╝          │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ Rendering 4 of 7 types fully; 3 types as raw JSON (see PRD).              │
└────────────────────────────────────────────────────────────────────────────┘
```

> **okta-expert m2 반영:** 모달 본문에 Basic fields 안내 추가.

**단축키:**

| 키       | 동작                                 |
|----------|--------------------------------------|
| `↑↓ j k` | 타입 이동                            |
| `Enter`  | 선택 타입으로 SCR-041                |
| `Esc`    | 취소 (이전 화면 복귀)                |

**힌트:**
- `(raw view)` 배지는 `rendererMode == "raw"`인 카탈로그 entry에 자동 부착 (§2.4).
- 카탈로그에 `enabled: false`로 등록된 타입(예: 현재 `ENTITY_RISK`)은 메뉴에 나타나지 않음.

**Bubble 매핑:** `huh` Select 또는 `bubbles/list` (간단 모드).

**근거:** REQ-R04 AC-1/AC-2, §2.4 카탈로그 외부화.

---

### SCR-041: Policies List (within type)

**와이어프레임:**
```
│ Policies › OKTA_SIGN_ON                         3 of 3                     │
├────────────────────────────────────────────────────────────────────────────┤
│ PRI  STATUS     NAME                            SYSTEM  UPDATED            │
│                                                                            │
│ > 1   ● ACTIVE   Default Policy                  SYS     never             │
│   2   ● ACTIVE   Require MFA for admins          -       2d ago            │
│   3   ○ INACTIVE Legacy Contractor Rule          -       90d ago           │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <Enter> detail  <h> change type  </> search  <R> refresh                  │
└────────────────────────────────────────────────────────────────────────────┘
```

**컬럼:** priority · status · name · system (배지) · lastUpdated (PRD REQ-R04 AC-3).

> **SYS 배지 렌더링 규약 (pm MINOR-5):** `system == true`인 정책은 SYSTEM 컬럼에 `SYS` 배지를 배경 색상(`styleBadgeSys`)으로 표시. 일반 정책은 `-`. 이 규약은 SCR-042 Overview의 `system` 필드와도 일관.

**단축키:**

| 키      | 동작                                            |
|---------|-------------------------------------------------|
| `Enter` | Policy Detail (SCR-042)                        |
| `h`     | 타입 선택 모달 재오픈 (SCR-040)                |
| `Esc`   | SCR-040                                        |
| `/`     | 클라이언트 필터                                |

**근거:** REQ-R04 AC-3.

---

### SCR-042: Policy Detail

**목적:** 정책 상세 + Rules 탭. **4 타입은 rich, 3 타입은 raw-only** (카탈로그 기반, §2.4).

**와이어프레임 (OKTA_SIGN_ON, rich 렌더링):**
```
│ Policies › OKTA_SIGN_ON › Default Policy                      id: 00p…xa  │
├────────────────────────────────────────────────────────────────────────────┤
│ [ Overview ] [ Rules 3 ] [ Raw JSON ]                                      │
├────────────────────────────────────────────────────────────────────────────┤
│   name                 Default Policy                                      │
│   type                 OKTA_SIGN_ON                                        │
│   priority             1                                                   │
│   status               ● ACTIVE                                            │
│   system               SYS (default — cannot be deactivated)               │
│   lastUpdated          never                                               │
│                                                                            │
│   Conditions                                                               │
│     applies to         all users                                           │
│                                                                            │
│   Settings                                                                 │
│     (no direct settings — see Rules)                                       │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <Tab> rules  <r> raw  <1~3> tabs  <y> copy  <Esc> back                     │
└────────────────────────────────────────────────────────────────────────────┘
```

**Rules 탭 (ACCESS_POLICY 예시, action 요약 렌더):**
```
│ [ Overview ] [ Rules 3 ] [ Raw JSON ]                                      │
├────────────────────────────────────────────────────────────────────────────┤
│ PRI  STATUS     NAME                    ACTION SUMMARY          UPDATED   │
│                                                                            │
│ > 1   ● ACTIVE   Admins require MFA      Require MFA (HW key)    3d ago   │
│   2   ● ACTIVE   Internal network        Allow w/o MFA           10d ago  │
│   3   ● ACTIVE   Catchall                Deny                    never    │
│                                                                            │
│                                                                            │
│ <Enter> rule detail · <r> raw · <Esc> back                                 │
```

**Action Summary 매퍼 (REQ-R04 AC-5, rendererKey 기반):**

| Policy type    | Summary 예시                                                        |
|----------------|--------------------------------------------------------------------|
| `ACCESS_POLICY`| "Require MFA (HW key)" / "Allow w/o MFA" / "Deny"                  |
| `OKTA_SIGN_ON` | "Session: 8h idle / 24h max · MFA required"                        |
| `PASSWORD`     | "min 12 · age 90d · history 10"                                    |
| `MFA_ENROLL`   | "required: Okta Verify, WebAuthn"                                  |

**raw-only 타입 상세 (PROFILE_ENROLLMENT):**
```
│ [ Overview ] [ Rules N ] [ Raw JSON ]                                      │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   ⓘ Rich view not yet available for PROFILE_ENROLLMENT.                    │
│     Press <r> or <:raw> for JSON pretty-print.                             │
│                                                                            │
│   Basic fields:                                                            │
│     name           Default Self-Service Profile                            │
│     priority       1                                                       │
│     status         ● ACTIVE                                                │
│     system         SYS                                                     │
│                                                                            │
```

**Raw JSON 뷰 (PRD REQ-R04 AC-6):**
```
│ [ Overview ] [ Rules 3 ] [ Raw JSON ]                                      │
├────────────────────────────────────────────────────────────────────────────┤
│ {                                                                          │
│   "id": "00p…xa",                                                          │
│   "type": "OKTA_SIGN_ON",                                                  │
│   "name": "Default Policy",                                                │
│   "priority": 1,                                                           │
│   "status": "ACTIVE",                                                      │
│   "system": true,                                                          │
│   "conditions": { … },                                                     │
│   "settings": { … }                                                        │
│ }                                                                          │
│                                                                            │
│ <j k> scroll  <y> copy  <r> back to rich  <Esc> back                       │
```

**단축키:**

| 키    | 동작                                            |
|-------|-------------------------------------------------|
| `Tab` | 탭 순환                                        |
| `1~3` | 탭 직접 이동                                    |
| `r`   | raw/rich 토글 (rich 지원 타입만)               |
| `y`   | JSON 복사                                      |
| `Enter`| Rules 탭에서 rule 선택 시 Rule 상세 (인라인)   |

**근거:** REQ-R04 AC-4/AC-5/AC-6/AC-7.

---

### SCR-050: Logs Search / Tail

**목적:** System Log 검색, 필터, tail 모드 (PRD UC-2, UC-5, REQ-R05).

**와이어프레임 (tail off, history 모드):**
```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.1.0 ─┐
│ Logs · since 24h · DESC                         1,024 loaded              │
│ filter: eventType eq "user.session.start" and outcome.result eq "FAILURE" │
├────────────────────────────────────────────────────────────────────────────┤
│ WHEN         SEV   EVENTTYPE                ACTOR              OUTCOME   IP│
│                                                                            │
│ > 2h ago     INFO  user.session.start       alice@acme.com     FAILURE   …│
│   3h ago     INFO  user.session.start       bob@acme.com       FAILURE   …│
│   7h ago     WARN  user.session.start       alice@acme.com     FAILURE   …│
│   1d ago     INFO  user.session.start       unknown@acme.com   FAILURE   …│
│   …                                                                        │
│                                                                            │
│   Loading next page…                                                       │
│                                                                            │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <s> tail  <f> follow  <Enter> detail  <P> presets  </>q  <:filter>         │
└────────────────────────────────────────────────────────────────────────────┘
```

**와이어프레임 (tail on, 일반):**
```
│ Logs · tail · since now-5m · 7s interval             [TAIL 7s] ▶           │
│ filter: eventType sw "user."                                               │
├────────────────────────────────────────────────────────────────────────────┤
│ WHEN         SEV   EVENTTYPE                ACTOR              OUTCOME   IP│
│                                                                            │
│ > just now   INFO  user.session.start       alice@acme.com     SUCCESS   …│   <- new, highlight
│   3s ago     INFO  user.authentication.au…  alice@acme.com     SUCCESS   …│
│   12s ago    INFO  user.session.end         bob@acme.com       SUCCESS   …│
│   30s ago    WARN  user.session.start       unknown            FAILURE   …│
│   …                                                                        │
│                                                                            │
│   ▲ 2 new events (press <f> to auto-follow)                                │
├────────────────────────────────────────────────────────────────────────────┤
│ <s> stop  <f> follow  <Enter> detail  <P> presets  <Esc> clear filter      │
└────────────────────────────────────────────────────────────────────────────┘
```

**와이어프레임 (tail on, adaptive 발동):**
```
│ Logs · tail · since now-5m · 15s interval     [TAIL 15s · ADAPTIVE] ▶      │
```

**Tail 인디케이터 규약 (pm MINOR-6 + okta MINOR-m5 통합):**

- **기본(7초, 일반 테넌트):** `[TAIL 7s]` 단일 표기. `ADAPTIVE: no` 같은 잡음 정보는 **표시하지 않음**.
- **Adaptive 발동(15초, 저한도 테넌트):** `[TAIL 15s · ADAPTIVE]` 단일 표기. 색상은 **시안(`styleInfo`)** — 경고가 아닌 정상 동작임을 나타냄 (pm MINOR-6).
- **Paused (429):** `[TAIL ⏸] · resuming in Ns` (red, `styleDanger`).
- **발동 타이밍 (okta MAJOR-4):**
  ```
  첫 폴링 응답 수신 직후 X-Rate-Limit-Limit 관찰. 60 이하면 즉시 인터벌을
  15초로 상향하고, 상태바에 1회성 토스트 "Adaptive polling enabled
  (low rate-limit tenant detected — switched to 15s)"를 2초 표시.
  첫 호출 이전에는 기본 7초로 진행 (안전 마진 우선).
  ```

**컬럼:**

| 컬럼       | 등급 | 폭   | 드롭 조건     | 근거             |
|------------|------|------|---------------|------------------|
| WHEN       | 필수 | 10   | 불가          | REQ-R05 AC-1     |
| SEV        | 필수 | 5    | 불가          | REQ-R05 AC-1     |
| EVENTTYPE  | 필수 | 26   | 불가          | REQ-R05 AC-1     |
| ACTOR      | 중요 | 22   | 폭<100 축약   | REQ-R05 AC-1     |
| OUTCOME    | 중요 | 8    | 폭<90 드롭    | REQ-R05 AC-1     |
| IP/GEO     | 보조 | 16   | 폭<110 드롭   | REQ-R05 AC-1     |

**severity 색상 (REQ-R05 AC-1):**

| severity | 색상    | 기호 |
|----------|---------|------|
| DEBUG    | gray    | `·`  |
| INFO     | green   | `ℹ`  |
| WARN     | yellow  | `!`  |
| ERROR    | red     | `✗`  |

**actor.type 아이콘:** `User` → (없음). `SystemPrincipal` → `⚙` 아이콘 앞에 붙임 (REQ-R05 AC-8).

**단축키:**

| 키          | 동작                                                              |
|-------------|-------------------------------------------------------------------|
| `s`         | tail on/off 토글                                                  |
| `f`         | 자동 스크롤(follow) on/off                                        |
| `P`         | Preset 필터 메뉴 열기                                             |
| `Enter`     | Log Event Detail (SCR-051)                                        |
| `:filter <expr>` | SCIM filter 입력                                             |
| `:since <dur>` | since 파라미터 변경 (예: `:since 24h`)                          |
| `:set tz=local` | TZ 토글 (REQ-R05 AC-7)                                         |
| `:set tz=utc`   | UTC                                                              |
| `y`         | 선택 이벤트 JSON 복사                                             |
| `/`         | 클라이언트 필터 (텍스트 substring)                                |
| `R`         | 강제 리프레시                                                     |

**Preset 메뉴 (`P`, REQ-R05 AC-5):**

pm MAJOR-4 + okta-expert 요청 반영 — "Group Rule Deactivations" 항목을 warning 토큰(`styleWarning`, yellow)으로 전체 렌더링.

```
│      ╔═══════════════════════════════════════════════════╗                 │
│      ║  Log Filter Presets                               ║                 │
│      ╠═══════════════════════════════════════════════════╣                 │
│      ║  > 1  Failed Sign-ins (24h)                       ║                 │
│      ║    2  Group Rule Changes                          ║                 │
│      ║    3  ⚠ Group Rule Deactivations                  ║   <- warning    │
│      ║       (may remove memberships)                    ║      styled row │
│      ║    4  API Token Activity                          ║                 │
│      ║    5  MFA Challenges                              ║                 │
│      ║                                                   ║                 │
│      ║  <1-5> load · <Enter> load selected · <Esc> cancel║                │
│      ╚═══════════════════════════════════════════════════╝                 │
```

> **색상 규약:** Preset 3 항목 전체 행(아이콘+라벨+부연)에 `styleWarning` 토큰 적용. 다른 항목은 기본 `styleFG`. 선택 하이라이트는 위 토큰 위에 `styleAccent` 오버레이.

**상태별:**

**tail 복구:**
```
│   ⏸ Paused (rate limited) · resuming in 6s…                                │
│   since: 2026-04-24T12:34:55Z (no data loss on resume)                     │
```
- 복구 시 같은 `since`로 재개, 데이터 구멍 없음 (REQ-R05 AC-3).

**Empty:**
```
│   No events match your filter in the selected time window.                 │
│                                                                            │
│   Try:                                                                     │
│     :since 7d                     expand time window                       │
│     :filter <simpler>             relax filter                             │
│     <P> preset                    load preset                              │
│                                                                            │
│   ⓘ Logs may lag a few seconds behind real-time events.                    │
```

**Boundary note (REQ-R05 AC-4):**
```
│   Reached end of retained logs (plan-dependent, ~90-180 days).             │
```

**Bubble 매핑:**
- `bubbles/table` (가상 스크롤, 대량 처리)
- `bubbles/spinner` (tail pulse + loading)
- `bubbles/textinput` (filter inline)

**근거:** REQ-R05 전부, REQ-E01 AC-3/AC-5.

---

### SCR-051: Log Event Detail

**목적:** 단일 로그 이벤트 상세 + 관련 리소스 점프.

**와이어프레임:**
```
│ Logs › 2026-04-24 10:32:14Z · user.session.start                            │
├────────────────────────────────────────────────────────────────────────────┤
│ [ Structured ] [ Raw JSON ]                                                │
├────────────────────────────────────────────────────────────────────────────┤
│   — Event ────────────────────                                             │
│   published       2026-04-24T10:32:14.567Z                                 │
│   eventType       user.session.start                                       │
│   legacyEventType core.user_auth.login_success                             │
│   severity        ℹ INFO                                                   │
│   displayMessage  User login to Okta                                       │
│   uuid            9e3…f2                                                   │
│                                                                            │
│   — Actor ──────────────────                                               │
│   type            User                                                     │
│   id              00u…x8  (press <U> to open user)                         │
│   alternateId     alice@acme.com                                           │
│   displayName     Alice Smith                                              │
│                                                                            │
│   — Client ─────────────────                                               │
│   ipAddress       203.0.113.5                                              │
│   userAgent       Chrome · Mac OS X                                        │
│   geo             US · California · San Francisco                          │
│   zone            OFF_NETWORK                                              │
│                                                                            │
│   — Target ─────────────────                                               │
│   (no targets)                                                             │
│                                                                            │
│   — Outcome ────────────────                                               │
│   result          SUCCESS                                                  │
│   reason          —                                                        │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ <Tab> raw  <U> open user  <T> open target  <y> copy  <Esc> back            │
└────────────────────────────────────────────────────────────────────────────┘
```

> **okta-expert m1 플래그:** `debugContext.debugData` 섹션(렌더링 시 별도 섹션으로 확장)의 free-form JSON에는 `email_address`, `phone_number` 등 PII가 포함될 수 있음. Phase 4 실전 로그 관찰 후 추가 마스킹 룰 필요 여부 재검토. 현재 MVP는 평문 표시. §7.4 노트 참고.

**단축키:**

| 키    | 동작                                                   |
|-------|--------------------------------------------------------|
| `Tab` | Structured/Raw 토글                                    |
| `r`   | Raw JSON 토글 (같은 효과)                              |
| `U`   | actor.id 기반 User Detail 열기 (REQ-R05 AC-6)          |
| `T`   | target[0].id 기반 리소스 열기 (User/Group/App)        |
| `y`   | JSON 복사                                              |
| `Esc` | SCR-050                                                |

**근거:** REQ-R05 AC-6/AC-7/AC-8/AC-9.

---

### SCR-900: Command Palette (overlay)

**목적:** `:` 프롬프트로 모든 명령 접근 (PRD REQ-U02).

**와이어프레임 (오버레이):**
```
├────────────────────────────────────────────────────────────────────────────┤
│ : use_                                                                     │
│   ▸ :users          switch to Users                                        │
│     :unmask         unmask a PII field                                     │
│                                                                            │
│   <Tab> complete · <↑↓> history · <Enter> run · <Esc> cancel              │
└────────────────────────────────────────────────────────────────────────────┘
```

**동작:**
- `Tab` 자동완성
- `↑↓` 히스토리 (세션 간 저장, 50개, REQ-U02 AC-4)
- `Ctrl-r` reverse-search
- 부분 매칭 (`:u` → `:users` 후보 상위)
- 무효 명령: "unknown command: `:xyz` — try `:help`"

**Bubble 매핑:** `bubbles/textinput` + 커스텀 자동완성 드롭다운.

---

### SCR-901: Search Prompt (overlay)

**목적:** `/`로 리스트 클라이언트 측 인크리멘털 필터.

**와이어프레임:**
```
├────────────────────────────────────────────────────────────────────────────┤
│ / ali|                            (3 matches · \C case-sensitive)          │
└────────────────────────────────────────────────────────────────────────────┘
```

**단축키:**

| 키       | 동작                                            |
|----------|-------------------------------------------------|
| `Enter`  | 확정, 프롬프트 닫고 필터 유지                   |
| `Esc`    | 취소, 필터 해제                                 |
| `n / N`  | (확정 후) 다음/이전 매치                        |
| `\C`     | 대소문자 구분 토글 (토글 표시 우측)             |

**근거:** REQ-U03 AC-2/AC-3/AC-4.

---

### SCR-902: Help (modal)

**목적:** 현재 화면 컨텍스트 + 글로벌 단축키 참조 (PRD REQ-U06).

**와이어프레임 (Screen 탭, Users List 컨텍스트):**
```
│      ╔═══════════════════════════════════════════════════════════╗         │
│      ║  Help · Users List                              / search  ║         │
│      ╠═══════════════════════════════════════════════════════════╣         │
│      ║                                                           ║         │
│      ║   [ Screen ] [ Global ] [ Commands ] [ Status icons ]     ║         │
│      ║                                                           ║         │
│      ║   Navigation                                              ║         │
│      ║     j, ↓            down one row                          ║         │
│      ║     k, ↑            up one row                            ║         │
│      ║     gg              top                                   ║         │
│      ║     G               bottom                                ║         │
│      ║     Ctrl-d / Ctrl-u half page                             ║         │
│      ║                                                           ║         │
│      ║   Actions                                                 ║         │
│      ║     Enter           user detail                           ║         │
│      ║     g               jump to Groups tab                    ║         │
│      ║     L               jump to Recent events tab             ║         │
│      ║     R               refresh (invalidate cache)            ║         │
│      ║                                                           ║         │
│      ║   Search                                                  ║         │
│      ║     /               client filter (case-insensitive)      ║         │
│      ║     :search <expr>  server SCIM search                    ║         │
│      ║     ⚠ Users: eventually consistent — recent creations     ║         │
│      ║       may not appear for minutes                          ║         │
│      ║                                                           ║         │
│      ║   ⓘ Write actions (delete/suspend/...) are not available  ║         │
│      ║     in MVP. See roadmap.                                  ║         │
│      ║                                                           ║         │
│      ║   <Tab> switch tab · </> filter help · <?> close · <q>    ║         │
│      ╚═══════════════════════════════════════════════════════════╝         │
```

**탭:**
- Screen — 현재 화면 전용 키
- Global — 전역 키
- Commands — 커맨드 팔레트 명령 목록
- Status icons — 아이콘/색상 범례 (색맹 대응 Help, 0.7 원칙)

**Status icons 탭 (pm MINOR-3 + okta-expert m3 통합 비교표):**

```
│      ║  User Status Reference                                    ║         │
│      ║                                                           ║         │
│      ║  Status          │Icon│Login│Data   │Revert        │Note ║         │
│      ║  ──────────────────────────────────────────────────────── ║         │
│      ║  ACTIVE          │ ●  │ yes │kept   │—             │norm ║         │
│      ║  SUSPENDED       │ ✗  │ no  │kept   │unsuspend OK  │temp ║         │
│      ║  DEPROVISIONED   │ ⊘  │ no  │kept   │reactivate(*) │off  ║         │
│      ║  DELETED         │ —  │ no  │REMOVED│NONE          │hidden from list │
│      ║  LOCKED_OUT      │ ⚠  │ no  │kept   │unlock OK     │auto ║         │
│      ║  PASSWORD_EXPIRED│ ◒  │ no  │kept   │reset OK      │     ║         │
│      ║  STAGED/PROVISION│ ○  │ no  │kept   │activate      │new  ║         │
│      ║                                                           ║         │
│      ║  (*) DEPROVISIONED → reactivate requires fresh tokens/    ║         │
│      ║      sessions (existing ones are invalidated).             ║        │
│      ║  DELETED users are excluded from default list responses.  ║         │
│      ║                                                           ║         │
│      ║  Log Severity                                             ║         │
│      ║    · DEBUG  ℹ INFO(green)  ! WARN(yellow)  ✗ ERROR(red)  ║         │
```

**Commands 탭 (pm MINOR-2 + MAJOR-3 이중 노출):**
```
│      ║  :debug open       prints log path. Use `tail -f` in       ║        │
│      ║                    another terminal to watch live.         ║        │
│      ║  :search <expr>    server-side SCIM search (Users/Groups)  ║        │
│      ║                    ⚠ Users: eventually consistent — recent ║        │
│      ║                    creations may not appear for minutes    ║        │
│      ║                    (indexing lag).                         ║        │
│      ║  :profile <name>   switch tenant profile (<2s reset).      ║        │
```

**단축키 내부:**
- `/` — Help 내부 검색
- `?` 또는 `Esc` — 닫기
- `Tab` — 탭 순환

**커스텀 바인딩 표시 (REQ-C03 AC-1):**
```
│      ║   g  ↦  user-detail-groups-tab   (default)                ║
│      ║   m  ↦  user-detail-groups-tab   (override: ~/.config/… )║
```

**근거:** REQ-U06 AC-1/AC-2/AC-3, pm MAJOR-3, pm MINOR-2, okta m3.

---

### SCR-903: Confirm Dialog (modal)

**목적:** 위험 동작 확인. **MVP는 Write 없음 → 현재 활용처는 `:unmask` 정도**.

**와이어프레임 (unmask PII):**
```
│       ╔════════════════════════════════════════════════════╗                │
│       ║  Unmask PII field · mobilePhone                    ║                │
│       ╠════════════════════════════════════════════════════╣                │
│       ║                                                    ║                │
│       ║  This will reveal the full value on screen for     ║                │
│       ║  the current session. Others looking at your       ║                │
│       ║  terminal will see it.                             ║                │
│       ║                                                    ║                │
│       ║  Type `unmask` to confirm · <Esc> cancel           ║                │
│       ║                                                    ║                │
│       ║  > _                                               ║                │
│       ║                                                    ║                │
│       ╚════════════════════════════════════════════════════╝                │
```

**패턴 규약 (v0.2 Write 대비 설계, §10 참조):**

| 위험 수준 | 확인 방식                         | 예시                                |
|-----------|-----------------------------------|-------------------------------------|
| L1 낮음   | `y/n` 단일 키                     | unmask (MVP)                        |
| L2 중간   | 단어 타이핑 (`yes` / `confirm`)   | group 멤버 제거 (v0.2)              |
| L3 높음   | 리소스 이름 타이핑 (rm -rf 수준) | group rule deactivate (v0.2 이후)   |

**근거:** 0.8 원칙, PRD §11.3 Write v0.2 대비.

---

### SCR-904: Error Detail / Session Errors (overlay)

**목적:** 토스트로 스치는 에러의 풀 메시지 + 세션 내 에러 히스토리 (PRD REQ-E02 AC-3).

**와이어프레임:**
```
│      ╔═══════════════════════════════════════════════════════════╗         │
│      ║  Session Errors (5)                                       ║         │
│      ╠═══════════════════════════════════════════════════════════╣         │
│      ║                                                           ║         │
│      ║   3m ago   E0000047  429   /logs · rate limited · retry OK║        │
│      ║   8m ago   E0000007  404   /users/00u…xz · refreshing…    ║        │
│      ║   12m ago  E0000006  403   /groups/{id}/apps · no scope   ║        │
│      ║   42m ago  NETWORK   -     DNS lookup failed · retried    ║        │
│      ║   1h ago   E0000001  400   /users search · bad filter     ║        │
│      ║                                                           ║        │
│      ║   <Enter> view detail · <y> copy · <x> clear · <Esc> back ║        │
│      ╚═══════════════════════════════════════════════════════════╝         │
```

**Bubble 매핑:** `bubbles/list`.

**근거:** REQ-E02 AC-3.

---

### SCR-905: About / RateLimit / Healthcheck

**About (`:about`):**
```
│      ╔═══════════════════════════════════════════════════════════╗         │
│      ║  ota                                                       ║        │
│      ╠═══════════════════════════════════════════════════════════╣        │
│      ║   version       0.1.0-dev (commit abcdef1)                ║        │
│      ║   build         2026-04-24T12:00:00Z                      ║        │
│      ║                                                           ║        │
│      ║   Tenant                                                  ║        │
│      ║     profile     prod                                      ║        │
│      ║     org_url     https://acme.okta.com                     ║        │
│      ║     token       env OKTA_API_TOKEN                        ║        │
│      ║     token age   ~68 days (best-effort estimate)           ║        │
│      ║                                                           ║        │
│      ║   Rate limits (last observed)                             ║        │
│      ║     admin       ok    586/600   resets in 34s   2s ago    ║        │
│      ║     logs        ok    112/120   resets in 42s   18s ago   ║        │
│      ║     policies    —     not yet observed                    ║        │
│      ║     apps        —     not yet observed                    ║        │
│      ║                                                           ║        │
│      ║   Adaptive polling                                        ║        │
│      ║     logs tail   7s (default)                              ║        │
│      ║                                                           ║        │
│      ║   PII masking                                             ║        │
│      ║     enabled     yes (default ON)                          ║        │
│      ║     logs.actor  not masked (default; see config)          ║        │
│      ║                                                           ║        │
│      ║   <Esc> close                                             ║        │
│      ╚═══════════════════════════════════════════════════════════╝         │
```

**`:ratelimit`** — 위 Rate limits 섹션 확장 (카테고리별 `X-Rate-Limit-*` 원 숫자, 관찰 시각, 7일 이력 스파크라인 — Nice-to-Have).

> AC-4 중요 한계 표기 (REQ-E01): 각 카테고리 값은 해당 카테고리 최근 호출의 관찰값("last observed"). 오래된 값은 gray.

**`:healthcheck` (pm MINOR-7 "모달" 확정 승격):**

```
│      ╔═══════════════════════════════════════════════════════════╗         │
│      ║  Health check · prod                                      ║         │
│      ╠═══════════════════════════════════════════════════════════╣        │
│      ║                                                           ║        │
│      ║   Connectivity                                            ║        │
│      ║     ✓ DNS resolves         acme.okta.com                  ║        │
│      ║     ✓ HTTPS handshake      200 ms                         ║        │
│      ║     ✓ Base URL reachable   GET /api/v1/org 200            ║        │
│      ║                                                           ║        │
│      ║   Authentication                                          ║        │
│      ║     ✓ Token valid          GET /api/v1/users/me 200        ║       │
│      ║     ✓ Role check           Read-Only Administrator         ║       │
│      ║                                                           ║        │
│      ║   Rate limits                                             ║        │
│      ║     ✓ admin     586/600   (97%)                           ║        │
│      ║     ✓ logs      112/120   (93%)                           ║        │
│      ║                                                           ║        │
│      ║   <Esc> close · <y> copy report                           ║        │
│      ╚═══════════════════════════════════════════════════════════╝         │
```

**근거:** REQ-C04 AC-1/AC-5, REQ-E01 AC-4, PRD §6.6, v1.0 MINOR-7 확정.

---

### SCR-910: Quit Confirm

**와이어프레임:**
```
│       ╔══════════════════════════════════════════════════════╗              │
│       ║  Quit ota?                                           ║              │
│       ║                                                      ║              │
│       ║  ⚠ Log tail is active.                               ║              │
│       ║    Stopping now will end polling.                    ║              │
│       ║                                                      ║              │
│       ║  <y> quit · <n> cancel · <Esc> cancel               ║              │
│       ╚══════════════════════════════════════════════════════╝              │
```

- tail/pending request 없으면 `q`는 **즉시 종료** (확인 없음).
- tail 중에는 확인. `Ctrl-c` 연타 시 보호 무시 (REQ-U07 AC-1).

**근거:** REQ-U07.

---

## 5. 컴포넌트 카탈로그 (Bubble 매핑)

각 화면에서 쓰는 Bubbletea 생태계 컴포넌트의 일관된 매핑.

| 디자인 개념                 | 1차 선택                          | 보조                                |
|----------------------------|-----------------------------------|-------------------------------------|
| 스크롤 리스트 (소규모)      | `bubbles/list` + custom delegate | —                                   |
| 테이블 (컬럼 기반)          | `bubbles/table`                   | 커스텀 렌더 (아이콘/배지)           |
| 장문 조회 (JSON, detail)    | `bubbles/viewport`                | —                                   |
| 인라인 텍스트 입력 (`/` 검색)| `bubbles/textinput`               | —                                   |
| 커맨드 팔레트 (`:`)         | `bubbles/textinput` + 커스텀 자동완성 | (v0.2) `huh.Input`              |
| 프로필 선택 폼              | `huh.Select`                      | `bubbles/list`                      |
| 토큰 마스킹 프롬프트        | `huh.Input` (EchoMode=Password)  | —                                   |
| 확인 다이얼로그             | `huh.Confirm` (단순)              | 커스텀 모달 (단어 타이핑)           |
| 로딩 스피너                 | `bubbles/spinner` (dot/line 조합) | —                                   |
| 진행 표시 (페이지네이션)    | `bubbles/progress`                | 텍스트 ("N loaded")                 |
| 도움말 힌트 바              | `bubbles/help`                    | 수동 렌더 (프로젝트 특화 포맷)      |
| Markdown 렌더 (Help 본문)   | `glamour` (호출)                  | 텍스트만 (fallback)                 |
| 스타일 (모든 곳)            | `lipgloss`                        | —                                   |

### 5.1. tea.Model 구조 제안 (개발자 참고)

```
app.Model
├── profileSelect   (SCR-000)
├── errorBoot       (SCR-001)
├── mainRouter      (SCR-010~050 분기)
│     ├── users.ListModel  / users.DetailModel
│     ├── groups.ListModel / groups.DetailModel
│     ├── rules.ListModel  / rules.DetailModel
│     ├── policies.TypeSelectModel / policies.ListModel / policies.DetailModel
│     │      (policyTypeCatalog 주입, §2.4)
│     └── logs.ListModel   / logs.DetailModel
├── overlay
│     ├── cmdPalette  (SCR-900)
│     ├── searchPrompt(SCR-901)
│     ├── help        (SCR-902)
│     ├── confirm     (SCR-903)
│     ├── errorsLog   (SCR-904)
│     └── about       (SCR-905: about / ratelimit / healthcheck)
└── statusBar / header (공통)
```

**Async 이벤트 (tea.Cmd):**
- `fetchResource(kind, query)` → `resourceLoaded{...}` / `resourceError{...}`
- `tickTail(interval)` → `tailPoll{since}`
- `rateLimitObserved{category, remaining, limit, reset}` → 상태바 업데이트
- `adaptivePollingToggled{enabled, newInterval}` → 1회성 토스트 + interval 변경
- `clipboardCopy(content)` → `toastMsg{"copied"}`
- `profileSwitched{name}` → "Switching to <name>… (invalidating cache)" 토스트 + 전체 상태 리셋 (pm MINOR-1)
- `groupMemberCountObserved{groupID, count}` → 200 초과 시 `largeBadgeAdded{groupID}` (okta M-3)

---

## 6. 색상 · 타이포 가이드

### 6.1. 테마 (Lip Gloss 토큰)

**기본: 다크 테마** (k9s 기본 팔레트 유사, PRD §11.3 리더 결정).

| 토큰                 | Lip Gloss Color (ANSI 256 + truecolor)       | 용도                                         |
|----------------------|-----------------------------------------------|----------------------------------------------|
| `styleBG`            | `#0b0f14` (truecolor) / `235` (256)          | 전체 배경                                    |
| `styleFG`            | `#d8dee9` / `253`                             | 기본 텍스트                                  |
| `styleMuted`         | `#5c6a7a` / `243`                             | 2차 정보, 비활성                             |
| `styleHeader`        | `#88c0d0` / `109` (cyan)                      | Header L1 제목                               |
| `styleAccent`        | `#81a1c1` / `110` (blue)                      | 하이라이트, 포커스, 선택 row                 |
| `stylePrimary`       | `#5e81ac` / `67`                              | 주요 버튼/액션 (거의 사용 안 함, MVP read-only) |
| `styleSuccess`       | `#a3be8c` / `108` (green)                     | ACTIVE, SUCCESS                              |
| `styleWarning`       | `#ebcb8b` / `179` (yellow)                    | SUSPENDED, WARN, 대용량 경고, **Group Rule Deactivations preset** (pm MAJOR-4) |
| `styleDanger`        | `#bf616a` / `167` (red)                       | LOCKED_OUT, ERROR, INVALID, Rate limit, Paused tail |
| `styleInfo`          | `#88c0d0` / `109` (cyan)                      | STAGED/PROVISIONED, INFO logs, **Adaptive polling 인디케이터** (pm MINOR-6) |
| `styleMagenta`       | `#b48ead` / `139`                             | PASSWORD_EXPIRED                             |
| `styleBadgeSys`      | `#4c566a` bg / `styleFG`                      | SYS 배지 (BUILT_IN, system=true policies)    |
| `styleBadgeRule`     | `#a3be8c` bg / black                          | RULE 배지 (동적 그룹)                        |
| `styleBadgeLarge`    | `#ebcb8b` bg / black                          | LARGE 배지 (런타임 감지, okta M-3)           |
| `styleBadgeUnmask`   | `#bf616a` bg / white, bold `[M!]`             | unmask 상태 경고                             |

### 6.2. 고대비 / Monochrome

- **high-contrast**: `styleBG=#000000`, `styleFG=#ffffff`, 색상은 그대로 유지하되 굵기 강화.
- **monochrome** (`NO_COLOR` 감지): 색 제거, 기호만 사용. 포커스는 `reverse video`로.

### 6.3. 타이포

- 모든 텍스트는 **터미널 폰트에 의존**. 별도 스타일 없음.
- `Bold`만 제한적 사용 (Header, 포커스 row, 에러).
- `Italic` 금지 (터미널 폰트별 렌더 불안).

### 6.4. 기호 사전

위 화면들에서 쓰인 유니코드 기호 일람. 모든 쉘·터미널에서 렌더 확인 (kitty/alacritty/iterm2/tmux 검증 대상).

| 용도           | 기호 | codepoint | fallback (monochrome) |
|----------------|------|-----------|------------------------|
| ACTIVE         | ●    | U+25CF    | `[+]`                  |
| STAGED         | ○    | U+25CB    | `[-]`                  |
| SUSPENDED      | ✗    | U+2717    | `[X]`                  |
| LOCKED_OUT     | ⚠    | U+26A0    | `[!]`                  |
| PASSWORD_EXP   | ◒    | U+25D2    | `[~]`                  |
| DEPROVISIONED  | ⊘    | U+2298    | `[/]`                  |
| INFO log       | ℹ    | U+2139    | `[i]`                  |
| WARN log       | !    | U+0021    | `[!]`                  |
| SystemPrincipal| ⚙    | U+2699    | `[S]`                  |
| Breadcrumb sep | ›    | U+203A    | `>`                    |
| Target arrow   | →    | U+2192    | `->`                   |
| Play/Tail      | ▶    | U+25B6    | `>>`                   |
| Pause          | ⏸    | U+23F8    | `\|\|`                 |
| New events up  | ▲    | U+25B2    | `^`                    |
| Divider        | ─    | U+2500    | `-`                    |
| Border corner  | ╔╗╚╝ | U+2554etc | `+ + + +`              |

> NO_COLOR 또는 `--ascii-fallback` 플래그 시 위 fallback 사용.

---

## 7. PII 마스킹 시각화

### 7.1. 마스킹 대상 (PRD §6.2)

| 필드                  | 리소스          | 마스킹 포맷              |
|-----------------------|-----------------|--------------------------|
| `profile.mobilePhone` | User profile    | `+1-***-***-1234`        |
| `profile.secondEmail` | User profile    | `a***@example.com`       |
| factor.profile.phoneNumber | User Factors (SMS/Voice) | `+1-***-***-1234` |
| factor.profile.email       | User Factors (Email)     | `a***@example.com`|

### 7.2. 시각 규약

- **기본 (마스킹 on):** 원본 값 대신 마스크 표시. 문자 색상은 평소 텍스트와 동일.
- **unmask 후:** 원본 값 표시 + 우측에 `[M!]` 빨간 배지 (`styleBadgeUnmask`).
- **복사 (`y`):**
  - 마스킹 on 상태에서 `y` → 마스킹된 값 복사 (보안 기본).
  - unmask 후 `y` → 원본 복사.
- **자동 재마스킹:** 화면 전환, `:mask` 커맨드, 세션 종료, inactivity 60초 → 자동 재마스킹.

### 7.3. 설정으로 마스킹 제어 (REQ-C01 AC-3)

v1.0에 okta-expert M-1 반영: Logs `actor.alternateId` 마스킹 설정 키 **예약**.

```yaml
# ~/.config/ota/config.yaml
ui:
  pii_masking:
    enabled: true                     # 기본 true (보안 기본 ON)
    default_unmask_on_copy: false
    logs_actor_email: false           # (reserved) true로 설정 시 Logs의 actor.alternateId도 마스킹
                                      # 규제/엄격 조직용. MVP는 기본 false; 구현은 v0.2에 확정.
```

- `enabled: false`로 설정하면 모든 PII가 평문 표시. `:about`에 `pii masking: OFF (configured)` 경고 표기.
- `logs_actor_email: true`로 설정하면 SCR-050 리스트의 ACTOR 컬럼과 SCR-051 Actor 섹션의 alternateId가 `a***@acme.com`로 렌더링.

### 7.4. Logs에서 마스킹 (간접 영향)

- **MVP 기본:** Logs의 `actor.alternateId`(일반적으로 login/email)는 **평문 표시**. 감사 가독성 우선 (§0.5).
- **설정 토글 (§7.3):** `logs_actor_email: true`로 변경 시 마스킹.
- **Debug log 파일:** 항상 PII 마스킹 적용 (PRD §6.2, REQ-O01 AC-2).
- **`debugContext.debugData` (okta-expert m1 플래그):** free-form JSON으로 `email_address`, `phone_number` 등이 포함될 수 있음. **MVP는 평문**. Phase 4 실전 로그 관찰 후 추가 마스킹 룰 필요 여부 재검토. 필요 시 v0.2에 렌더 단계 필드 스캔 기반 마스킹 도입.

**근거:** PRD §6.2, REQ-R01 AC-6 (Factors phoneNumber), okta-expert M-1/m1.

---

## 8. 애니메이션 · 피드백

### 8.1. 스피너 (로딩)

- Bubbletea `spinner.Dot` (점 4개 회전): `⠋ ⠙ ⠹ ⠸`
- 대안: `spinner.Line` (`- \ | /`)
- NO_COLOR에서도 동작 (색 없이)

### 8.2. Tail pulse (저잡음 표시)

- Tail on 상태: Header L2 우측 `[TAIL 7s] ▶` (cyan `styleInfo` 유지).
- 새 이벤트 도착: 2초간 `styleAccent`로 flash → 복귀. 폰트 변화·박스 이동 금지 (과한 움직임 금지).
- 사용자가 follow(`f`) off 상태면 신규 이벤트를 상단에 누적하되 스크롤 안 함. 리스트 상단에 `▲ 2 new events` 인디케이터.
- **Adaptive 전환 시 (okta M-4):** 1회성 토스트 "Adaptive polling enabled (15s)" 2초간 상태바에 표시.

### 8.3. Rate-limited 애니메이션

- Paused 상태: `[TAIL ⏸] · resuming in 8s` 초 단위 카운트다운 (8→7→6...).
- 429 발생 시 한 번만 짧은 shake 효과 (Header L1 한 줄 flash) — **선택적** (과하면 제거).

### 8.4. 토스트

- 상태바 오른쪽에 텍스트로: `copied 1 row to clipboard` (녹색 short).
- 3초 후 자동 사라짐 (REQ-E02 AC-1).
- `Esc`로 즉시 제거.
- **`:profile` 전환 토스트 (pm MINOR-1):** `Switching to prod… (invalidating cache)` → 전환 완료 시 `Switched to prod`.

### 8.5. 포커스 이동

- 탭 전환: 하이라이트가 **즉시** 새 탭으로 이동. 애니메이션 없음 (60fps 체감, 비기능 §6.1).
- 리스트 커서: 키 입력마다 즉시 이동. 스크롤도 동기화.

---

## 9. 접근성 및 국제화

### 9.1. 색맹 친화 (원칙 0.7)

- 모든 상태 표시는 **색 + 기호** 이중 채널.
- 색상만으로 의미 전달 금지.
- 고대비 모드(config) + monochrome 모드(`NO_COLOR`) 지원.

### 9.2. 스크린리더

- **일반 TUI는 스크린리더 친화가 어렵다** — Bubbletea는 ANSI 이스케이프 기반.
- MVP 목표:
  - (a) `--plain` 모드: 애니메이션/박스 문자 없이 단순 텍스트 리스트 출력 후 종료 (read-only 조회에 유용)
  - (b) 주요 상태 변화를 터미널 알림(visual bell)로도 전달 (선택, Nice-to-Have)
- 본격 스크린리더 지원은 v0.3+ (Bubbletea upstream accessibility 패턴 수용 시).

### 9.3. 키보드 접근성

- **모든 기능은 키보드로 완결**. 마우스 지원 없음 (MVP).
- 한 손 접근성 예외: 치명적 키 조합(`Ctrl-Alt-Shift-x` 류) 금지.
- 확인 키는 일관: `Enter` 또는 단어 타이핑. 마우스 클릭 불필요.

### 9.4. 국제화 (MVP는 영어)

- 모든 UI 문구는 영어 고정 (PRD §6.4).
- 타임스탬프 포맷: UTC 기본 + 로컬 토글 (REQ-R05 AC-7).
- 날짜 표기: `2026-04-24` ISO. 상대 시각 `2h ago` 등은 영어 관용구.

### 9.5. 터미널 호환 (PRD §6.4)

- 검증 대상: `xterm-256color`, `tmux`, `kitty`, `alacritty`, `wezterm`, `iterm2`.
- Linux 콘솔 / macOS Terminal.app 최소 동작 (박스 문자 깨짐 감수, fallback ASCII).
- Windows는 WSL만 지원 (PRD §4.2).

### 9.6. 텍스트 크기 / 줌

- 터미널이 처리. 별도 크기 조절 없음.
- 작은 터미널 감지(< 80x24) 시 진입 차단 + 안내.

---

## 10. 위험 동작 확인 패턴 (v0.2 대비 설계)

MVP는 mutation이 없으므로 **현재는 `:unmask`만** 해당. 그러나 v0.2 Write를 위해 패턴을 예약한다.

### 10.1. 3단 확인 체계 (v0.2+)

| 단계 | 이름            | 용도                                    | UX                                         |
|------|-----------------|-----------------------------------------|--------------------------------------------|
| L1   | Soft confirm    | 되돌림 쉬운 액션, 저영향                | `y/n` 한 키                                |
| L2   | Word confirm    | 되돌림 가능하나 즉시 파급               | `yes` 또는 `confirm` 타이핑                |
| L3   | Name confirm    | 되돌림 불가 또는 대량 영향              | 리소스 이름 타이핑 (예: `engineering`)     |

**예시 매핑 (v0.2):**

| 액션                             | 단계 | 근거 (PRD §11.3)      |
|----------------------------------|------|-----------------------|
| unmask PII (MVP)                 | L1   | 세션 한정, 되돌림 쉬움 |
| Group 정적 멤버 추가             | L1   | 되돌림 쉬움            |
| Group 정적 멤버 제거             | L2   | 사용자 영향 즉각       |
| User unsuspend / unlock          | L2   | 되돌림 가능, 즉시 영향 |
| Group Rule deactivate            | L3   | 멤버십 대량 제거       |
| User suspend                     | L3   | 사용자 로그인 차단     |

### 10.2. 영향 범위 표시

위험 액션의 confirm 화면에는 **영향 범위를 수치로** 표시 (okta-expert M-2 함정 강조와 일관):

```
│   Deactivate rule "Engineers to Eng group"?                                │
│                                                                            │
│   This will remove the rule-based membership for an estimated N users      │
│   from the "Engineering" group. (Exact count cannot be retrieved via API.) │
│                                                                            │
│   • Users with another rule producing the same membership are unaffected.  │
│   • Re-activation is NOT instant (Okta re-evaluates, may take minutes).    │
│   • Downstream policies / app assignments depending on membership change.  │
│                                                                            │
│   Type `engineering` (target group name) to confirm · <Esc> cancel         │
```

### 10.3. 감사 로그 힌트

Write 액션 시 "This action will be recorded in Okta System Log as <eventType> by your admin identity." 안내 (도메인 §0.4).

---

## 11. REQ-ID 매핑 매트릭스

PRD v1.0.0의 각 REQ가 본 설계의 어느 화면/키/모달에서 충족되는지 추적.

> **자기 검증 (v1.0, 2026-04-24):** 21개 REQ (U01~U07, R01~R05, C01~C05, E01~E03, O01) 전부가 "충족 위치" 컬럼을 가진다. 초안(v0.1) 대비 충족 위치가 확장된 REQ: U04 (Empty+Help 이중 노출, pm MAJOR-3), U06 (Status icons 행동 차이 비교표, okta m3), R01 (Recent 탭 + DUO vendorName, pm MAJOR-1/okta m4), R02 (LARGE 런타임 감지, okta M-3), R03 (5포인트 배너, okta M-2), R04 (§2.4 카탈로그 외부화, pm MAJOR-2), R05 (ADAPTIVE 인디케이터 규약 + Preset warning 색상, pm MAJOR-4/okta M-4/m5), C02 (전환 토스트, pm MINOR-1), O01 (Help 보완, pm MINOR-2). 어떤 REQ도 AC가 추가/축소/의미 변경되지 않았다 — 전부 동일 AC를 **더 정확한 UX로 충족**하는 확장이다.

### 11.1. 공통 UX

| REQ       | 제목                     | 충족 위치                                                             |
|-----------|--------------------------|-----------------------------------------------------------------------|
| REQ-U01   | Vim 내비게이션           | §3.2 전역 네비 키; 모든 리스트/상세 (SCR-010/011/020/…)                |
| REQ-U02   | 커맨드 프롬프트 `:`       | SCR-900; §3.4 팔레트 명령 목록                                        |
| REQ-U03   | 인크리멘털 검색 `/`       | SCR-901; 각 리스트의 `/` 바인딩                                       |
| REQ-U04   | 서버측 검색 (`search`/`filter`) | `:search` / `:filter` 커맨드 (§3.4); Help Commands+Status icons 탭; SCR-010 Empty 힌트 (eventually consistent 이중 노출) |
| REQ-U05   | 드릴다운 (상세↔연관)      | SCR-011 탭, SCR-021 탭, SCR-031 Target, SCR-051 U/T 점프               |
| REQ-U06   | 도움말 `?`                | SCR-902 (4탭: Screen/Global/Commands/Status icons — 행동 차이 비교표) |
| REQ-U07   | 종료 보호                | SCR-910 Quit Confirm; Ctrl-c 연타 해제 (§3.1)                          |

### 11.2. 리소스별

| REQ     | 충족 위치                                                                                |
|---------|------------------------------------------------------------------------------------------|
| REQ-R01 | SCR-010 (리스트), SCR-011 (Profile/Credentials/Timestamps/Groups/**Factors**/Recent 탭 + DUO vendorName 시연) |
| REQ-R02 | SCR-020 (리스트 + RULE/SYS/LARGE 배지 런타임 감지), SCR-021 (Info/Members/Apps/Rules 탭) |
| REQ-R03 | SCR-030 (INVALID 배너 + 경고색), SCR-031 (강화 5포인트 경고 배너 + Expression monospace + Targets) |
| REQ-R04 | SCR-040 (타입 선택 모달, `(raw view)` 배지, Basic fields 안내), SCR-041 (리스트 + SYS 배지), SCR-042 (Overview/Rules/Raw JSON), §2.4 카탈로그 외부화 |
| REQ-R05 | SCR-050 (search/tail, `[TAIL 7s]`/`[TAIL 15s · ADAPTIVE]` 인디케이터, Preset 메뉴 with warning 색상), SCR-051 (Structured/Raw + U/T 점프) |

### 11.3. 설정 및 인증

| REQ     | 충족 위치                                                                          |
|---------|------------------------------------------------------------------------------------|
| REQ-C01 | 설정 파일 자체는 UI 없음. 파싱 에러는 SCR-001에서 표시. §7.3 yaml 예시              |
| REQ-C02 | SCR-000 Profile Select; `:profile` 팔레트 (§3.4); Header L1의 `<tenant-name>·<env>`; 전환 토스트 (§8.4) |
| REQ-C03 | SCR-902 Help의 "Global" 탭에 커스텀 바인딩 표기                                    |
| REQ-C04 | SCR-000 마스킹 프롬프트; SCR-001 에러 매핑 테이블; SCR-905 About의 token info      |
| REQ-C05 | 모든 화면: 토큰 값은 UI에 노출되지 않음 (SCR-905도 "env OKTA_API_TOKEN" 소스만)     |

### 11.4. 에러 / Rate Limit / 관측성

| REQ     | 충족 위치                                                                                |
|---------|------------------------------------------------------------------------------------------|
| REQ-E01 | Header L1 `[RL: ok/warn/limited]` 배지; SCR-050의 `[TAIL]`/`[ADAPTIVE]`/`⏸`; SCR-905 Rate limits; 각 리스트의 "Paused" 상태 |
| REQ-E02 | Status Bar 토스트 (3초); SCR-904 Session Errors; 각 리스트의 Error 상태                  |
| REQ-E03 | Header L1 `offline` 배지; 각 리스트의 "offline — cached" 상태                            |
| REQ-O01 | `:debug open` 팔레트 명령 (§3.4) — 경로 안내 메시지 + Help 보완 문구                    |

### 11.5. Nice-to-Have (PRD §4.3)

| 기능             | 설계 위치                                                |
|------------------|----------------------------------------------------------|
| 북마크 (`m`)     | v0.2. MVP에서 `m`은 "members 탭 점프"(SCR-020)로 선점. v0.2에서는 **`B` 또는 `:bookmark <name>`** 예약 (§12.3). |
| 최근 목록 (`r`)  | v0.2 `:recent` 명령으로 이관. MVP `r`은 raw JSON 토글로 선점 (pm NIT-1 확정). |
| YAML/JSON 복사   | `y` / `yy` / `yf` 배정 완료                              |
| Admin 딥링크     | `o` 배정 완료                                            |
| 필터 프리셋 저장 | v0.2 (Logs Preset 메뉴는 내장만)                         |

---

## 12. 키 충돌 검증

### 12.1. 전역 vs 화면별

| 키  | 전역 용도              | 화면 전용 용도                         | 처리                                        |
|-----|------------------------|----------------------------------------|---------------------------------------------|
| `q` | close / app quit       | (해당 없음)                            | 전역 우선, 검색 모드에서는 문자로.          |
| `R` | refresh                | (해당 없음)                            | 전역.                                       |
| `r` | raw JSON toggle        | (SCR-042/051에서만 의미)               | 무의미한 화면에서는 toast "no action here". |
| `g` | nav.top_double(`gg`)   | SCR-010에서 "Groups 탭 점프"           | **충돌** — `g` 1회 후 `g` 대기 (300ms)가 아니면 "Groups 탭" 발동. |
| `m` | (없음)                 | SCR-020에서 "members 탭"               | 화면 전용. v0.2 북마크는 `B` 로 이관.       |
| `L` | (없음)                 | SCR-010에서 "Recent events 점프"       | 화면 전용. (탭 라벨은 "Recent"로 변경됨)    |
| `s` | (없음)                 | SCR-050에서 "tail toggle"              | 다른 화면에서는 no-op + 경고 toast.         |
| `f` | (없음)                 | SCR-050에서 "follow toggle"            | 동상.                                       |
| `i` | (없음)                 | SCR-030에서 "INVALID 필터"             | 동상. `/` 검색 모드에서는 문자.             |
| `a` | (없음)                 | SCR-030 "ACTIVE 필터" / SCR-020 "Apps 탭 점프" | 화면별 서로 다름 — 허용 (문맥 명확).   |

### 12.2. `gg` 대기 창 (Vim 관례)

- `g` 1회 후 300ms 내 `g` 재입력 → `nav.top` (맨 위).
- 300ms 초과 → 단일 `g` 액션 (SCR-010에서는 "Groups 탭 점프").
- SCR-011, SCR-021 등 단일 `g`에 의미 없는 화면에서는 단일 `g`도 `nav.top`으로 흡수.

### 12.3. Reserved (v0.2+ 위해 현재 미배정)

- `d`, `D` — delete/deactivate 류 mutation 예약
- `x` — SCR-904 errors clear 용 (이미 사용)
- `p` — paste (mutation 예약, MVP 없음)
- **`B` — v0.2 북마크 추가 (`:bookmark <name>` 과 쌍)** (pm MINOR-4 예약)

---

## 13. 결정 테이블 (§13.1 원본의 v1.0 확정 상태)

| # | 항목 | v1.0 확정 |
|---|------|-----------|
| 1 | `r` 키 이중 의미 | **raw 우선**, 최근 목록은 v0.2 `:recent`로 이관 |
| 2 | Wide 모드(180+) 사이드 패널 | **v0.2 유지** (MVP는 단일 패널) |
| 3 | 타임존 토글 UI | **커맨드만** (`:set tz=local`) |
| 4 | `:healthcheck` 출력 | **모달 확정** (SCR-905 healthcheck 뷰) |
| 5 | 색상 테마 기본값 | **다크 + k9s 유사 확정** (PRD §11.3 D-2) |
| 6 | 모달 오버레이 구현 | **Phase 4 개발자 판단** |
| 7 | DEPROVISIONED 기본 포함/제외 | **포함 유지** (PRD REQ-R01 AC-7) |
| 8 | 에러 매핑 소스 오브 트루스 | **PRD §7.7** (team-lead 결정, v0.2 E0000054/E0000068 재검토) |

---

## 14. 오픈 이슈 (v0.2+ 재검토)

- **E0000054 / E0000068 에러 매핑** — v0.2 Write 스코프에서 재검토 예정 (team-lead 결정 §13 #8). E0000054는 E0000001로 subsumed 가능성, E0000068은 factor verification(v0.2 Write) 시 의미.
- **`debugContext.debugData` PII 마스킹** — okta-expert m1 플래그. Phase 4 실전 로그 관찰 후 렌더 단계 필드 스캔 기반 마스킹 도입 여부 결정.
- **Wide 모드 사이드 패널 (180+ 폭)** — v0.2. 리스트+상세 동시 표시. 터미널 사용 패턴 관찰 후 우선순위 결정.
- **`:recent` 최근 목록** — v0.2. 북마크(`B`)와 함께 도입. 히스토리 크기·저장 위치 설계 필요.
- **Policy 타입 카탈로그 `ENTITY_RISK` / `CONTINUOUS_ACCESS` 편입** — GA 확인 후 카탈로그 entry 추가. TUI 설계 변경 없음 (§2.4 규약).
- **스크린리더 지원** — v0.3+. `--plain` 모드는 MVP. 본격 지원은 Bubbletea upstream accessibility 패턴 수용 시.

---

## 15. Renderable Reference Specs (Lip Gloss 매핑)

> **추가 배경 (v1.1.0, 2026-04-24):** v1.0.0의 와이어프레임은 정보 구조와 키 바인딩까지는 정확히 명세했지만, 실제 구현 단계에서 `internal/tui/users/list.go:144-146`의 주석 — *"Output is deliberately plain text so teatest's golden comparisons are stable without style-token dependencies"* — 처럼 **테스트 단순화를 이유로 시각 명세가 무시**되었다. 결과적으로 사용자가 실제 바이너리를 실행했을 때 색·박스·테이블·k9s 스타일 chrome이 하나도 보이지 않는 plain-text 출력만 나왔다.
>
> 본 §15는 와이어프레임을 **실제 Bubbletea + Lip Gloss 코드로 변환할 때 따라야 할 1:1 매핑**을 정의한다. 와이어프레임이 *"무엇이 어디에 보이는가"*를 다룬다면, 본 절은 *"어떤 컴포넌트로 어떤 토큰을 써서 어떤 보더로 그리는가"*를 다룬다. 개발자는 본 절을 보고 코드를 작성할 수 있어야 하고, QA는 본 절의 항목을 체크리스트로 검증할 수 있어야 한다.

### 15.1. 글로벌 Chrome (모든 화면 공통)

모든 리스트/상세 화면은 **3-zone vertical stack**으로 구성된다. k9s의 standard chrome을 ota-Okta 컨텍스트로 차용.

```
┌────────────────────────────────────────────────────────────────────────┐ ← Header (border: RoundedBorder, fg: tokens.Header)
│  Row 0: TitleBar    (left: brand · org · env)  (right: RL · TZ · ver)  │
│  Row 1: ContextBar  (resource name · count · filter/breadcrumb)        │
├────────────────────────────────────────────────────────────────────────┤ ← MainBody divider (NormalBorder horizontal, fg: tokens.Muted)
│                                                                        │
│  Body: Bubble component (table / viewport / list / textinput)          │
│                                                                        │
├────────────────────────────────────────────────────────────────────────┤ ← StatusBar divider
│  Row N-1: KeyHints  (bubbles/help, fg: tokens.Muted, key: tokens.FG)   │
└────────────────────────────────────────────────────────────────────────┘
```

**컴포넌트 트리 (모든 화면):**

```
app.RootView
├── HeaderBar      (lipgloss.JoinVertical[TitleBar, ContextBar])
│   ├── TitleBar      (lipgloss.JoinHorizontal[BrandSegment, Spacer, RightSegment])
│   │   ├── BrandSegment   ("ota · " + tenant + " · " + EnvBadge)
│   │   └── RightSegment   (RLBadge + " " + TZ + " " + version)
│   └── ContextBar    (lipgloss.JoinHorizontal[Breadcrumb, Spacer, Counter+Filter])
├── MainBody       (lipgloss.NewStyle().Border(NormalBorder, false, true, true, true))
│   └── <screen-specific component>  (see §15.2~15.10)
└── StatusBar      (bubbles/help.Model.View() + 트레일링 트로스트)
```

**토큰 매핑 (§6.1 토큰 → chrome):**

| 영역             | 텍스트 토큰     | 보더 토큰     | 보더 스타일                  | 비고 |
|------------------|----------------|---------------|------------------------------|------|
| Header outer     | `tokens.FG`    | `tokens.Header` | `lipgloss.RoundedBorder()` (top edges) | k9s 스타일 둥근 corner |
| TitleBar brand   | `tokens.Header` (Bold) | — | — | "ota" 자체는 항상 Bold |
| TitleBar env=prod | `tokens.Danger` (BG, white FG) | — | — | 환경 식별 |
| TitleBar env=staging | `tokens.Warning` (BG, black FG) | — | — | |
| TitleBar env=dev | `tokens.Muted` (BG, white FG) | — | — | |
| RLBadge ok       | `tokens.Success` | — | — | `[RL: ok]` |
| RLBadge warn     | `tokens.Warning` | — | — | `[RL: warn]` |
| RLBadge limited  | `tokens.Danger` (Bold) | — | — | `[RL: ⏸ limited]` |
| ContextBar       | `tokens.Accent` (resource name) + `tokens.Muted` (counter/filter) | — | — | |
| Body divider     | — | `tokens.Muted` | `NormalBorder().Bottom`/`Top` | 박스 외곽이 아닌 split line |
| MainBody border  | — | `tokens.Muted` | `NormalBorder()` (left+right) | 좌우만, 상하는 divider가 담당 |
| StatusBar fg     | `tokens.Muted` (label) + `tokens.FG` (key) | — | — | 키는 `<>` 안에 굵게 |
| Selected row     | — | — | `tokens.Accent` background + `tokens.BG` foreground (Reverse-style) | k9s 행 하이라이트 |

**Bubbles 컴포넌트 사용:**

| 화면 영역 | Bubble | 호출 |
|----------|--------|------|
| 키 힌트 | `github.com/charmbracelet/bubbles/help` | `help.New()`, `m.help.View(m.keys)` |
| 리스트 (테이블) | `github.com/charmbracelet/bubbles/table` | `table.New(table.WithColumns(...), table.WithRows(...))` |
| 장문 (Detail/Raw JSON) | `github.com/charmbracelet/bubbles/viewport` | `viewport.New(w, h)`, `vp.SetContent(...)` |
| 인라인 입력 (`/`, `:`) | `github.com/charmbracelet/bubbles/textinput` | `textinput.New()`, `ti.Placeholder = "..."` |
| 로딩 | `github.com/charmbracelet/bubbles/spinner` | `spinner.New(spinner.WithSpinner(spinner.Dot))` |
| 페이지 진행 | `github.com/charmbracelet/bubbles/progress` | (Logs 페이징에만, 선택) |

> **주의:** `bubbles/table`은 자체 chrome (헤더 행 + body)을 가진다. 화면 chrome이 이걸 다시 감싸므로 `table.WithStyles(table.DefaultStyles())`를 그대로 쓰지 말고 §15.2의 컬럼 정의대로 커스텀 스타일을 적용한다.

### 15.2. SCR-010 Users List — 시각 명세

**컴포넌트 선택:** `bubbles/table`. (와이어프레임이 컬럼 4~6개의 정형 테이블이고 정렬이 명시적이므로 `bubbles/list`보다 `bubbles/table`이 적합.)

**컴포넌트 트리:**
```
users.ListModel.View()
└── HeaderBar (글로벌)
    ContextBar:
      Breadcrumb: tokens.Header.Render("Users")
      Spacer
      Counter: tokens.Muted.Render(fmt.Sprintf("%d of %d", len, total))
      FilterChip (선택적): tokens.Accent.Render("· q=\"al\"")
└── MainBody
    └── table.Model
        ├── Header row (Bold + tokens.Header, BottomBorder)
        └── Body rows (selected: Reverse + tokens.Accent BG)
└── StatusBar (글로벌)
    KeyHints: "<j k> nav  </> search  <:search> SCIM  <Enter> detail  <?> help  <q> back"
```

**컬럼 정의 (5개 — 표준 모드 100~139):**

| # | Title (uppercase) | Width | Align | Cell renderer | 토큰 |
|---|-------------------|-------|-------|---------------|------|
| 1 | STATUS | 14 | left | `<icon> <label>` (e.g. `● ACTIVE`) | icon: §15.2 표 아래 매핑 / label: same color |
| 2 | LOGIN | 28 | left | `u.Profile.Login` | `tokens.FG` |
| 3 | DISPLAY NAME | 18 | left | `u.Profile.FirstName + " " + u.Profile.LastName` | `tokens.FG` |
| 4 | LAST LOGIN | 10 | right | relative time (`2h ago`, `—`) | `tokens.Muted` |
| 5 | CHANGED | 8 | right | relative time | `tokens.Muted` |

**컬럼 → 상태 → 색상 매핑:**

| User.Status | Icon | Token | Mono fallback (NO_COLOR) |
|-------------|------|-------|--------------------------|
| `ACTIVE` | `●` | `tokens.Success` | `[+]` |
| `STAGED` / `PROVISIONED` | `○` | `tokens.Info` | `[-]` |
| `SUSPENDED` | `✗` | `tokens.Warning` | `[X]` |
| `LOCKED_OUT` | `⚠` | `tokens.Danger` | `[!]` |
| `PASSWORD_EXPIRED` | `◒` | `tokens.Magenta` | `[~]` |
| `DEPROVISIONED` | `⊘` | `tokens.Muted` | `[/]` |

**반응형 컬럼 드롭 (1.2 기준):**

| 폭 | 표시 컬럼 (drop 순서: → DISPLAY NAME → CHANGED → LAST LOGIN) |
|----|--------------------------------------------------------------|
| 140+ | STATUS, LOGIN, DISPLAY NAME, LAST LOGIN, CHANGED, **DEPARTMENT** (확장 모드) |
| 100~139 | STATUS, LOGIN, DISPLAY NAME, LAST LOGIN, CHANGED |
| 90~99 | STATUS, LOGIN, DISPLAY NAME, LAST LOGIN (CHANGED 드롭) |
| 80~89 | STATUS, LOGIN, LAST LOGIN (DISPLAY NAME 드롭) |
| < 80 | "ota requires minimum 80x24 terminal" 화면 |

> 드롭 발생 시 ContextBar 우측에 `tokens.Muted.Render("[-1 col]")` 표기.

**보더:**
- 외곽: 글로벌 chrome이 담당 (`RoundedBorder` top, `NormalBorder` left+right+bottom)
- 헤더 행 ↔ 본문: `BottomBorder` 1줄 (`tokens.Muted`)
- 컬럼 사이: 공백 2칸 (`"  "`) — 보더 문자 사용 안 함 (k9s convention)

**k9s 비교:** `kubectl get pods` 화면 = `STATUS NAME READY RESTARTS AGE` 5컬럼. ota Users도 같은 패턴 (5컬럼 ± 1).

### 15.3. SCR-020 Groups List — 시각 명세

**컬럼 정의 (5개):**

| # | Title | Width | Align | Cell renderer |
|---|-------|-------|-------|---------------|
| 1 | TYPE | 4 | center | icon (`◆` `▣` `◈`) |
| 2 | NAME | 24 | left | `g.Profile.Name` |
| 3 | DESCRIPTION | 28 | left | `g.Profile.Description` (truncate `…`) |
| 4 | UPDATED | 10 | right | relative time |
| 5 | TAGS | 12 | left | badges (`RULE` / `SYS` / `LARGE`) |

**Group Type → 아이콘 → 토큰 매핑:**

| Type | Icon | Token | Mono |
|------|------|-------|------|
| `OKTA_GROUP` | `◆` | `tokens.FG` | `[O]` |
| `APP_GROUP` | `▣` | `tokens.Info` | `[A]` |
| `BUILT_IN` | `◈` | `tokens.Magenta` | `[B]` |

**Tag 배지 렌더링 (lipgloss BG 색):**

| Tag | Token | Foreground |
|-----|-------|------------|
| `RULE` | `tokens.BadgeRule` (green BG) | black |
| `SYS` | `tokens.BadgeSys` (slate BG) | white |
| `LARGE` | `tokens.BadgeLarge` (yellow BG) | black |

배지는 `lipgloss.NewStyle().Background(...).Foreground(...).Padding(0, 1)` 로 패딩 1 적용.

**반응형 드롭:** TAGS → DESCRIPTION → UPDATED 순. (TYPE/NAME 필수.)

**k9s 비교:** `k9s` Namespaces 화면 = `STATUS NAME LABELS AGE`. ota Groups 동일 패턴.

### 15.4. SCR-030 Group Rules List — 시각 명세

**컬럼 정의 (4개):**

| # | Title | Width | Cell renderer |
|---|-------|-------|---------------|
| 1 | STATUS | 14 | `<icon> <label>` |
| 2 | NAME | 30 | `r.Name` |
| 3 | TARGETS | 22 | comma-joined assigned group names (truncate) |
| 4 | UPDATED | 10 | relative time |

**Rule Status → 색상:**

| Status | Icon | Token |
|--------|------|-------|
| `ACTIVE` | `●` | `tokens.Success` |
| `INACTIVE` | `○` | `tokens.Muted` |
| `INVALID` | `⚠` | `tokens.Danger` (Bold) — **PRD REQ-R03 AC-2: 즉시 눈에 띄어야 함** |

**INVALID 배너 (리스트 하단):** `tokens.Danger`로 `⚠ N rule(s) in INVALID state — expression cannot be evaluated by Okta.` 5포인트 경고 (okta-expert M-2 반영). 보더 없음, 단일 줄.

### 15.5. SCR-041 Policies List — 시각 명세

**컬럼 정의 (5개):**

| # | Title | Width | Align | Cell renderer |
|---|-------|-------|-------|---------------|
| 1 | PRI | 4 | right | `p.Priority` (정수) |
| 2 | STATUS | 12 | left | `<icon> <label>` |
| 3 | NAME | 30 | left | `p.Name` |
| 4 | SYSTEM | 6 | center | `system==true ? SYS_BADGE : "-"` |
| 5 | UPDATED | 10 | right | relative or `never` |

**SYS 배지:** `tokens.BadgeSys` BG. `never`는 `tokens.Muted`.

**ContextBar:** `Policies › <TYPE>    3 of 3` — 타입은 `tokens.Accent`.

### 15.6. SCR-050 Logs List — 시각 명세

**컬럼 정의 (6개):**

| # | Title | Width | Cell renderer |
|---|-------|-------|---------------|
| 1 | WHEN | 12 | relative time (tail에선 `just now`, `3s ago`) |
| 2 | SEV | 5 | severity icon + label (e.g. `ℹ INFO`) |
| 3 | EVENTTYPE | 24 | `e.EventType` (truncate) |
| 4 | ACTOR | 18 | `e.Actor.AlternateID` (or masked) |
| 5 | OUTCOME | 9 | `SUCCESS` / `FAILURE` |
| 6 | IP | 15 | client IP (drop on width < 110) |

**Severity → 색상:**

| Severity | Icon | Token |
|----------|------|-------|
| `DEBUG` | `·` | `tokens.Muted` |
| `INFO` | `ℹ` | `tokens.Info` |
| `WARN` | `!` | `tokens.Warning` |
| `ERROR` | `✗` | `tokens.Danger` |

**Outcome → 색상:** `SUCCESS` → `tokens.Success`, `FAILURE` → `tokens.Danger`.

**Tail indicator (ContextBar 우측):**
- `[TAIL 7s] ▶` — `tokens.Info` (정상)
- `[TAIL 15s · ADAPTIVE] ▶` — `tokens.Info` (정상, 단지 인터벌 다름. 경고 아님)
- `[TAIL ⏸] · resuming in 8s` — `tokens.Danger`

**New events 배너 (tail mode):** `tokens.Accent.Render("▲ 2 new events (press <f> to auto-follow)")`. 위치는 테이블 하단 (Status Bar 위).

### 15.7. SCR-011 User Detail — 시각 명세

**컴포넌트 트리:**
```
users.DetailModel.View()
└── HeaderBar
    ContextBar Breadcrumb: tokens.Header("Users") + tokens.Muted(" › ") + tokens.FG(login) + tokens.Muted(" id: 00u…x8")
└── MainBody
    ├── TabBar (lipgloss.JoinHorizontal of tab cells)
    │     active tab:    tokens.Accent (Bold, Underline)
    │     inactive tab:  tokens.Muted
    │     count loading: tokens.Muted.Render("Groups …")
    │     count failed:  tokens.Danger.Render("Groups ?")
    │     count loaded:  tokens.FG.Render("Groups 4")
    ├── (1줄 dvider, NormalBorder horizontal)
    └── TabContent (active tab에 따라 다른 viewport/table)
└── StatusBar
```

**탭 셀 렌더링 (Lip Gloss):**
```go
tabActive   := lipgloss.NewStyle().
    Foreground(tokens.Accent.GetForeground()).
    Bold(true).
    Underline(true).
    Padding(0, 2)
tabInactive := lipgloss.NewStyle().
    Foreground(tokens.Muted.GetForeground()).
    Padding(0, 2)
```

각 탭은 `[ <label> <count> ]` 포맷. count는 별도 토큰으로 색칠.

**Profile 탭 본문 — 정의 리스트 (key-value):**
- 좌측 키 컬럼: 폭 16, `tokens.Muted`, right-align
- 우측 값 컬럼: `tokens.FG`
- masked 값: 우측에 `tokens.Muted.Render("<- masked · :unmask <field>")`
- unmask된 값: 값 우측에 `tokens.BadgeUnmask.Render(" [M!] ")`
- Custom fields 섹션 separator: `— Custom fields ` + dashes (`tokens.Muted`)

**Groups 탭 본문:** 미니 테이블 (group name + role + assignedAt). `bubbles/table` 컴팩트 모드 (헤더 1줄 + 본문).

**Factors 탭:** OKTA Factors 카드형. 각 factor:
```
  ● PUSH (active)        Okta Verify
    enrolled  2024-08-15 12:00:00 UTC
    last used 2h ago
```
factorType은 `tokens.Header`, status icon은 색상 매핑 (active=Success, pending=Warning, expired=Danger).

### 15.8. SCR-900 Command Palette (overlay) — 시각 명세

**Overlay 패턴 (모든 modal 공통):**
- 배경: 본문은 그대로 두고 dim filter (선택 — Lip Gloss에 직접 dim API 없으므로 본문 위에 `tokens.Muted` 텍스트 유지로 대신).
- 모달 박스: `lipgloss.RoundedBorder()`, fg `tokens.Header`.
- 위치: 화면 하단 fixed (3줄). Help/Confirm은 화면 중앙.

**컴포넌트 트리:**
```
overlay.PaletteModel.View()
└── ModalBox (RoundedBorder, fg: tokens.Header)
    ├── PromptLine: ":" + textinput.Model.View()
    ├── SuggestionList (max 5):
    │     selected:   tokens.Accent.Background  + tokens.BG.Foreground
    │     unselected: tokens.FG
    │     description: tokens.Muted (right-aligned)
    └── HelpLine: tokens.Muted("<Tab> complete · <↑↓> history · <Enter> run · <Esc> cancel")
```

**Bubble 컴포넌트:** `bubbles/textinput` (필터 입력) + 자체 list rendering.

### 15.9. SCR-902 Help (modal) — 시각 명세

**컴포넌트 트리:**
```
overlay.HelpModel.View()
└── ModalBox (RoundedBorder, fg: tokens.Header) — 전체 화면 dim 위
    ├── TitleBar: "Help · <screen name>" + "/ search"
    ├── TabBar (Screen / Global / Commands / Status icons)
    ├── Content (bubbles/viewport — 스크롤 가능)
    │     section heading: tokens.Header.Bold
    │     key column:      tokens.FG.Bold (10폭, left-align)
    │     desc column:     tokens.Muted
    │     warnings:        tokens.Warning prefix "⚠"
    │     info:            tokens.Info prefix "ⓘ"
    └── HelpLine: bubbles/help
```

Status icons 탭의 비교표는 monospace 정렬 박스(ASCII grid)로 렌더 — 각 셀 `tokens.FG`, 헤더 row `tokens.Muted.Bold`.

### 15.10. SCR-903 Confirm Dialog — 시각 명세

**모달 박스:** `RoundedBorder`, fg `tokens.Danger` (위험 동작이므로 빨간 보더).
- Title: `tokens.Danger.Bold.Render("Unmask PII field · mobilePhone")`
- Body: `tokens.FG`
- Prompt label: `tokens.Muted.Render("Type ") + tokens.Header.Render("\`unmask\`") + tokens.Muted.Render(" to confirm · <Esc> cancel")`
- Input: `bubbles/textinput`, prefix `> `

### 15.11. 토큰 적용 요약 매트릭스

빠른 참조용 — 화면별 주요 토큰 사용:

| 화면 | Header | Accent | Success | Warning | Danger | Muted | 배지 |
|------|--------|--------|---------|---------|--------|-------|------|
| SCR-010 Users | brand, count | selected row, breadcrumb name | ACTIVE | SUSPENDED | LOCKED_OUT | metadata | — |
| SCR-011 Detail | tab labels | active tab | status field | masked label | unmask `[M!]` | section sep | BadgeUnmask |
| SCR-020 Groups | brand | selected | (n/a) | LARGE 배지 | (n/a) | metadata | BadgeRule, BadgeSys, BadgeLarge |
| SCR-030 Rules | brand | selected | ACTIVE | (n/a) | INVALID, banner | INACTIVE | — |
| SCR-041 Policies | brand | selected | ACTIVE | (n/a) | (errors) | INACTIVE | BadgeSys |
| SCR-050 Logs | brand | selected | SUCCESS | WARN sev | ERROR sev, FAILURE | DEBUG sev, IP | — |
| SCR-900 Palette | modal title | selected suggestion | — | — | (errors only) | desc | — |
| SCR-902 Help | section heads | active tab | — | warning rows | error rows | desc | — |
| SCR-903 Confirm | (n/a) | (n/a) | — | — | border, title | hints | — |

---

## 16. Golden Snapshots (NO_COLOR)

> **목적:** test-engineer가 `model.View()` 결과를 비교할 골든 파일의 **권위 있는 reference**. 색상은 NO_COLOR(monochrome) 모드 결과만 명세 — ANSI escape이 골든 비교를 흔들지 않게 하기 위함. 색상 표현 검증은 별도의 visual-regression 테스트(§18.3) 또는 수동 QA로 수행.
>
> 모든 골든은 **120x30 표준 모드** + **NO_COLOR**로 캡처된 가정. 데이터는 §16.0의 fixture를 사용한다.

### 16.0. 표준 Fixture 데이터 (모든 골든이 공유)

**Users (5명):**
```yaml
- id: 00u00000001    login: alice@acme.com         displayName: Alice Smith   status: ACTIVE          lastLogin: 2h_ago    statusChanged: 14d_ago
- id: 00u00000002    login: alan.turing@acme.com   displayName: Alan Turing   status: ACTIVE          lastLogin: 1d_ago    statusChanged: 60d_ago
- id: 00u00000003    login: alex.lee@acme.com      displayName: Alex Lee      status: LOCKED_OUT      lastLogin: nil       statusChanged: 3m_ago
- id: 00u00000004    login: amy.wong@acme.com      displayName: Amy Wong      status: STAGED          lastLogin: nil       statusChanged: 1d_ago
- id: 00u00000005    login: aaron.k@acme.com       displayName: Aaron K.      status: SUSPENDED       lastLogin: 5d_ago    statusChanged: 5d_ago
```

**Groups (3개), Rules (3개), Policies (3개), Logs (5건)** — 분량상 §16.X 골든 본문에 직접 포함. 동일 fixture를 모든 화면에서 재사용.

**고정값:**
- 시계: `2026-04-24T12:00:00Z`
- 테넌트: `acme.okta.com`, env=`prod`, profile=`prod`
- ratelimit: ok
- terminal: 120x30
- NO_COLOR=1, TZ=UTC

### 16.1. SCR-010 Users List — 정상 상태 (golden)

**파일 경로 제안:** `internal/tui/users/testdata/golden/list_default.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Users                                                          5 of 5             │
├───────────────────────────────────────────────────────────────────────────────────┤
│ STATUS         LOGIN                       DISPLAY NAME       LAST LOGIN  CHANGED │
│ ─────────────────────────────────────────────────────────────────────────────────  │
│ [+] ACTIVE     alice@acme.com              Alice Smith            2h ago   14d ago│
│ [+] ACTIVE     alan.turing@acme.com        Alan Turing            1d ago   60d ago│
│ [!] LOCKED_OUT alex.lee@acme.com           Alex Lee                    —    3m ago│
│ [-] STAGED     amy.wong@acme.com           Amy Wong                    —    1d ago│
│ [X] SUSPENDED  aaron.k@acme.com            Aaron K.               5d ago    5d ago│
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <j k> nav  </> search  <:search> SCIM  <Enter> detail  <?> help  <q> back         │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

**검증 포인트:**
- 첫 행은 `╭─` `─╮`로 시작 (RoundedBorder 상단).
- 마지막 행은 `╰─` `─╯`로 끝 (RoundedBorder 하단).
- 좌우 변은 `│`.
- 헤더 ↔ 본문 사이, 본문 ↔ 상태바 사이는 `├─` `─┤` (NormalBorder horizontal).
- 컬럼 사이 공백 2칸. `STATUS` 열은 좌측 정렬, 우측 두 컬럼은 우측 정렬.
- 첫 데이터 행 (`alice@acme.com`)이 selected — NO_COLOR 모드에선 색 차이 없으므로 골든에는 표시 없음 (시각 검증은 §18.3).
- 빈 행은 trailing space 없이 `│` + space-padding + `│`로 정확히 121 columns.

### 16.2. SCR-010 Users List — 로딩 상태 (golden)

**파일 경로 제안:** `internal/tui/users/testdata/golden/list_loading.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Users                                                          loading…           │
├───────────────────────────────────────────────────────────────────────────────────┤
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                          (.) Fetching users...                                    │
│                              GET /api/v1/users?limit=200                          │
│                              Press <Esc> to cancel                                │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <Esc> cancel  <?> help                                                            │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

> 스피너 프레임은 테스트에서 frozen-clock 모드로 고정 (`(.)` 첫 프레임). 실제 렌더는 spinner.Dot.

### 16.3. SCR-010 Users List — Empty (필터 결과 0)

**파일 경로 제안:** `internal/tui/users/testdata/golden/list_empty_filter.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Users                                                  0 of 5 · q="zzznomatch"    │
├───────────────────────────────────────────────────────────────────────────────────┤
│                                                                                   │
│   No users match your filter.                                                     │
│                                                                                   │
│   Try:                                                                            │
│     /                                  clear filter                               │
│     :search status eq "SUSPENDED"      switch to SCIM search                      │
│                                                                                   │
│     Note: `/` uses Okta `q` (free text). Use `:search` for fields.                │
│     Note: recently created users may take minutes to appear in search             │
│           (indexing lag — eventually consistent).                                 │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ </> filter  <:search> SCIM  <?> help                                              │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.4. SCR-010 Users List — Error (403)

**파일 경로 제안:** `internal/tui/users/testdata/golden/list_error_403.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Users                                                          (error)            │
├───────────────────────────────────────────────────────────────────────────────────┤
│                                                                                   │
│   [X] Failed to load users                                                        │
│                                                                                   │
│       E0000006 · 403 · Insufficient permissions for /users                        │
│       Token may be Read-Only + Admin role may lack user read scope.               │
│                                                                                   │
│   <R> retry     <:about> token info     <:errors> history                         │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <R> retry  <:about>  <:errors>  <?> help  <q> back                                │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.5. SCR-020 Groups List — 정상 (golden)

**파일 경로 제안:** `internal/tui/groups/testdata/golden/list_default.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Groups                                                         3 of 3             │
├───────────────────────────────────────────────────────────────────────────────────┤
│ TYPE NAME                     DESCRIPTION                  UPDATED    TAGS        │
│ ─────────────────────────────────────────────────────────────────────────────────  │
│ [O]  Engineering              All engineers                 2h ago    [RULE]      │
│ [A]  Jira Users               Synced from Atlassian         3h ago                │
│ [B]  Everyone                 All organization members      1m ago    [SYS][LARGE]│
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <j k> nav  </> search  <Enter> detail  <m> members  <R> refresh  <?> help         │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.6. SCR-030 Group Rules List — INVALID 배너 포함 (golden)

**파일 경로 제안:** `internal/tui/rules/testdata/golden/list_with_invalid.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Group Rules                                                    3 of 3             │
├───────────────────────────────────────────────────────────────────────────────────┤
│ STATUS         NAME                           TARGETS                  UPDATED    │
│ ─────────────────────────────────────────────────────────────────────────────────  │
│ [+] ACTIVE     Engineers to Eng               Engineering               2h ago    │
│ [-] INACTIVE   Legacy Eng Mapping             Engineering              30d ago    │
│ [!] INVALID    Broken Dept Rule               Sales                     3h ago    │
│                                                                                   │
│ [!] 1 rule in INVALID state — expression cannot be evaluated by Okta.             │
│     Open the rule to view why and what to fix.                                    │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <j k> nav  <Enter> detail  </> search  <i> invalid only  <a> active only  <?>     │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.7. SCR-041 Policies List (within OKTA_SIGN_ON) — golden

**파일 경로 제안:** `internal/tui/policies/testdata/golden/list_okta_sign_on.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Policies › OKTA_SIGN_ON                                        3 of 3             │
├───────────────────────────────────────────────────────────────────────────────────┤
│ PRI STATUS       NAME                                  SYSTEM   UPDATED           │
│ ─────────────────────────────────────────────────────────────────────────────────  │
│   1 [+] ACTIVE   Default Policy                        [SYS]    never             │
│   2 [+] ACTIVE   Require MFA for admins                  -       2d ago           │
│   3 [-] INACTIVE Legacy Contractor Rule                  -      90d ago           │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <Enter> detail  <h> change type  </> search  <R> refresh  <?> help                │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.8. SCR-050 Logs List — history 모드 (golden)

**파일 경로 제안:** `internal/tui/logs/testdata/golden/list_history.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Logs · since 24h · DESC                                        5 loaded           │
│ filter: eventType eq "user.session.start" and outcome.result eq "FAILURE"         │
├───────────────────────────────────────────────────────────────────────────────────┤
│ WHEN         SEV    EVENTTYPE                ACTOR              OUTCOME  IP       │
│ ─────────────────────────────────────────────────────────────────────────────────  │
│ 2h ago      [i] INFO user.session.start      alice@acme.com     FAILURE  10.0.1.5 │
│ 3h ago      [i] INFO user.session.start      bob@acme.com       FAILURE  10.0.1.6 │
│ 7h ago      [!] WARN user.session.start      alice@acme.com     FAILURE  10.0.1.5 │
│ 1d ago      [i] INFO user.session.start      unknown@acme.com   FAILURE  10.0.1.7 │
│ 2d ago      [X] ERR  user.session.start      svc-sync@acme      FAILURE  10.0.1.8 │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <s> tail  <f> follow  <Enter> detail  <P> presets  </>q  <:filter>                │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.9. SCR-011 User Detail — Profile 탭 (golden)

**파일 경로 제안:** `internal/tui/users/testdata/golden/detail_profile.txt`

```
╭─ ota · acme.okta.com · prod                              [RL: ok]    UTC  v0.1.0 ─╮
│ Users › alice@acme.com                                          id: 00u00000001   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ [Profile] [ Credentials ] [ Timestamps ] [ Groups 4 ] [ Factors 2 ] [ Recent ]    │
│ ─────────────────────────────────────────────────────────────────────────────────  │
│           login    alice@acme.com                                                 │
│           email    alice@acme.com                                                 │
│       firstName    Alice                                                          │
│        lastName    Smith                                                          │
│     displayName    Alice Smith                                                    │
│          status    [+] ACTIVE                                                     │
│     mobilePhone    +1-***-***-1234       <- masked · :unmask mobilePhone          │
│     secondEmail    a***@personal.com     <- masked                                │
│                                                                                   │
│   — Custom fields ─────────────────────────────────────                           │
│      department    Engineering                                                    │
│           title    Senior SWE                                                     │
│      costCenter    ENG-42                                                         │
│                                                                                   │
├───────────────────────────────────────────────────────────────────────────────────┤
│ <Tab> next tab  <y> copy  <o> admin console  <L> recent  <Esc> back               │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.10. SCR-900 Command Palette — overlay (golden)

**파일 경로 제안:** `internal/tui/overlay/testdata/golden/palette_default.txt`

> overlay는 본문 위 fixed 하단 3줄. 골든은 overlay 부분만 캡처 (본문은 §16.1 결과를 전제).

```
├───────────────────────────────────────────────────────────────────────────────────┤
│ : us|                                                                             │
│   > :users          switch to Users                                               │
│     :unmask         unmask a PII field                                            │
│                                                                                   │
│ <Tab> complete · <↑↓> history · <Enter> run · <Esc> cancel                        │
╰───────────────────────────────────────────────────────────────────────────────────╯
```

### 16.11. SCR-902 Help — Screen 탭 (golden)

**파일 경로 제안:** `internal/tui/overlay/testdata/golden/help_screen_users.txt`

```
        ╭───────────────────────────────────────────────────────────────╮
        │  Help · Users List                                  / search  │
        ├───────────────────────────────────────────────────────────────┤
        │  [Screen] [ Global ] [ Commands ] [ Status icons ]            │
        │                                                               │
        │  Navigation                                                   │
        │      j, down       down one row                               │
        │      k, up         up one row                                 │
        │      gg            top                                        │
        │      G             bottom                                     │
        │      Ctrl-d/u      half page                                  │
        │                                                               │
        │  Actions                                                      │
        │      Enter         user detail                                │
        │      g             jump to Groups tab                         │
        │      L             jump to Recent events tab                  │
        │      R             refresh (invalidate cache)                 │
        │                                                               │
        │  Search                                                       │
        │      /             client filter (case-insensitive)           │
        │      :search ...   server SCIM search                         │
        │      [!] eventually consistent — recent creations may         │
        │          not appear for minutes                               │
        │                                                               │
        │  (i) Write actions are not available in MVP. See roadmap.     │
        │                                                               │
        │  <Tab> switch tab · </> filter help · <?> close · <q>         │
        ╰───────────────────────────────────────────────────────────────╯
```

### 16.12. SCR-903 Confirm — unmask 다이얼로그 (golden)

**파일 경로 제안:** `internal/tui/overlay/testdata/golden/confirm_unmask.txt`

```
        ╭───────────────────────────────────────────────────────────────╮
        │  Unmask PII field · mobilePhone                               │
        ├───────────────────────────────────────────────────────────────┤
        │                                                               │
        │  This will reveal the full value on screen for the current    │
        │  session. Others looking at your terminal will see it.        │
        │                                                               │
        │  Type `unmask` to confirm · <Esc> cancel                      │
        │                                                               │
        │  > _                                                          │
        │                                                               │
        ╰───────────────────────────────────────────────────────────────╯
```

### 16.13. 골든 파일 변환 약속

NO_COLOR 골든 파일은 다음 변환을 통과해야 한다 (test-engineer 합의 필요):

1. `lipgloss.SetColorProfile(termenv.Ascii)` 또는 동등한 환경 변수(`NO_COLOR=1`, `CLICOLOR=0`)로 색을 제거.
2. `regexp.MustCompile("\x1b\\[[0-9;]*m").ReplaceAllString(view, "")` 헬퍼로 잔존 ANSI escape 제거.
3. **trailing whitespace는 보존** (테이블 cell padding이 의미를 가지므로). 단, 라인 끝 직후의 진짜 trailing은 제거 가능.
4. 박스 외곽 (`╭─╮│╰─╯├┤`) 문자는 유지. ASCII 모드(`--ascii-fallback` 또는 `LC_ALL=C`)에서는 `+-+|+-++-+|+-+`로 fallback (별도 골든: `*_ascii.txt`).

---

## 17. Error Surfacing 명세

PRD §7.7의 8종 errorCode가 화면에 어떻게 보여야 하는지를 정의한다. 모든 에러 메시지는 `internal/errormap.UserMessage(err)`에서 생성된 사용자 친화 문구를 사용하며, **이 절은 그 문구를 어디에 어떻게 그릴지**를 다룬다.

### 17.1. 에러 표시 모드 3종

| 모드 | 위치 | 사용 시점 |
|------|------|----------|
| **Inline Error Panel** | MainBody 중앙 (테이블 자리에 대체) | 화면 진입 시 list/detail fetch가 완전 실패했을 때 |
| **Banner** | MainBody 상단 (테이블 위 1~2줄) | 부분 실패 — 주요 데이터는 있으나 보조 데이터 실패 (예: User Detail 진입 후 Groups 탭만 403) |
| **Toast** | StatusBar 위 1줄, 3초 후 페이드 | 사용자 액션 결과 (refresh 실패, copy 실패) |
| **Overlay (SCR-904)** | 모달 | `:errors` 명령 또는 toast 클릭 시 풀 메시지 |

### 17.2. errorCode → 표시 매트릭스

| Code | HTTP | 표시 모드 | UserMessage | 추가 액션 라인 |
|------|------|-----------|-------------|----------------|
| `E0000001` | 400 | Inline (search 시 Banner) | "Validation failed: <field>: <reason>" (errorCauses 파싱) | `<R> retry · <:errors> history` |
| `E0000004` | 401 | Inline (entire screen) | "API token invalid or revoked. Rotate and retry." | `<:about> token info · <q> quit` |
| `E0000006` | 403 | Inline (해당 리소스만) | "Insufficient permissions for `<resource>`. Token may be Read-Only or lack scope." | `<R> retry · <:about> · <:errors>` |
| `E0000007` | 404 | Toast + auto-refresh | "Resource not found. Refreshing list..." | (auto-action — 사용자 입력 불요) |
| `E0000011` | 401 | Inline (entire screen) | "Token expired or revoked. Refresh your token and restart." | `<:about> · <q> quit` |
| `E0000022` | 400 | Toast (info) | "Deactivate before deleting (write actions not available in MVP)." | (informational) |
| `E0000038` | 400 | Inline (해당 영역만) | "This feature is disabled for your organization. Contact your Okta administrator." | `<:about> · <Esc> back` |
| `E0000047` | 429 | Banner (RL paused) + Header `[RL: limited]` | "Paused · Rate limited on `<resource>` · Resuming in `<N>s`" | (auto-recovery, retry-after countdown) |
| `NETWORK` | — | Inline (offline mode) | "offline — network unreachable. Cached data from `<N>m` ago shown." | `<R> retry when online` |
| `UNKNOWN` | — | Toast + log | "Unknown error (logged). See `:debug open` for details." | `<:errors>` |

### 17.3. 시각 토큰

| 모드 | 보더 | 색상 | 아이콘 | 예시 |
|------|------|------|--------|------|
| Inline error | 없음 (MainBody 안 textual) | `tokens.Danger` (Bold for code+title) + `tokens.FG` (body) + `tokens.Muted` (action hint) | `[X]` (NO_COLOR: `[X]`) | §16.4 |
| Banner | 없음, prefix `▸` | `tokens.Danger` (text) | `⚠` / `[!]` | (e.g. R03 INVALID 배너) |
| Toast (error) | 없음, 1줄 | `tokens.Danger` (BG) + `tokens.BG` (FG) | `✗` / `[X]` | StatusBar 위 |
| Toast (info) | 없음 | `tokens.Info` (FG) | `ⓘ` / `(i)` | |
| RL paused banner | 없음 | `tokens.Warning` (text) | `⏸` / `\|\|` | "Paused · Rate limited..." |

### 17.4. RateLimitedError 카운트다운

- 표시 위치: MainBody 상단 banner
- 갱신 주기: 1초 (tea.Tick)
- 포맷: `"⏸ Paused · Rate limited on /users · Resuming in 8s..."` → 매 초 `8s → 7s → 6s ...`
- 0초 도달: banner 즉시 제거 + 자동 retry 시작 + spinner

**구현 힌트:**
```go
type rlCountdownTickMsg struct{ remaining time.Duration }
func rlCountdownTick(d time.Duration) tea.Cmd {
    return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
        return rlCountdownTickMsg{remaining: d - time.Second}
    })
}
```

### 17.5. 에러 우선순위 (동시 발생)

여러 에러가 동시에 발생할 때 노출 우선순위:

1. **Token 무효 (401)** → Inline (전체 차단), 다른 에러 무시
2. **Rate limit (429)** → Banner + countdown, 본문 fetch는 대기
3. **403 권한 부족** → 해당 리소스만 Inline / 다른 화면은 영향 없음
4. **404** → Toast + auto-refresh
5. **400 validation** → Search 시 Banner / 다른 경우 Inline
6. **NETWORK** → 모든 fetch 실패 시 entire screen offline 모드

---

## 18. Testability Guide (test-engineer를 위한 권고)

### 18.1. ANSI escape 제거

색상 검증은 별도 trace, 골든은 항상 plain text 비교:

```go
// internal/tui/testutil/strip.go
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func StripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// 또는 Lip Gloss profile 강제:
import "github.com/charmbracelet/lipgloss"
import "github.com/muesli/termenv"
func init() {
    if os.Getenv("NO_COLOR") != "" {
        lipgloss.SetColorProfile(termenv.Ascii)
    }
}
```

### 18.2. Static View 테스트 (골든 비교)

teatest로 화면 전이 흐름을 검증하되, **각 정적 상태의 시각 비교는 model.View() 결과를 직접 골든 파일과 비교**한다:

```go
func TestUsersList_DefaultGolden(t *testing.T) {
    t.Setenv("NO_COLOR", "1")
    m := users.NewListModel(users.Deps{
        InitialUsers: testFixtures.Users,
        Width: 120, Height: 30,
        Clock: clock.Frozen(time.Date(2026,4,24,12,0,0,0,time.UTC)),
    })
    got := testutil.StripANSI(m.View())
    want := testutil.ReadGolden(t, "testdata/golden/list_default.txt")
    if diff := cmp.Diff(want, got); diff != "" {
        t.Fatalf("View mismatch (-want +got):\n%s", diff)
    }
}
```

**골든 업데이트 메커니즘:** `go test -update-golden ./...` 플래그로 expected 파일 재생성. CI에서는 사용 금지 (`-update-golden`이 set이면 fail).

### 18.3. Color/Style trace 테스트 (선택적, NO_COLOR 외)

색상이 제대로 적용되었는지를 검증하려면 ANSI escape 자체를 검사:

```go
func TestUsersList_ActiveStatusIsGreen(t *testing.T) {
    m := users.NewListModel(...)
    view := m.View()  // NO_COLOR 미설정
    // ANSI escape "\x1b[32m" (green) 또는 truecolor "\x1b[38;2;..."가 ACTIVE 라인에 있어야 함
    activeLine := lineContaining(view, "alice@acme.com")
    if !strings.Contains(activeLine, "\x1b[") {
        t.Fatal("expected ANSI color codes for ACTIVE row")
    }
}
```

### 18.4. 레이아웃 검증

```go
import "github.com/charmbracelet/lipgloss"
view := m.View()
gotW := lipgloss.Width(view)   // 시각 폭 (ANSI 제외)
gotH := lipgloss.Height(view)
if gotW != 120 || gotH != 30 {
    t.Fatalf("expected 120x30, got %dx%d", gotW, gotH)
}
```

### 18.5. teatest는 흐름 전용

teatest는 **인터랙션 시퀀스 검증**(키 입력 → 상태 전이 → 다음 화면) 전용으로만 쓴다. teatest의 `golden` 비교는 변동성 큰 frame을 캡처하므로 사용하지 않는다. 대신 위 §18.2의 정적 view 골든을 쓴다.

### 18.6. 시계·랜덤·외부 의존 frozen

```go
// internal/clock/clock.go
type Clock interface { Now() time.Time }

func Frozen(t time.Time) Clock { return frozenClock{t} }
```

골든 테스트에서는 항상 frozen clock + frozen UUID/random 사용. relative time (`2h ago`)은 `now - lastLogin`을 frozen clock 기준으로 계산.

---

## 19. 변경 이력 (문서 자체)

| 날짜       | 버전         | 변경점                                                | 작성자       |
|------------|--------------|-------------------------------------------------------|--------------|
| 2026-04-24 | 0.1.0-draft  | 최초 초안 작성, pm+okta 리뷰 요청                    | tui-designer |
| 2026-04-24 | 1.0.0        | pm MAJOR 4 + okta MAJOR 2 + MINOR 11 전면 반영. team-lead M5 결정 (PRD §7.7이 소스 오브 트루스) 반영. docs/TUI_DESIGN.md로 확정. | tui-designer |
| 2026-04-24 | 1.1.0        | **Phase 6d 시각 충실도 사이클.** §15 Renderable Reference Specs 추가 (Lip Gloss 토큰 매핑, 컴포넌트 트리, 보더 스타일, 컬럼 정의), §16 Golden Snapshots 12개 추가 (NO_COLOR 모드, 9 화면 + 3 overlay), §17 Error Surfacing 명세 추가 (PRD §7.7 8종 errorCode → 표시 모드 매트릭스, RateLimit countdown), §18 Testability Guide 추가 (ANSI strip 헬퍼, golden 비교 방식, layout 검증). 와이어프레임은 그대로 유지 — §15 이후 절은 보강. | tui-designer-2 |

---

**END OF TUI_DESIGN v1.1.0**

*v1.0.0의 와이어프레임 + 키맵을 그대로 두고, 시각 충실도 보강을 위해 §15~§18을 추가. developer는 §15의 토큰/컴포넌트 매핑을, test-engineer는 §16의 골든과 §18의 헬퍼를, qa는 §17의 표시 매트릭스를 각각 검수 기준으로 삼는다.*
