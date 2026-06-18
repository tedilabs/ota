# SCR-012 v2 Redesign — Implementation Report

**문서 ID:** `_workspace/edit-form-users/redesign/06_impl_v2.md`
**근거:** `_workspace/edit-form-users/redesign/03_tui_design_v2.md` (tui-designer-5, 2026-06-18)
**대상 변경:** SCR-012 Profile Edit Form 시각 표현 v1.3 → v2 (centered modal over dimmed body)
**작성:** go-tui-developer (2026-06-18)
**상태:** GREEN — `go build ./...` 통과, `go test ./... -race -count=1` 전 패키지 PASS

---

## 1. 한 줄 요약

SCR-012 Edit Form을 풀스크린 take-over → 폭 74 centered modal over dimmed previous-screen 패턴으로 교체하고 (D-W17/D-W18), `form` 패키지에 v2 focus-lift 렌더 헬퍼를 신설했다. 기존 `EditModel.View()`는 teatest 골든 호환을 위해 plain 모드로 유지되며, App Shell이 `composeBody`에서 `m.active == ScreenUserEdit`을 별도 분기 처리해 `EditModel.RenderModal(...)`를 호출한다.

## 2. 변경 파일 목록

| 파일 | 변경 종류 | 라인 변동 |
|------|----------|---------|
| `internal/tui/shared/form/render.go` | **신규** | +169 |
| `internal/tui/shared/form/form.go` | 신규 메서드 5개 추가 (`Specs`, `Current`, `InlineErrors`, `OtherErrors`, `PIIAllUnmasked`) | +52 |
| `internal/tui/users/edit.go` | `RenderModal` + 4 helper 신규, `View()` 주석 보강 (semantics 동일) | +173 |
| `internal/app/app.go` | `composeBody` 분기 + `previousScreenForBackdrop` + `composeModalOverScreenDimmed` 신규 | +89 |

총 4 파일, ~480 라인 net 증가 (대부분 신규 helper).

## 3. 신규 헬퍼 함수 시그니처

### 3.1 `internal/tui/shared/form/render.go` (신규 파일)

```go
type FieldRowOpts struct {
    Label, Value      string
    Focused, Dirty    bool
    ReadOnly, Masked  bool
    InlineErr         string
    LabelCol, InputCol int
}

func RenderFieldRow(tk shared.Tokens, opts FieldRowOpts) string
func RenderSectionHeader(tk shared.Tokens, name string, dirtyCount, width int) string
```

- `RenderFieldRow`는 D-W20의 5가지 라인 변형(non-focus clean/dirty, focus clean/dirty, read-only, masked) + 인라인 에러 부속 라인을 단일 함수로 처리한다.
- `RenderSectionHeader`는 D-W21의 `─ Identity · 2* ─────` (dirty=Header+bold) / `─ Contact ─────` (clean=Muted) 두 톤을 dirtyCount로 분기한다.

### 3.2 `internal/tui/shared/form/form.go` (메서드 추가)

```go
func (f Form) Specs() []FieldSpec          // 카탈로그 readonly 사본
func (f Form) Current() map[string]string  // 라이브 값 사본
func (f Form) InlineErrors() map[string]string
func (f Form) OtherErrors() []string
func (f Form) PIIAllUnmasked() bool        // Alt+m 글로벌 토글 상태
```

모두 값-반환 copy 패턴 — 외부에서 mutation 못 함. v2 모달 렌더가 내부 필드를 직접 만지지 않고 인터페이스로 접근.

### 3.3 `internal/tui/users/edit.go` (메서드 추가)

```go
func (m EditModel) RenderModal(tk shared.Tokens, width, bodyBudget int) string
```

단일 entrypoint — state별로 분기:

| state | 모달 | 톤 |
|-------|------|----|
| `EditStateLoading` | `Edit User · Loading…` 타이틀 + placeholder underscore 폼 + spinner footer | Accent |
| `EditStateErrored` | 작은 에러 박스 | Danger |
| `EditStateEditing` | live 필드 + dirty-aware 섹션 헤더 + single-line footer | Accent |
| `EditStateSaving` | non-focused 필드 (입력 disabled) + `Saving… · Ctrl+C` footer | Accent |
| `EditStateDiscardConfirm` | editing 모달 본문 끝에 `─ Discard unsaved changes? (y/N) ─` 강조 strip + 수정 필드 목록 추가 | Accent |

