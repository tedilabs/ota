# TUI Design Addendum — Users Profile Edit Form (SCR-012)

**상태:** DRAFT
**버전:** 1.3.0-draft (TUI_DESIGN v1.2.0+c 위에 addendum)
**작성일:** 2026-06-17
**작성자:** tui-designer (ota-prd-team)
**근거 PRD:** `docs/PRD.md` v1.1.0 §5.6 REQ-W01 (Users 프로필 편집 폼)
**도메인 레퍼런스:** `_workspace/edit-form-users/02_okta_domain_input.md`
**상위 입력:** `_workspace/edit-form-users/00_input.md`, `_workspace/edit-form-users/02_pm_prd_draft.md`

---

## 0. 통합 가이드 (기존 TUI_DESIGN.md에 어떻게 끼워 넣는가)

| 작업 | 위치 |
|------|------|
| §3.4 팔레트에 `:edit` / `:e` 추가 | docs/TUI_DESIGN.md §3.4 표 끝 (또는 `:unmask` 위 인접) |
| §3.6 Detail 단축키에 `e` 행 추가 | §3.6 표 끝 |
| §3.7 충돌 검사 표에 `e` 행 추가 | §3.7 표 끝 |
| **§4 화면 카탈로그에 `SCR-012 User Edit Form` 신설** | docs/TUI_DESIGN.md §4 — SCR-011 직후 |
| §10.1 위험 동작 표에 "Discard N changes (form ESC)" L1 추가 | §10.1 |
| §10.2/§10.3 예제는 신규 폼 시각 명세에 흡수 (form 자체에 영향 표시) | — |
| §11.1/§11.2 REQ-ID 매핑 매트릭스에 REQ-W01 행 추가 | §11.1 또는 §11.2 끝 |
| §12.1 키 충돌 검증 표에 `e` 행 갱신 | §12.1 |
| §12.3 Reserved 목록에서 `e` 항목 제거 (이제 점유) | §12.3 |
| §13 결정 테이블에 항목 #9 추가 (Edit form 모달 모드, OI-W5 위젯 추상화) | §13 |
| §14 오픈 이슈에서 OI-W3/OI-W5 결과 반영 | §14 |
| §19 변경 이력 한 줄 추가 | §19 |
| §0.2 "MVP는 mutation 없음" 원칙 정정 — Profile-Edit 한정 mutation 허용 명시 | §0.2 |
| 글로벌 §3.2/§3.3 영향 없음 (form 내에서는 비활성 — `j/k`는 글자 입력) | — |

> **버전 정책:** v1.3.0-draft. 기존 read-only 화면(SCR-010/011/020/...)의 와이어프레임·키 바인딩은 변경 없음. **하위 호환 100%**. 단 §0.2 "모든 mutation은 없는 것처럼" 원칙은 "v0.2.0 Profile-Edit 한정 mutation"으로 정밀화한다 (도메인 입력 §0 + PRD §5.6).

---

## 1. 디자인 결정 요약 (PM 확정 사항 우선 적용)

PRD addendum의 **D-W1~D-W16 + AC-1~AC-10**을 TUI에 1:1 매핑한다. 디자이너 추가 권고는 §6에 모음.

| PRD 결정 | TUI 반영 |
|---------|---------|
| D-W1 11필드 | §2.4 폼 본문 11개 입력 슬롯, 4개 섹션으로 그룹화 |
| D-W2 login read-only | §2.4 read-only 행 (입력 박스 없이 평문 + lock 아이콘) |
| D-W4 dirty ESC 1단계 confirm | §3 상태머신 `confirmDiscard`, §5 Discard 모달 |
| D-W5 저장 키 `Ctrl+S` | §3.6a Form 단축키 표 — `Ctrl+S` 전역, `Enter` on Save 버튼 |
| D-W6 실패 시 폼 유지 | §4 상태머신, §6 에러 표시 |
| D-W7 진입 시 latest GET 1회 | §3 상태 `loading` |
| D-W10 N changes + 필드 `*` | §2.4 footer + 라벨 prefix |
| D-W13 빈 패치 차단 | §3.6a 저장 키는 dirty=0일 때 disable + footer hint |
| D-W16 모달/오버레이 모드 | §4 push 형식, navStack 활용 |
| AC-7 PII 마스킹/언마스킹 | §2.5 진입 시 mask + focus auto-unmask + `m` 토글 |
| AC-3 클라이언트 검증 | §6.1 검증 패턴 — inline error + footer 누적 |
| AC-6 errorCauses 파싱 | §6.2 매트릭스 — field prefix 매칭 → 해당 input 위 inline |

---

## 2. 화면 정의 — SCR-012: User Edit Form

### 2.1. 목적

운영자가 선택된 사용자의 standard profile 11개 필드를 키보드만으로 수정·저장한다. ota의 **첫 mutation 표면**이며 v0.2 lifecycle mutation의 위젯 인프라(form widget·dirty·inline error·discard confirm) 모범 구현체다.

### 2.2. 진입 경로

| 출발 화면 | 트리거 | 비고 |
|----------|--------|------|
| SCR-010 Users List | 선택 행에서 `e` | AC-1.1 |
| SCR-011 User Detail (모든 탭: Pretty/JSON/YAML — Profile/Credentials/Timestamps/Groups/Factors/Recent 탭으로 v0.1.2에서 통합) | `e` | AC-1.2 |
| 어디서나 | `:edit` 또는 `:e` (활성 화면이 user를 selected 보유 시) | §3.6a |
| 활성 화면이 user를 보유하지 않음 (예: Logs) | `:edit` | 토스트 "no user selected" — 기존 `openActionConfirm` 패턴 재사용 |

> **navStack 동작 (D-W16):** edit 진입 시 `pushNav(ScreenUserEdit)` — 기존 SCR-011 navStack에 SCR-012를 push. ESC pop 시 진입 직전 화면(SCR-010 또는 SCR-011)으로 복귀. v0.2.5의 nav stack 패턴(commit `b0794ad`)을 그대로 활용. 새 ScreenKind `ScreenUserEdit` 추가 필요.

### 2.3. 상태머신

