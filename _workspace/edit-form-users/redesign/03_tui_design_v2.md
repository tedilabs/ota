# SCR-012 Redesign — Profile Edit as Centered Modal

**문서 ID:** `_workspace/edit-form-users/redesign/03_tui_design_v2.md`
**대상:** `docs/TUI_DESIGN.md` §SCR-012 in-place 교체
**근거:** REQ-W01 출시 후 사용자 피드백 *"디자인이 너무 구려"* + 명시 요구 (`종료 팝업 스타일 모달` / `세련·모던 TUI`)
**작성:** tui-designer-5 (2026-06-18)
**호환:** PRD §5.6 AC-1~AC-10 전건 무변경 충족 (와이어프레임 시각 표현만 교체)

---

## 0. TL;DR — 한 줄 컨셉

> **"풀스크린 폼"을 폐기하고 `Quit/Action Confirm`과 같은 dimmed-body-over-modal 패턴으로 통일. 폭 74·dynamic 높이의 중앙 모달에 4 섹션을 카드처럼 깔고, 포커스 필드만 강조 보더로 들어올림.**

---

## 1. 무엇이 구렸나 (v1.3 vs v2)

### 1.1. v1.3 (기존, 어제 출시) 문제 진단

| # | 증상 | 영향 |
|---|------|-----|
| P1 | **풀스크린 take-over** — chrome 전체를 폼이 점령. 사용자가 어디서 왔는지(`Users › alice@…`) breadcrumb만으로 인지 | "내가 list로 돌아갈 수 있나"가 즉각 안 보임 → Esc 망설임 |
| P2 | **dimmed body 패턴 미사용** — Quit/Help/Action Confirm/API Recorder 4개 modal은 전부 `composeModalOverDimmedBody`로 떠 있는데 SCR-012만 풀스크린 take-over. 시각 언어 분열 | 신규 mutation 표면을 시도하는 운영자가 "modal인지 화면인지" 판단 비용 발생 |
| P3 | **단일 컬럼 30라인 폭격** — 11 필드 + 4 섹션 헤더 + 푸터가 wide 터미널(180+ 폭)에서 너무 wide하게 늘어남. textinput이 가용 폭을 다 흡수 (`[ value ____________________________ ]`) | 와이드 모니터에서 우측 60% 빈 공간 → "왜 이렇게 휑한가" |
| P4 | **modern 인풋 표현 부족** — `[ value ]` ASCII 박스 + `▸ ` prefix만으로는 현대적 TUI(k9s `:edit` form, lazygit input prompt) 대비 1990년대 느낌 | 사용자 인용 *"디자인이 너무 구려"* |
| P5 | **섹션 헤더 시각화 약함** — `── Identity ──` 단순 divider. 어떤 섹션이 dirty 인지 한눈에 모름 | "방금 뭘 바꿨더라" 추적 비용 |
| P6 | **푸터 키힌트 분산** — `<Ctrl+S> save  <Tab> next  <Alt+m> toggle PII  <Esc> cancel` + 상단 `3 changes` 카운터가 따로. footer 한 줄에 통합 안 됨 | 시선 이동 비용 |

### 1.2. v2 디자인 응답

| # | 응답 |
|---|------|
| P1, P2 | 폼을 **modal**로 격하 — `composeModalOverDimmedBody` 패턴 재사용. 폭 74 (Quit 60, Help 60+, Palette 70 사이 위치). 뒤에 list/detail 화면 dimmed로 보임 → "여기 뒤로 갈 수 있다" 즉각 인지 |
| P3 | **고정 폭 모달**(74) — 와이드 터미널에서도 모달 크기 일정. 텍스트필드는 라벨 우측에 고정폭 입력 박스(폭 = `modalContent - labelCol - 4`). 와이드 모니터에서도 휑하지 않음 |
| P4 | **포커스 lift up** — 포커스 필드만 `▎` left bar + bold 라벨 + `┃ value          ┃` 강조 보더. 비-포커스는 muted underscore 라인 `value______________`. 모던 form UI 관례 |
| P5 | **섹션 카드** — 섹션 헤더에 dirty 카운트 inline (`─ Identity · 2* ────`). 컬러 토큰 활용 (dirty 있으면 Header bold, 없으면 Muted). NO_COLOR에서는 `· 2*` 텍스트로 식별 |
| P6 | **single footer line** — `3 changes  ·  Ctrl+S save  ·  Esc cancel  ·  Tab next  ·  Alt+m PII`. 좌→우 순으로 정보 → 액션. 좁은 모드는 축약 |

---

## 2. 결정 사항