내부 helper (unexported):
- `renderFormBody` — 섹션 + 필드 행 조립
- `renderPlaceholderBody` — 로딩 placeholder
- `composeFooter` — state별 단일 라인 footer
- `buildDiscardStrip` — 로딩이 strong "Discard" 라인 + 수정 필드 lst
- `maskPII` — `•` repeat

### 3.4 `internal/app/app.go` (헬퍼 추가)

```go
func (m Model) previousScreenForBackdrop() Screen
func (m Model) composeModalOverScreenDimmed(modal string, bgScreen Screen) string
```

- `previousScreenForBackdrop`: `navStack[len-2]` 반환, 없으면 `ScreenUsers` fallback.
- `composeModalOverScreenDimmed`: `composeModalOverDimmedBody`와 동일한 splice 로직이되 backdrop을 외부에서 명시. SCR-012의 active = ScreenUserEdit 자기참조 무한루프 회피.

`composeBody` 분기:

```go
if m.active == ScreenUserEdit {
    if edit, ok := m.screens[ScreenUserEdit].(users.EditModel); ok {
        width := 74
        if capW := clampWidth(m.width) - 8; capW > 0 && capW < width {
            width = capW
        }
        if width < 60 { width = 60 }
        bodyBudget := clampBodyLines(m.height) - 4
        modal := edit.RenderModal(activeTokens(), width, bodyBudget)
        return m.composeModalOverScreenDimmed(modal, m.previousScreenForBackdrop())
    }
}
```

폭 clamp 정책은 D-W18 정확 적용 (80-cell 터미널 → 72, 100+ → 74).

## 4. 테스트 결과

### 4.1 `go build ./...`
```
(no output, exit 0)
```

### 4.2 `go test ./... -race -count=1`
모든 패키지 PASS:

```
ok    cmd/ota                              1.8s
ok    internal/apilog                      1.4s
ok    internal/app                         5.1s   <-- 4 QA-W01 regression incl.
ok    internal/config                      3.3s
ok    internal/domain                      7.0s
ok    internal/keys                        2.9s
ok    internal/logger                      4.1s
ok    internal/mask                        2.5s
ok    internal/okta                        8.6s
ok    internal/okta/errormap               3.7s
ok    internal/okta/pagination             2.1s
ok    internal/okta/ratelimit              7.3s
ok    internal/oktastatus                  5.8s
ok    internal/security                    4.5s
ok    internal/service                     6.2s
ok    internal/testfx                      5.4s
ok    internal/tui/apps                    6.5s
ok    internal/tui/groups                  6.5s
ok    internal/tui/logs                    6.5s
ok    internal/tui/overlay                 6.4s
ok    internal/tui/policies                6.5s
ok    internal/tui/rules                   6.5s
ok    internal/tui/shared                  6.4s
ok    internal/tui/shared/form             6.4s
ok    internal/tui/users                   6.7s
```

### 4.3 QA-W01 회귀 4종 (특별 확인)
```
--- PASS: Test_AppShell_PaletteEdit_ResolvesScreenUserEdit (0.00s)
--- PASS: Test_AppShell_PaletteE_ResolvesScreenUserEdit (0.00s)
--- PASS: Test_AppShell_Esc_DuringSaving_DoesNotPopNav (0.00s)
--- PASS: Test_AppShell_Esc_OnDirtyEditForm_OpensDiscardConfirm (0.00s)
```

DiscardConfirm 경로에서 `m.View()`가 "Discard" 문자열을 포함하는지 확인:
- `EditModel.RenderModal`이 `EditStateDiscardConfirm`일 때 `buildDiscardStrip`을 호출 → 모달 본문에 `─ Discard unsaved changes? (y/N) ─` 문자열이 들어감 → App Shell의 `composeBody` 출력에 포함 → 통과.

### 4.4 `go vet ./...`
```
(no output, exit 0)
```

## 5. 알려진 한계