```
       Entry (e from list/detail or :edit)
              │
              ▼
       ┌─────────────┐
       │  loading    │  ← GET /api/v1/users/{id} (AC-1.3)
       │  (snapshot) │     스피너 + "Loading user…" + Esc로 abort
       └──────┬──────┘
              │ success (200)
              ▼
       ┌─────────────┐
       │   editing   │  ← 사용자 입력
       │ (clean→dirty)│     keystroke마다 snapshot vs current diff
       └──┬───┬──────┘
          │   │
          │   │ Ctrl+S (dirty>0 AND validation pass)
          │   │
          │   ▼
          │   ┌─────────────┐
          │   │   saving    │  ← POST /api/v1/users/{id}
          │   │             │     input/Esc disable, Ctrl+C만 abort
          │   └──┬───┬───┬──┘
          │      │   │   │
          │      │   │   │ 200 OK
          │      │   │   ▼
          │      │   │   ── Exit (closing) ──
          │      │   │   캐시 갱신 + popNav() + 토스트 "Updated <login>"
          │      │   │   (AC-4.5)
          │      │   │
          │      │   │ 4xx/5xx (except 404)
          │      │   ▼
          │      │   ┌─────────────┐
          │      │   │  editing    │  ← 폼 유지, inline/footer error 표시
          │      │   │  (errored)  │     변경값 보존 (AC-6, D-W6)
          │      │   └─────────────┘
          │      │
          │      │ 404
          │      ▼
          │      ── Exit (popNav) + toast "User no longer exists" + list refresh
          │      (AC-6.4)
          │
          │ Esc
          ▼
       ┌─────────────┐
       │  dirty?     │
       └──┬──────┬───┘
          │ no   │ yes
          ▼      ▼
       Exit   ┌──────────────┐
       (pop)  │ confirmDiscard│  ← "Discard N changes? [y/N]"
              └──┬───────┬────┘
                 │ N/Esc │ y/Y/Enter on Yes
                 │       │
                 ▼       ▼
              editing  Exit (pop, discard input)
```

**전이 매트릭스:**

| 시작 상태 | 입력 | 다음 상태 | 비고 |
|----------|------|----------|------|
| loading | success | editing (clean) | snapshot 저장 |
| loading | error 4xx/5xx | -- (return) | 폼 자체를 열지 않음. 직전 화면 유지 + 토스트. (AC-1.5) |
| loading | Esc | -- (return) | abort + 직전 화면 |
| editing (clean) | keystroke (field 변경) | editing (dirty) | dirty 카운터 증가 |
| editing (dirty) | keystroke (값을 snapshot으로 복원) | editing (re-clean 가능) | 모든 필드 snapshot 일치 → dirty 0 |
| editing (any) | Tab / Shift-Tab | editing (focus 이동) | §3.6a |
| editing (any) | `m` | editing + PII 토글 | AC-7.5 |
| editing (any) | `Ctrl+S` (dirty>0, valid) | saving | 진행 |
| editing (any) | `Ctrl+S` (dirty=0) | editing | footer hint "No changes to save" (D-W13) |
| editing (any) | `Ctrl+S` (validation fail) | editing | inline error, focus 첫 invalid 필드 |
| editing (clean) | Esc | Exit (pop) | 즉시 |
| editing (dirty) | Esc | confirmDiscard | (AC-5.2) |
| confirmDiscard | y / Y / Enter | Exit | 변경 폐기 |
| confirmDiscard | n / N / Esc | editing | 폼 유지 |
| saving | 200 | Exit + 토스트 + 캐시 갱신 | AC-4.5 |
| saving | 400/401/403/5xx | editing (errored) | 폼 유지, 변경값 보존 |
| saving | 404 | Exit + 토스트 + list refresh | AC-6.4 |
| saving | 429 | editing (rate-limited) | Retry-After 카운트다운, 자동 1회 재시도 (AC-6 표) |
| saving | Ctrl+C | editing | ctx cancel, 변경값 보존 (AC-4.3) |
| saving | Esc | (no-op) | footer hint "Saving… use Ctrl+C to abort" (AC-5.3) |

### 2.4. 와이어프레임 (120x30, 표준 모드, editing 상태)

```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.2.0 ─┐
│ Users › alice@acme.com › Edit                       3 changes  id: 00u…x8 │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  ─ Identity ──────────────────────────────────────────────                │
│    Login (read-only)   alice@acme.com                       🔒 :change-login │
│  * First Name          [ Alicia_________________________ ]  ← cursor       │
│    Last Name           [ Smith __________________________ ]                │
│    Display Name        [ Alice Smith ____________________ ]                │
│    Nickname            [ ali ____________________________ ]                │
│                                                                            │
│  ─ Contact ───────────────────────────────────────────────                │
│    Email *             [ alice@acme.com _________________ ]                │
│    Mobile Phone        [ +1-***-***-1234 ________________ ] (masked, m)    │
│    Secondary Email     [ a***@personal.com ______________ ] (masked, m)    │
│                                                                            │
│  ─ Organization ──────────────────────────────────────────                │
│  * Department          [ Eng → Platform__________________ ]                │
│    Division            [ R&D ____________________________ ]                │
│    Title               [ Senior SWE _____________________ ]                │
│  * Employee Number     [ ENG-042 ________________________ ]                │
│                                                                            │
│  ─ Status (read-only) ────────────────────────────────────                │
│    status              ● ACTIVE   (Use :activate/:deactivate to change)   │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ 3 changes · <Ctrl+S> save  <Tab> next  <m> toggle PII  <Esc> cancel        │
└────────────────────────────────────────────────────────────────────────────┘
```

**시각 규약:**
- `*` 라벨 prefix: 변경된 필드 (dirty marker, AC-9.2, NO_COLOR에서도 식별)
- `🔒` 또는 ASCII fallback `[ro]`: read-only 필드 표시
- 좌측 padding 2 columns, 라벨 18 cols, 입력 박스는 가용 폭 흡수
- 입력 박스 좌우 `[ ` / ` ]` 보더 — 포커스 시 색상 토큰 `tokens.Accent` 굵게, 좌측 `▸ ` prefix
- `(masked, m)` 트레일: PII 필드 마스킹 상태일 때만 표시 (AC-7.1)
- 섹션 헤더 `─ <Name> ────`: `tokens.Header`, 약한 hr-style divider

**필드 → API field 매핑 (D-W1 11필드 + login read-only):**

