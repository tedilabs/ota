# Phase 4 Tech Docs Addendum — REQ-W01 Users Profile Edit Form

**상태:** APPLIED (직접 패치)
**버전:** 1.0
**작성일:** 2026-06-17
**작성자:** go-tui-developer
**근거 입력:**
- `_workspace/edit-form-users/00_input.md` (사용자 직접 요청)
- `_workspace/edit-form-users/02_okta_domain_input.md` (okta-expert)
- `_workspace/edit-form-users/02_pm_prd_draft.md` (PM PRD addendum)
- `_workspace/edit-form-users/03_tui_design_draft.md` (TUI designer addendum)
- 기존 `docs/{ARCHITECTURE,CONVENTIONS,PROJECT_STRUCTURE,TESTING,TECH_STACK}.md` v1.0.x

---

## 0. 한 줄 요약

REQ-W01 (Users Profile Edit Form)을 ota의 **첫 profile-mutation 표면**으로 정착시키면서, 향후 lifecycle write가 재사용할 수 있는 **재사용 가능 form 위젯 추상화**(`internal/tui/shared/form/`)를 패키지 경계로 분리. 도메인 sentinel·errormap·navStack 등 기존 인프라는 그대로 활용. 4개 docs 파일에 addendum 패치.

---

## 1. 핵심 기술 결정 사항 (Phase 4 확정)

| # | 결정 | 결과 | 근거 |
|---|------|------|------|
| **D-T1** | 폼 Mount mode (screen vs modal) | **Screen** — 새 `ScreenUserEdit` ScreenKind로 등록. navStack push. | TUI Design §13 #9, D-W16 (PM). 11 fields + 4 sections이 modal overlay에는 너무 크고, navStack 기존 패턴과 정합 (commit `b0794ad`). |
| **D-T2** | Form 추상화 위치 | **`internal/tui/shared/form/`** 신설 패키지 | OI-W5 Option C. 도메인-agnostic 가드를 패키지 경계로 강제 (depguard). v0.2 lifecycle reason input이 재사용. |
| **D-T3** | Save 흐름 | **client validate → screen이 form.Diff() 받아 patch 조립 → svc.UpdateProfile → 성공 시 popNav + UserUpdatedMsg broadcast + toast.** Optimistic UI 없음. 응답 받은 후 view 갱신. | AC-4.2/4.5. last-write-wins 보정을 위해 서버 응답 User로 cache 패치 (도메인 §5.2). |
| **D-T4** | Patch sparse encoding | **`*string` 포인터 패턴**. nil = unchanged (omit), 값 = set. 명시적 null clear는 MVP 제외 (D-W13 / 도메인 §1.2). | omitempty + JSON marshal 자연스러움. 명시적 클리어 미지원은 PRD에서 deferred. |
| **D-T5** | Empty patch 처리 | **도메인 sentinel `domain.ErrEmptyPatch`**. UsersPort.UpdateProfile은 IsEmpty()시 API 호출 없이 sentinel 반환. TUI는 Save 버튼 자체 disable이 1차 가드, 도메인 sentinel은 2차 방어. | D-W13. UX 무의미 호출 차단 + 어댑터 단위 테스트로 보장. |
| **D-T6** | Error 매핑 레이어 | **`internal/tui/shared/form/errmap.go` ErrorMapper** — `*domain.BadRequestError`의 Causes → field-level inline + OtherErrors footer. 새 parser 작성 없음 — 기존 `okta/errormap.splitCause`가 이미 `field:` prefix를 `FieldError.Field`로 파싱. | AC-6.1. ARCHITECTURE §9.5. 기존 `BadRequestError.Causes`를 그대로 활용. |
| **D-T7** | dirty detection | **각 field의 Original vs Current 비교 (lazy diff per render)** | AC-9.1. 11 fields × ~60fps = 660 비교/s — 무시 가능. revert-to-snapshot은 자연스럽게 dirty=0 복원. |
| **D-T8** | PII 마스킹 키 | **form 내부: `Alt+m`** (글로벌 `m`은 textinput이 글자로 소비). focus auto-unmask + blur-no-change re-mask. | TUI Design §3.3 DR-2. tea.KeyMsg.Alt 안정 동작 (CONVENTIONS §10a.7). |
| **D-T9** | PUT (strict) 차단 | **어댑터에 PUT 경로 자체를 노출하지 않음.** depguard로 SDK PUT helper import도 차단 (v0.2 SDK 전환 시 검토). | 도메인 §1.3 / D-W15. 데이터 손실 회피. |
| **D-T10** | Form ↔ domain import 가드 | **Form 패키지는 `domain.*` import 금지** (CONVENTIONS §10a.9). screen이 `Form.Diff() map[string]string`을 받아 patch 조립. depguard 강제. | 재사용성 보장 — 위젯이 도메인에 종속되면 v0.2 다른 mutation에서 재사용 불가. |

