# 00. 초기 입력 (User Request)

**날짜:** 2026-04-24
**프로젝트명:** ota (Okta TUI)
**위치:** `/Users/austin/workspace/tedilabs/ota`

## 컨셉
- k9s(쿠버네티스 TUI)와 동등한 수준의 UX를 가진 Okta 관리 TUI
- 언어/프레임워크: Golang + Bubbletea (Charm 생태계)

## 초기 타겟 기능 (MVP Scope)
| # | 리소스 | 비고 |
|---|-------|------|
| 1 | Users | 리스트/상세/검색 |
| 2 | Groups | 리스트/상세/멤버 조회 |
| 3 | Group Rules | 리스트/상세 |
| 4 | Policies | 타입별 (Authentication, Password, MFA Enrollment, Sign-On, Global Session 등) |
| 5 | System Logs | 스트림/필터 |

## 핵심 기능 요구사항
1. **k9s 컨셉**: 리소스 리스트 → 상세 전환, 모달, 네임스페이스/필터, 풍부한 단축키, 커맨드 프롬프트(`:`)
2. **Configuration File 지원**: k9s `~/.config/k9s` 스타일. 계정(tenant) 프로필, UI 옵션, 단축키 매핑
3. **Okta API Token 인증**: 환경변수(`OKTA_API_TOKEN`, `OKTA_ORG_URL`) + 설정 파일 둘 다 지원
4. **Vim 친화적 단축키**: `hjkl` 이동, `:` 커맨드, `/` 검색. 설정 파일로 커스터마이징

## 도메인 전문가
- okta-expert (플러그인 1호)

## 산출물 관리
- `docs/*.md`: 확정된 문서
- `_workspace/*.md`: 중간 산출물(보존)

## 오픈 이슈 (Phase 2에서 해결)
- Okta API Token 테스트 환경 가이드 필요
- Rate Limit 정책 (tenant별 상이)
- SCIM vs Core API 선택 기준
- MFA 팩터 조회/관리 범위 (초기 범위 포함 여부)
