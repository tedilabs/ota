# 03. PM 디자인 검수 — TUI_DESIGN v0.1.0-draft

**리뷰어:** pm (ota-prd-team)
**날짜:** 2026-04-24
**대상:** `/Users/austin/workspace/tedilabs/ota/_workspace/03_tui_design_draft.md` (1,995줄)
**기준 문서:** `docs/PRD.md` v1.0.0 (REQ-U01~O01, §11.3 리더 결정 D-1~D-6)
**리뷰 관점:** (a) REQ-ID 매핑 충족성 / (b) 모호성 / (c) PRD 정의 불일치

---

## 총평

**결론: APPROVE WITH MINOR CHANGES.**

21개 REQ 전부가 명시적 매핑을 가지며 대부분은 정확하고 구체적이다. §0 디자인 원칙은 PRD §6.2 PII 마스킹, §11.3 리더 결정(k9s+Vim, 다크 테마, 7초 tail), 도메인 §9 운영 함정까지 강행 규칙으로 흡수했다. ASCII 와이어프레임·상태별 UI(Loading/Empty/Error/Rate-limited/Offline)·키 바인딩 충돌 검증·Bubble 매핑까지 구현 가능 수준으로 작성됨.

| 수준 | 건수 |
|------|------|
| BLOCKER (Phase 3 이관 차단) | 0 |
| MAJOR (반영 권고) | 4 |
| MINOR (개선 권고) | 7 |
| NIT (선택) | 3 |

Must-fix 0건. v2에서 MAJOR 4건 반영하면 `docs/TUI_DESIGN.md`로 확정 가능.

---

## 1. REQ-ID 매핑 충족성 점검

| REQ | PRD 핵심 요구 | 디자인 충족 | 판정 |
|-----|---------------|-------------|------|
| REQ-U01 | Vim hjkl/gg/G/Ctrl-d/u/f/b | §3.2 전역 네비 14키 전부 매핑 | ✅ PASS |
| REQ-U02 | `:` 팔레트, 탭 자동완성, 부분 매칭, 50개 히스토리 | SCR-900 + §3.4 팔레트 18개 명령, Ctrl-r reverse-search까지 | ✅ PASS |
| REQ-U03 | `/` 즉시 필터, n/N, \C 대소문자 | SCR-901 + AC-2/3/4 전부 | ✅ PASS |
| REQ-U04 | `/`=q, `:search`=SCIM, 리소스별 문법 차이 Help | §3.4 `:search`/`:filter` 분리, Help Commands 탭 | ✅ PASS (AC-5 eventually consistent 안내 배치는 m2 참고) |
| REQ-U05 | 리스트↔상세 탭↔연관 드릴다운, Esc 취소 | SCR-011/021 탭, Breadcrumb, `L/g/a/m` 직행 키 | ✅ PASS |
| REQ-U06 | 컨텍스트별 Help, 내부 `/` 검색, 커스텀 키 반영 | SCR-902 4탭, `/` 검색, 오버라이드 표시 | ✅ PASS |
| REQ-U07 | Ctrl-c 연타 즉시, 종료 보호 | SCR-910 + §3.1 hard_quit | ✅ PASS |
| REQ-R01 | Users 7 AC 전부 (컬럼, 색상, 탭 6종, 검색, Factors, DEPROVISIONED 포함) | SCR-010 + SCR-011, MAJOR M2는 Logs 탭 vs Recent Logs 탭 네이밍 정합 확인 필요 | ⚠ PASS with MAJOR-1 |
| REQ-R02 | Groups 5 AC (타입 아이콘, RULE 배지, BUILT_IN 배너, 앱 카운트) | SCR-020/021 — type 아이콘·RULE/SYS/LARGE 배지·Everyone 특별 라벨 | ✅ PASS |
| REQ-R03 | Group Rules 6 AC (상태 컬러, 비활성화 경고, id→name 해소) | SCR-030/031 전부 충족 | ✅ PASS |
| REQ-R04 | Policies 8 AC (4+3 분할, raw-only 배지, 확장성) | SCR-040/041/042, `(raw view)` 배지, Action Summary 매퍼 4종 | ⚠ PASS with MAJOR-2 (AC-8 확장성 연결 누락) |
| REQ-R05 | Logs 9 AC (tail 알고리즘, adaptive, 프리셋, SystemPrincipal) | SCR-050/051, `[TAIL 7s]`/`[ADAPTIVE]` 인디케이터, 프리셋 5종 | ✅ PASS |
| REQ-C01 | 설정 파일 4 AC | §11.3에 "UI 없음" + SCR-001 파싱 에러 | ✅ PASS |
| REQ-C02 | 프로필, `:profile`, 2s 전환 | SCR-000 + `:profile` 명령 | ⚠ PASS with MINOR-1 (2s 전환 상태 UI 미명시) |
| REQ-C03 | 키 커스터마이징 4 AC | Help에 오버라이드 표시, 리로드 재실행 | ✅ PASS |
| REQ-C04 | 토큰 우선순위 + errorCode 8종 매핑 | SCR-001 테이블 9종(NETWORK/DNS 추가 보너스) + SCR-905 token info | ✅ PASS |
| REQ-C05 | 시크릿 유출 방지 4 AC | §11.3 "토큰 값 UI 노출 없음", `:about`에서 소스만 노출 | ✅ PASS |
| REQ-E01 | Rate Limit 6 AC (동적 헤더, last-observed, 카테고리별, 30s 캐시) | §11.4 Header [RL], SCR-905 Rate limits, `[ADAPTIVE]`. **M3 last-observed 표시 완벽 반영** | ✅ PASS |
| REQ-E02 | 에러 토스트, 카운터, `:errors` | SCR-904 Session Errors, 3초 토스트 | ✅ PASS |
| REQ-E03 | offline 배지, 캐시 유지, 복구 재개 | Header `offline` 배지, 리스트별 "offline — cached" 상태 | ✅ PASS |
| REQ-O01 | 디버그 로그, `:debug open` | `:debug open` 팔레트 명령, 경로 안내 | ⚠ PASS with MINOR-2 (tail 접근성 논의) |