---

## 2. 파일별 패치 요약

### 2.1. `docs/ARCHITECTURE.md` (v1.0.1 → v1.1.0)

| 섹션 | 변경 | 내용 |
|------|------|------|
| 변경 이력 | 추가 1행 | v1.1.0 REQ-W01 통합 |
| §6.1 도메인 | **새 하위 블록** | `UserProfilePatch` struct + `ErrEmptyPatch` sentinel. *string 포인터 패턴 근거. login 의도적 제외. |
| §6.2 service Port | **메서드 추가** | `UsersPort.UpdateProfile(ctx, id, patch) (User, error)` — 시그니처, 에러 매핑 6종, IsEmpty 가드, 응답 User 반환 사유 (race 보정). |
| §6.8 form widget (**신설**) | 새 섹션 | `internal/tui/shared/form/` — FieldSpec/FieldKind, Form Bubbletea Model, ErrorMapper, 책임 분리 (Form vs Screen vs App Shell), Bubble 컴포넌트 매핑, 금지사항. |
| §7.4 mutation flow (**신설**) | 새 섹션 | mutation 경로 표 (REQ-W01 + 미래 후보 행), REQ-W01 시퀀스 다이어그램, buildPatch 함수, 실패 경로 표, race 보정. |
| §9.5 server error mapping (**신설**) | 새 섹션 | BadRequestError.Causes → form field-level inline 매핑 계층, FieldKey 식별 규약, inline error lifecycle, OtherErrors fallback. |
| §13.4 write surface 플레이북 (**신설**) | 새 섹션 | 새 mutation 추가 시 단계별 변경 파일 목록 (REQ-W01 기준 ~5 신규 / ~6 수정). |

### 2.2. `docs/CONVENTIONS.md` (v1.0.0 → v1.1.0)

| 섹션 | 변경 | 내용 |
|------|------|------|
| 변경 이력 | 추가 1행 | v1.1.0 |
| §3.2 타입 명명 | **추가 bullet** | `<Resource>Patch` 패턴, *string 포인터, IsEmpty + Err sentinel 규약. |
| §3.3 함수/Msg 명명 | **추가 bullet** | Cmd 동사 `save<Noun>`, `Open<X>EditMsg`/`<X>UpdatedMsg`/`form.<Verb>Msg` 패턴. |
| §10a Form Widget Pattern (**신설**) | 새 섹션 | FieldSpec/Form 인터페이스, dirty 추적, validation lifecycle 표, save lifecycle (Form ↔ Screen 책임), Discard-confirm 흐름, ErrorMapper, PII+Alt+m, 키 가로채기, 금지사항. |
| §12.3 폼 컨텍스트 마스킹 (**신설**) | 새 섹션 | PII focus auto-unmask, Alt+m 토글, debug log 가드. |
| §17 동기화 표 | **추가 2행** | Form 위젯 인터페이스 변경 시 영향 / 새 mutation 표면 추가 시 영향. |

### 2.3. `docs/PROJECT_STRUCTURE.md` (v1.0.0 → v1.1.0)

| 섹션 | 변경 | 내용 |
|------|------|------|
| 변경 이력 | 추가 1행 | v1.1.0 |
| §2 디렉토리 트리 | **추가** | `internal/tui/shared/form/` 디렉토리 (spec/form/keys/errmap/msgs/discard/view + tests). `internal/tui/users/edit.go`, `edit_spec.go`, `edit_patch.go`, `edit_flow_test.go`, `edit_save_test.go`, `edit_pii_test.go`. `internal/domain/user_patch.go`. `internal/tui/shared/msgs.go`에 OpenUserEditMsg/UserUpdatedMsg 주석 추가. |
| §4 패키지 매트릭스 | **추가 1행** | `internal/tui/shared/form` — Bubbletea, lipgloss, mask 허용. domain/service/okta/app/tui-resource 금지 (depguard). |
| §8.3 (**신설**) | 새 섹션 | 신규 Write 표면 / 폼 화면 추가 플레이북 — 13단계 순서. |

### 2.4. `docs/TESTING.md` (v1.0.1 → v1.1.0)