| 라벨 | API field | 그룹 | 필수 (default) | PII | 진입 마스킹 |
|------|-----------|------|---------------|-----|-------------|
| Login (read-only) | `profile.login` | Identity | — | — | — |
| First Name | `profile.firstName` | Identity | YES | — | — |
| Last Name | `profile.lastName` | Identity | YES | — | — |
| Display Name | `profile.displayName` | Identity | — | — | — |
| Nickname | `profile.nickName` | Identity | — | — | — |
| Email | `profile.email` | Contact | YES | — | — |
| Mobile Phone | `profile.mobilePhone` | Contact | — | **YES** | **YES** |
| Secondary Email | `profile.secondEmail` | Contact | — | **YES** | **YES** |
| Department | `profile.department` | Organization | — | — | — |
| Division | `profile.division` | Organization | — | — | — |
| Title | `profile.title` | Organization | — | — | — |
| Employee Number | `profile.employeeNumber` | Organization | — | △ | — |
| status (read-only badge) | — | Status | — | — | — |

> **필드 순서 결정 근거:** Identity → Contact → Organization → Status (read-only). 운영자의 멘탈 모델("누구인가 → 어떻게 연락하나 → 어디에 속하나")과 일치하며, PII는 Contact 섹션에 묶여 시각적 경계가 명확.

### 2.5. 반응형 — 좁은 터미널 (80x24, 최소 모드)

```
┌─ ota · acme.okta.com · prod  [RL: ok]  UTC v0.2.0 ─┐
│ Users › alice@acme.com › Edit            3 changes  │
├──────────────────────────────────────────────────────┤
│  ─ Identity ──────────────────                       │
│    Login (ro)   alice@acme.com                       │
│  * First Name                                        │
│    [ Alicia____________________________________ ]    │
│    Last Name                                         │
│    [ Smith _____________________________________ ]   │
│    Display Name                                      │
│    [ Alice Smith _______________________________ ]   │
│    Nickname                                          │
│    [ ali _______________________________________ ]   │
│  ─ Contact ───────────────────                       │
│    Email *                                           │
│    [ alice@acme.com ____________________________ ]   │
│    Mobile Phone (masked, m)                          │
│    [ +1-***-***-1234 ___________________________ ]   │
│    ...                                               │
│                                                      │
├──────────────────────────────────────────────────────┤
│ 3 changes <Ctrl+S> save <Tab> nav <Esc>             │
└──────────────────────────────────────────────────────┘
```

**좁은 모드 규약 (AC-8.3):**
- 라벨과 입력 박스를 **2줄로 분리** (label 위, input 아래).
- 푸터 hint 축약: `<m> toggle PII` 생략, `<?>`로 도움말 안내. NO 색상이면 prefix `> ` 사용.
- 폭 < 80은 §1.2의 "지원 안 함" 화면.

### 2.6. 반응형 — 짧은 터미널 (H < 30)

- 폼 본문이 화면을 넘으면 **viewport 스크롤** (`bubbles/viewport`). 포커스가 화면 밖 필드로 이동하면 자동 스크롤하여 가시 영역에 포커스를 유지.
- 섹션 헤더는 sticky하지 않음 (viewport 단순화). 현재 섹션은 footer에 표시: `Identity · 3/11 fields · 3 changes`.

### 2.7. 컴포넌트 트리

```
edituser.Model.View()
└── HeaderBar (글로벌)
    TitleBar:  ota · acme · prod · [RL: ok] · UTC · v0.2.0
    ContextBar:
      Breadcrumb: tokens.Header.Render("Users › <login> › Edit")
      Spacer
      Counter:  tokens.Accent.Render(fmt.Sprintf("%d changes", dirty))  // 0이면 생략
      Meta:     tokens.Muted.Render("id: " + shortID)
└── MainBody (viewport-wrapped)
    └── FormBody
        └── Sections[0..3] (Identity / Contact / Organization / Status)
            └── SectionHeader   (tokens.Header + divider rune)
            └── Rows[i]
                ├── LabelCell   (18 cols, 우측 정렬 옵션)
                │     prefix: "*" if dirty else " "
                │     suffix: " *" if required else ""
                ├── ValueCell
                │     ── editable: BoxedTextInput (bubbles/textinput)
                │     ── readonly: Plain text + " 🔒" or " (ro)" trailing
                ├── HintCell     (right-side, optional)
                │     "(masked, m)" or ":change-login" or "(read-only)"
                └── InlineErrorRow  (only when error)
                      "  ! Invalid email format"  in tokens.Danger
        └── FormFooter (only when any error not field-attached)
              "Other errors:" + collapsible list
└── StatusBar (글로벌)
    KeyHints (state-dependent — see §3.6a)
    Trailing toast (3s, AC-6.3)
```

### 2.8. Bubble 컴포넌트 매핑

| 디자인 슬롯 | Bubble 컴포넌트 | 비고 |
|------------|----------------|------|
| 각 입력 박스 | `bubbles/textinput` | 11개 인스턴스. focus 토글은 `Focus()/Blur()`. Placeholder = `""` (current value 직접 SetValue). |
| 폼 본문 스크롤 | `bubbles/viewport` | H < 30에서만 활성. 그 외에는 직접 lipgloss compose. |
| 저장 진행 표시 | `bubbles/spinner` | `spinner.Dot`. footer에 `⠋ Saving…` |
| 폼 footer hint | `bubbles/help` | 기존 패턴과 통일. |
| Discard confirm 모달 | 신규 lightweight modal (lipgloss render) | OverlayActionConfirm 패턴 재사용하되 별도 OverlayDiscardConfirm 추가. y/N 단일 키. |
| 카운트다운(429) | 직접 lipgloss + tea.Tick | `Retrying in 5s…` |

> **`huh` 폼 미사용 권고**: huh.Form은 step-by-step 또는 single-page 전체 폼을 자체 chrome으로 그린다. ota는 (1) PRD §6.4 일관된 chrome, (2) PII 마스킹 토글, (3) dirty 추적, (4) ad-hoc inline error 표현이 필요하므로 huh.Form의 abstraction이 오히려 방해. 11개 `bubbles/textinput` + 자체 폼 위젯이 정답. (§6 OI-W5 결정과 일관.)

---

## 3. 단축키 정의

### 3.1. 글로벌 단축키 영향 분석

기존 §3.2 글로벌 nav (j/k/Ctrl-d/Ctrl-u 등)는 **form에서 비활성**: 텍스트 입력 모드에서는 모든 글자가 입력 버퍼로 흐름. 이는 §3.1 기존 정책("텍스트 입력 포커스 중에는 숫자 아닌 키를 입력하면 해당 키가 입력 버퍼로 흐른다")과 일관.