**결과:** 21/21 모두 매핑됨. BLOCKER 0건.

---

## 2. MAJOR — 반영 권고

### MAJOR-1. REQ-R01 User 상세 탭 네이밍: "Logs" vs "Recent Logs"
- **위치:** SCR-011 탭 바 "Logs" (line 595) vs REQ-R01 AC-3 "Recent Logs" 명시 + §11.1 "Logs 탭"
- **문제:** PRD는 **"Recent Logs"**로 표기 (최근 N건, 30d 범위라는 범위 컨텍스트 포함). 와이어프레임의 탭 라벨 "Logs"는 ota 전체 Logs 화면(SCR-050)과 동명이어서 혼동 유발 가능.
- **권고:** 탭 라벨을 **"Recent"** 또는 **"Activity"**로 변경. 내부 본문에 "Recent events for Alice (last 100 within 30d)" 유지. Help/문서에서 "User→Recent 탭은 해당 사용자의 System Log 부분 조회, 전체 Logs는 `:logs`"로 대비 명확화.

### MAJOR-2. REQ-R04 AC-8 확장성 — Policy 타입 카탈로그 설정화 연결 부재
- **위치:** PRD REQ-R04 AC-8은 "새 Policy 타입(예: CONTINUOUS_ACCESS) 추가 시 코드 변경 최소로 추가 가능하도록 내부 타입 카탈로그를 설정 가능 구조로" 명시. TUI_DESIGN의 SCR-040/042는 타입 7종을 하드코딩한 듯한 인상이며 확장성 언급이 §11.5 Nice-to-Have·§13.2에만 간접 등장.
- **문제:** Phase 4(구현)에서 타입 추가 시 TUI_DESIGN 수정 없이 config/카탈로그 추가만으로 확장 가능한가의 **명시적 설계 근거가 문서에 없음**.
- **권고:** §2.1 IA 또는 §5.1 tea.Model 제안 근처에 "**Policy 타입 카탈로그는 내부 `policy_types.yaml` 혹은 코드 상수로 외부화**. SCR-040 타입 선택 메뉴는 카탈로그를 순회해 렌더링. 새 타입 추가 시 (1) 카탈로그 entry (2) 풀 렌더러 또는 raw-only 매핑만으로 족함"이라는 **1~2문장 규약**을 추가.

### MAJOR-3. 도메인 §9.9/§9.7 일관성 — Users `search` eventually consistent 안내 위치
- **위치:** PRD REQ-U04 AC-5는 "Help에 eventually consistent 경고 명시". TUI_DESIGN SCR-902 Help는 4탭 구조(Screen/Global/Commands/Status icons). SCR-010 Users List의 Empty 힌트(line 519)에 "`/` uses Okta `q`, Use `:search` for fields"는 있으나 **eventually consistent 경고가 UI 어디에도 명시되지 않음**.
- **문제:** 운영자가 "방금 만든 사용자가 안 보인다"고 판단하기 전에 이 지식을 만나야 함. Help만으로는 발견률 낮음.
- **권고:** (a) SCR-902 Help의 **Commands 탭 또는 Screen 탭**의 `:search` 설명에 "⚠ Users: eventually consistent — recent creations may not appear for minutes" 한 줄 추가. (b) SCR-010 "Empty (필터 결과 0)" 상태 힌트의 `:search` 예시 밑에 "Note: recently created users may take minutes to appear in search (indexing lag)" 추가. 이중 노출로 발견률 확보.