| ID | 결정 | 근거 |
|----|------|------|
| **D-W17** | **Mount mode = centered modal over dimmed body** (`composeModalOverDimmedBody` 재사용) | 사용자 명시 요구 + ota 4개 modal 시각 언어 통일. v1.3의 navStack push (D-W16)는 **유지** — modal이지만 ESC pop 대신 ESC=취소 의미는 동일하게 동작. App Shell에서 `OverlayDiscardConfirm`도 modal-over-modal로 떠야 하므로 nesting 처리 §6 참조. |
| **D-W18** | **Modal 폭 = 74 cells** (clamp: max `clampWidth(width)-8`, min 60) | Quit(60) 보다 넓게 — 필드 라벨 18+입력 ~30이 들어와야 함. Help(`contentWidth-4`, 보통 80+) 보다 좁게 — 4섹션 11필드는 helpAccount 수준 정보량은 아님. 70~80 범위에서 가독성/공백비율 가장 우수. 실제 구현은 `width := 74; if cap := clampWidth(m.width)-8; cap > 0 && cap < width { width = cap }` 패턴 (renderQuitConfirmModal과 동형). |
| **D-W19** | **Modal 높이 = dynamic, viewport scroll** (max content lines = `clampBodyLines(height) - 4`) | 11필드 + 4섹션 헤더 + 푸터 + 보더 = ~25 라인. 24행 터미널(body 16~17)에서는 viewport 활용. 25~30행 터미널에서는 풀 표시. `bubbles/viewport`로 포커스 필드 자동 가시 영역 유지 (form package 신규 책임). |
| **D-W20** | **Focus lift — `▎ Label  ┃ value           ┃`** | non-focus는 `  Label  value______________`. focus만 강조 보더(`┃`) + 좌측 vertical bar(`▎`). dirty marker `*`는 라벨 좌측. NO_COLOR에서도 보더와 `▎`로 식별 가능. |
| **D-W21** | **섹션 헤더 dirty 카운트 inline** (`─ Identity · 2* ─────────────`) | 어느 섹션이 변경됐는지 한눈에. clean 섹션은 `─ Contact ──────────────` (Muted). dirty 있는 섹션은 `─ Contact · 1* ─────` (Accent bold). |
| **D-W22** | **Footer 단일 라인 통합** (`3 changes  ·  <Ctrl+S> save  ·  <Esc> cancel  ·  <Tab> next  ·  <Alt+m> PII`) | 좌→우 = state → action 흐름. Tokens: 카운터=Accent, 키=Muted. 좁은 모드는 `<Alt+m>` 생략. |
| **D-W23** | **Read-only 표시 — `Login  alice@acme.com  (read-only)`** (회색 + 텍스트 신호) | NO_COLOR에서도 `(read-only)` 텍스트로 식별. `🔒` emoji는 폐기 (NO_COLOR/Unicode 호환). |
| **D-W24** | **DiscardConfirm 모달은 nested modal** — outer SCR-012 modal 위에 다시 떠 있음 | `composeModalOverDimmedBody`를 두 번 호출하는 대신, EditModel.View()가 `discard state`일 때 작은 nested 모달을 outer 모달 body의 중앙에 stamp. outer는 그대로 dim. 구현 §6 참조. |
| **D-W25** | **Loading은 placeholder form + spinner**, 풀 spinner 모달 폐기 | "Loading user…" 단독 모달 → 빈 모달 우측 상단에 `⠋` + "Loading…" 라벨. 폼 스켈레톤(섹션 헤더 + placeholder underscores)은 처음부터 보임. 100ms+ 지연 시 spinner. AC-1.4 충족. |

> **불변(invariant)**: PRD §5.6 AC-1~AC-10 전건 + REQ-W01 §13 #9~#15 결정(D-W4 confirm 단계, D-W5 Ctrl+S, D-W6 변경값 보존, D-W13 빈 패치, DR-2 Alt+m, DR-3 form-context palette 비활성) 모두 유지. 본 redesign은 시각 표현만 교체.

---

## 3. 와이어프레임 5종 (120×30 기준)

> **표기 규약**:
> - `█` = dimmed body 셀 (실제는 lipgloss `Muted.Faint(true)` 처리된 직전 화면 콘텐츠 일부)
> - `▎` = focus left bar (1 cell, Accent)
> - `┃` = focus input border (1 cell, Accent)
> - `·` = footer separator dot
> - `*` = dirty marker (라벨 좌측 또는 섹션 헤더 inline)

### 3.1. State: `editing (clean)` — 진입 직후

```
┌─ ota · acme.okta.com ·         prod         [RL: ok]        UTC   v0.2.0 ─┐
│ Users                                              42 of 1,205 · type=USER│
├────────────────────────────────────────────────────────────────────────────┤
│ ███ STATUS  LOGIN                       NAME              DEPT       █████ │
│ ██████████████████ ╭───────────────────────────────────────────────╮ █████ │
│ ██ ●ACT a██████████│ Edit User  ·  alice@acme.com                  │ █████ │
│ ██ ●ACT b██████████├───────────────────────────────────────────────┤ █████ │
│ ██ ●ACT c██████████│                                               │ █████ │
│ ██ ●ACT d██████████│  ─ Identity ─────────────────────────────────│ █████ │
│ ██ ◐SUS e██████████│    Login         alice@acme.com (read-only)  │ █████ │
│ ██ ●ACT f██████████│  ▎ First Name  ┃ Alicia                    ┃ │ █████ │
│ ██ ●ACT g██████████│    Last Name     Smith                       │ █████ │
│ ██ ●ACT h██████████│    Display Name  Alice Smith                 │ █████ │
│ ██ ●ACT i██████████│    Nickname      ali                         │ █████ │
│ ██ ●ACT j██████████│                                               │ █████ │
│ ██ ●ACT k██████████│  ─ Contact ──────────────────────────────────│ █████ │
│ ██ ●ACT l██████████│    Email         alice@acme.com              │ █████ │
│ ██ ◐SUS m██████████│    Mobile Phone  +1-***-***-1234   (masked)  │ █████ │
│ ██ ●ACT n██████████│    Secondary     a***@personal.com  (masked) │ █████ │
│ ██ ●ACT o██████████│                                               │ █████ │
│ ██ ●ACT p██████████│  ─ Organization ─────────────────────────────│ █████ │
│ ██ ●ACT q██████████│    Title         Senior SWE                  │ █████ │
│ ██ ●ACT r██████████│    Division      R&D                         │ █████ │
│ ██ ●ACT s██████████│    Department    Engineering                 │ █████ │
│ ██ ●ACT t██████████│    Employee #    ENG-042                     │ █████ │
│ ██ ●ACT u██████████│                                               │ █████ │
│ ██████████████████ ├───────────────────────────────────────────────┤ █████ │
│ ██████████████████ │ No changes · Ctrl+S save · Esc cancel · Tab → │ █████ │
│ ██████████████████ ╰───────────────────────────────────────────────╯ █████ │
├────────────────────────────────────────────────────────────────────────────┤
│ <j k> nav  </> search  <Enter> detail  <e> edit  <R> refresh  <?>         │
└────────────────────────────────────────────────────────────────────────────┘
```