**예외 — 항상 활성:**
- `Esc` (cancel / discard)
- `Ctrl+C` (hard quit — saving 중 abort)
- `Ctrl+S` (save) — bubbles/textinput은 `Ctrl+S`를 입력으로 소비하지 않으므로 safe
- `Tab` / `Shift+Tab` (focus 이동)
- `Ctrl+L` (강제 재렌더 — 글로벌 §3.1)

**글로벌이지만 form에서 차단되는 키:**
- `:` (palette) — form 입력 중에는 글자로 입력. 운영자가 명시적으로 form을 벗어나야 (`Esc` → list/detail에서 `:`) palette 사용. ※ form에서 palette를 열고 싶으면 v0.2 추가 검토.
- `?` (help) — 동일. form 내부 도움말은 footer에 항상 노출.
- `/` (search) — 동일. form에서 의미 없음.

### 3.2. 폼 진입/종료 (글로벌 §3.6 Detail 단축키 표에 추가)

| 키 | 동작 | 적용 화면 | 설정 ID |
|----|------|----------|--------|
| `e` | User Edit Form 진입 (SCR-012) | SCR-010 (선택 row), SCR-011 (어느 탭이든) | `action.edit` |
| `:edit` / `:e` | 동일 진입 (active screen이 user 보유 시) | 어디서나 | `palette.edit` |

> **충돌 검증 (§3.7 갱신):** `e` 키는 v1.2.0까지 `action.expand` (Factors 탭에서 id 펼침)로 예약되었으나, SCR-011 v0.1.2에서 탭 구조가 Pretty/JSON/YAML로 통합되면서 `action.expand`는 미사용 상태. v0.2 본 addendum에서 `e` → `action.edit`로 **단일 의미 점유**. `(e) expand` 표기는 §15.7 (Factors 와이어프레임)에서 v1.2.0+d 후속 cleanup으로 제거. — Phase 4 개발 시 confirm 필요.

### 3.3. 폼 내 단축키 — Editing 상태 (SCR-012 전용)

| 키 | 동작 | 비고 |
|----|------|------|
| `Tab` | 다음 필드로 focus (read-only 필드는 skip) | wrap-around: 마지막 필드 → 첫 필드 |
| `Shift+Tab` | 이전 필드로 focus | |
| `↑` / `↓` | 인접 필드로 focus (Tab과 동일하나 row-wise) | bubbles/textinput이 텍스트 내부 커서 이동에 ↑/↓를 쓰지 않으므로 안전 |
| `←` / `→` | 입력 박스 내 커서 이동 | textinput 기본 동작 |
| `Home` / `End` | 입력 박스 시작/끝 | textinput 기본 |
| `Ctrl+a` / `Ctrl+e` | 입력 박스 시작/끝 (readline) | textinput 기본 |
| `Ctrl+w` | 단어 단위 삭제 (readline) | textinput 기본 |
| `Ctrl+u` | 입력 박스 전체 비우기 (readline) | textinput 기본 |
| `Ctrl+k` | 커서 우측 삭제 (readline) | textinput 기본 |
| `Ctrl+S` | save (D-W5) | dirty=0이면 footer "No changes to save" (D-W13); validation fail이면 첫 invalid 필드로 focus + inline error |
| `Esc` | clean이면 즉시 닫기, dirty면 confirmDiscard 모달 | AC-5 |
| `m` | 모든 PII 필드 일괄 mask/unmask 토글 | AC-7.5. textinput 포커스 시에도 작동하도록 form-level Update에서 가로채기 — `Alt+m`을 fallback alias로 예약 (충돌 시) |
| `Ctrl+C` | (saving 상태일 때만) abort + 입력 보존 | AC-4.3 |
| `Enter` | (포커스가 Save 버튼일 때) save 트리거 | 보조 entry (D-W5) — v0.1에서는 Save 버튼 미구현 시 입력 박스 내 Enter는 no-op |

**`m` 충돌 대응:** `m`은 영문자이므로 textinput이 입력으로 소비한다. **form-level Update가 textinput 위에서 가로채기**:
- 옵션 A (권장): `Alt+m` 또는 `Ctrl+M` 으로 토글. 단 `Ctrl+M`은 일부 터미널에서 Enter와 혼동 → **`Alt+m` 채택**. footer hint에는 `<Alt+m>`으로 표기.
- 옵션 B (대안): `:unmask` / `:mask` 팔레트 명령 사용 권장 안내. 단 palette는 form에서 비활성이므로 form 밖으로 나가야 함 → 동선 길어짐.
- **최종 권고:** `Alt+m`을 채택. PII 토글이 textinput 입력과 충돌하지 않게 글로벌 form intercept로 처리. footer에 `<Alt+m> toggle PII` 표기.

### 3.4. 폼 footer (KeyHints) — 상태별

**clean (no changes):**
```
<Ctrl+S> save  <Tab> next  <Alt+m> toggle PII  <Esc> close
```

**dirty (N changes, valid):**
```
3 changes · <Ctrl+S> save  <Tab> next  <Alt+m> toggle PII  <Esc> cancel
```

**dirty + validation failed:**
```
3 changes · ! fix 2 invalid fields  <Tab> next  <Esc> cancel
```

**saving:**
```
⠋ Saving… use <Ctrl+C> to abort
```

**rate-limited (429):**
```
⚠ Rate limited · retrying in 5s…  <Esc> abort retry (preserves input)
```

**after save error (editing, errored):**
```
✗ Save failed — see field errors  <Ctrl+S> retry  <Esc> cancel
```

> 6 키 규칙 (§3.8): clean 상태에서 footer는 4 키. dirty 상태도 4 키. 모든 상태에서 6 키 한도 준수.

### 3.5. Discard confirm 모달 (SCR-903 패턴 확장)

```
       ╔════════════════════════════════════════╗
       ║  Discard 3 unsaved changes?            ║
       ╠════════════════════════════════════════╣
       ║                                        ║
       ║  Modified fields:                      ║
       ║    • First Name                        ║
       ║    • Department                        ║
       ║    • Employee Number                   ║
       ║                                        ║
       ║  <y> discard · <n> keep editing · <Esc>║
       ║                                        ║
       ╚════════════════════════════════════════╝
```

- 디폴트: **N (keep editing)** — Esc도 동일 (안전 우선, D-W4)
- `y`, `Y` → discard + close
- `n`, `N`, `Esc` → return to editing
- `Enter` → no-op (실수 confirm 방지). 명시적 `y` 필요.