### MAJOR-4. REQ-R05 AC-5 프리셋 — "Group Rule Deactivations" 위험 강조 어센트 확인
- **위치:** SCR-050 Preset 메뉴 line 1274 "`⚠ Group Rule Deactivations` (may remove memberships)"
- **문제:** 시각 경고(`⚠`) 및 부연 텍스트는 좋으나, 다른 프리셋(Failed Sign-ins, Group Rule Changes 등)과의 **구분이 문자 하나뿐**. PRD는 이 이벤트가 "멤버십 제거 부작용을 갖는 가장 위험한 이벤트"로 취급하므로 색상도 적용 권장.
- **권고:** 프리셋 메뉴에서 이 항목만 **노란색 또는 빨간색으로 렌더** (테마에서 "warning" 토큰 사용). `§6.1` 테마에 preset-warning 토큰 추가 또는 기존 warning 재사용.

---

## 3. MINOR — 개선 권고

### MINOR-1. REQ-C02 AC-3 "프로필 전환 < 2s" 관찰 UI 부재
- **위치:** SCR-000 `:profile` 명령 전환 시 로딩/확정 상태 명시가 없음.
- **권고:** `:profile prod` 후 "Switching to prod… (invalidating cache)" 1줄 토스트 추가. 완료 시 새 테넌트 표시가 Header L1에 반영됨을 강조.

### MINOR-2. REQ-O01 AC-4 `:debug open` 경로 안내 + 실 tail 부재 논의
- **위치:** `:debug open`은 "파일 tail 대체 — 경로 안내 메시지"로 설계됨. PRD AC-4는 "별도 창 대신 설명 메시지 OK"라 허용.
- **권고:** 현 설계 OK. 다만 Help Commands 탭에 "`:debug open` prints log path; use `tail -f` in another terminal" 한 줄 보완 권장.

### MINOR-3. SCR-011 User 상세 탭 제목의 카운트 숫자 로딩 표시 규약
- **위치:** "[ Groups 4 ] [ Factors 2 ]" 처럼 진입 전 카운트 표기되는 예시.
- **문제:** 카운트는 별도 API 호출 필요(`/users/{id}/groups`, `/users/{id}/factors`). 첫 진입 시 데이터가 없는 상태에서의 탭 제목 규약(로딩 중, 실패, 0) 미정.
- **권고:** `[ Groups … ]` (로딩 중), `[ Groups ? ]` (조회 실패), `[ Groups 0 ]` (실제 0). 또는 숫자 없이 `[ Groups ]`로 시작해 로드 후 숫자 채움.

### MINOR-4. SCR-020 `m` vs Nice-to-Have 북마크 `m` 충돌 주석 재확인
- **위치:** §11.5 "북마크 `m` → v0.2. MVP에서 `m`은 members 탭 점프로 선점". §12.1 표도 동일.
- **권고:** v0.2 북마크 도입 시 충돌 해결안(예: `B` 또는 `:bookmark`)을 §12.3 Reserved 섹션에 선예약 기록. "v0.2에서 북마크는 `B` 또는 `:bookmark <name>` 권장" 한 줄.

### MINOR-5. SCR-042 Policy Detail — system=true 배지 "SYS" 일관성
- **위치:** line 1094 "system SYS (default — cannot be deactivated)"
- **권고:** 다른 화면(SCR-041 Policies List)에도 `SYS` 배지 렌더링 규약 명시. 현재 SCR-041 섹션에서는 system 배지 표현이 생략된 상태. PRD REQ-R04 AC-3와 일관되게 리스트에서도 배지 노출.

### MINOR-6. SCR-050 Adaptive `[ADAPTIVE: yes]` 색상 — 사용자 관점 교란 가능성
- **위치:** line 1227 "적응 on (X-Rate-Limit-Limit < 60 감지 시): `[TAIL 15s] [ADAPTIVE: yes]` (yellow)"
- **문제:** 노란색은 `경고`로 관습 사용. Adaptive polling은 정상 동작이며 경고가 아님. 사용자에게 "뭔가 잘못됐다" 오해 유발 가능.
- **권고:** 색상을 **시안/cyan(info)** 또는 일반 텍스트로 변경. 배지 자체는 유지하되 "adaptive = 저한도 tenant에서 자동 최적화" 메시지로 긍정 프레이밍.