| 섹션 | 변경 | 내용 |
|------|------|------|
| Header | 버전 / 날짜 / Sources 갱신 | v1.1.0 / 2026-06-17 / addendum 입력 추가 |
| §6.7 Form 화면 teatest 패턴 (**신설**) | 새 섹션 | 3층 분리표 (Form unit / Edit screen / Adapter integration). Form unit dirty 테스트 예시. EditModel teatest save success 예시. 400 validation 시나리오 예시. 단축키 매트릭스 (`e`/`Tab`/`Ctrl+S`/`Esc`/`Alt+m`/`Ctrl+C` × 11 시나리오). UsersPortFake.UpdateProfile 확장 가이드 + ValidationErrorFake 헬퍼. |
| §8.7 REQ-W01 AC 매트릭스 (**신설**) | 새 섹션 | AC-1~10 전체 → 테스트 함수 표. 11 fields × dirty matrix table-driven 코드. errorCauses 매핑 table 코드. partial-merge body assertion table 코드. Adapter integration httptest 코드 + ErrEmptyPatch 가드 테스트. |
| §12 매트릭스 | **추가 1행** | REQ-W01 → §8.7 전체 매핑. |
| §14 변경 이력 | **추가 1행** | v1.1.0 |

---

## 3. Phase 5 진입 시 test-engineer가 알아야 할 인터페이스

### 3.1. 도메인 타입 (`internal/domain/`)

```go
// internal/domain/user_patch.go (신규)
type UserProfilePatch struct {
    FirstName      *string
    LastName       *string
    DisplayName    *string
    NickName       *string
    Email          *string
    Title          *string
    Division       *string
    Department     *string
    EmployeeNumber *string
    MobilePhone    *string
    SecondEmail    *string
}

func (p UserProfilePatch) IsEmpty() bool

var ErrEmptyPatch = errors.New("empty patch: no fields to update")
```

### 3.2. Port 메서드 (`internal/domain/ports.go`)

```go
type UsersPort interface {
    // ... 기존 메서드 ...
    UpdateProfile(ctx context.Context, userID string, patch UserProfilePatch) (User, error)
}
```

에러 매핑:
- `*domain.BadRequestError` (E0000001) — `Causes []FieldError`
- `domain.ErrTokenInvalid` (E0000004 / E0000011)
- `domain.ErrForbidden` (E0000006)
- `domain.ErrNotFound` (E0000007)
- `domain.ErrFeatureDisabled` (E0000038)
- `*domain.RateLimitedError` (E0000047 / 429)
- `domain.ErrEmptyPatch` (IsEmpty 시 — API 호출 없음)

### 3.3. Form 위젯 인터페이스 (`internal/tui/shared/form/`)

```go
// 패키지 외부에서 사용할 핵심 API
type FieldKind int
const (
    KindText FieldKind = iota
    KindEmail
    KindPhone
    KindReadOnly
)

type FieldSpec struct {
    Key      string
    Label    string
    Kind     FieldKind
    Required bool
    PII      bool
    Section  string
    Hint     string
    MaxLen   int
}

type Form struct { /* unexported */ }

func New(specs []FieldSpec, initial map[string]string, opts ...Option) Form

// Bubbletea 인터페이스
func (f Form) Init() tea.Cmd
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd)
func (f Form) View() string

// Inspection
func (f Form) Dirty() int
func (f Form) DirtyFields() []string
func (f Form) Validate() (ok bool, firstInvalid string)
func (f Form) Snapshot() map[string]string
func (f Form) Diff() map[string]string
func (f Form) SetSaving(on bool) Form
func (f Form) ApplyServerErrors(causes []domain.FieldError) Form  // ErrorMapper 경유

// 메시지 (form 패키지 owned)
type SaveRequestedMsg struct{}
type DiscardRequestedMsg struct{ Confirmed bool }
type PIIToggleMsg struct{}
type FieldFocusedMsg struct{ Key string }
type FieldBlurredMsg struct{ Key string }
```

### 3.4. Screen-level msgs (`internal/tui/shared/msgs.go`)

```go
// 신규 추가
type OpenUserEditMsg struct{ ID string }  // list/detail의 e 키
type UserUpdatedMsg struct{ User domain.User }  // 저장 성공 시 broadcast
```

### 3.5. App Shell 등록 (`internal/app/app.go`)

```go
// 신규 ScreenKind
const (
    // ... 기존 ...
    ScreenUserEdit
)
func (s Screen) String() string {
    // ...
    case ScreenUserEdit: return "user-edit"
    // ...
}

// 신규 OverlayKind
const (
    // ... 기존 ...
    OverlayDiscardConfirm
)
```

### 3.6. Edit screen 인터페이스 (`internal/tui/users/edit.go`)

