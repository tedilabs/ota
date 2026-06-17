# ota Conventions

**Version:** v1.0.0
**Status:** Final (Phase 4)
**Last updated:** 2026-04-24
**Authors:** developer (lead) + test-engineer (test & PR sections)

> **단일 출처.** 이 문서와 충돌하는 관례는 전부 이 문서를 따르거나 이 문서를 업데이트한다.

---

## 변경 이력

| 버전 | 날짜 | 변경 | 작성자 |
|------|------|------|-------|
| v0.1.0-draft | 2026-04-24 | 초안 | developer |
| v0.1.1-draft | 2026-04-24 | §13 테스트 섹션 확장 (파일 위치·testfx·Fail-First 로그·fake 템플릿 등) | test-engineer |
| v1.0.0 | 2026-04-24 | §8.1 Screen Model Deps 생성자 추가 (SetXxx 테스트-only setter 금지). §10.1 Elm 원칙 재진술. domain.UsersPort/domain.UsersQuery 레퍼런스 통일. | developer |
| v1.1.0 | 2026-06-17 | REQ-W01 통합: §10a 신설(Form Widget Pattern — FieldSpec/Form/ErrorMapper, dirty/validation/save lifecycle, discard-confirm). §3.3 Msg 명명에 `OpenUserEditMsg`/`UserUpdatedMsg`/`form.*Msg` 추가. §3.2/§3.7 새 도메인 명명: `UserProfilePatch`, `ErrEmptyPatch`, `UsersPort.UpdateProfile`. §12에 폼 PII 마스킹 통합 규약(Alt+m, focus auto-unmask). §17 동기화 표에 form 패턴 변경 시 영향 문서 추가. | developer |

---

## 1. 포맷

- **`gofumpt`**로 자동 포맷. CI에서 `gofumpt -l -d .` diff 없어야 통과.
- 줄 길이: soft 100, hard 120. 넘으면 쪼개라.
- import 블록 3개: stdlib / 3rd party / `github.com/tedilabs/ota/internal/...`. gofumpt가 자동 정렬.
- LF, UTF-8, 파일 끝에 빈 줄 1개.

---

## 2. 린트

- **`golangci-lint run`** CI 통과 필수. 설정은 `.golangci.yml`.
- 활성 리너: errcheck / govet / staticcheck / ineffassign / unused / gosec / gocritic / revive / copyloopvar / bodyclose / contextcheck / exhaustive / nilerr / prealloc / tparallel.
- 경고 disable은 **줄 단위**로만 (`//nolint:linter // reason`). 패키지·파일 단위 disable 금지. 이유 주석 필수.

---

## 3. 명명

### 3.1. 패키지

- 소문자, 짧게(1~2 단어). 복수형 금지 (`users/` 디렉토리 안의 패키지명은 `users`이지만 단수가 어색하지 않다면 단수를 우선).
- `util`/`helpers`/`common` 같은 **쓰레기통 이름 금지**. 목적으로 명명.

### 3.2. 타입

- 구조체 · 인터페이스 · alias: PascalCase.
- 인터페이스는 **동사+er** (`Reader`, `Lister`) 또는 **명사 + Port** (`UsersPort`). 소비자가 정의.
- enum: `type UserStatus string` + 상수 `UPPER_SNAKE_CASE`. 도메인 의미 타입이 값 혼용 방지.
- **Patch / Mutation 입력 타입** (REQ-W01부터): `<Resource>Patch` 또는 `<Resource><Action>Input`. 모든 mutation-input은 partial-merge 시맨틱을 명시적으로 갖는다 — `*string` 포인터 패턴 (`nil` = unchanged, value = set). `IsEmpty()` 메서드 + `Err<Resource>EmptyPatch` 센티넬 함께 제공. 예: `UserProfilePatch`, `ErrEmptyPatch`. (ARCHITECTURE §6.1 참조)

### 3.3. 함수

- PUblic: PascalCase. 동사로 시작.
- private: camelCase. 동사로 시작.
- Cmd 팩토리: `fetch<Noun>`, `refresh<Noun>`, `save<Noun>` (write 동사). 항상 `tea.Cmd` 반환.
- Msg 생성: `<Noun>Msg` 구조체, `NewXxxMsg` 생성자는 만들지 말고 리터럴 사용.
- **Mutation Msg 명명** (REQ-W01 패턴):
  - 진입 요청: `Open<Resource>EditMsg{ID string}` — list/detail에서 e 키 시 발송. 예: `OpenUserEditMsg`.
  - 결과 broadcast: `<Resource>UpdatedMsg{<Resource> domain.<Resource>}` — 저장 성공 시 발송. cache 갱신 트리거. 예: `UserUpdatedMsg{User domain.User}`.
  - Form 위젯 내부 msg: `form.<Verb>Msg` (form 패키지 owned, 외부 screen이 수신). 예: `form.SaveRequestedMsg`, `form.DiscardRequestedMsg`, `form.PIIToggleMsg`. (CONVENTIONS §10a)

### 3.4. 변수

- 짧은 지역: `u`, `g`, `err`, `ctx` OK.
- 긴 지역·필드·export: 의미 있는 명사.
- 부정형 (`notReady`, `noCache`) 피하고 긍정형 (`ready`, `cached`) + `!` 사용.