> 모달 폭 49 cells (74-25 좌우 dim) — 실제 production은 74 폭. 와이어프레임 가독성을 위해 위처럼 표기. 모달 본문 47 cells(74-3 padding/border) 안에 다 들어가야 함.

### 3.2. State: `editing (dirty)` — 3 필드 변경

```
                    ╭───────────────────────────────────────────────╮
                    │ Edit User  ·  alice@acme.com         3 changes│
                    ├───────────────────────────────────────────────┤
                    │                                               │
                    │  ─ Identity · 1* ────────────────────────────│
                    │    Login         alice@acme.com (read-only)  │
                    │ *▎ First Name  ┃ Alicia                    ┃ │
                    │    Last Name     Smith                       │
                    │    Display Name  Alice Smith                 │
                    │    Nickname      ali                         │
                    │                                               │
                    │  ─ Contact ──────────────────────────────────│
                    │    Email         alice@acme.com              │
                    │    Mobile Phone  +1-***-***-1234   (masked)  │
                    │    Secondary     a***@personal.com  (masked) │
                    │                                               │
                    │  ─ Organization · 2* ────────────────────────│
                    │    Title         Senior SWE                  │
                    │    Division      R&D                         │
                    │  * Department    Eng → Platform              │
                    │  * Employee #    ENG-042 → ENG-099           │
                    │                                               │
                    ├───────────────────────────────────────────────┤
                    │ 3 changes · Ctrl+S save · Esc cancel · Tab → │
                    ╰───────────────────────────────────────────────╯
```

**시각 신호 (NO_COLOR-safe):**
- **섹션 헤더 dirty 카운트**: `· 1*` / `· 2*` inline. clean 섹션(Contact)은 카운트 표기 없음.
- **라벨 좌측 `*`**: dirty 필드 마커 (AC-9.2). NO_COLOR에서도 식별.
- **`→` 글리프**: dirty 필드의 라벨 우측에 *옛값 → 새값* 미리보기 표기를 굳이 하지 않고, 현재 값만. 비-focus 입력에서 `(old: Eng)` 트레일 hint는 v0.3 검토 (OI-W6 신규).
- **카운터 우측 정렬**: 타이틀 라인 우측에 `3 changes` (Accent). footer와 중복으로 보이지만 footer는 가려질 수 있어 헤더에도 표시.

### 3.3. State: `editing (validation error)` — 첫 invalid 필드로 focus jump 후

```
                    ╭───────────────────────────────────────────────╮
                    │ Edit User  ·  alice@acme.com         3 changes│
                    ├───────────────────────────────────────────────┤
                    │                                               │
                    │  ─ Identity · 1* ────────────────────────────│
                    │    Login         alice@acme.com (read-only)  │
                    │ *▎ First Name  ┃                           ┃ │
                    │    ! First Name is required                  │
                    │    Last Name     Smith                       │
                    │    Display Name  Alice Smith                 │
                    │    Nickname      ali                         │
                    │                                               │
                    │  ─ Contact · 1* ─────────────────────────────│
                    │    Email                                     │
                    │  *▎ Email      ┃ alice@acme                ┃ │
                    │    ! Invalid email format                    │
                    │    Mobile Phone  +1-***-***-1234   (masked)  │
                    │                                               │
                    │  ─ Organization · 1* ────────────────────────│
                    │  * Department    Eng → Platform              │
                    │                                               │
                    ├───────────────────────────────────────────────┤
                    │ ! 2 invalid · Ctrl+S retry · Esc cancel · Tab │
                    ╰───────────────────────────────────────────────╯
```

**시각 신호:**
- **`! <message>` 인라인**: 필드 박스 바로 아래, 라벨 정렬에 맞춤. tokens.Danger.
- **focus는 첫 invalid 필드**: `Validate()` 후 form.Focus(firstInvalid). focus되면 PII가 아니어도 강조 보더.
- **footer 변경**: `3 changes` → `! 2 invalid` (Danger). 사용자에게 "저장 막혔다 + 몇 개 고치면 됨" 전달.
- **AC-6.2 자동 클리어**: 사용자가 해당 필드에 1글자라도 입력 → 즉시 inline error 제거. form.Update에서 `delete(f.inlineErr, key)` 기존 로직 유지.

### 3.4. State: `saving` — POST in flight

```
                    ╭───────────────────────────────────────────────╮
                    │ Edit User  ·  alice@acme.com         3 changes│
                    ├───────────────────────────────────────────────┤
                    │                                               │
                    │  ─ Identity · 1* ────────────────────────────│
                    │    Login         alice@acme.com (read-only)  │
                    │  * First Name    Alicia                      │
                    │    Last Name     Smith                       │
                    │    Display Name  Alice Smith                 │
                    │    Nickname      ali                         │
                    │                                               │
                    │  ─ Contact ──────────────────────────────────│
                    │    Email         alice@acme.com              │
                    │    Mobile Phone  +1-***-***-1234   (masked)  │
                    │    Secondary     a***@personal.com  (masked) │
                    │                                               │
                    │  ─ Organization · 2* ────────────────────────│
                    │  * Department    Platform                    │
                    │  * Employee #    ENG-099                     │
                    │                                               │
                    ├───────────────────────────────────────────────┤
                    │ ⠋ Saving…  POST /api/v1/users/00u…x8          │
                    │           Ctrl+C to abort (preserves draft)   │
                    ╰───────────────────────────────────────────────╯
```

**시각 신호:**
- **모든 필드 muted + non-focus 표시**: focus 보더 제거 (입력 disabled). `▎` 마커도 제거.
- **footer 2줄**: spinner + endpoint + abort hint. `<Esc>` 표기 사라짐 (AC-4.3, AC-5.3 → 비활성).
- **AC-4.3**: `Ctrl+C`만 abort 허용 — App Shell이 saving 상태에서 Ctrl+C를 SCR-012로 라우팅하여 입력 보존 + state → editing 복귀.

