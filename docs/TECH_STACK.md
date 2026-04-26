# ota Tech Stack

**Version:** v1.0.0
**Status:** Final (Phase 4)
**Last updated:** 2026-04-24
**Authors:** developer (lead), test-engineer (review — test libs)

---

## 변경 이력

| 버전 | 날짜 | 변경 | 작성자 |
|------|------|------|-------|
| v0.1.0-draft | 2026-04-24 | 초안 | developer |
| v1.0.0 | 2026-04-24 | D-E~D-L 합의 확정(Go 1.23 / koanf / slog / goleak / 수동 fake 우선). test-engineer 교차 리뷰 통과. | developer |
| v1.0.1 | 2026-04-24 | Phase 6 결정 반영: MVP는 직접 `net/http` (§4.1). SDK는 v0.2+ 옵션 (§4.2). team-lead 승인. | developer |

---

## 1. 선택 기준

모든 의존성은 아래 기준을 동시에 통과해야 한다.

1. **유지보수 활발성** — 최근 6개월 내 커밋, 이슈 응답. Archived 저장소 금지.
2. **의존성 간소함** — 간접 의존성 체인이 깊지 않은 라이브러리 우선. 특히 `internal/domain`은 stdlib만.
3. **라이선스 호환** — Apache-2.0 / MIT / BSD-3-Clause. LGPL·AGPL 금지.
4. **Go 생태계 관례 준수** — `context.Context` 수용, 명시적 에러, 인터페이스 중심.
5. **테스트 가능성** — 인터페이스 노출, DI 가능, httptest와 호환.

"적을수록 좋다"는 규칙을 강하게 적용한다. 비슷한 기능 두 라이브러리가 있으면 하나만 선택.

---

## 2. 언어 · 런타임

### Go 1.23+

**선택 이유:**

- **단일 바이너리 배포** — cross-compile로 macOS/Linux/WSL 즉시 지원.
- **`log/slog` (stdlib)** — 구조화 로깅. 외부 로거 라이브러리 불필요.
- **`slices`, `maps` (stdlib)** — 제네릭 컬렉션 유틸.
- **`iter.Seq`/`iter.Seq2`** — 페이지네이션 반복자 표현 가능성. (현재는 custom `Iterator[T]` 사용, v0.2에서 stdlib iter 전환 검토)
- **우수한 동시성 원시 타입** — Bubbletea의 tea.Cmd 비동기 모델과 잘 어울림.

**최소 버전:** `go 1.23` in `go.mod`.
**Toolchain:** `toolchain go1.23.N` (이하 Go 릴리즈에서 릴리즈된 최신 패치).

**대안 비교:**

| 대안 | 탈락 이유 |
|------|---------|
| Go 1.22 | `range over func`(1.23)가 페이지네이션에 유용. 1.22 지원 이점 없음. |
| Rust | TUI 생태(ratatui)도 탄탄하지만 Okta SDK 부재, 팀 숙련도, 배포 단순성에서 Go 우위. |

---

## 3. TUI 프레임워크 (Charm 생태)

### 3.1. Bubbletea
- **패키지:** `github.com/charmbracelet/bubbletea`
- **역할:** TUI 런타임. Model-Update-View + Cmd(비동기).
- **선택 이유:**
  - Elm Architecture 기반 → **순수 Update 함수 = 테스트 용이**.
  - k9s, gh-dash, lazygit-adjacent 등 검증된 프로덕션 TUI들이 사용.
  - `tea.Cmd`로 I/O 격리 → Hexagonal과 자연스럽게 결합.
- **대안 탈락:**
  - **tcell** — 저수준. 컴포지션/상태머신 직접 구현 부담.
  - **gocui** — 유지보수 활발도 낮음.
  - **termui** — 대시보드 중심, 인터랙션 부족.

### 3.2. Bubbles (표준 컴포넌트)
- **패키지:** `github.com/charmbracelet/bubbles`
- **사용 컴포넌트** (TUI_DESIGN §5 매핑):
  - `list` — 소규모 리스트 (profile select 등)
  - `table` — 대부분의 리스트 화면 (컬럼 기반)
  - `viewport` — 상세 (JSON pretty, Help 본문)
  - `textinput` — `/` 검색, `:` 커맨드 팔레트
  - `spinner` — 로딩 (Dot)
  - `progress` — 페이지네이션 진척
  - `help` — 도움말 힌트 바