### 3.5. 테스트 이름

- 기본: `TestXxx` + 서브 테스트 (`t.Run("returns ErrNotFound", ...)`).
- 복잡한 시나리오: `Test_<Unit>_<Scenario>_<Expectation>`.
- 상세 규약·회귀·Lock-in 테스트 네이밍은 §13.2 + TESTING §7 참조.

### 3.6. 파일

- snake_case.go (§PROJECT_STRUCTURE §5).
- 한 파일은 한 주제. 섞지 마라.

---

## 4. 에러 처리

### 4.1. 래핑

```go
if err := port.List(ctx, q); err != nil {
    return fmt.Errorf("listing users (q=%q): %w", q.Raw, err)
}
```

- `%w`로 감싸서 `errors.Is`/`errors.As` 체인 유지.
- 메시지는 **현재 층에서 무엇을 하려 했는지**. 하위 에러 메시지 반복 금지.
- 사용자 노출 메시지는 TUI 레이어에서 `domain.Err*` 센티넬로 분기해 만들어라. 내부 에러 문자열을 그대로 사용자에게 보이지 말 것.

### 4.2. 센티넬

```go
// internal/domain/errors.go
var (
    ErrNotFound        = errors.New("not found")
    ErrForbidden       = errors.New("forbidden")
    ErrRateLimited     = errors.New("rate limited")
    ErrTokenInvalid    = errors.New("token invalid or expired")
    ErrBadRequest      = errors.New("bad request")
    ErrOktaServer      = errors.New("okta server error")
    ErrFeatureDisabled = errors.New("feature disabled")
    ErrNetwork         = errors.New("network error")
)
```

- 레이어 간 에러 식별은 **센티넬 only**. 커스텀 에러 타입은 추가 정보가 필요할 때만 (예: `RateLimitedErr{RetryAfter}` — `errors.As`로 꺼냄).

### 4.3. Must 금지

- `Must...` 패턴은 `cmd/ota`의 부팅 단계에만 허용 (의존성 조립 실패 → panic OK).
- 런타임 코드에서 `panic` 금지. 항상 `error` 반환.

### 4.4. 에러 판별

```go
switch {
case errors.Is(err, domain.ErrNotFound):
    return nil, toast.Warn("Not found. Refreshing…")
case errors.Is(err, domain.ErrRateLimited):
    return nil, scheduleRetry(err)
case errors.Is(err, domain.ErrForbidden):
    return nil, toast.Err("Insufficient permissions for " + resource)
default:
    return nil, toast.Err("Unexpected error. See :errors.")
}
```

---

## 5. Context

- **모든 외부 I/O + 장기 작업** 함수는 첫 인자 `ctx context.Context`.
- `ctx`를 구조체 필드에 저장 금지. 호출마다 전달.
- Esc 키: App.Model이 `context.WithCancel`로 생성한 ctx를 현재 Cmd에 전달. Esc 시 cancel.
- timeout 기본값: 30s(관리 API), 60s(Logs), 5s(healthcheck). 설정 가능.

---

## 6. 로깅

### 6.1. slog 사용 패턴

```go
log := s.log.With(slog.String("resource", "users"), slog.String("op", "list"))
log.Info("started", slog.String("q", q.Raw), slog.Int("limit", q.Limit))
// ...
log.Info("completed", slog.Int("returned", len(items)), slog.Duration("took", dur))
```

- 필드 이름: snake_case.
- 레벨:
  - **ERROR**: 사용자에게 보이는 실패 + 스택 남기고 싶은 경우
  - **WARN**: 부분 실패(예: id→name 해소 실패 후 id 표시), 캐시 폴백
  - **INFO**: 유스케이스 경계 (`list started`, `list completed`), 상태 전이
  - **DEBUG**: HTTP 요청/응답 헤더, SDK 내부 로그, tail tick
- 기본 INFO. `--debug` 또는 `debug: true` → DEBUG.

### 6.2. 민감 정보 금지

**절대 로그에 남기지 말 것:**
- API token 값 (`Authorization` 헤더, `api_token_env` 해소된 값)
- `profile.mobilePhone`, `profile.secondEmail`, factor `phoneNumber`/`email` — `mask` 경유 후에만
- Okta user `login`은 평문 허용 (식별에 필수) — 단 설정 `logs_actor_email=true`면 logs 섹션에서도 마스킹 (TUI_DESIGN §7.3)

**강제 수단:**
- `internal/logger/mask_attr.go`의 `slog.ReplaceAttr`이 키 이름 기반으로 `authorization`, `api_token`, `mobile_phone`, `second_email` 값 `***` 치환.
- 직접 `fmt.Sprintf` 로 문자열 조립 후 로깅 금지 — 반드시 field 전달.

### 6.3. 상관 ID

- `session_id` (UUIDv4)를 부팅 시 생성. 모든 로그에 자동 부착 (`logger.With(slog.String("session_id", id))`).
- 프로필 전환 시 **새 session_id 발급**하되 이전 id를 "parent_session" 필드로 1회 기록.

### 6.4. 출력 대상