### 3.5. State: `discard confirm` — nested modal over dimmed outer

```
                    ╭───────────────────────────────────────────────╮
                    │ ████ ████ █████ █  █████████████████ █████████│
                    │ ███████████████████████ ██████████████████████│
                    │ ████ █████████ ╭────────────────────────────╮ │
                    │ ████ ██████████│  Discard 3 unsaved changes?│ │
                    │ ████ ██████████├────────────────────────────┤ │
                    │ ████ ██████████│  Modified fields:          │ │
                    │ ████ ██████████│    • First Name            │ │
                    │ ████ ██████████│    • Department            │ │
                    │ ████ ██████████│    • Employee Number       │ │
                    │ ████ ██████████│                            │ │
                    │ ████ ██████████│  <y> discard  <n/Esc> keep │ │
                    │ ████ ██████████╰────────────────────────────╯ │
                    │ ████ █████████████████████████████████ ███████│
                    │ ████ █████████████████████████████████ ███████│
                    │ ████ █████████████████████████████████ ███████│
                    │ ████ █████████████████████████████████ ███████│
                    │ ████ █████████████████████████████████ ███████│
                    │                                               │
                    ├───────────────────────────────────────────────┤
                    │ 3 changes · Esc keep · y discard              │
                    ╰───────────────────────────────────────────────╯
```

**시각 신호:**
- **Outer modal body 내부에 nested modal stamp**: outer는 그대로 보이고, body 영역이 추가 dim. nested는 폭 40, 높이 가변.
- **default N**: y만 명시. n/Esc는 keep editing. (AC-5.2, D-W4 유지)
- **footer 변경**: outer footer가 `Esc keep · y discard` 안내로 교체.
- **Modified fields 리스트**: 최대 5개 표시. 그 이상은 `… and N more`.

> **구현 노트**: outer modal은 `MountModal(...)` 결과 문자열. nested는 outer body 영역에 별도 `MountModal` 결과를 좌표 stamp. App Shell의 `composeModalOverDimmedBody`는 SCR-012 외 1개만 처리하므로, EditModel.View()가 자체적으로 nested stamping을 책임. §6 코드 hint 참조.

### 3.6. State: `loading` — 진입 직후 fetch in flight

```
                    ╭───────────────────────────────────────────────╮
                    │ Edit User  ·  Loading…                        │
                    ├───────────────────────────────────────────────┤
                    │                                               │
                    │  ─ Identity ─────────────────────────────────│
                    │    Login         _____________________       │
                    │    First Name    _____________________       │
                    │    Last Name     _____________________       │
                    │    Display Name  _____________________       │
                    │    Nickname      _____________________       │
                    │                                               │
                    │  ─ Contact ──────────────────────────────────│
                    │    Email         _____________________       │
                    │    Mobile Phone  _____________________       │
                    │    Secondary     _____________________       │
                    │                                               │
                    │  ─ Organization ─────────────────────────────│
                    │    Title         _____________________       │
                    │    Division      _____________________       │
                    │    Department    _____________________       │
                    │    Employee #    _____________________       │
                    │                                               │
                    ├───────────────────────────────────────────────┤
                    │ ⠋ GET /api/v1/users/00u…x8 · Esc cancel       │
                    ╰───────────────────────────────────────────────╯
```

**시각 신호:**
- **타이틀 우측에 `Loading…`**: 사용자 식별 전이므로 login 자리에 표시.
- **placeholder underscores**: 섹션/필드 구조는 보임 → "이런 폼이 뜬다" 미리 인지 (AC-1.4 충족: chrome 즉시 표시).
- **footer**: spinner + endpoint + `Esc cancel` (AC-1.4). saving과 다르게 Esc 활성.

---

## 4. 폭/높이 계산 로직

### 4.1. 모달 폭 계산 (App Shell에서, EditModel 외부)

```go
// app.go composeBody()에 추가 (renderQuit/Action confirm과 동형):
if m.active == ScreenUserEdit && m.screens[ScreenUserEdit] != nil {
    edit := m.screens[ScreenUserEdit] // *users.EditModel
    width := 74
    if cap := clampWidth(m.width) - 8; cap > 0 && cap < width {
        width = cap // 좁은 터미널에서 축소 (80-8=72)
    }
    bodyBudget := clampBodyLines(m.height) - 4 // 모달 보더+타이틀+푸터 reserve
    modal := edit.RenderModal(activeTokens(), width, bodyBudget)
    return m.composeModalOverDimmedBody(modal)
}
```

| 터미널 폭 | clampWidth | cap = -8 | 모달 폭 |
|----------|-----------|---------|--------|
| 80 (최소) | 80 | 72 | **72** (축소) |
| 100 | 100 | 92 | **74** |
| 120 (표준) | 120 | 112 | **74** |
| 180 (와이드) | 180 | 172 | **74** |

> **fixed 74 정책**의 근거: helpAccount는 width 100+에서도 풀 width 활용해야 함(키 레퍼런스가 많음). SCR-012는 11필드라 폭 74면 충분 + 와이드 터미널에서도 dimmed body가 더 잘 보이는 게 컨텍스트 유지에 유리.

### 4.2. 모달 본문 (content area) 계산

```
modalContentWidth = 74 - 3 = 71  // 보더 1 + 좌padding 1 + 우보더 1
labelCol = 16 ("Display Name " = 13, +3 buffer)
inputCol = modalContentWidth - labelCol - 4 = 71 - 16 - 4 = 51
// focus 보더 `┃ ... ┃` 추가 시: inputCol - 2 = 49 (텍스트 가용)
```

### 4.3. 높이 계산 + viewport scroll

