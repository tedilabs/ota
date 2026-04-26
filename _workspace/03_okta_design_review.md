# 03. Okta 도메인 안전성 관점 TUI Design 리뷰 — v0.1.0-draft

**리뷰어:** okta-expert
**날짜:** 2026-04-24
**대상:** `/Users/austin/workspace/tedilabs/ota/_workspace/03_tui_design_draft.md` (v0.1.0-draft, 1995 lines)
**기준 문서:**
- `docs/PRD.md` (v1.0.0, 특히 §6.2 PII, REQ-R01 AC-6, REQ-R03 AC-2/5, REQ-R05 AC-2)
- `_workspace/02_okta_domain_input.md` (도메인 §1~§12)
**리뷰 범위:** **도메인 안전성만.** UX 일반 품질·PRD 충족도는 PM 영역이므로 중복 검수 안 함. 도메인 제약 위반과 도메인 관례 이탈로 나누어 표기.

## 총평

**결론: APPROVE WITH MINOR CHANGES.** 도메인 안전성 관점에서 CRITICAL 차단 사유 0건. 특히 **SUSPENDED/DEPROVISIONED 듀얼 채널 아이콘(`✗` yellow vs `⊘` gray) + 비교표**, **PII 자동 재마스킹(60초 inactivity / 화면 전환 / 세션 종료)**, **`[M!]` unmask 경고 배지**는 도메인 권고를 초과하는 우수한 디자인. INVALID 규칙 카운터 배너, Everyone 전용 라벨도 정확. 다만 MAJOR 3건 (PII 범위 확장, Everyone 외 BUILT_IN 판정 기준, 에러 매핑 누락 2개), MINOR 5건 반영 권고.

| 수준 | 건수 |
|------|------|
| CRITICAL | 0 |
| MAJOR (도메인 제약 위반에 근접) | 3 |
| MINOR (도메인 관례 이탈 또는 보완) | 5 |

---

## 1. 검수 요청 5 포인트 직답

### Q1. PII 마스킹 범위 타당성 — `actor.alternateId` 제외 결정
**[유보 → 조건부 권장]**

**현재 설계 (§7.1):** `profile.mobilePhone`, `profile.secondEmail`, factor `phoneNumber`/`email` 마스킹. Logs의 `actor.alternateId`(= login email)는 **마스킹 제외**, 사유 "로그 가독성 vs 보안" + "PRD에서 명시 안 됨".

**도메인 관점 판단:**
- **로그 가독성 우선은 합리적.** `actor.alternateId`는 감사·인시던트 조사의 핵심 필드. 마스킹하면 "누가 이 동작을 했는가"를 즉시 판별 불가 → TUI 가치 훼손. Sam(보안 감사자) 페르소나의 UC-2에서 `alice@acme.com`이 보이지 않으면 필터 조합이 성립 안 함.
- **PRD § 6.2는 User profile 필드를 마스킹 대상으로 명시**했지, Logs actor를 마스킹하라고 하지 않음. 설계가 PRD 준수.
- **그러나 완전 누락은 아님.** 아래 MAJOR M1 참고 — 지역 규제·조직 정책에 따라 Logs PII 마스킹을 설정으로 켤 수 있는 옵션은 필요.

**추가 필드 검토:**
- **`client.ipAddress`**: 마스킹 제외 정당. 감사에 필수. Okta 자체가 로그에 그대로 저장.
- **`geographicalContext` (city/state)**: 마스킹 제외 정당. 같은 이유.
- **`actor.id` (00u...)**: 마스킹 제외 정당. 이건 이미 scoped 불투명 토큰성 식별자라 직접 PII 아님. `→ U` 점프에도 필요.
- **`securityContext.asNumber` / `isp`**: 제외 정당.
- **`debugContext.debugData.*`**: 여기에 종종 인증 컨텍스트 JSON이 들어가는데 `email_address`, `phone_number` 같은 필드가 그대로 포함될 수 있음 → **추가 확인 필요 [확인필요]**. MINOR m1 참조.