- 기본: 파일 `~/.cache/ota/debug.log` (`0600`). lumberjack 로테이션 10MB × 3.
- stdout: **금지** (TUI와 충돌).
- `--debug` 플래그는 파일 활성화 + 레벨 DEBUG.

---

## 7. 설정 키 네이밍

- YAML key: snake_case.
- 계층: `<top>.<subsection>.<leaf>`. 3단계 이하 유지.
- 예:
  ```yaml
  profiles:
    dev:
      org_url: "https://example.okta.com"
      api_token_env: "OKTA_API_TOKEN"
      default_log_filter: ""
  ui:
    theme: "dark"          # dark | high_contrast | monochrome
    pii_masking:
      enabled: true
      default_unmask_on_copy: false
      logs_actor_email: false
  keybindings:
    nav.down: "j"
    nav.up: "k"
    app.quit: "q"
    search.open: "/"
  logs:
    poll_interval_seconds: 7
  debug: false
  ```
- Key ID (`nav.down` 등)는 `internal/keys/keys.go`의 상수와 1:1. 오타는 `validate.go`가 실패시킴.

---

## 8. 생성자 · Options

```go
// Recommended form
func NewUsersService(port domain.UsersPort, opts ...ServiceOption) *UsersService { ... }

type ServiceOption func(*serviceOptions)

func WithCacheTTL(sec int) ServiceOption    { return func(o *serviceOptions) { o.CacheTTLSeconds = sec } }
func WithClock(clk clock.Clock) ServiceOption { ... }
func WithLogger(log *slog.Logger) ServiceOption { ... }
```

- 필수 의존은 positional, 선택 의존은 Options.
- `...Option` slice 없는 경우도 OK, 하지만 테스트 시 Logger/Clock 주입은 거의 공통 → Options로 수용.

### 8.1. Screen Model 생성자

Bubbletea Screen Model은 `Deps` 구조체 주입을 기본 패턴으로 한다:

```go
// internal/tui/users/list.go
type Deps struct {
    Port   domain.UsersPort
    Clock  clock.Clock
    Logger *slog.Logger
    Width  int   // 초기 터미널 크기 (teatest 테스트용)
    Height int
}

func NewListModel(deps Deps) ListModel { ... }
```

- 테스트는 `NewListModel(Deps{Port: fake, Clock: clock.NewFake(...)})`로 주입.
- **production 코드에 `SetUsers(xs)` 같은 테스트-only setter 금지** (§10.1 Elm 원칙). 초기 상태가 필요한 테스트는 생성자/Deps 경로로 전달.

---

## 9. Goroutine · 동시성

- **Goroutine 직접 생성 금지.** 모든 비동기 I/O는 `tea.Cmd` 내부에서 실행.
- `tea.Cmd`의 함수도 자체적으로 `go func(){}` spawn 금지. Cmd는 동기 함수 → Msg 반환.
- 진정 필요하다면 (예: tail 중 병렬 rate limit monitor 구독) **코드 주석으로 이유 + 종료 책임자 명시**.
- `sync.Mutex` 사용 금지 원칙. Elm arch가 상태 변경을 직렬화. 예외는 주석 + 테스트.
- `context.Context` 취소가 항상 `select` 내에서 처리되도록 작성.

---

## 10. Bubbletea 관용

### 10.1. Update 함수

- **순수.** I/O 없음, 시간 없음, 난수 없음.
- 리턴: `(tea.Model, tea.Cmd)`. 변경 없으면 `return m, nil`.
- **상태 변경은 오직 Update를 통해서만** (Elm 원칙). Screen Model에 외부에서 상태를 덮어쓰는 public setter 금지. 테스트용 초기 상태도 생성자 `Deps`로 주입 (§8.1).
- 거대한 switch 대신 **헬퍼 메서드로 분리**:

```go
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKey(msg)
    case UsersLoadedMsg:
        return m.onLoaded(msg)
    case ErrorMsg:
        return m.onError(msg)
    }
    return m, nil
}
```

### 10.2. View 함수

- 순수 렌더. 에러나 I/O 금지.
- 터미널 크기 읽기는 `tea.WindowSizeMsg`를 저장해 두고 View에서 참조.
- 스타일은 `shared/styles.go`의 토큰만 사용. inline `lipgloss.NewStyle()` 남발 금지.
- **테이블 컬럼 렌더링은 `bubbles/table` 또는 동등 implementation(자체 컬럼 포매터)을 허용.** TUI_DESIGN §15는 컬럼 헤더·정렬·반응형 drop 등 *시각 결과*를 명세하며, 구현 방식(bubbles/table vs 자체)은 자유. v0.2에서 `bubbles/table` 일괄 마이그레이션은 옵션 (PM 판단 2026-04-26).

### 10.3. Msg 타입

- suffix `Msg`. pkg 단위로 선언.
- 전역 Msg(`RateLimitObservedMsg`, `ErrorMsg`, `ToastMsg`, `ProfileSwitchedMsg`)는 `internal/app/msg.go`에.
- Msg는 값 타입(struct) 선호. 포인터는 크고 mutable일 때만.

### 10.4. Cmd 팩토리