| 영역 | 라인 수 |
|------|--------|
| 보더 top/bottom | 2 |
| 타이틀 + divider | 2 |
| 본문 (4 섹션 헤더 + 11 필드 + 4 빈줄 패딩) | ~20 |
| Footer divider + footer | 2 |
| **합계** | **~26** |

- **터미널 H ≥ 28**: 전체 폼 한 화면 (스크롤 없음).
- **터미널 H = 24 (최소)**: chrome reserve 7행 → body 17행. 모달은 body 17행에 맞춰 viewport scroll. 보더(2) + 타이틀(2) + footer(2) = 6행 고정. 본문 11행만 viewport에 들어감. 포커스 이동 시 자동 스크롤.
- **inline error 발생 시**: 해당 필드 아래 1행 추가 → viewport 자동 확장.

**구현 hint:**
```go
// internal/tui/shared/form/form.go에 viewport 추가:
type Form struct {
    // ...existing fields...
    viewport viewport.Model // bubbles/viewport, H 가변
    maxH     int            // App Shell이 RenderModal로 넘겨준 body budget
}
```

---

## 5. 키바인딩 — 변경/유지

### 5.1. 무변경 (기존 §SCR-012 단축키 표 그대로)

| 키 | 동작 |
|----|------|
| `Tab` / `↓` | 다음 필드 focus (read-only skip, wrap-around) |
| `Shift+Tab` / `↑` | 이전 필드 focus |
| `←` / `→` / `Home` / `End` / `Ctrl+a` / `Ctrl+e` | 입력 박스 내 커서 이동 |
| `Ctrl+w` / `Ctrl+u` / `Ctrl+k` | 단어/전체/우측 삭제 |
| `Ctrl+S` | save (D-W5, dirty=0이면 footer "No changes to save") |
| `Esc` | clean이면 즉시 닫기, dirty면 nested DiscardConfirm (D-W24) |
| `Alt+m` | 모든 PII 필드 일괄 mask/unmask 토글 (AC-7.5 form-context 변형) |
| `Ctrl+C` | (saving 상태일 때만) abort + 입력 보존 (AC-4.3) |
| `Enter` | 일반 필드 focus 시 다음 필드 (Tab과 동일). DiscardConfirm 활성 시 confirm 보조. |

### 5.2. footer hint 키 노출 (v2 갱신)

```
[clean]   No changes · Ctrl+S save · Esc cancel · Tab →
[dirty]   3 changes · Ctrl+S save · Esc cancel · Tab → · Alt+m PII
[invalid] ! 2 invalid · Ctrl+S retry · Esc cancel · Tab → next invalid
[saving]  ⠋ Saving…  POST /api/v1/users/00u…x8
          Ctrl+C to abort (preserves draft)
[discard] 3 changes · Esc keep · y discard
[narrow W<90] Ctrl+S save · Esc cancel · Tab → (Alt+m 생략)
```

### 5.3. 충돌 검사 — 무영향

기존 §3.7 충돌 표는 **갱신 불필요** (D-W17~D-W25는 mount mode + 시각 표현 변경. 키 자체는 모두 동일):
- `e`, `Ctrl+S`, `Alt+m` 점유 그대로
- form-context palette/help/search 비활성 정책 그대로 (DR-3)
- nested DiscardConfirm은 outer SCR-012 modal의 자식 — `?` Help는 form 전체에서 비활성이므로 nested 등장 시에도 비활성 유지

---

## 6. 컴포넌트/코드 매핑 — 개발자 적용 hint

### 6.1. EditModel.View() → RenderModal(...) 분리

```go
// 기존 (edit.go):
func (m EditModel) View() string { ... b.WriteString("Edit User · " + ...) ... }

// v2:
func (m EditModel) View() string {
    // teatest용 plain (App Shell 없을 때) — 기존 형식 유지하여 골든 호환
    return m.renderPlain()
}

// 신규 — App Shell composeBody()에서 호출
func (m EditModel) RenderModal(tk shared.Tokens, width, bodyBudget int) string {
    switch m.state {
    case EditStateLoading:    return m.renderLoadingModal(tk, width, bodyBudget)
    case EditStateErrored:    return m.renderErroredModal(tk, width)
    case EditStateSaving:     return m.renderSavingModal(tk, width, bodyBudget)
    case EditStateDiscardConfirm:
        // Outer + nested. Outer는 editing-dirty 그대로, nested를 outer body에 stamp.
        outer := m.renderEditingModal(tk, width, bodyBudget, /*dimInner*/true)
        return stampNestedConfirm(outer, m.discardModal(tk))
    default: // EditStateEditing
        return m.renderEditingModal(tk, width, bodyBudget, false)
    }
}
```

### 6.2. App Shell — `composeBody()`에 분기 추가

```go
// app.go composeBody() 내부, OverlayActionMenu/Help/APIRecorder 처리 직후:
if m.active == ScreenUserEdit {
    if edit, ok := m.screens[ScreenUserEdit].(*users.EditModel); ok && edit != nil {
        width := 74
        if cap := clampWidth(m.width) - 8; cap > 0 && cap < width {
            width = cap
        }
        bodyBudget := clampBodyLines(m.height) - 4
        modal := edit.RenderModal(activeTokens(), width, bodyBudget)
        return m.composeModalOverDimmedBody(modal)
    }
}
// ...existing screen render fallthrough...
```

> **dimmed body 의미**: SCR-012는 ScreenUserEdit이 active. `composeModalOverDimmedBody`는 `m.screens[m.active].View()`를 dim하는데, active가 SCR-012 자신이면 무한루프. 해결: 직전 화면(navStack top-1, e.g. ScreenUsers)을 dim 대상으로 사용. App Shell이 navStack 어휘를 알므로 helper 추가:

```go
// app.go 신규:
func (m Model) previousScreenForBackdrop() Screen {
    if len(m.navStack) >= 2 {
        return m.navStack[len(m.navStack)-2]
    }
    return ScreenUsers // fallback
}

// composeModalOverDimmedBody 변형 — 백드롭 스크린 명시:
func (m Model) composeModalOverScreenDimmed(modal string, bgScreen Screen) string {
    // 기존 composeModalOverDimmedBody와 동일하나, body := m.screens[bgScreen].View()
}
```

### 6.3. 신규 helper (`internal/tui/shared/form/render.go`)

| Helper | 목적 |
|--------|------|
| `RenderFieldRow(tk, label, value, focused, dirty, readOnly, masked, labelCol, inputCol int) string` | 필드 1줄 렌더링 (focus는 `▎`+`┃`+`┃`, 비-focus는 underscore line) |
| `RenderSectionHeader(tk, name string, dirtyCount, width int) string` | `─ Identity · 2* ───────` |
| `RenderFooterLine(tk Tokens, dirty int, state string, width int) string` | clean/dirty/invalid/saving/discard 별 footer 라인 |
| `StampNestedConfirm(outer, nested string) string` | outer modal body 영역 중앙에 nested 모달 stamp |

### 6.4. bubbles/textinput 도입은 **여전히 보류**

v1.3의 정책 그대로: 자체 `form.fieldState{cursor int}` 유지. textinput 도입은 v0.3 (OI-W7 신규).

이유:
- v2는 시각 표현만 바뀜. cursor 로직(Home/End/Ctrl+w/u/k/Left/Right/Backspace/Delete) form.go에 이미 있음.
- textinput은 자체 chrome(prompt, cursor blink)을 그려 v2 focus lift 패턴과 충돌. Lipgloss로 보더만 그리면 됨.

### 6.5. PII 마스킹 통합 — 기존 로직 그대로

`form.shouldShowPII(s, focused)` 로직 변경 없음. RenderFieldRow에서 masked 값을 받으면 라벨 우측에 `(masked)` 트레일 텍스트 추가. Alt+m 일괄 unmask 상태는 `[M!]` 배지 (기존 §7.2 토큰) 라벨 우측.

---

## 7. 색상·토큰 매핑

| 슬롯 | Token | 비고 |
|------|-------|-----|
| Modal 보더 | `tk.Muted` | Quit/Action confirm 동일 |
| Modal 타이틀 라인 | `tk.Header` (bold) | "Edit User · alice@…" |
| 타이틀 우측 카운터 | `tk.Accent` | "3 changes" |
| 섹션 헤더 (clean) | `tk.Muted` | `─ Identity ───` |
| 섹션 헤더 (dirty) | `tk.Header` (bold) + Accent | `─ Identity · 2* ───` |
| 필드 라벨 (비-focus) | `tk.Muted` | |
| 필드 라벨 (focus) | `tk.Accent` (bold) | |
| 필드 라벨 prefix `*` | `tk.Warning` | dirty marker |
| Focus 보더 `┃` | `tk.Accent` | |
| Focus left bar `▎` | `tk.Accent` | |
| 입력 값 (clean) | `tk.FG` | |
| 입력 값 (dirty) | `tk.Header` (bold) | 변경된 값 강조 |
| `(read-only)` 트레일 | `tk.Muted` | |
| `(masked)` 트레일 | `tk.Muted` | |
| `[M!]` 배지 | `tk.BadgeUnmask` | 기존 §7.2 |
| `! <error>` 인라인 | `tk.Danger` | |
| Footer (state side) | `tk.Accent` (dirty/error)/`tk.Muted`(clean)/`tk.Warning`(saving)/`tk.Success`(saved) | |
| Footer (action side) | `tk.Muted` | "Ctrl+S save" 등 |
| Spinner | `tk.Accent` | `bubbles/spinner` Dot |

**NO_COLOR**: 모든 token이 plain/bold/reverse로 fallback. dirty `*`, `(read-only)`, `(masked)`, `[M!]`, `! `, focus 보더 `┃`, `▎` 모두 텍스트/문자로 식별 가능.

---

## 8. 상태 전이 — 무변경

전이 매트릭스(§SCR-012 기존):

```
loading → editing(clean) (200)
loading → Return (Esc / 4xx/5xx)
editing(clean) ↔ editing(dirty)
editing(dirty) → saving (Ctrl+S, valid)
editing(*) → editing(invalid) (Ctrl+S with invalid)
editing(invalid) → editing (사용자 수정 → AC-6.2 clear)
saving → Return + cache (200)
saving → editing (4xx/5xx/429 — 변경값 보존)
saving → editing (Ctrl+C — AC-4.3)
saving → Return + refresh (404)
editing(clean) → Return (Esc)
editing(dirty) → DiscardConfirm (Esc)
DiscardConfirm → editing (n/Esc)
DiscardConfirm → Return (y/Y/Enter — 변경 폐기)
```

> 모든 전이는 기존 §SCR-012 `상태머신` 다이어그램과 동일. 본 redesign은 각 state의 **View() 표현**만 변경.

---

## 9. 접근성 체크리스트

| 항목 | 보장 방법 |
|------|----------|
| NO_COLOR에서 dirty 식별 | 라벨 좌측 `*` + 섹션 헤더 `· N*` |
| NO_COLOR에서 required 식별 | `! First Name is required` 인라인 (Validate 후) — 기존 로직 |
| NO_COLOR에서 read-only 식별 | 라벨 우측 `(read-only)` 텍스트 |
| NO_COLOR에서 focus 식별 | `▎` left bar + `┃ ... ┃` 보더 |
| NO_COLOR에서 PII masked 식별 | 라벨 우측 `(masked)` 트레일 + 값 `***` 문자 |
| NO_COLOR에서 dirty 섹션 식별 | 섹션 헤더 `· 2*` inline |
| 색맹 (적녹/청황) | Danger=`!` prefix, Success=`✓` prefix (footer/toast), 색에만 의존 안 함 |
| 키보드 only | 모든 액션 `Tab/Shift+Tab/Ctrl+S/Esc/Alt+m/Ctrl+C/y/n` (AC-8.1) |
| 80×24 동작 | 모달 폭 72로 축소, viewport scroll, footer 축약 |
| 스크린리더 | `--plain` 모드는 EditModel.View()의 renderPlain() 그대로 사용 |
| 카드/박스 글리프 호환 | `╭╮╰╯─│┃▎` Unicode box drawing. ASCII fallback은 chrome.go 일관 (Phase 4 추가 검토) |