1. **Viewport scroll 미적용**: D-W19에서 명시한 `bubbles/viewport` 본문 스크롤은 이번 구현에 포함되지 않았다. 24행 터미널에서는 모달 본체가 chrome body 위로 오버플로될 수 있다. 80×30 이상의 표준 터미널에서는 11필드 + 4섹션 헤더 + footer가 전부 모달 안에 들어맞으므로 MVP로 충분. 후속 OI에서 viewport 추가 권장.

2. **Nested DiscardConfirm 좌표 stamp 미구현**: D-W24의 "outer 모달 위 nested 작은 confirm box를 좌표 stamp" 패턴 대신, outer 모달 본문 끝에 강조된 `─ Discard unsaved changes? (y/N) ─` strip을 append하는 단순화 안을 채택했다 (task hint §5 명시 OK). 시각적으로는 outer 위 nested가 아니지만, AC-5.2 / D-W4 contract (y → discard / n/Esc → keep, "Discard" 텍스트 가시화)는 충족. 진짜 nested stamp가 필요하면 후속 작업으로 `form.StampNestedConfirm(outer, nested string)` 헬퍼 추가 가능.

3. **`(masked, Alt+m)` 짧은 hint 폐기 → footer로 이동**: v1.3은 PII 필드 라벨 우측에 `(masked, Alt+m)` 트레일을 표시했으나, v2는 `(masked)`만 표시하고 `Alt+m PII` hint는 footer로 일원화 (D-W23 + Q-W20 권고). 이로 인해 PII 필드 옆에서 "어떻게 unmask 하는지" 정보가 한 단계 떨어졌다. footer를 못 보는 좁은 모드에서 UX 저하 가능 — 후속 회귀에서 모니터링 권장.

4. **`bubbles/textinput` 미도입**: D-W26/OI-W7 그대로. 현재 cursor 로직은 form.go의 fieldState{cursor int}로 처리. v0.3에서 검토.

5. **EditModel 타입 단언 — value vs pointer**: App Shell의 `composeBody` 분기는 `m.screens[ScreenUserEdit].(users.EditModel)` (값 단언). 현재 ensureScreen이 NewEditModel을 값으로 반환하므로 안전. 후속에서 EditModel을 포인터로 옮기면 이 단언도 같이 갱신 필요 (단일 사이트).

6. **`go-tui-developer` 자체 단위 테스트 미추가**: 현재 통합 + teatest 골든이 v2 렌더의 핵심(label/value/Discard/N changes/Updated/Email is not valid 등) 식별자를 모두 검증하고 있으므로 GREEN을 유지. RenderFieldRow / RenderSectionHeader는 별도 단위 테스트(`form_render_test.go`) 추가가 바람직하나, 본 task 범위 밖이므로 후속 PR로 분리.

---

## 6. 점진 마이그레이션 안전성

- v1.3 plain `View()`는 변경 없음 → teatest 골든 호환 100%. 11개 edit_test.go 케이스 (AC-1.3, AC-1.5, AC-2, AC-4.1/4.2/4.5, AC-5.1/5.2, AC-6, AC-9.3) 모두 PASS.
- 새 modal 경로는 `m.active == ScreenUserEdit` 단일 분기 — 다른 화면 무영향.
- App Shell의 다른 overlay 분기(Help/Palette/QuitConfirm/ActionConfirm/APIRecorder/ActionMenu)는 modal 분기보다 *먼저* 평가되므로 SCR-012 위에 떠 있어야 하는 오버레이(예: 운영 중 `q` Quit)는 정상 동작.

---

## 7. 회귀 위험 요약

| 위험 | 평가 | 근거 |
|------|------|------|
| teatest 골든 무효화 | **낮음** | `View()` 변경 없음 |
| 다른 화면 렌더 무효화 | **없음** | composeBody 분기 추가, 기존 fallthrough 유지 |
| Race condition | **없음** | `-race` 통과 |
| 80×24 터미널에서 modal overflow | **중간** | viewport 미구현 → 좁은 H에서 라인 끊김 가능. 표준(120×30)에서는 무문제 |
| `(read-only)` 컬럼 정렬 | **낮음** | LabelCol/InputCol fixed 16/inputCol — 모든 행 동일 정렬 |
| NO_COLOR mode (`MonochromeEnabled`) | **낮음** | `styled()` helper가 GetForeground nil 가드 → raw string 반환 |

---

**End of impl report.**