**권고 (MAJOR M1):** 설정 옵션 추가. `ui.pii_masking.logs_actor_email: false` (기본). 규제/엄격 조직이 `true`로 설정하면 Logs `actor.alternateId`도 `a***@acme.com`으로 마스킹. MVP에서 꼭 구현할 필요 없고, §7.3 yaml 스키마에 toggle 키 **예약**만 해두면 향후 추가 용이.

### Q2. Group Rule Deactivate 경고 문구 (SCR-031) — 함정 충분 전달?

**[반대 — 강화 필요]** (MAJOR M2)

**현재 문구:**
> "ⓘ Deactivating this rule would remove all memberships it created. This action is disabled in read-only mode (MVP)."

**도메인 함정 전달 부족 부분:**
1. **"all memberships it created"의 의미가 모호.** 운영자가 "이 규칙이 지금 몇 명에게 멤버십을 주고 있는지"를 모르면 위험 감각이 안 생김.
2. **가역성 오해 위험.** 비활성화가 "토글"처럼 느껴져서 "다시 활성화하면 원래대로" 라고 가정할 수 있음. 실제로는 재활성화 후 재평가까지 시간차 + 그 사이에 정책/앱 할당이 멤버십 기반이면 접근 손실. 이 시간차가 수 분 단위.
3. **단일 규칙 의존 vs 중복 규칙 차이 안내 없음.** 한 사용자가 여러 규칙으로 같은 그룹에 속해있으면 하나 비활성화해도 유지되지만, 단일 규칙 의존이면 손실.
4. **ⓘ (info) 아이콘**은 도메인 리스크 대비 너무 경량. 이 함정은 경고 레벨.

**권고 재작성안 (v0.2 Write 설계 시 적용, MVP 읽기 모드 배너도 동일 문구로 선반영):**
```
│   ⚠ Deactivating this rule removes group memberships it granted.           │
│                                                                            │
│     • Rule-based members of the target group(s) will lose membership.      │
│     • Users with another rule producing the same membership are unaffected.│
│     • Re-activation is NOT instant: Okta re-evaluates (may take minutes).  │
│     • Downstream policies / app assignments depending on group membership  │
│       will also change immediately. Verify access impact first.            │
│                                                                            │
│     This action is disabled in read-only mode (MVP).                       │
```

**아이콘은 `ⓘ` → `⚠` (yellow).** L2 경고 레벨. v0.2 Write 시 동일 배너 유지 + 이중 확인(L3) 추가.

### Q3. 대용량 그룹 배너 — Everyone 외 BUILT_IN에 수만명 가능?

**[유보 — 판정 기준 명확화 권고]** (MAJOR M3)

**현재 설계:**
- `BUILT_IN` → `◈` + `SYS` 배지
- `Everyone` 전용 "all organization members" 라벨
- `LARGE` 배지는 "예상 멤버 > 10k"
- Everyone 선택 시 "tens of thousands" 경고 배너

**도메인 사실 [확정 + 확인필요]:**
- `BUILT_IN` 타입의 시스템 그룹은 사실상 **`Everyone` 하나가 표준**입니다. 다른 BUILT_IN이 있을 수 있지만 일반적이지 않음.
- 제가 종종 관찰한 잠재 BUILT_IN: `Okta Administrators` (일부 조직), 내부용 시스템 그룹 (tenant별 상이). 이들은 **수십~수백 명 규모**지 수만 명 아님.
- 수만 명 그룹은 BUILT_IN이 아닌 **OKTA_GROUP** 또는 **APP_GROUP**에서 많이 나옴. 예: "All Employees" 같은 수동/동적 그룹, AD 동기화 그룹 (전사 사용자).

**현재 설계의 실질 위험:**
- "OKTA_GROUP/APP_GROUP인데 수만 명" 그룹에서 멤버 로드 시 배너·progressive loading이 트리거되는가? 현재 설계는 **BUILT_IN 조건으로만 배너**. `LARGE` 배지는 "예상 > 10k"라고 했지만 "예상 수"를 어디서 얻는지 명시 없음 (Okta는 그룹 멤버 수 count API가 없음).
- 결과: **"All Employees" 같은 큰 OKTA_GROUP 진입 시 배너 없이 progressive load만 일어남** — 사용자 인지 지연.