```go
func fetchUsers(ctx context.Context, svc *service.UsersService, q domain.UsersQuery) tea.Cmd {
    return func() tea.Msg {
        iter, err := svc.Search(ctx, q)
        if err != nil {
            return ErrorMsg{Err: err, Source: "users.list"}
        }
        // iterator 첫 페이지 draining (작게, UI 빠르게)
        items, next, err := domain.DrainPage(iter)
        if err != nil {
            return ErrorMsg{Err: err, Source: "users.list.drain"}
        }
        return UsersLoadedMsg{Items: items, Next: next}
    }
}
```

- Cmd는 ctx와 의존성을 **인자로** 받는다 (클로저 + 전역 지양).

---

## 10a. Form Widget Pattern (REQ-W01 v0.2)

**위치:** `internal/tui/shared/form/` — 도메인-agnostic 재사용 가능한 다중 필드 입력 폼. 첫 사용처는 REQ-W01 Users Edit (`internal/tui/users/edit.go`). v0.2 lifecycle mutation에서도 재사용 기대 (OI-W5 Option C).

**철학:** Form 위젯은 *상태와 검증을 owning* 하되 *사이드 이펙트는 owning screen에 위임*한다. 도메인 import 금지, tea.Cmd 발사 금지.

### 10a.1. FieldSpec / Form / 매퍼 구조

```go
// internal/tui/shared/form/spec.go
type FieldKind int
const (
    KindText FieldKind = iota
    KindEmail        // *@*.* 클라이언트 hint (느슨)
    KindPhone        // E.164 advisory only — 차단 없음
    KindReadOnly     // 표시만, 입력 박스 없음 — 예: profile.login
)

type FieldSpec struct {
    Key      string    // Okta API field 이름 (예: "firstName") — BadRequestError.FieldError.Field 와 1:1
    Label    string    // 표시 라벨 (예: "First Name")
    Kind     FieldKind
    Required bool      // *  prefix + Validate에서 빈값 차단
    PII      bool      // mask.* 라우팅 + Alt+m 토글
    Section  string    // 섹션 헤더 (예: "Identity")
    Hint     string    // 입력 박스 아래 advisory (info color)
    MaxLen   int       // 0 = unbounded
}

// 폼 외부에서는 specs + initial map 으로 인스턴스화.
form := form.New(specs, initial map[string]string,
    form.WithLogger(log),
    form.WithErrorMapper(form.DefaultErrorMapper()),  // 기본 BadRequestError 매퍼
)
```

### 10a.2. dirty 추적 — snapshot vs current

```go
// 매 keystroke 시 form.Update가 호출 → fields[i].Current 갱신
// dirty 비교는 lazy + O(N) per render (N=11). 11 fields × 60fps = 660 비교/s, 무시 가능.

func (f *Field) Dirty() bool { return f.Current != f.Original }

func (f Form) Dirty() int    { /* count fields with f.Dirty() */ }
func (f Form) Diff() map[string]string { /* Spec.Key -> Current for dirty only */ }
```

- **same-as-snapshot 복원도 dirty=0로 인식**: 사용자가 변경했다가 원래대로 돌리면 dirty 마커가 사라진다. AC-9.1/9.2 명세.
- **PII 마스킹 상태는 dirty와 무관**: focus auto-unmask가 Current를 변경하지는 않는다. 입력 자체가 dirty의 기준.

### 10a.3. Validation lifecycle

| 단계 | 트리거 | 동작 |
|------|--------|------|
| Field-level (focus-out) | focus 이동 시 | 해당 필드만 `Validate()` → `InlineError` 설정 또는 클리어 |
| Form-level (Save attempt) | Ctrl+S | 모든 필드 `Validate()` → 실패하면 첫 invalid 필드로 focus, `form.SaveRequestedMsg` 발송 안 함 |
| Server-level (Save 응답) | `form.ApplyServerErrors(causes)` | `Field.InlineError` 또는 `OtherErrors` 누적 (§9.5) |

- **클라이언트 검증은 느슨** (AC-3.1~3.5): `Required` + KindEmail의 `*@*.*` 정도만. 길이·복잡한 형식은 서버에 위임 → 사용자가 빠르게 진행 가능, 클라이언트 잘못된 차단 회피.
- inline error는 **사용자가 해당 필드를 수정하면 자동 클리어** (낙관적 UX, AC-6.2). Form.Update에서 keystroke 처리 후 `InlineError = ""`.

### 10a.4. Save lifecycle (Form ↔ Screen 책임)

```
Form (재사용 위젯)                    Screen (예: tui/users/edit.go)
─────────────────────                ────────────────────────
1. user types … dirty++              -
2. user presses Ctrl+S
3. Validate() — pass                 -
4. emit form.SaveRequestedMsg ──────► onSaveRequested():
                                       - state = saving
5. SetSaving(true) (called by screen)
   - blur all inputs
   - dim styling
   - disable Esc                        - patch := buildPatch(form.Diff())
                                        - return Cmd{saveUserProfile(ctx,...,patch)}
6.                  (Cmd resolves)
                                      onSucceeded(UserUpdateSucceededMsg):
                                        - popNav()
                                        - broadcast UserUpdatedMsg{user}
                                        - toast "Updated <login>"
                                      onFailed(UserUpdateFailedMsg):
                                        - SetSaving(false)
                                        - if BadRequestError:
                                            form.ApplyServerErrors(causes)
                                        - if Forbidden/TokenInvalid/OktaServer:
                                            toast + form 유지
                                        - if NotFound:
                                            popNav + RefreshScreenMsg
                                        - if RateLimited:
                                            footer countdown + auto retry tick
```