### 3.3. Lip Gloss
- **패키지:** `github.com/charmbracelet/lipgloss`
- **역할:** 스타일링 (ANSI/truecolor, 박스, 정렬).
- **사용 원칙:** 모든 스타일은 `internal/tui/shared/styles.go` 한 곳에서 토큰으로 정의 (`styleHeader`, `styleDanger` 등). TUI_DESIGN §6.1 매핑 준수.

### 3.4. Huh (Forms)
- **패키지:** `github.com/charmbracelet/huh`
- **MVP 사용처:**
  - Profile Select (SCR-000) — `huh.Select`
  - 토큰 대화식 입력 — `huh.Input(EchoMode=Password)`
- **그 외는 사용 금지** (Huh는 형식 있는 폼용. 일상 UI는 Bubble 컴포넌트로 충분).

### 3.5. Glamour (Markdown 렌더)
- **패키지:** `github.com/charmbracelet/glamour`
- **사용처:** Help 본문(SCR-902)만. Markdown을 터미널 색상으로 렌더.
- **Fallback:** 터미널이 ANSI 미지원 시 plain text.

### 3.6. charmbracelet/x/exp/teatest (테스트)
- **패키지:** `github.com/charmbracelet/x/exp/teatest`
- **역할:** Bubbletea 프로그램 테스트 하네스. 키 주입·출력 snapshot.
- **사용 범위:** 화면 전환·키 흐름·tail 갱신 시나리오 (D-B 합의 반영). 순수 렌더는 golden 파일 비교로 충분.

**모든 Charm 패키지는 latest stable.** 서브-마이너 업그레이드는 Renovate로 주 단위 PR. 메이저는 별도 PR + 마이그레이션 노트.

---

## 4. Okta 통합

### 4.1. Okta 통합 전략 (MVP: 직접 net/http)

**v0.1 MVP 결정 (Phase 6, 2026-04-24):** MVP는 **`net/http` + `encoding/json`** 직접 구현을 채택한다. 이유는 httptest 시나리오 드라이버와 SDK의 base-URL 교체 동작 불일치, 그리고 내부 래퍼가 이미 얇고 Port 경계로 서비스·TUI에 영향이 없다는 점. SDK는 Phase 6+에 선택적으로 per-endpoint 전환 가능.

- **패키지:** `net/http`, `encoding/json` (stdlib만)
- **격리:** `internal/okta/` 외에 HTTP 코드 금지. 모든 요청은 `internal/okta/client.go`의 `doGet`을 경유. SSWS 헤더·429 Retry-After·rate-limit 헤더 관찰·errormap 매핑이 한 곳에서 처리됨.
- **향후:** `okta-sdk-golang/v5`는 v0.2에서 per-endpoint 전환 후보 (§4.2). 오픈 이슈로 기록.

### 4.2. Okta Go SDK v5 (옵션, Phase 6+)
- **패키지:** `github.com/okta/okta-sdk-golang/v5`
- **현 상태:** 의존성은 `go.mod`에 포함되어 있으나 MVP 런타임에서는 사용하지 않음. 테스트 하네스(`httptest.Server`)와 SDK의 host 설정 주입 방식이 맞지 않아 Phase 6에서 직접 HTTP 구현으로 우회.
- **격리:** `internal/okta/` 외 SDK import 금지 (depguard). 도입 시에도 SDK 타입은 어댑터 경계에서 `domain.*`로 매핑.
- **대안 탈락:**
  - **okta-sdk-golang/v2** — deprecated.

---

## 5. 설정

### 5.1. knadh/koanf (결정)
- **패키지:** `github.com/knadh/koanf/v2` + `providers/file`, `providers/env`, `parsers/yaml`
- **선택 이유:**
  - 가벼운 core, 필요한 provider/parser만 넣음 (의존성 최소).
  - "layered" merge 자연스러움 (default → file → env).
  - 테스트 용이 (provider swap 간단).
- **대안 비교:**

| 대안 | 탈락 이유 |
|------|---------|
| **spf13/viper** | 의존성 크다(2025년 기준 20+ indirect). cobra와 세트 문화. |
| **caarlos0/env + stdlib yaml.v3** | 직접 병합·오버라이드 코드 수동. 코드량 ↑. |
| **rawYAML+직접 병합** | 가능하지만 key 별 오버라이드·기본값이 번거로움. |

### 5.2. YAML 파서
- **패키지:** `gopkg.in/yaml.v3` (koanf 내부 사용)
- **이유:** Go 생태 표준. `v2`는 deprecated.