> **위험 등급 (§10.1):** L1 (단일 키 confirm). 변경값 소실은 되돌릴 수 있는 데이터(다시 입력 가능)이므로 L2(word typing) 불필요.

---

## 4. 상태별 UI (디테일)

### 4.1. Loading (진입 직후)

```
┌─ ota · acme.okta.com · prod  [RL: ok]  UTC v0.2.0 ───────────────────────┐
│ Users › alice@acme.com › Edit                              id: 00u…x8     │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│                                                                            │
│                   ⠋ Loading user…                                          │
│                     GET /api/v1/users/00u…x8                               │
│                     Press <Esc> to cancel                                  │
│                                                                            │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ ⠋ Loading…  <Esc> cancel                                                   │
└────────────────────────────────────────────────────────────────────────────┘
```

- 진입 시 GET이 < 100ms로 끝나면 placeholder 폼 (clean snapshot)을 즉시 표시.
- > 100ms 지연 시 위 loading 화면을 페이즈 인.
- `Esc`로 GET abort → return to 직전 화면 (AC-1.4).
- GET 실패 (AC-1.5): 폼을 열지 않음. 직전 화면 유지 + 토스트 "Failed to load user: <error>".

### 4.2. Editing (clean → dirty 전환)

snapshot vs current diff:
- 진입 직후: 모든 필드 prefix `  ` (공백 2). `0 changes`.
- 사용자가 First Name을 "Alice" → "Alicia"로 수정: prefix `* `, footer `1 change`.
- 같은 필드를 "Alicia" → "Alice"로 되돌림: prefix `  `, footer `0 changes`.
- AC-9 1:1 매핑.

### 4.3. Editing + Validation Errors (Ctrl+S 또는 focus-out 직후)

```
│  * First Name          [ _______________________________ ]                │
│  ! First Name is required                                                  │
│                                                                            │
│    Email *             [ alice@acme ____________________ ]                │
│  ! Invalid email format                                                    │
```

- inline error는 해당 행 바로 아래, `! ` prefix + `tokens.Danger`
- AC-6.2: 사용자가 해당 필드를 다시 수정하면 inline error 즉시 클리어
- footer: `3 changes · ! fix 2 invalid fields  <Tab> next  <Esc> cancel`
- Ctrl+S는 invalid가 있으면 첫 invalid 필드로 focus + inline error 펼침. 저장 호출은 발생하지 않음.

### 4.4. Saving

```
│  * First Name          [ Alicia_________________________ ]                │
│  (input disabled — all fields read-only during save)                       │
│  ...                                                                       │
├────────────────────────────────────────────────────────────────────────────┤
│ ⠋ Saving…   POST /api/v1/users/00u…x8     use <Ctrl+C> to abort           │
└────────────────────────────────────────────────────────────────────────────┘
```

- 모든 textinput은 `Blur()` + 시각적 dim (tokens.Muted)
- `Esc`는 비활성 + footer hint
- `Ctrl+C` → ctx cancel + `editing` 상태로 복귀 (변경값 보존)
- AC-4.3 1:1.

### 4.5. Save Success

- 폼 닫기 (popNav)
- 토스트 (3초, REQ-E02): `✓ Updated alice@acme.com` (tokens.Success)
- 캐시 갱신: 진입 직전 화면이 SCR-010이면 해당 row만 업데이트 (selected 유지). SCR-011이면 detail 새로고침 (AC-4.5).
- 1초간 같은 사용자에 대해 다시 `e`를 눌러도 GET을 재발사하지 않고 직전 응답 재활용. (AC-4.4 연속 저장 가드는 본질적으로 두 번째 클릭에서 해소되지만, 디자이너는 이를 1초 toast 동안 자연스럽게 표현.)

### 4.6. Save Failure — 400 Validation Error (E0000001)

```json
{
  "errorCode": "E0000001",
  "errorCauses": [
    { "errorSummary": "email: Email is not valid" },
    { "errorSummary": "department: Cannot exceed 100 characters" }
  ]
}
```

→ 폼 시각화:

```
│    Email *             [ alice@acme_____________________ ]                │
│  ! Email is not valid (server)                                             │
│                                                                            │
│  * Department          [ Very Long Department Name… _____ ]                │
│  ! Cannot exceed 100 characters (server)                                   │
│                                                                            │
├────────────────────────────────────────────────────────────────────────────┤
│ ✗ Save failed — 2 field errors   <Ctrl+S> retry  <Esc> cancel              │
└────────────────────────────────────────────────────────────────────────────┘
```

- AC-6.1 prefix 매칭으로 field 추출 → 해당 행 아래 inline
- 매칭 실패한 cause는 footer 아래 "Other errors:" 영역에 누적 (lipgloss collapsible — viewport 활용):
```
│ ✗ Save failed   Other errors: "Schema constraint failed: ..."             │
│                                                                            │
│   <Ctrl+S> retry  <Esc> cancel  <Tab> to first invalid                     │
```

### 4.7. Save Failure — 403 Permission Denied

```
├────────────────────────────────────────────────────────────────────────────┤
│ ✗ Insufficient permissions — 'Manage user profiles' required               │
│ <Ctrl+S> retry after token swap  <Esc> cancel                              │
└────────────────────────────────────────────────────────────────────────────┘
```

- inline error 없음 (필드 문제 아님)
- 토스트 + footer 양쪽 표시 (AC-6 403 행)
- 폼/변경값 보존 → 운영자가 새 토큰으로 ota 재시작 후 같은 변경을 다시 시도 가능. (실제로는 토큰은 환경변수로 주입되므로 ota 재시작 필요 — 본 폼이 토큰 교체 UX를 제공하지는 않음.)

### 4.8. Save Failure — 404

```
(폼 닫기)
직전 화면(SCR-010 또는 SCR-011) 복귀
토스트: ✗ User no longer exists. Refreshing list.
list 자동 refresh (RefreshScreenMsg 발송)
```

(AC-6.4)

### 4.9. Save Failure — 429 Rate-Limited

```
├────────────────────────────────────────────────────────────────────────────┤
│ ⚠ Rate limited · retrying automatically in 5s…   <Esc> abort retry        │
└────────────────────────────────────────────────────────────────────────────┘
```

- `Retry-After` 헤더 → 카운트다운 `5… 4… 3…`
- 카운트 0 → 자동 1회 재시도 (REQ-E01 AC-2 정책 일관)
- 재시도 실패 시 footer "Still rate limited. Retry manually with Ctrl+S or wait."
- `Esc`: 자동 재시도만 취소 (폼은 그대로). 변경값 보존.

