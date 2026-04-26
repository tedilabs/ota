# testdata/scenarios/

복합 시나리오 묶음. 한 시나리오가 여러 요청·응답 시퀀스를 포함할 수 있다.

## 구조
```
scenarios/
├── rate_limit_429_recovery.json        # 429 → Retry-After → 재시도 성공
├── rate_limit_429_exhausted.json       # 3회 모두 429 → 최종 실패
├── pagination_multi_page.json          # 3페이지 Link 헤더 체인
├── logs_tail_hole_free.json            # 429 중 since 유지 → 복구 후 구멍 없이 재개
├── users_eventually_consistent.json    # 방금 생성한 사용자 search 반영 지연
```

각 시나리오 JSON 스키마:
```json
{
  "name": "...",
  "req_id_coverage": ["REQ-E01 AC-2", ...],
  "steps": [
    {
      "request": {"method": "GET", "path": "/api/v1/users", "query": "limit=200"},
      "response": {
        "status": 200,
        "headers": {"Link": "...", "X-Rate-Limit-Remaining": "598"},
        "body_fixture": "oktaapi/users/list_page1.json"
      }
    },
    ...
  ]
}
```

`body_fixture`는 `testdata/` 루트 기준 경로.