---

## 10. PRD AC 충족 검증

| AC | v1.3 충족 | v2 충족 | 변경점 |
|----|----------|---------|--------|
| AC-1.1 (`e` Users list) | ✓ | ✓ | 동일 |
| AC-1.2 (`e` User detail) | ✓ | ✓ | 동일 |
| AC-1.3 (latest GET on entry) | ✓ | ✓ | 동일 |
| AC-1.4 (loading placeholder) | ✓ | ✓ (개선) | placeholder underscore + chrome 즉시 표시 |
| AC-1.5 (4xx/5xx no form) | ✓ | ✓ | erroredModal로 표시 |
| AC-2 (11 필드 4 섹션) | ✓ | ✓ | 동일 |
| AC-3 (client validation) | ✓ | ✓ | 동일 |
| AC-4.1 (`Ctrl+S` save) | ✓ | ✓ | 동일 |
| AC-4.2 (partial-merge diff) | ✓ | ✓ | 동일 (form.Diff) |
| AC-4.3 (saving spinner + Ctrl+C abort) | ✓ | ✓ (개선) | footer 2줄로 안내 명확 |
| AC-4.4 (1s save 가드) | ✓ | ✓ | 동일 (App Shell) |
| AC-4.5 (200 → pop + toast + cache) | ✓ | ✓ | 동일 |
| AC-5.1 (clean Esc → pop) | ✓ | ✓ | 동일 |
| AC-5.2 (dirty Esc → confirm) | ✓ | ✓ (개선) | nested modal로 outer-context 유지 |
| AC-5.3 (saving Esc 비활성) | ✓ | ✓ | 동일 |
| AC-6 (4xx/5xx/429 inline + 변경값 보존) | ✓ | ✓ (개선) | inline error 시각 통합 |
| AC-7 (PII 마스킹 + Alt+m) | ✓ | ✓ | `(masked)` 트레일로 명확 |
| AC-8.1 (키보드 only) | ✓ | ✓ | 동일 |
| AC-8.2 (NO_COLOR) | ✓ | ✓ (개선) | 보더 `┃▎` 추가, 색 없이도 focus 식별 |
| AC-8.3 (80×24) | ✓ | ✓ | viewport + 폭 72 축소 |
| AC-8.4 (focus 3채널: 색+굵기+prefix) | ✓ | ✓ (개선) | 색+`▎`+`┃` 3채널 |
| AC-9.1 (snapshot diff) | ✓ | ✓ | 동일 |
| AC-9.2 (dirty `*` marker) | ✓ | ✓ | 동일 + 섹션 헤더 카운트 |
| AC-9.3 (`N changes` footer) | ✓ | ✓ (개선) | 타이틀 + footer 양쪽 표시 |
| AC-9.4 (diff body) | ✓ | ✓ | 동일 |
| AC-10.1/2/3 (폼 외 미오염) | ✓ | ✓ | 동일 |

**v2 회귀 없음.** 시각 표현 개선만 + dimmed body로 인지 부담 ↓.

---

## 11. PM 결정 대기

| ID | 안건 | 디자이너 권고 | PM 확정 |
|----|------|--------------|--------|
| Q-W18 | dimmed background 대상 — navStack top-1 화면. v1.3 entry가 `Users list` 또는 `User detail`이므로 둘 중 하나 (보통 list). PM 확정? | navStack top-1 사용 (top-1이 없으면 ScreenUsers fallback) | TBD |
| Q-W19 | `RenderModal(tk, width, bodyBudget)` 시그니처를 EditModel에 추가하는 안 — 또는 App Shell이 form.Form.RenderModal로 직접 호출하고 EditModel은 wrapper만 노출. 개발자 선호 | EditModel.RenderModal(...) 단일 entrypoint (Loading/Errored/Discard 분기 통합 책임 유지) | TBD (개발자 협의) |
| Q-W20 | `(masked)` 텍스트 트레일 — v1.3은 `(masked, Alt+m)`이라고 길게. v2는 `(masked)`만 + Alt+m은 footer에. 정보 중복 제거 OK? | OK — Alt+m hint는 footer에 일관 표시 | TBD |
| Q-W21 | nested DiscardConfirm 위치 — outer 모달 body의 정중앙에 stamp. 좁은 H에서 outer가 viewport scroll 상태일 때도 nested는 중앙? | yes — viewport 무관 outer body 영역 중앙. outer 본문은 nested 뒤로 dim. | TBD |
| Q-W22 | `→ new value` preview (예: `Eng → Platform`) 와이어프레임 3.2에 표기됐는데 실제 도입? OI-W6 별도? | v2 MVP는 **표기 안 함** (현재 값만 표시 + 라벨 좌측 `*`). preview는 OI-W6로 v0.3 검토. | TBD |

---

## 12. 비교 — Before / After

### 12.1. 시각 변화 요약