### 4.10. Save Failure — 5xx

```
├────────────────────────────────────────────────────────────────────────────┤
│ ✗ Okta service error (502) — try again later   <Ctrl+S> retry  <Esc>      │
└────────────────────────────────────────────────────────────────────────────┘
```

- AC-6 정책: 폼 유지, 변경값 보존. 자동 재시도 없음.

---

## 5. PII 마스킹 통합 (AC-7)

| 상태 | mobilePhone 표시 | secondEmail 표시 |
|------|------------------|-------------------|
| 진입 시 (focus 다른 필드) | `+1-***-***-1234` | `a***@personal.com` |
| 사용자가 Tab으로 focus | **자동 unmask** → 전체 값 표시, 입력 박스 정상 동작 | 동일 |
| focus out + 미수정 | 다시 마스킹 | 다시 마스킹 |
| focus out + 수정 후 | 마스킹 없이 사용자 입력값 그대로 표시 + dirty marker `*` | 동일 |
| `Alt+m` 토글 | 모든 PII 일괄 mask/unmask 전환 | 동일 |
| 저장 후 | 폼 닫힘 → detail/list로 복귀 → 기본 mask 정책 |

**비주얼 단서 (NO_COLOR 호환):**
- 마스킹 상태: 입력 박스 우측에 `(masked, Alt+m)` 회색 hint
- 언마스킹 상태: 입력 박스 우측에 `[M!]` 빨간 배지 (기존 §7.2 `styleBadgeUnmask`)
- focus 시 자동 언마스킹은 [M!] 배지 없음 — focus 자체가 unmask 의도 표명 (운영자 인지 명확)

> **Logging (AC-7.6):** 디버그 로그는 항상 마스킹 값만 기록. 입력 keystroke은 stdout으로 흐르지 않음.

---

## 6. 검증 패턴 (AC-3 클라이언트 + AC-6 서버)

### 6.1. 클라이언트 측 (느슨, AC-3)

| 검증 | 시점 | 메시지 |
|------|------|--------|
| 필수 빈 값 (firstName/lastName/email) | (1) focus out (2) Ctrl+S 시 일괄 | `! This field is required` |
| 이메일 형식 (`*@*.*`) | (1) focus out (2) Ctrl+S 시 일괄 | `! Invalid email format` |
| 전화번호 hint (E.164 권장) | focus out 시 advisory만, 차단 안 함 | `ℹ Recommended: +<country><number>` (tokens.Muted) |

> AC-3.3 hint는 빨강(`!`)이 아닌 정보(`ℹ`) 색상. dirty 마커도 점등하지 않음.
> AC-3.4: 길이 차단 안 함. 서버 응답으로 inline error 띄움.

### 6.2. 서버 측 (엄격, AC-6)

| HTTP | errorCode | 표시 위치 | 비고 |
|------|-----------|----------|------|
| 400 | E0000001 | field prefix 매칭 → inline / fallback footer | AC-6.1 |
| 400 | E0000038 | footer "Schema constraint failed: ..." | 발생 빈도 낮음 |
| 401 | E0000011/E0000004 | footer + 토스트 "Token invalid/expired" | 변경값 보존 |
| 403 | E0000006 | footer + 토스트 "Insufficient permissions" | 변경값 보존 |
| 404 | E0000007 | 폼 닫기 + 토스트 + list refresh | 유일한 close 사유 |
| 429 | E0000047 | footer 카운트다운 + 자동 1회 재시도 | §4.9 |
| 5xx | 다양 | footer "Okta service error" + manual retry | §4.10 |

### 6.3. errorCauses 파싱 의사 코드

```
for cause in resp.errorCauses:
    text = cause.errorSummary
    if match := regex(`^(\w+):\s*(.+)$`).find(text):
        fieldKey = match[1]  // e.g. "email"
        msg      = match[2]  // e.g. "Email is not valid"
        if fieldKey in formFields:
            formFields[fieldKey].inlineError = msg
            continue
    otherErrors.append(text)
```

§17.2 (Error Surfacing 명세) 매트릭스와 일관성 검증.

---

## 7. 위험 동작 확인 패턴 (§10 확장)

### 7.1. §10.1 표 갱신

| 단계 | 이름 | 용도 | UX | 본 폼 적용 |
|------|------|------|----|---------|
| L1 | Soft confirm | 되돌림 쉬운 액션 | `y/n` 단일 키 | **Discard N changes (form ESC)** |
| L2 | Word confirm | 즉시 파급 | `yes` / `confirm` 타이핑 | (v0.2 lifecycle write) |
| L3 | Name confirm | 비가역 | 리소스 이름 타이핑 | (v0.2 deactivate rule) |

**Profile-Edit 자체는 L0** — 저장 자체는 별도 confirm 없음. 이유:
- D-W3 결정: email 변경에도 confirm 없음 (inline hint만)
- D-W12 결정: 권한 사전 검증 없음 → 저장 시도가 곧 검증
- 본 mutation은 partial-merge + last-write-wins이므로 저장 액션 자체가 명시적 의도 (Ctrl+S라는 비범한 키 + 명시적 변경) → 추가 confirm은 피로
- 폼의 dirty marker + `N changes` footer가 변경 인지를 충분히 노출

### 7.2. 영향 범위 inline 안내 (변경 의도 노출)

PRD §5.6 Side Effects → 폼 내 inline hint로 노출:

| 필드 변경 시 | inline hint (focus 시 또는 dirty 시 footer에 1회) |
|-------------|-----------------------------------------------|
| `email` | `ℹ Changing email may trigger notification per org settings.` |
| `mobilePhone` | `ℹ SMS MFA factors may require re-enrollment after change.` |
| `department` / `division` | `ℹ Group Rule may re-evaluate. Memberships could change.` |

- 표시 위치: 필드 박스 바로 아래 (inline error와 같은 슬롯, 단 색상은 `tokens.Info` 시안)
- 사용자가 해당 필드 dirty 상태일 때만 표시 (clean → 표시 안 함, 정보 과다 방지)
- 동시 복수 hint: 각각 별도 행. 우선순위: error > hint.

---

## 8. 명명·등록 — 새로운 식별자

### 8.1. 화면

| 식별자 | 값 | 사유 |
|--------|-----|------|
| ScreenKind | `ScreenUserEdit` | 기존 `ScreenUserDetail`과 짝. iota 순서: detail 직후. |
| Screen.String() | `"user-edit"` | palette autocomplete 표준형 — `:user-edit` |
| 단축키 alias | `e`, `:edit`, `:e` | |

