package home

// logsSampleSize is the per-fetch cap for log walks. Okta returns
// up to 1000 events per page on /api/v1/logs; bounding the
// Activity card to one page keeps the logs category rate-limit
// budget intact at the cost of an "≈" caveat on busy tenants.
//
// The dashboard's headline metrics are derived entirely from the
// logs category (Option A — 2026-06) so this constant is the only
// remaining sample bound after the per-resource count cards were
// removed.
const logsSampleSize = 1000