**권고 (MAJOR M3):**

SCR-020 AC-3 / SCR-021 Members 탭에 다음 추가:

```
판정 기준:
1. type == "BUILT_IN" → 항상 배너 (현재 OK)
2. profile.name == "Everyone" → 추가 "all organization members" 라벨 (현재 OK)
3. type in ("OKTA_GROUP", "APP_GROUP") 이더라도 멤버 로딩 중 200명 초과 감지 시
   → progressive 배너를 "Large group — may contain thousands" 경고 배너로 업그레이드
   → 이 시점에 배지에 LARGE 추가 (런타임 계산)
```

`LARGE` 배지 설명을 §SCR-020 "배지" 테이블에 명확히 — "예상 멤버 > 10k" 대신 "로딩 중 200명 초과 관찰 시 자동 부착, 보수적 감지".

### Q4. Policy 타입 `(raw view)` 배지 — 사용자 인지 충분?

**[권장]** (설계대로 유지)

**현재 설계 우수 포인트:**
- 모달에 `(raw view)` 배지 + 하단 `ⓘ` 설명 ("show JSON only, no rich render")
- 바닥 상태바에 "Rendering 4 of 7 types fully; 3 types as raw JSON (see PRD)"
- 상세 화면에서도 "Rich view not yet available for PROFILE_ENROLLMENT. Press `r` for JSON pretty-print" 명시

이 트리플 안내는 도메인 현실(3 타입 스키마 차이가 커서 풀 렌더가 어렵다는 점)을 사용자에게 충분히 전달. Dana/Sam/Kim 페르소나 모두 JSON이 익숙하므로 부담 없음.

**MINOR m2 보완:**
- raw 타입이더라도 **공통 필드(name/priority/status/system/lastUpdated)는 rich로 표시**하는 절충이 현재 SCR-042 "raw-only 타입 상세"에 반영됨 ("Basic fields" 섹션). 좋음. 단 이 부분을 §SCR-040 모달 하단 안내에도 언급하면 사용자 기대 관리 더 정확: "Basic fields shown; full conditions/actions require raw JSON mode."

**ENTITY_RISK / CONTINUOUS_ACCESS 확장 경로:**
- REQ-R04 AC-8 설정 카탈로그로 설계한 것 확인. 도메인 변경에 강건. 훌륭.

### Q5. DEPROVISIONED vs DELETED 혼동 방지 (SCR-010)

**[권장]** (설계대로 유지, 작은 개선 권고)

**현재 설계 우수 포인트:**
- 아이콘 듀얼 채널: SUSPENDED=`✗`/yellow, DEPROVISIONED=`⊘`/gray. 기호와 색 모두 다름.
- Help에 1:1 비교표 포함 예정 (§SCR-010 Line 485 "Help에 1:1 비교 표 포함")
- DELETED는 API 응답 제외이므로 리스트에 안 나타남을 명시 (§SCR-010 Line 697 "User not found or deleted. <R> refresh list")

**도메인 관점 확인:**
- ⊘ (U+2298) 기호는 **"금지/비어 있음"** 의미로 DEPROVISIONED(되돌릴 수 있는 비활성)보다 DELETED(되돌릴 수 없는) 뉘앙스. 그러나 DELETED는 화면에 안 나타나므로 실사용 혼동 낮음. 유지 OK.
- LOCKED_OUT(`⚠`/red)와 DEPROVISIONED(`⊘`/gray) 색 구분 뚜렷 — 혼동 낮음.

**MINOR m3 — Help 비교표에 행동 차이까지 명시:**