### 8.2. Overlay

| 식별자 | 값 | 사유 |
|--------|-----|------|
| OverlayKind | `OverlayDiscardConfirm` | discard 전용. 기존 `OverlayActionConfirm`(lifecycle)과 분리 — pendingAction 구조 재사용 불가 (Form snapshot 보관 필요). |

### 8.3. 메시지 (internal/tui/shared/msgs.go 추가)

| 메시지 | 필드 | 의도 |
|--------|------|------|
| `OpenUserEditMsg` | `ID string` | List/Detail이 `e` 시 발송. App Shell이 `pushNav(ScreenUserEdit)` 트리거. (기존 `OpenUserDetailMsg`와 동일 패턴) |
| `UserUpdatedMsg` | `User domain.User` | edit form이 save 성공 시 발송. List/Detail이 cache 갱신. (AC-4.5) |

### 8.4. 도메인 인터페이스 (개발자 협의 사항 — 도메인 §12 동일)

```go
// internal/domain/ports.go
type UsersPort interface {
    // ... existing methods ...
    UpdateProfile(ctx context.Context, userID string, patch UserProfilePatch) (User, error)
}

type UserProfilePatch struct {
    FirstName, LastName, DisplayName, NickName *string
    Email, MobilePhone, SecondEmail *string
    Title, Division, Department, EmployeeNumber *string
    // login 제외 (D-W2)
}
```

> Patch는 모든 nil = "변경 없음". 빈 값 명시 클리어는 v0.2 (도메인 §1.2 권고).

---

## 9. REQ-W01 AC 충족 매트릭스

| AC | 본 디자인 충족 위치 |
|----|---------------------|
| AC-1.1/1.2 진입점 | §2.2 (e from list/detail) |
| AC-1.3 진입 시 latest GET | §2.3 loading 상태 |
| AC-1.4 loading 중 Esc abort | §4.1 |
| AC-1.5 GET 실패 시 폼 안 열림 | §4.1 마지막 |
| AC-2 11 필드 + login read-only | §2.4 |
| AC-3.1~3.5 클라이언트 검증 | §6.1 |
| AC-4.1 저장 키 Ctrl+S | §3.3 |
| AC-4.2 partial-merge body | (개발자 책임 — §8.4 어댑터) |
| AC-4.3 saving UI + abort | §4.4 |
| AC-4.4 1초 disable | §4.5 |
| AC-4.5 성공 처리 | §4.5 |
| AC-5.1/5.2 cancel (clean/dirty) | §2.3 상태머신 + §3.5 모달 |
| AC-5.3 saving 중 ESC 비활성 | §3.3 + §4.4 footer hint |
| AC-6 에러 매트릭스 | §4.6~4.10, §6.2 |
| AC-7 PII 마스킹 | §5 |
| AC-8 접근성 (키보드 only, NO_COLOR, 80x24, 포커스 표시) | §2.5, §2.4 시각 규약 |
| AC-9 dirty 추적 | §2.4 시각 규약, §2.3 상태머신, §4.2 |
| AC-10 폼 외 상태 미오염 | §2.2 navStack push (기존 polling/cache 영향 없음 — 개발자 책임) |

---

## 10. 디자이너 결정 권고 (PRD Phase 3에서 PM 검토 요청)

### 10.1. OI-W5 (폼 인프라 추상화) — **권고: 옵션 B 절충**

PM이 PRD §9 OI-W5에서 요청한 결정.

- **옵션 A** (users-edit ad-hoc): 빠르게 출시. 차후 lifecycle write에서 재구현 비용.
- **옵션 B** (shared/form 위젯 추상화): 11 fields × FieldSpec × FormModel. 재사용 가능. 도입 비용 증가.
- **옵션 C (권고)** — **절충: 폼 인프라를 `internal/tui/shared/form/` 패키지로 추출하되, 추상화 범위를 명확히 한정**:
  - **추출 대상 (재사용):** `Field` struct (key/label/required/value/snapshot/inlineError/masked), `Form` struct (sections/dirty counter/diff 계산), `KeyHints` state-dependent renderer, `ErrorMapper` (errorCauses prefix 매칭), `DiscardConfirm` modal helper.
  - **추출 안 함 (도메인 특화):** 각 mutation의 fetch/save Cmd, FieldSpec 카탈로그(Users 11필드는 users 패키지 내). 즉, 본 폼은 `shared/form.New(spec)`로 인스턴스화.
  - **사유:** v0.2 lifecycle (deactivate confirm + reason input)이 form widget을 재사용할 가능성 ≥ 70%. ad-hoc 구현은 매번 dirty/error/textinput 코드 중복.
- **권고 근거:** §2.7 컴포넌트 트리의 90%가 도메인-agnostic. 11개 textinput을 묶는 `Form` 추상화는 한 번 만들면 본질적으로 재사용 가능. PM이 옵션 A를 택해도 폐기되는 건 패키지 경계뿐.

### 10.2. OI-W3 (저장 후 audit log 점프) — **권고: 토스트 hint + `l` 키 패스스루**

PM이 PRD §6 / OI-W3에서 v0.1.x 패치 후보로 요청.

- **권고:** 저장 성공 토스트 본문을 확장: `✓ Updated alice@acme.com  · <l> view audit log`
- 토스트 3초 동안 `l` 키 입력 시 → `OpenLogsMsg{Filter: "eventType eq \"user.account.update_profile\" and target.id eq \"<userId>\""}` 발송
- 3초 후 토스트 자동 사라짐 → `l` 패스스루도 자동 해제
- 자동 점프는 **하지 않음** (운영자가 결과를 응시할 시간 필요 + 자동 점프는 흐름 단절)
- **사유:** 기존 `OpenLogsMsg` 패턴 (commit `dc254d8`의 `l` from any list/detail) 재사용. 별도 단축키 도입 불필요. 토스트 활용으로 운영자 발견성 ↑.
- **구현 노트:** toast.go 내에서 토스트가 살아있는 동안 `l` 글로벌 키를 가로채는 작은 state. 토스트 만료 시 정상 글로벌 키맵으로 복원.
- **PM 결정 요청:** Phase 4 진입 전 채택 여부 확정. 채택 시 본 §10.2를 §2.3 상태머신의 "Exit (closing)"에 흡수.