| 측면 | Before (v1.3) | After (v2) |
|------|--------------|-----------|
| Mount | 풀스크린 take-over | Modal over dimmed body (74×dynamic) |
| 컨텍스트 | breadcrumb만 (`Users › alice@…`) | 뒤에 list/detail dimmed 보임 + breadcrumb 불필요 |
| 폭 | 가용 폭 전체 (와이드에서 빈공간) | 고정 74 (와이드에서도 일정) |
| Focus 표현 | `▸ ` prefix + `[ ... ]` 박스 | `▎` left bar + `┃ ... ┃` 보더 + Accent bold 라벨 |
| 비-focus 필드 | `[ value ]` 박스 (focus와 동일 박스) | `value` (박스 없음) — focus만 lift |
| 섹션 헤더 | `─ Identity ───` (clean/dirty 무관) | `─ Identity · 2* ───` (dirty 시 카운트 inline) |
| Dirty 표시 | 라벨 좌측 `*` | 라벨 좌측 `*` + 섹션 헤더 `· N*` + 타이틀 우측 `N changes` |
| Read-only | `🔒` emoji + ASCII fallback | `(read-only)` 텍스트 (emoji 폐기) |
| PII masked | `(masked, Alt+m)` 트레일 | `(masked)` 트레일 (Alt+m은 footer) |
| Loading | `⠋ Loading user…` 중앙 1줄 | placeholder underscore 폼 + footer spinner |
| Discard confirm | 별도 풀스크린 modal | outer 위 nested modal (outer dim) |
| Footer | 키힌트 1줄 + 상단 dirty 카운터 분리 | state(좌) + actions(우) 단일 라인 통합 |

### 12.2. 패턴 일관성

| Modal | Mount | 폭 | v2 정렬 |
|-------|------|----|--------|
| Quit (Q) | dimmed body | 60 | ✓ |
| Action Confirm (Y/n) | dimmed body | 60 | ✓ |
| Help (?) | dimmed body | `contentWidth-4` (보통 80+) | ✓ |
| API Recorder (~) | dimmed body | `contentWidth-3` (full) | ✓ |
| Action Menu | dimmed body | 가변 | ✓ |
| **SCR-012 v1.3 (어제)** | **풀스크린 take-over** | full | ✗ (분열) |
| **SCR-012 v2 (이 안)** | **dimmed body** | **74** | **✓** |

→ v2는 ota 시각 언어 완전 통일.

---

## 13. 개발자 코드 변경 인터페이스 hint (요약)

### 13.1. 새 시그니처 (3개)

```go
// EditModel — 단일 modal entrypoint
func (m EditModel) RenderModal(tk shared.Tokens, width, bodyBudget int) string

// EditModel — 기존 View()는 plain mode (teatest 골든 호환)
func (m EditModel) View() string  // renderPlain() = 기존 v1.3 로직

// App Shell — 백드롭 스크린 명시 변형
func (m Model) composeModalOverScreenDimmed(modal string, bgScreen Screen) string
```

### 13.2. 새 helper 파일 — `internal/tui/shared/form/render.go`

```go
package form

// RenderFieldRow — 필드 1줄 (focus 상태에 따라 보더 토글)
func RenderFieldRow(tk Tokens, opts FieldRowOpts) string

type FieldRowOpts struct {
    Label      string
    Value      string
    Focused    bool
    Dirty      bool
    ReadOnly   bool
    Masked     bool
    InlineErr  string
    LabelCol   int
    InputCol   int
}

// RenderSectionHeader — `─ Identity · 2* ───────`
func RenderSectionHeader(tk Tokens, name string, dirtyCount, width int) string

// StampNestedConfirm — outer modal body 영역 중앙에 nested 모달 stamp
func StampNestedConfirm(outerModal, nestedModal string) string
```

### 13.3. App Shell 변경 — `internal/app/app.go composeBody()`

```go
// 기존:
if child, ok := m.screens[m.active]; ok {
    return child.View()  // SCR-012도 여기서 풀스크린으로 떴음
}

// v2:
if m.active == ScreenUserEdit {
    if edit, ok := m.screens[ScreenUserEdit].(*users.EditModel); ok && edit != nil {
        width := 74
        if cap := clampWidth(m.width) - 8; cap > 0 && cap < width {
            width = cap
        }
        bodyBudget := clampBodyLines(m.height) - 4
        modal := edit.RenderModal(activeTokens(), width, bodyBudget)
        return m.composeModalOverScreenDimmed(modal, m.previousScreenForBackdrop())
    }
}
if child, ok := m.screens[m.active]; ok {
    return child.View()
}
```

### 13.4. 영향 받는 테스트

| 파일 | 영향 |
|------|------|
| `internal/tui/users/edit_test.go` | View() 출력은 plain 모드(renderPlain) — 기존 골든 호환. RenderModal 신규 단위 테스트 추가 필요. |
| `internal/tui/users/golden_test.go` | SCR-012 골든 갱신 (v2 modal 출력). teatest 통합 테스트는 plain 모드 사용. |
| `internal/tui/shared/form/form_test.go` | RenderFieldRow / RenderSectionHeader / StampNestedConfirm 단위 테스트 신설. |
| `internal/app/app_test.go` | composeModalOverScreenDimmed + previousScreenForBackdrop 단위 테스트 추가. |

### 13.5. 마이그레이션 안전 — 점진 도입

1. Step 1: `form/render.go` 신규 + 단위 테스트 (RED → GREEN).
2. Step 2: `EditModel.RenderModal` 추가, `View()`는 기존 그대로 (plain mode로 격하). 골든 테스트 무영향.
3. Step 3: `App Shell composeBody()` 분기 추가 + `composeModalOverScreenDimmed`. SCR-012 진입 시 modal 표시.
4. Step 4: 사용자 검증 → SCR-012 골든 업데이트.

→ 각 단계 독립 PR 가능.

---

## 14. 변경 이력

| 날짜 | 변경 | 사유 |
|------|------|------|
| 2026-06-18 | v2 초안 | 사용자 피드백 *"디자인이 너무 구려"* + 명시 요구(종료 팝업 스타일 모달) 대응. PRD AC 무변경. |

---

**End of redesign v2 draft.** PM 검수 → `docs/TUI_DESIGN.md` §SCR-012 in-place 교체 + §13/§14 패치 진행.