### 10a.5. Discard-confirm 흐름 (Esc on dirty)

```
Form receives Esc:
  if Dirty() == 0:    emit form.DiscardRequestedMsg{Confirmed: true}
  else:               emit form.DiscardRequestedMsg{Confirmed: false}  // screen opens modal

Screen.onDiscardRequested:
  if msg.Confirmed:    popNav()  // immediate
  else:                m.overlay = OverlayDiscardConfirm
                       // modal renders Form.DirtyFields() labels

OverlayDiscardConfirm:
  y / Y:  popNav() — discard
  n / N / Esc:  close overlay, return to editing
```

- Form은 모달을 자기 안에서 띄우지 않는다 (Overlay 등록은 App Shell 책임). 단 `DirtyFields() []string`을 expose하여 모달이 변경된 필드 라벨을 표시할 수 있게 한다.

### 10a.6. ErrorMapper — `BadRequestError.Causes` → field-level inline

```go
// internal/tui/shared/form/errmap.go
type ErrorMapper interface {
    Apply(form *Form, err error)
}

// DefaultErrorMapper 가 표준 처리:
//   - *domain.BadRequestError → causes 순회, FieldError.Field 가 FieldSpec.Key 와 매치되면
//                              해당 Field.InlineError = cause.Summary. 미매칭 → OtherErrors.
//   - 그 외 도메인 sentinel → screen 이 toast 로 처리 (Form 외부 책임)
func DefaultErrorMapper() ErrorMapper
```

- ota는 새 parser를 만들지 않는다. `errormap.splitCause`가 이미 `<field>: <reason>` prefix를 파싱하여 `FieldError{Field, Summary}` 로 채워 준다 (도메인 §6.1). Form은 그 결과를 받기만 한다.
- `FieldSpec.Key` ↔ `domain.FieldError.Field` 문자열 일치가 매핑의 단일 기준. Users edit의 11개 키는 **모두 Okta API field 이름과 동일** (예: `firstName`, `mobilePhone`).

### 10a.7. PII 마스킹 + 폼 통합 규약

`internal/mask/` 의 기존 함수를 form 컨텍스트에서 그대로 사용. 단 form-specific 동작 명시:

| 상태 | 표시 | 비고 |
|------|------|------|
| 진입 시 (`PII=true`, not focused) | `mask.Phone(v)` / `mask.Email(v)` 적용된 값 | 기본 마스킹 ON (AC-7.1) |
| 사용자 Tab/click으로 focus | **자동 unmask** — 입력 박스 안 값이 원본으로 교체 | AC-7.2. unmask 배지(`[M!]`) 표시하지 않음 (focus 자체가 의도 표명) |
| focus out + 미수정 | 다시 마스킹 | AC-7.3 |
| focus out + 수정됨 | 마스킹 안 함 (사용자가 입력한 값 그대로) + dirty marker `*` | AC-7.4 |
| `Alt+m` (form context) | 모든 PII 필드 일괄 mask/unmask 토글 (`PIIToggleMsg` 발송) | AC-7.5. **글로벌 `m`은 form에서 의미 변경됨 — Alt+m으로 재배치** (TUI Design §3.3 DR-2). `m`은 영문자이므로 textinput이 입력으로 소비. |

**Alt 키 호환성:** Bubbletea `tea.KeyMsg.Alt` 필드가 모든 주요 터미널(iTerm2, Alacritty, Kitty, Terminal.app, GNOME Terminal)에서 ESC prefix 시퀀스로 안정 동작. tmux 환경에서는 `set -g xterm-keys on` 권장. fallback 없음 (Ctrl+M = Enter 충돌).

### 10a.8. 키 가로채기 (textinput 위에서)

Form은 `bubbles/textinput` 위에 **얇은 key intercept** 레이어를 둔다:

```go
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
    if km, ok := msg.(tea.KeyMsg); ok {
        // form-level keys — textinput 에 도달하기 전에 가로챔
        switch {
        case key.Matches(km, f.keys.Save):    // Ctrl+S
            return f.handleSave()
        case key.Matches(km, f.keys.Cancel):  // Esc
            return f.handleCancel()
        case key.Matches(km, f.keys.NextField):    // Tab
            return f.focusNext()
        case key.Matches(km, f.keys.PrevField):    // Shift+Tab
            return f.focusPrev()
        case km.Alt && km.Runes != nil && km.Runes[0] == 'm':  // Alt+m
            return f.togglePII()
        }
    }
    // 그 외는 active field 의 textinput.Update 로 위임
    f.fields[f.focused].input, cmd = f.fields[f.focused].input.Update(msg)
    f.recomputeDirty()
    f.clearFieldErrorOnEdit()
    return f, cmd
}
```