### 5.3. XDG 경로
- **직접 구현** (단순 — `os.UserConfigDir` + `$XDG_CONFIG_HOME` 우선).
- 대안 `adrg/xdg`: 기능 과잉. 단순 path resolution만 필요.

---

## 6. 로깅

### 6.1. log/slog (결정)
- **패키지:** `log/slog` (stdlib)
- **선택 이유:**
  - Go 1.21+ stdlib. 외부 의존 0.
  - JSON handler 내장. 필드 기반 구조화.
  - 레벨링 내장.
- **대안 탈락:**
  - **zerolog** — 더 빠르지만 ota 로그 볼륨 작음. 성능 이득 < 의존성 비용.
  - **zap** — 동일 사유. 학습 곡선도 있음.
- **Handler 설정:** JSONHandler → file(`~/.cache/ota/debug.log`, 0600) + `io.Discard` (prod stdout은 TUI 충돌 방지).
- **마스킹 middleware:** slog의 `ReplaceAttr`로 민감 키 (`authorization`, `api_token`, `mobilePhone`, `secondEmail`) 값 `***` 치환. `internal/logger/mask_attr.go`.

### 6.2. 로그 로테이션
- **패키지:** `gopkg.in/natefinch/lumberjack.v2`
- **역할:** 10MB × 3 파일 rotation (REQ-O01 AC-3).
- **대안:** `rs/zerolog/log` 자체 제공 로테이션 없음, slog도 없음 → lumberjack로 통일.

---

## 7. 유틸리티

### 7.1. UUID
- **패키지:** `github.com/google/uuid`
- **용도:** session_id 생성 (REQ-O01 상관ID).
- **대안:** stdlib `crypto/rand` + 직접 hex — 가능하나 몇 줄 더. `uuid`는 이미 간접 의존성에 포함될 가능성 높음.

### 7.2. 시간 유틸 / Clock 주입
- **직접 구현** — `internal/clock`에 `Clock` 인터페이스.
- 대안 `benbjohnson/clock`: 유명하지만 프로젝트 archived 상태 확인 필요. 자체 3줄 인터페이스로 충분.

### 7.3. Terminal detection
- **패키지:** `github.com/mattn/go-isatty` (Bubbletea 간접)
- **용도:** NO_COLOR 검출 시 fallback.

---

## 8. 테스트 스택

### 8.1. 표준 & assertion
- **`testing`** (stdlib)
- **`github.com/stretchr/testify`**
  - `assert`, `require` — 강타입 단정
  - `mock` — 경량 mock (수동 fake 우선, 불가피할 때만)

### 8.2. HTTP 시뮬레이션
- **`net/http/httptest`** (stdlib) — **주** 도구. Okta API 응답 흉내 서버.
- **`github.com/jarcoal/httpmock`** — (보조) 특정 URL 매칭형 테스트 (선택).

### 8.3. TUI 테스트
- **`github.com/charmbracelet/x/exp/teatest`** — 실험적이지만 Charm 공식.
- 스냅샷 비교: **자체 golden helper** (`testdata/golden/<case>.txt`, `-update` 플래그).

### 8.4. Goroutine leak
- **`go.uber.org/goleak`** — 테스트 main에서 리크 감지 (D-K 합의 반영).

### 8.5. Clock in tests
- `internal/clock.FakeClock` — 자체 구현.

---

## 9. 품질 도구

### 9.1. 포맷
- **gofumpt** (`mvdan.cc/gofumpt`) — gofmt 엄격 버전. CI에서 `gofumpt -l -d .` 차이 없어야 통과.

### 9.2. 린트
- **golangci-lint** — 런너.
- **활성 리너** (`.golangci.yml`):
  - `errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused` (기본)
  - `gosec` — 보안 (토큰/쉘/path 취약성)
  - `gocritic` — style·performance
  - `revive` — naming
  - `copyloopvar` — Go 1.22+ loop var semantic
  - `bodyclose` — HTTP body leak
  - `contextcheck` — context 전파
  - `exhaustive` — switch 완전성(enum)
  - `nilerr` — error 값 처리
  - `prealloc` — slice preallocation
  - `tparallel` — t.Parallel 올바른 사용

### 9.3. 취약점
- **govulncheck** — stdlib/의존성 CVE. CI 주 단위.

### 9.4. 빌드
- **Makefile** 타겟:
  - `make build` — 정적 바이너리
  - `make test` — unit + adapter integration + tui component
  - `make test-race` — `-race` 추가
  - `make test-e2e` — 실 Okta Sandbox (선택, 환경변수 요구)
  - `make lint` — gofumpt + golangci-lint + govulncheck
  - `make ci` — 위 전부
  - `make run` — 로컬 실행 헬퍼