### MINOR-7. `:healthcheck` 결과 모달 vs 토스트 (§13.1 결정 대기)
- **위치:** §13.1에서 PM 판단 요청 "모달 vs 토스트, 현재 모달".
- **PM 결정:** **모달 유지** 권고. `:healthcheck`는 여러 카테고리(연결성 / rate limit / 토큰 / 프로필) 결과를 종합 보여줘야 하므로 3초 토스트로는 부족. SCR-905 About과 같은 모달 형식이 맞음. 이 결정을 §13.1에서 "확정"으로 승격.

---

## 4. NIT — 선택 반영

### NIT-1. §13.1 `r` 키 이중 의미 — PM 결정
- **현재:** "raw JSON toggle" 우선. 최근 목록은 v0.2 `:recent`로 이관.
- **PM 결정:** **확정 OK.** raw JSON은 PRD REQ-R04 AC-6의 명시된 MVP 기능이므로 우선권 명확. 최근 목록은 Nice-to-Have.

### NIT-2. §13.1 Wide 모드(180+) 사이드 패널 — PM 결정
- **현재:** v0.2 유지 (MVP는 단일 패널).
- **PM 결정:** **v0.2 유지.** PRD §4.1에는 Wide 모드 미포함. 140~179에서 컬럼 추가만 MVP (Users의 `department` 등 이미 문서화됨).

### NIT-3. §13.1 DEPROVISIONED 기본 포함/제외 — PM 결정
- **현재:** 포함 (REQ-R01 AC-7 근거).
- **PM 결정:** **포함 유지 (confirmed).** PRD AC-7 "Deactivated(DEPROVISIONED) users ARE included unless filtered out"에 맞음. 제외 원하면 `:search status ne "DEPROVISIONED"`. Help에 명시되어 있음.

---

## 5. 도메인 전문가 검수가 필요한 부분 (참고)

디자이너 §13.2 요청 항목은 **okta-expert가 별도 검수**해야 할 항목이므로 본 PM 리뷰 범위 밖이다. 다만 PM 관점에서 2건 추가 관찰:
- 도메인 §9.8 "Preview vs Production 차이"가 SCR-000 프로필 env 배지(prod/test/dev) 구분으로 잘 반영됨.
- `actor.type = SystemPrincipal` 아이콘(`⚙`)은 도메인 §1.7 REQ-R05 AC-8에 일관.

---

## 6. PM 판단 반영 (디자이너 §13.1 요청에 대한 답)

| # | 항목 | 현재 설계 | PM 결정 |
|---|------|-----------|---------|
| 1 | `r` 키 이중 의미 | raw 우선 | ✅ **유지** (NIT-1) |
| 2 | Wide 모드(180+) 사이드 패널 | v0.2 | ✅ **v0.2 유지** (NIT-2) |
| 3 | 타임존 토글 UI | `:set tz=local` 커맨드만 | ✅ **유지** (상태바 클릭은 MVP 기본 철학과 어긋남) |
| 4 | `:healthcheck` 출력 | 모달 | ✅ **모달 유지** (MINOR-7) |
| 5 | 색상 테마 기본값 | 다크 + k9s 유사 | ✅ **확정** (PRD §11.3 D-2) |
| 6 | 모달 오버레이 구현 | 개발자 위임 | ✅ **Phase 4 개발자 판단** (PM/디자인 범위 밖) |
| 7 | DEPROVISIONED 기본 포함 | 포함 | ✅ **포함 유지** (NIT-3) |

---

## 7. PM 수정 요청 우선순위

**Should-fix (v2에서 반영 권고, `docs/TUI_DESIGN.md` 확정 전):**
- MAJOR-1 탭 네이밍 "Logs" → "Recent"
- MAJOR-2 Policy 타입 카탈로그 외부화 규약 추가 (AC-8 대응)
- MAJOR-3 Users `search` eventually consistent Help+Empty 힌트 노출
- MAJOR-4 Group Rule Deactivations 프리셋 색상 강조

**Nice-to-have (v2 또는 Phase 4 구현 시):**
- MINOR 1~7 (특히 6 adaptive 색상, 5 SCR-041 SYS 배지)

---

## 8. 결론

TUI_DESIGN v0.1.0-draft는 PRD v1.0.0의 모든 REQ를 체계적으로 흡수했고, 디자인 원칙(§0)이 리더 결정·도메인 함정까지 녹여내어 품질이 매우 높다. 1,995줄 중에 애매한 부분은 MAJOR 4건으로 한정됨.

**승인: APPROVE WITH MINOR CHANGES.** MAJOR 4건 반영 후 `docs/TUI_DESIGN.md`로 확정 가능. 사이클 1회로 종결 예상.

— pm, 2026-04-24