- Form 내부에서 글로벌 `:` palette / `?` help / `/` search 는 **활성화하지 않음** (TUI Design §3.1 DR-3). textinput이 그대로 글자로 소비.
- `Ctrl+C` 만 saving 상태에서 의미 있음 — screen 레이어에서 ctx cancel.

### 10a.9. 금지 사항

- **Form은 `domain.*` 타입을 import 하지 않는다.** screen이 `Form.Diff() map[string]string`을 받아 `domain.UserProfilePatch`로 조립한다. 이 가드가 form의 재사용성을 보장.
  - 예외: `domain.FieldError`는 `BadRequestError`의 일부로 `ErrorMapper`가 받아 처리. 단 form은 이 타입의 *해석*만 하고 *생성*하지 않음.
- **Form은 `tea.Cmd`를 발사하지 않는다.** 모든 사이드 이펙트는 screen이 트리거. Form은 메시지만 emit.
- **Form은 `internal/okta`, `internal/service`를 import 하지 않는다.** depguard로 강제 (`.golangci.yml`).
- **Form 내부 상태에 대한 외부 setter 금지** (§8.1 Elm 원칙). 초기 상태는 `form.New(specs, initial map)`로만.

### 10a.10. 테스트 경계 (TESTING.md §6.7 / §8.7 참조)

- Form 자체는 **순수 unit test** — Port fake 없이 keystroke 시퀀스만으로 동작 검증.
- screen (예: `users.EditModel`)은 **teatest + UsersPortFake** — fetch / save / failure 시나리오 통합 검증.
- 11 fields × 4 sections × dirty matrix는 **테이블 드리븐**으로 작성 (TESTING.md §8.7 참조).

---

## 11. 키 바인딩 규약

### 11.1. Key ID 체계

- 형식: `<scope>.<verb>`
  - scope: `nav` / `app` / `search` / `cmd` / `resource.users` / `resource.policies.typeselect` 등
  - verb: `down`, `open`, `quit`, `refresh`, `tail.toggle`, …
- TUI_DESIGN §3의 모든 키를 `internal/keys/keys.go`에 상수로 등록.
- 사용자 override는 **정확히 이 ID를 YAML에 명시**.

### 11.2. 키 문자열 포맷

- 단일 키: `"j"`, `"q"`, `"/"`
- Ctrl: `"Ctrl-c"`, `"Ctrl-b"`
- 합성(chord): `"g g"` (공백 구분)
- 특수: `"Esc"`, `"Enter"`, `"Tab"`, `"Shift-Tab"`, `"Space"`

### 11.3. 충돌

- 사용자 override가 빌트인과 충돌 시 **사용자 우선** (REQ-C03 AC-2).
- 한 화면 내 동일 키 중복 매핑 → 부팅 시 경고 로그 + `:errors`에 기록.

---

## 12. PII 마스킹 규약

### 12.1. 원칙

- **뷰가 기본 마스킹**. 데이터 저장 · 로그 · 서비스 계층은 원본 유지 (unmask 가능해야 하므로).
- `internal/mask.Phone(v, keepLast=4) string` 같은 순수 함수.
- 마스킹 토글: `:unmask` 커맨드 + 자동 재마스킹 트리거(세션 전환, 60s inactivity) — TUI_DESIGN §7.2.

### 12.2. 코드 규칙

- View 렌더 직전에 `mask.Phone(user.MobilePhone)` 호출. Service/도메인은 원본 통과.
- Clipboard 복사(`y`): 기본 마스킹된 값 복사. unmask 상태일 때만 원본.
- 로거에는 `mask.Phone(v)` 대신 **값 자체를 넘기지 않기**. 필드 이름으로만 기록하고 value 생략.

### 12.3. 폼 컨텍스트 마스킹 (REQ-W01)

Form 위젯(`internal/tui/shared/form/`)에서의 PII 필드 동작은 §10a.7과 동일 — 요약:

- 폼 진입 시 `PII=true` 필드는 **기본 마스킹 ON**.
- 사용자 focus → 자동 unmask (focus가 의도 표명. 별도 `[M!]` 배지 없음).
- focus out + 미수정 → 다시 마스킹.
- focus out + 수정 → 사용자 입력값 그대로 표시(마스킹 안 함).
- **토글 키는 form 컨텍스트에서 `Alt+m`** — 글로벌 `m`은 textinput이 글자로 소비하므로 충돌 회피 (TUI Design §3.3).
- debug log에는 PII 필드의 raw 값을 **절대** 흘리지 않는다. form Update 경로에서 추가 로그를 남기지 말 것 (keystroke마다 PII가 흐를 수 있음).

---

## 13. 테스트 (test-engineer 리드 — 상세는 TESTING.md)

> 본 섹션은 **CONVENTIONS 스코프의 요약**이다. 피라미드·시나리오 매핑·teatest 패턴·REQ 매트릭스는 `docs/TESTING.md` 참조.

### 13.1. 기본 원칙