---

## 10. CI / CD

### 10.1. GitHub Actions

**Pipeline:**

1. `lint` — gofumpt + golangci-lint
2. `test` — go test ./... + coverage upload
3. `test-race` — go test -race ./...
4. `vuln` — govulncheck
5. `build-matrix` — darwin-amd64, darwin-arm64, linux-amd64, linux-arm64
6. (weekly) `deps-update` — Renovate PR

### 10.2. 의존성 관리
- **Renovate** — 주간 마이너 업그레이드 PR, 메이저는 별도 PR.
- **go.sum** 검증 (`go mod verify`).

---

## 11. 의도적 배제 (선택하지 않은 것들)

| 후보 | 배제 이유 |
|------|---------|
| **spf13/cobra** | CLI 플래그 ≤5개, `flag` (stdlib)로 충분. cobra 도입 시 sub-command 구조 강제, ota는 단일 명령. |
| **spf13/viper** | koanf 선택(§5). |
| **google/wire**, **uber-go/fx** | 의존성 수동 주입. `cmd/ota/main.go` 한 곳에서 명시적. 프로젝트 규모 대비 DI 프레임워크 과잉. |
| **gorilla/mux** / 어떤 HTTP 라우터 | ota는 HTTP 서버가 아니다. |
| **GORM / sqlx** | DB 없음. |
| **protobuf / gRPC** | Okta는 REST. |
| **Cobra + Viper + Logrus + Gin** 조합 | 통념이지만 본 프로젝트 요구 대비 과한 스택. |
| **gomock / mockgen** | 코드 생성 단계 추가. 수동 fake + testify/mock로 충분 (D-H 합의). |
| **ginkgo / gomega** | BDD 스타일이 Go stdlib testing과 이질적. testing + testify로 통일. |
| **rs/cors, fsnotify, urfave/cli** | 불필요. |

---

## 12. 의존성 원 (최종 예상 go.mod direct)

```go
module github.com/tedilabs/ota

go 1.23

require (
    github.com/charmbracelet/bubbletea  vX.Y.Z
    github.com/charmbracelet/bubbles    vX.Y.Z
    github.com/charmbracelet/lipgloss   vX.Y.Z
    github.com/charmbracelet/huh        vX.Y.Z
    github.com/charmbracelet/glamour    vX.Y.Z
    github.com/charmbracelet/x          vX.Y.Z // exp/teatest (test-only tag 고려)
    github.com/okta/okta-sdk-golang/v5  vX.Y.Z
    github.com/knadh/koanf/v2           vX.Y.Z
    github.com/knadh/koanf/providers/file vX.Y.Z
    github.com/knadh/koanf/providers/env  vX.Y.Z
    github.com/knadh/koanf/parsers/yaml   vX.Y.Z
    gopkg.in/natefinch/lumberjack.v2    vX.Y.Z
    github.com/google/uuid              vX.Y.Z
    github.com/stretchr/testify         vX.Y.Z // test-only
    github.com/jarcoal/httpmock         vX.Y.Z // test-only (optional)
    go.uber.org/goleak                  vX.Y.Z // test-only
)
```

**버전 핀:** Phase 5 시작 직전에 각 패키지 최신 stable로 일괄 pin. `go get -u ./... && go mod tidy` → 릴리즈 노트 검토 후 커밋.

---

## 13. 버전 정책

- **MVP (v0.1):** 의존성 메이저 업그레이드 금지(기능 freeze).
- **마이너/패치:** Renovate 자동 PR, CI 통과 시 머지.
- **메이저:** 별도 Issue + 마이그레이션 PR. 영향받는 코드·테스트 함께 업데이트.
- **Go 버전:** 1.23. Go 1.24 릴리즈 후 2 릴리즈 뒤 승격 검토.

---

## 14. 라이선스 요약

| 의존성 | 라이선스 |
|--------|---------|
| charmbracelet/* | MIT |
| okta-sdk-golang | Apache-2.0 |
| knadh/koanf | MIT |
| stretchr/testify | MIT |
| google/uuid | BSD-3 |
| natefinch/lumberjack | MIT |
| jarcoal/httpmock | MIT |
| goleak | Apache-2.0 |

모두 허용 라이선스. ota 자체 라이선스는 Apache-2.0 (프로젝트 조직 관례).

---

**END of TECH_STACK.md draft**