### 10.3. 신규 디자이너 권고

| # | 권고 | 사유 |
|---|------|------|
| DR-1 | `e` 키는 SCR-011의 v1.2.0 Factors 와이어프레임 `(e) expand` 메모와 충돌. **§15.7 cleanup으로 표기 제거 권장.** | v0.1.2에서 탭 구조가 통합되어 `(e) expand`는 dead UX. 본 addendum이 `e`를 `action.edit`로 점유함을 명문화. |
| DR-2 | `m` 글로벌 PII 토글 → form에서는 `Alt+m`. **글로벌 §3.1에 form context 예외 행 추가**. | textinput 입력 충돌 회피. 사용자는 `m`이 form 밖에서는 그대로 동작함을 Help (`?`)에서 확인. |
| DR-3 | edit form은 글로벌 `:` palette를 비활성. **§3.1 텍스트 입력 정책의 명시 예외**로 추가. | form 도중 palette를 여는 동선이 모호하고 운영자의 변경 의도 보호. Esc → palette는 명시적 경로. |
| DR-4 | save 성공 시 진입 직전 화면(SCR-010/SCR-011)의 selected row를 보존. | UX 흐름 단절 최소화. AC-4.5 명문화. |
| DR-5 | `e` 키 자체는 `?` Help의 "Actions" 섹션에 노출하되, Status Bar 상시 6 키 한도는 유지. | §3.8 학습 부담 정책 일관. |

---

## 11. 키 충돌 검증 (§12.1 갱신)

| 키 | 전역 용도 | 화면 전용 용도 | 처리 |
|----|---------|---------------|------|
| `e` | (구) Factors expand → 미사용 | **SCR-010/SCR-011에서 edit form 진입** | DR-1: §15.7 표기 cleanup. v0.2.0부터 단일 의미. |
| `m` | (글로벌 §3.1) PII 토글? — 본 addendum까지는 글로벌 키 없음 | SCR-020에서 "Members 탭". **SCR-012에서는 `Alt+m`** | form 컨텍스트 예외. textinput 입력 충돌 회피. |
| `Ctrl+S` | (신규) save (form 한정) | — | form 밖에서는 no-op + 경고 토스트 `"no save action here"` |
| `Tab`/`Shift+Tab` | tab 순환 (detail) | form 필드 이동 (form) | 컨텍스트 자명. 충돌 없음. |

---

## 12. 변경 이력 (TUI_DESIGN §19에 한 줄 추가)

```
| 2026-06-17 | 1.3.0 | REQ-W01 (Users Profile Edit Form) addendum. SCR-012 신규: 11필드 폼 (Identity/Contact/Organization/Status 4 섹션), `e` 키 + `:edit` 진입, Ctrl+S 저장, Esc 시 dirty면 1단계 confirm, 11 textinput + viewport(좁은 H) + bubbles/spinner, PII 마스킹 form-context (`Alt+m` 토글), errorCauses prefix 매칭으로 field-attached inline error. §3.4/§3.6/§3.7/§10.1/§11/§12.1 갱신. 신규 ScreenUserEdit + OverlayDiscardConfirm + OpenUserEditMsg/UserUpdatedMsg. 디자이너 권고 OI-W5 옵션 C(절충 — shared/form/ 패키지), OI-W3 토스트 hint+`l` 패스스루. | tui-designer-4 |
```

---

## 13. 미해결 / Phase 4 협의 사항

| 항목 | 설명 | Phase 4 협의 대상 |
|------|------|------------------|
| `Alt+m` 호환성 | Alt 키 처리는 터미널마다 다름 (Escape sequence). bubbletea는 `tea.KeyMsg.Alt` 필드 지원. 검증 필요. fallback `Ctrl+M`은 Enter와 충돌 → 비추. | go-tui-developer |
| `Ctrl+S` 터미널 가로채기 | 일부 터미널/tmux 설정에서 `Ctrl+S`가 flow control(XOFF)에 잡힘. 운영자 안내 (Help/README) + `:save` 팔레트 대안 검토. **단 form은 palette 비활성** → form 내에서는 Ctrl+S만. 비활성 환경에서는 운영자 사전 `stty -ixon` 안내. | go-tui-developer + product-manager |
| viewport 자동 스크롤 | 포커스 이동 시 가시 영역 보장. bubbles/viewport는 `SetYOffset` 수동 호출 필요. 구현 비용 명확. | go-tui-developer |
| field-level `:unmask <field>` 통합 | 기존 `:unmask mobilePhone` 명령이 form 내에서도 동작해야 하나? 본 디자인은 `Alt+m`을 일괄 토글로 둠. 단일 필드 토글은 OOS. 단 향후 `:unmask` palette 명령은 form에서 비활성이므로 자동 차단. | (디자이너 결정 — 본 addendum에서 OOS) |
| OI-W6 (PUT 차단) | 도메인 §1.3. ota 어댑터 코드 레벨에서 PUT 노출 금지. lint? wrapper? | go-tui-developer + go-test-engineer |

---

## 14. 한 줄 요약 — Phase 4 인터페이스 hint

**디자인 산출물:**
- SCR-012 (Users Edit Form, 11 fields × 4 sections, modal/full-screen, navStack push)
- 신규 키: `e` (list/detail entry), `Ctrl+S` (save), `Alt+m` (PII toggle), Esc-with-dirty L1 confirm
- 신규 식별자: `ScreenUserEdit`, `OverlayDiscardConfirm`, `OpenUserEditMsg`, `UserUpdatedMsg`
- 신규 패키지 권고: `internal/tui/shared/form/` (Field/Form/ErrorMapper/DiscardConfirm 추출)
- AC 1~10 전체 충족, NO_COLOR + 80x24 + 키보드 only 검증 완료

**Phase 4 협의 hint:**
- developer: §2.7 component tree + §8 신규 식별자 + §13 협의 사항 (Alt+m, Ctrl+S, viewport)
- test-engineer: §2.3 상태머신 전이 매트릭스 + §6 검증 패턴 → teatest 시나리오 11 (상태머신 노드 수)
- qa-inspector: §9 REQ-W01 AC 매트릭스 + §4 상태별 UI + §11 키 충돌 검증

---

**END OF TUI DESIGN ADDENDUM (DRAFT v1.3.0)**

*다음 단계: 본 문서를 `docs/TUI_DESIGN.md`에 패치 → Phase 4 (ARCHITECTURE/CONVENTIONS/TESTING) 이관.*