권고 Help 비교표 내용:
| 상태 | 기호 | 로그인 | 데이터 보존 | 되돌림 | 비고 |
|---|---|---|---|---|---|
| ACTIVE | ● / green | 가능 | 유지 | - | 정상 |
| SUSPENDED | ✗ / yellow | 불가 | 유지 | **unsuspend로 즉시 복귀** | 임시 차단 |
| DEPROVISIONED | ⊘ / gray | 불가 | 유지 | **reactivate 가능하나 토큰·세션 재발급 필요** | 비활성화 |
| DELETED | - | 불가 | **삭제됨** | **불가** | 영구 삭제 (리스트에 안 나타남) |
| LOCKED_OUT | ⚠ / red | 불가 | 유지 | unlock으로 즉시 복귀 | 반복 로그인 실패 자동 |

- 특히 "DEPROVISIONED는 reactivate 해도 토큰/세션 재발급 필요"가 도메인 중요 포인트 — Help에 포함하면 v0.2 Write에서 운영자 혼동 예방.
- DELETED 행 "리스트에 안 나타남" 명시는 현재 §SCR-010 AC-7에 있지만 Help에서 한눈에 보이게.

---

## 2. 추가 점검 항목

### Factors 탭 — factor type별 필드 매핑 (REQ-R01 AC-6)

**[권장]** SCR-011 Factors 탭 와이어프레임 검증 결과:

**Okta Verify (Push) 예시 (line 623~628):**
- ✅ `factorType`=Okta Verify (Push), `provider`=OKTA/OKTA
- ✅ `profile.deviceType`=iPhone 14 Pro, `profile.name`=Alice's iPhone — 두 필드 모두 Okta Verify의 판별에 필수. 정확.
- ✅ created/lastUpdated 상대 시간, id (e) expand로 복사용
- ⚠️ `status` ACTIVE 명시 OK, 색상은 green으로 추정 (명시 없음)

**SMS 예시 (line 630~634):**
- ✅ `factorType`=SMS, `status`=EXPIRED (경고색 적용 확인 필요 — line 905 ⚠ 기호 있음)
- ✅ `phoneNumber`=`+1-***-***-1234` + `:unmask phoneNumber` 힌트 우수
- ✅ created/lastUpdated
- ⚠️ `vendorName` 필드 표시 없음. 이 예시는 provider=OKTA/OKTA라 동일해서 생략된 듯하지만, **DUO / Google Authenticator 등 3rd party는 vendorName 다르므로 표시 권고** — MINOR m4 참조.

**MVP 포함 권고 추가 factor 타입 예시 (m4):**
- WebAuthn: `credentialId` (키 별칭) 표시 — 설계안에 언급만 되고 와이어프레임 예시 없음. 실사용자는 여러 security key 구분 필요.
- TOTP (token:software:totp): 별도 `profile` 필드 거의 없음 (seed는 노출 안 됨). 최소 `factorType` + `status` + created만으로 충분.
- "No WebAuthn / TOTP / Email / Security Question factors." 문구(Line 636)는 우수 — 미등록 factor도 명시적으로.

### Rate-limit `[ADAPTIVE 15s]` 발동 조건 — 도메인 현실성

**[권장]** (MAJOR M4 경량 조정)

**현재 설계 (REQ-R05 AC-2):**
- 기본 7초
- `X-Rate-Limit-Limit for /logs < 60` 감지 시 → 15초 자동 상향
- Paused 표기: `[TAIL ⏸] · resuming in 8s`

**도메인 현실성 분석:**
- **Enterprise tenant**: `/logs` 한도 분당 ~120. `Limit < 60` 감지는 Developer Free tenant나 제약된 trial에서만 발생 예상. **발동 조건 적절.**
- **Developer Free tenant**: 정확한 `/logs` 한도는 [확인필요]지만 경험상 60 이하. 자동 adaptive가 올바르게 작동.
- **경계 조건 엣지 케이스**: 첫 호출 이전에는 `Limit` 값을 모름 → **첫 tail 호출 후 응답 헤더를 읽고 판단** 필요. 이 동작 흐름이 AC-2에 "첫 호출" 표현 있지만, UI 표시 타이밍 명시 권고:

**MAJOR M4 보강 (§SCR-050 AC-2):**
```
첫 폴링 응답 수신 직후 X-Rate-Limit-Limit 관찰. 60 이하면 즉시 인터벌을
15초로 상향, 상태바 `[ADAPTIVE: no → yes]` 전환 토스트 (1회, 2초).
첫 호출 이전에는 기본 7초로 진행 (안전 마진이 넉넉한 방향).
```

**추가 제안 (MINOR m5):**
- `[TAIL 7s] [ADAPTIVE: no]` 대신 **`[TAIL 7s]` 하나만 기본** 표시하고, adaptive 발동됐을 때만 `[TAIL 15s · ADAPTIVE]`로 바꾸는 쪽이 시각 노이즈 낮음. 현재는 `[ADAPTIVE: no]`가 매번 보여서 정보 밀도 과다.

### 에러 매핑 팀-리드 지시 vs 설계 현황 (E0000054 / E0000068)

**[반대 — 누락 2개 확인]** (MAJOR M5)

**Team-lead가 명시한 8종:** E0000001, E0000006, E0000011, E0000022, E0000038, E0000047, **E0000054**, **E0000068**.

**현재 설계 (SCR-001 boot + SCR-904 session errors):**
- E0000001, E0000004, E0000006, E0000007, E0000011, E0000022, E0000038, E0000047 — 8종. 하지만 **E0000054와 E0000068이 없고, E0000004와 E0000007이 대신 들어있음.**

**도메인 사실:**
- **E0000004** (Authentication failed): PRD §7.7에 포함. 유지 합리적.
- **E0000007** (Not found): PRD §7.7에 포함. 유지 합리적.
- **E0000054** (Invalid attribute value): PRD §7.7에 **없음**. 제 Q&A 답변(Q5 답변)에서 "이미 E0000001로 묶여 들어와서 별도 엔트리 불필요"로 분류했었음. team-lead 지시에 추가됐다면 PRD 업데이트 필요 사항일 가능성.
- **E0000068** (Invalid Passcode/Answer): 보통 인증 factor 검증 실패. MVP read-only에서는 거의 발생 안 함. v0.2 Write에서만 관련.

**판단:**
- PRD §7.7 에러 매핑은 현재 8종 (E0000001/04/06/07/11/22/38/47). 설계도 이에 따름.
- team-lead 지시의 E0000054/E0000068이 "설계에 추가되어야 할 것"이면 PRD를 먼저 확장해야 하고, "감안해서 점검하라"면 현재 설계는 PRD와 일치이므로 OK.

**권고 (MAJOR M5):**
- team-lead의 8종 목록이 PRD §7.7과 불일치. 이는 설계자 잘못이 아닌 **지시 간 일관성 문제**.
- 설계는 PRD와 일치하므로 현 상태 유지.
- 결정 요청: (a) PRD §7.7에 E0000054, E0000068 추가 후 설계도 확장, 또는 (b) team-lead 지시의 8종이 예시 목록이었음을 확인하고 현 8종 유지.

### Logs 화면 PII 자동 마스킹 시각화

**[권장]** (M1 반영 권고와 연동)

- 현재 설계 §7.4는 **actor.alternateId 마스킹 제외 + Debug log는 마스킹 적용** 명시. 합리적.
- `client.ipAddress`, `geo`, IP chain 등은 마스킹 없이 평문. 감사 필수이므로 맞음.
- Q1의 M1 권고(Logs actor email 마스킹 설정 옵션)는 시각화에도 동일 규칙 적용 — toggle on 시 `actor.alternateId` 위치에 `a***@acme.com` 렌더링.

---

## 3. MAJOR (도메인 제약 위반에 근접 — 반영 권고)

### M1. Logs PII 마스킹 설정 옵션 예약
**위치:** §7 PII 마스킹, §7.3 yaml 스키마
**내용:** `ui.pii_masking.logs_actor_email: false` 키를 §7.3 yaml 예시에 추가 (기본 false). MVP에 구현 의무 없음. 규제/엄격 조직 대비 **예약만**. 아울러 `debugContext.debugData.*` 내 PII 필드는 [확인필요] — Phase 4 실전 로그 관찰 후 필요 시 추가 마스킹.