```go
type EditDeps struct {
    Svc    *service.UsersService
    UserID string
    Clock  clock.Clock     // optional
    Logger *slog.Logger    // optional
    Width  int             // 초기 터미널 폭
    Height int
}

type EditModel struct { /* unexported */ }

func NewEditModel(deps EditDeps) EditModel

// tea.Model
func (m EditModel) Init() tea.Cmd
func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m EditModel) View() string

// 테스트 helper로 노출할 수 있음 (testfx)
func (m EditModel) State() EditState   // loading / editing / saving / discardConfirm
func (m EditModel) Form() form.Form    // 단, Form.Diff() 같은 inspection 메서드만 노출
```

### 3.7. UsersPortFake 확장 (`internal/service/fakes/users_port_fake.go`)

```go
type UsersPortFake struct {
    // ... 기존 ...
    UpdateProfileFunc func(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error)
}

func (f *UsersPortFake) UpdateProfile(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error)

// 헬퍼 — 400 시뮬레이션
func ValidationErrorFake(causes map[string]string) func(context.Context, string, domain.UserProfilePatch) (domain.User, error)
```

---

## 4. 테스트 시작점 (Phase 5 RED 순서 제안)

test-engineer가 첫 `_workspace/05_test_fail_log_2026-06-17.txt`에 채울 RED 순서:

1. **Domain unit** — `Test_UserProfilePatch_IsEmpty` (all-nil → true, any-set → false). 가장 작은 단위, 도메인부터 확정.
2. **Form unit** — `Test_Form_New_NoDirty`, `Test_Form_Dirty_TrackedPerKeystroke`, `Test_Form_RevertToSnapshot_ClearsDirty`. Form 위젯 인터페이스 모양 확정.
3. **Form unit** — `Test_Form_ApplyServerErrors_PrefixMatchesFieldSpecKey` (ErrorMapper).
4. **Form unit** — `Test_Form_AltM_TogglesAllPII`, `Test_Form_FocusAutoUnmask`.
5. **Adapter integration** — `Test_OktaUsersAdapter_UpdateProfile_PartialMerge_BodyShape` (가장 중요한 정합성 — omit on nil). `Test_OktaUsersAdapter_UpdateProfile_EmptyPatch_NoHTTPCall`.
6. **Adapter integration** — `Test_OktaUsersAdapter_UpdateProfile_ErrorMapping` (table — 400/403/404/429).
7. **TUI flow** — `Test_UsersList_eKey_EmitsOpenUserEditMsg`. 진입점부터.
8. **TUI flow** — `Test_UserEdit_OnEntry_CallsPortGet_Once`, `Test_UserEdit_Loading_4xx_DoesNotOpenForm`.
9. **TUI flow** — `Test_UserEdit_Save_PartialMergeBody_Success`, `Test_UserEdit_Save_BadRequestError_InlineFieldErrors`.
10. **TUI flow** — `Test_UserEdit_Esc_Dirty_OpensDiscardConfirm`, `Test_DiscardConfirm_Y_Discards` / `Test_DiscardConfirm_N_Preserves`.
11. **TUI flow** — `Test_UserEdit_Save_Success_BroadcastsUserUpdatedMsg` + `Test_UsersList_ReceivesUserUpdatedMsg_PatchesRow`.
12. 표 §8.7.2~8.7.4의 table-driven 매트릭스 모두 (11 fields × dirty / errorCauses / partial-merge).

---

## 5. Phase 4 산출물 체크리스트

- [x] `docs/ARCHITECTURE.md` — §6.1 Patch 타입, §6.2 Port 메서드, §6.8 form widget, §7.4 mutation flow, §9.5 error mapping, §13.4 플레이북, 변경 이력 v1.1.0
- [x] `docs/CONVENTIONS.md` — §3.2/3.3 명명, §10a form widget pattern (10 하위 절), §12.3 폼 PII, §17 동기화 표, 변경 이력 v1.1.0
- [x] `docs/PROJECT_STRUCTURE.md` — 디렉토리 트리, §4 패키지 매트릭스, §8.3 플레이북, 변경 이력 v1.1.0
- [x] `docs/TESTING.md` — §6.7 form teatest, §8.7 REQ-W01 매트릭스 (4 하위 절), §12 매트릭스 행, header 갱신, 변경 이력 v1.1.0
- [x] `_workspace/edit-form-users/04_tech_design_outline.md` (본 문서)

다음 단계: test-engineer가 Phase 5 진입 — 본 §3 인터페이스 시그니처를 기준으로 `t.Skip("REQ-W01 AC-X — not yet implemented")` 자리표시자 테스트 함수를 위 §4 순서로 작성 후 RED 시작.