- **테이블 드리븐 기본.** 동일 로직의 다중 케이스는 항상 서브 테스트.

  ```go
  func TestMaskPhone(t *testing.T) {
      cases := []struct {
          name, in, want string
      }{
          {"us 10 digits", "+1-555-123-4567", "+1-***-***-4567"},
          {"short no change", "1234", "1234"},
      }
      for _, c := range cases {
          t.Run(c.name, func(t *testing.T) {
              t.Parallel()
              got := mask.Phone(c.in)
              if got != c.want {
                  t.Fatalf("mask(%q) = %q, want %q", c.in, got, c.want)
              }
          })
      }
  }
  ```

- **`t.Parallel()` 기본 on.** 공유 가변 상태가 있으면 병렬 금지가 아니라 **상태부터 제거**한다 (설계 스멜).
- **Fail-First.** 새 기능·버그 수정은 실패 테스트 먼저. Red 확인 → Green → Refactor. 자세한 절차·로그 규약은 `tdd-fail-first` 스킬 + TESTING §7 참조.
- **REQ-ID 추적.** 각 테스트는 함수명 또는 선두 주석에 대응 REQ/AC 명시. 예: `// REQ-R01 AC-6 Factors 섹션 표시`.

### 13.2. 테스트 이름

- 기본: `TestXxx` + 서브 테스트 (§3.5 재확인).
- 복합 조건: `Test_<Unit>_<Scenario>_<Expectation>` — 예: `Test_UsersService_Search_CacheHit_ReturnsCached`.
- 회귀 테스트: 함수명은 자유, **첫 줄 주석에 `// regression: <issue/PR>`** 포함.
- Fail-First 아닌 Lock-in 테스트: 파일 상단에 `// Lock-in test (not Fail-First derived): <이유>`.

### 13.3. Mock 전략

- **Port 인터페이스 수준 fake가 기본.** SDK 자체를 mock하지 않는다 (SDK-어댑터 경계 버그를 놓치므로).
- **수동 fake 우선** — `Func` 필드 패턴:

  ```go
  // internal/service/fakes/users_port_fake.go
  type UsersPortFake struct {
      t        *testing.T
      ListFunc func(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error)
      GetFunc  func(ctx context.Context, id string) (domain.User, error)
  }
  func (f *UsersPortFake) List(ctx context.Context, q domain.UsersQuery) (domain.Iterator[domain.User], error) {
      f.t.Helper()
      if f.ListFunc == nil { f.t.Fatalf("UsersPortFake.List called but ListFunc not set") }
      return f.ListFunc(ctx, q)
  }
  ```

- **복잡 응답 시퀀스만** `testify/mock` 예외 허용.
- **`gomock`/`mockgen` 금지** (D-H 합의, 코드 생성 단계 회피).

### 13.4. 테스트 파일 위치