### M2. Group Rule Deactivate 배너 강화
**위치:** §SCR-031 Line 938~939
**내용:** 현재 `ⓘ` info 아이콘 + 1줄 경고는 함정 강도에 부족. `⚠` warning + 5포인트 불릿 재작성안(§1.Q2 참고). 아이콘 색상 yellow/red. v0.2 Write 대비 패턴 예약도 동일 문구로.

### M3. Large Group 판정을 type 넘어 런타임 관찰로 확장
**위치:** §SCR-020 배지/상태별, §SCR-021 Members 탭
**내용:** `type=BUILT_IN` 단일 조건 대신 **멤버 로딩 중 200명 초과 관찰 시 런타임 `LARGE` 배지 자동 부착**. OKTA_GROUP/APP_GROUP의 대형 그룹 대응 공백 해결. 배너 문구도 "system-wide" 대신 조건별로 분기 (BUILT_IN → "system-wide", 그 외 LARGE → "Large group — may contain thousands").

### M4. Adaptive polling 발동 타이밍·토스트 명시
**위치:** §SCR-050 Adaptive polling 인디케이터
**내용:** 첫 폴링 응답 수신 직후 `X-Rate-Limit-Limit` 관찰해 판정, 전환 시 1회 2초 토스트. 첫 호출 이전 기본 7초 진행. AC-2 문구 보강.

### M5. 에러 매핑 8종 정합성 확인
**위치:** PRD §7.7 및 SCR-001/SCR-904
**내용:** team-lead 지시의 E0000054/E0000068이 PRD §7.7과 불일치. 설계는 PRD 따름. PRD 업데이트 필요 여부를 PM/team-lead에 확인. 설계자 과실 아님. **결정 에스컬레이션 사항.**

---

## 4. MINOR (도메인 관례 이탈 또는 보완)

### m1. `debugContext.debugData` 내 PII 추가 마스킹 가능성 확인
**위치:** §7.4 Logs 마스킹
**내용:** Okta Log의 `debugContext.debugData`는 free-form JSON으로 인증 시도 시 email/phone이 들어갈 수 있음. SCR-051 Structured 뷰 렌더링 시 해당 값이 평문 노출될 가능성. Phase 4 실전 로그 관찰 후 마스킹 룰 확장 필요 여부 재검토.

### m2. Raw-only 타입 상세의 "Basic fields" 안내를 모달에도 노출
**위치:** §SCR-040 모달 하단
**내용:** "Rendering 4 of 7 types fully; 3 types as raw JSON" 다음 줄에 "Basic fields (name/priority/status/system/lastUpdated) still shown; full conditions/actions require raw JSON mode." 추가.

### m3. User 상태 Help 비교표에 "행동 차이" 포함
**위치:** Help 상태 아이콘 범례
**내용:** §1.Q5의 비교표처럼 로그인 가능 여부, 데이터 보존, 되돌림 조건을 행별로 명시. 특히 "DEPROVISIONED reactivate 후에도 토큰/세션 재발급 필요" 추가.

### m4. Factors 탭 vendorName 표시 시연 추가
**위치:** §SCR-011 Factors 탭 와이어프레임
**내용:** 현재 예시는 provider=OKTA/OKTA라 vendorName 차이가 안 보임. DUO 또는 Google Authenticator 예시를 하나 추가해 vendorName 표시를 시연:
```
│   ● Duo Mobile                                  ACTIVE    registered 90d   │
│     provider         OKTA / DUO     <- vendorName highlighted              │
│     factorType       Push                                                  │
```

### m5. Adaptive 인디케이터 기본 표시 간결화
**위치:** §SCR-050 Tail 인디케이터
**내용:** 기본 상태에서 `[ADAPTIVE: no]` 표시 생략. Adaptive 발동 시에만 `[TAIL 15s · ADAPTIVE]`로 강조. 기본 상태는 `[TAIL 7s]`만.

---

## 5. 우수 포인트 (보존·확산 권고)

도메인 관점에서 특별히 칭찬할 디자인 결정들:

1. **SUSPENDED/DEPROVISIONED 듀얼 채널 아이콘 + 비교표** — 도메인에서 가장 흔한 운영자 혼동 원인을 기호·색·Help 3중으로 해결. 권고 수준 초과.
2. **`[M!]` unmask 경고 배지** — "어깨너머 인지 도우미" 표현 우수. 도메인 §9.1 token rotation 및 PII 운영 관례에 부합.
3. **PII 자동 재마스킹 60초 inactivity** — PRD 권고를 넘어선 세션 위생. 보안 팀 관점에서 모범.
4. **INVALID 카운터 배너 (§SCR-030)** — 리스트 하단에 "1 rule in INVALID state" 카운터는 도메인 §1.4 함정을 잘 전달.
5. **Everyone 전용 라벨 + 로딩 중단 허용** (`<Esc> to stop`) — 대용량 그룹 UX 모범. k9s의 리소스 중단과 동형.
6. **Tail 복구 시 "no data loss on resume" 명시** — 도메인 §1.7 폴링 알고리즘의 핵심 불변식을 사용자에게 가시화. 신뢰도 상승.
7. **Log 상세 `<U> open user` / `<T> open target`** — 도메인 §1.7의 "actor/target id 점프" 권고 정확 구현.

---

## 6. 역할 경계 (명시)

**이 리뷰는 도메인 안전성에 한정.** 다음 항목은 의도적으로 검토하지 않음 (PM/tui-designer 영역):
- UX 일반 품질 (색상 조합, 레이아웃 미학, 인지 부하)
- PRD REQ 충족도 (PM의 검수 대상)
- Bubbletea 컴포넌트 선택 적절성 (go-tui-developer Phase 4 검수)
- 터미널 호환성 (go-test-engineer Phase 5)

---

## 7. 결론

**APPROVE WITH MINOR CHANGES.** Must-fix 수준: M2 (Group Rule 배너 강화), M3 (Large group 런타임 감지), M5 (에러 매핑 정합성 — 에스컬레이션). M1/M4는 v0.2 대비 예약으로 충분. Minor 5건은 반영 시 품질 상승.

**Phase 3 이관 전 권고:**
1. M2 강화 반영 (도메인 함정 전달력 문제)
2. M3 런타임 감지 반영 (실사용 시나리오 누락 방지)
3. M5 에스컬레이션 (PM/team-lead 확인 후 PRD/설계 일관성 복구)

**Cycle 1 / 2 중 1회 사용.** v2 초안 수령 시 남은 1회로 regression 검토 준비.

— okta-expert, 2026-04-24

---

# Regression Addendum — v1.0.0 (2026-04-24 Cycle 2)

**대상:** `/Users/austin/workspace/tedilabs/ota/docs/TUI_DESIGN.md` (v1.0.0, 2193 lines)
**리뷰 범위:** tui-designer 보고한 변경 지점의 회귀 및 도메인 함정 재도입 여부만. 미변경 섹션 재검토 없음.

## PASS — 회귀 없음

| 항목 | 결과 | 증거 (라인) |
|------|------|-----|
| **M1 logs PII 설정 키 예약** | ✅ PASS | §7.3 line 1928 — `logs_actor_email: false` 주석 "(reserved)" + "규제/엄격 조직용. MVP는 기본 false; 구현은 v0.2에 확정" 완비. §7.4 line 1937~1938 기본 동작과 토글 설명 분리. |
| **M2 Group Rule Deactivate 배너** | ✅ PASS | SCR-031 line 1022~1029 — `⚠` warning + 5포인트 불릿 모두 존재. "another rule producing the same membership" / "Re-activation is NOT instant" / "Downstream policies / app assignments" 핵심 문구 보존. styleWarning 토큰 지정(line 1053). |
| **M3 Large group 런타임 감지** | ✅ PASS | §SCR-020 line 811 "BUILT_IN 조건에 한정되지 않고 OKTA_GROUP/APP_GROUP에도 적용". Line 834 "200명 초과 관찰 시" 조건 명시. Line 858 "All Employees members 탭 · `Loading: 205 members so far… <LARGE detected>`" 시연. §5.1 `groupMemberCountObserved` tea.Cmd 배선. |
| **M4 Adaptive polling 타이밍** | ✅ PASS | §SCR-050 line 1330~1331 "기본(7초, 일반)/Adaptive 발동(15초, 저한도)". §8.2 line 1959 "1회성 토스트 'Adaptive polling enabled (15s)' 2초" 토스트 명시. 첫 폴링 응답 관찰 후 전환 흐름 명확. |
| **M5 에러 매핑** | ✅ PASS | §14 오픈 이슈에 "E0000054/E0000068 v0.2 재검토" 한 줄 기록 확인. 본문 에러 테이블은 PRD §7.7 8종 유지. 설계 변경 없음 — team-lead (A) 결정 정확 반영. |
| **m1 debugContext.debugData 플래그** | ✅ PASS | §7.4 line 1940 MVP 평문 + Phase 4 재검토 [확인필요] 태그 유지. SCR-051 line 1477에도 주석. |
| **m2 Raw-only Basic fields 모달 안내** | ✅ PASS | SCR-040 모달 본문 갱신 확인 (PRD §Q4 권고 반영). |
| **m3 Help 상태 비교표 행동 차이** | ✅ PASS | SCR-902 Status icons 탭 line 1590~1609 — 7 상태 × 5 행동 차이 열(Icon/Login/Data/Revert/Note). "DEPROVISIONED → reactivate requires fresh tokens/sessions" 명시. "DELETED excluded from default list responses" 명시. 권고 초과 수준. |
| **m4 Factors DUO 시연** | ✅ PASS | SCR-011 line 669~691 — `provider: OKTA / DUO` 명시 + "3rd party (vendorName)" 주석. 실사용 시연 성공. |
| **m5 Adaptive 인디케이터 간결화** | ✅ PASS | SCR-050 line 1306 기본 `[TAIL 7s] ▶` 단일. Line 1325 adaptive 시 `[TAIL 15s · ADAPTIVE] ▶`. Line 1330 "`ADAPTIVE: no` 같은 잡음 정보는 표시하지 않음" 명시. |

## 우수 포인트 보존 확인 (v0.1 우수 포인트 7건)

| 포인트 | 보존 여부 |
|------|-----|
| SUSPENDED/DEPROVISIONED 듀얼 채널 + 비교표 | ✅ SCR-902 line 1595~1597 |
| `[M!]` unmask 경고 배지 | ✅ §7.2 line 1912 styleBadgeUnmask |
| PII 자동 재마스킹 60초 inactivity | ✅ §7.2 line 1916 |
| INVALID 카운터 배너 | ✅ SCR-030 보존 |
| Everyone 라벨 + `<Esc> stop` | ✅ SCR-020/021 보존 |
| Tail 복구 "no data loss on resume" | ✅ SCR-050 보존 |
| Log `<U>/<T>` 점프 | ✅ SCR-051 보존 |

## 신규 우수 포인트 발견 (v1.0에서 추가)

- **`:healthcheck` 모달 Role check "Read-Only Administrator" 명시** (line 1747) — 도메인 §4 권장(Read-Only Admin으로 토큰 발급)을 런타임 검증으로 연결. 토큰이 예상보다 강한 권한이면 사용자에게 경고 가능해지는 기반.
- **Status icons 탭 Log Severity 섹션 통합** (line 1607~1608) — Help 한 곳에서 User 상태와 Log severity를 같이 참조할 수 있어 Sam 페르소나 UC-2에 유용.

## 신규 지적 사항

**없음.** MINOR 이상 신규 도메인 이슈 발견 안 됨.

## 결론

**PASS — Cycle 2 1회 통과.** 도메인 안전성 관점 모든 요구사항 충족. `docs/TUI_DESIGN.md` v1.0.0은 Phase 3 종료 조건을 만족하며 Phase 4(기술 문서 + 테스트) 이관 가능.

— okta-expert, 2026-04-24 (Cycle 2 종료)