- **로컬 골든 (TUI 렌더):** `internal/tui/<resource>/testdata/<Test_...>.golden`. 해당 패키지의 테스트가 소유. 업데이트는 `go test -update ./internal/tui/...`.
- **공유 HTTP 픽스처 (Adapter integration):** `testdata/oktaapi/<resource>/<scenario>.json`. 여러 패키지가 공동 사용 (service integration 테스트, okta adapter 테스트 모두).
- **설정 파일 샘플:** `testdata/config/`.
- **공유 Port fake:** `internal/<consumer>/fakes/<port>_fake.go`. 소비자 패키지(service, tui) 기준.
- 규약: TUI 로컬 자원은 **패키지-로컬 testdata/**, 어댑터·인테그레이션 입력은 **루트 testdata/**. 이 분리를 깨지 말 것 (중앙 `testdata/golden/`로 몰면 TUI 리팩터가 루트를 건드려 diff가 커진다).

### 13.5. 의존성 주입 (테스트 편의)

- 모든 서비스·어댑터는 `WithClock`, `WithLogger` 같은 Options 수용. (§8)
- 테스트에서는 `clock.NewFake(...)`, `slog.NewJSONHandler(io.Discard, ...)` 주입.
- Model 생성자는 `Deps` struct 또는 Options 형태로 외부 의존 모두 주입받도록. production에 테스트-only setter(`SetXxx`) 금지 — 필요 시 생성자 경로로.

### 13.6. 시간·랜덤·네트워크

- **시간:** `internal/clock.Clock` 주입. `FakeClock`으로 `Advance(d)` 명시적 제어.
- **랜덤:** seed 고정 또는 `math/rand/v2` + `rand.New(rand.NewPCG(1, 1))`. Jitter도 `internal/clock.Jitter` 주입.
- **네트워크:** 기본 `httptest.Server` + `testdata/oktaapi/` fixture. 실 Okta 호출은 `//go:build integration` 태그로 완전 분리.

### 13.7. `testfx` 활용

- **위치:** `internal/okta/testfx/`. 테스트 헬퍼 전용 패키지. `_test.go`가 아닌 일반 Go 파일이지만 프로덕션 진입점(`cmd/ota`)이 import하지 않아 바이너리에서 제외된다 (lint 규칙으로 `cmd/ota`의 testfx import 금지).
- **제공 API(예시):**
  - `testfx.NewFakeOktaServer(t, scenario)` — 시나리오 이름으로 httptest.Server 조립
  - `testfx.LoadFixture(t, path)` — JSON 파일 + meta 파일 로드
  - `testfx.LoadHTTPResponse(t, path)` — 에러 fixture를 `*http.Response`로
- TUI 테스트도 `testfx`의 `NewFakeOktaServer`를 경유해 service 전체 스택을 돌릴 수 있다. Port fake보다 무겁지만 실제 HTTP 경로를 거친다.

### 13.8. Fail-First 로그

- 새 테스트의 **첫 Red 실행 출력**을 `_workspace/05_test_fail_log_YYYY-MM-DD.txt`에 append.
- 포맷 및 흐름은 TESTING §7.2 참조. 미적 장식 없이 `go test` 원문 + REQ-ID 헤더만.
- Phase 5 PR에는 본 로그 링크를 설명에 포함.

### 13.9. Flaky 방지

- **`-race` 필수 CI.**
- **goroutine leak:** `goleak.VerifyTestMain(m)` 각 패키지 `TestMain`에. teatest 관련 내부 goroutine은 allowlist로 (TESTING §9.3).
- 임시 격리 시 `//go:build flaky` 태그 + GitHub issue. 수정 전 머지 금지 (TESTING §9.5).

---

## 14. Commit · PR

### 14.1. Commit 메시지 (Conventional Commits)

- 형식: `<type>(<scope>): <subject>`
- type: `feat` / `fix` / `refactor` / `test` / `docs` / `chore` / `ci` / `perf` / `build` / `revert`.
- scope: 가장 영향받는 패키지 (예: `tui/users`, `okta`, `service`, `docs`).
- subject: 소문자 시작, 50자 이내, 마침표 금지, 동사 현재형.
- 본문(선택): WHY. 72자 줄바꿈.

**예:**
```
feat(tui/users): add factors tab with phone masking

- Adds Factors section per REQ-R01 AC-6.
- Uses mask.Phone for SMS/Voice factor phoneNumber.
```

### 14.2. PR

- **크기:** 가능하면 300 LOC 이하 diff. 큰 기능은 시리즈 PR로.
- **내용:**
  - 관련 REQ-ID / SCR-ID / 이슈 번호 명시
  - 테스트 추가 (신규 기능) / 업데이트 (리팩터)
  - 문서 변경 (영향받는 경우)
- **CI 통과 필수.** 실패 상태에서 머지 금지.
- **리뷰어:** 최소 1명. 설계 변경은 2명.
- **머지:** squash or merge-commit. rebase는 긴 시리즈만.

### 14.3. 브랜치

- `main`: 항상 배포 가능.
- 기능: `feat/<topic>`, 버그: `fix/<topic>`, 문서: `docs/<topic>`.
- 머지 후 브랜치 삭제.

---

## 15. 의존성 추가 심사

새 라이브러리 추가 PR은 다음을 본문에 포함해야 한다:

1. **대안 비교 (최소 2개).** 왜 이걸 골랐는가?
2. **라이선스** — Apache-2.0 / MIT / BSD-3-Clause만 허용.
3. **유지보수 지표** — 최근 커밋, 오픈 이슈 수.
4. **크기** — 간접 의존성 개수.
5. **Go 버전 호환** — 1.23 이상.

없이 추가 금지. 리뷰어는 이 섹션으로 심사.

---

## 16. 보안 체크리스트 (모든 PR)

- [ ] 토큰·PII 로그 노출 없음
- [ ] 새 외부 호출에 `context.Context` 전달
- [ ] 민감 필드 구조체에 Stringer 오버라이드 또는 마스킹
- [ ] HTTP URL 검증 (https only in config)
- [ ] 입력 검증: 설정 키, 키 바인딩 문자열, 쿼리 파라미터
- [ ] gosec 린터 경고 없음

---

## 17. 문서 동기화

다음을 변경한 PR은 해당 문서도 업데이트:

| 변경 | 업데이트 필요 |
|------|-------------|
| 새 REQ / AC 추가 | `docs/PRD.md` — PM 사전 승인 |
| 화면 · 키 변경 | `docs/TUI_DESIGN.md` — tui-designer 리뷰 |
| 레이어 / 포트 변경 | `docs/ARCHITECTURE.md` |
| 디렉토리 · 패키지 추가 | `docs/PROJECT_STRUCTURE.md` |
| 새 라이브러리 | `docs/TECH_STACK.md` + 이 문서 §15 체크 |
| 새 코드 규칙 | `docs/CONVENTIONS.md` (이 문서) |
| 테스트 전략 변경 | `docs/TESTING.md` |
| Form 위젯 인터페이스 변경 (`internal/tui/shared/form/`) | `docs/ARCHITECTURE.md` §6.8 + 이 문서 §10a + 사용 screen의 teatest 패턴 |
| 새 mutation 표면 추가 (REQ-W##) | `docs/ARCHITECTURE.md` §7.4 mutation 경로 표 + §13.4 플레이북 + 이 문서 §3.2 Patch 타입 명명 |

---

## 18. 레퍼런스

- Effective Go: https://go.dev/doc/effective_go
- Go Code Review Comments: https://github.com/golang/go/wiki/CodeReviewComments
- Uber Go Style Guide (참고): https://github.com/uber-go/guide/blob/master/style.md
- Bubbletea 예제: https://github.com/charmbracelet/bubbletea/tree/master/examples

---

**END of CONVENTIONS.md draft**
