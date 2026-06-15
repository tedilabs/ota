package home

import (
	"context"
	"strings"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// PostureMetrics is the Risk & Governance card payload.
//
// Each field captures one signal an Okta admin would otherwise need
// to derive by clicking through three different list pages — the
// card collapses them into one scan-friendly column.
type PostureMetrics struct {
	SuperAdmins        int
	TotalAdmins        int
	ExpiringTokens7d   int
	TotalTokens        int
	InvalidGroupRules  int
	TotalGroupRules    int
	InactiveAuthenticators int
	TotalAuthenticators    int
	ObservedAt         time.Time
	Errs               []string
}

// countPosture fans out per-port fetches that the Risk & Governance
// card reads. Each sub-fetch is gated on the matching port being
// non-nil so a tenant that hasn't wired (e.g.) API tokens still
// renders the rest. Errors are collected into Errs rather than
// failed-fast — a partial card is more useful than no card.
func countPosture(ctx context.Context, deps Deps, now time.Time) PostureMetrics {
	out := PostureMetrics{ObservedAt: now}

	if deps.Administrators != nil {
		admins, err := deps.Administrators.List(ctx)
		if err != nil {
			out.Errs = append(out.Errs, "admins: "+truncate(err.Error(), 32))
		} else {
			out.TotalAdmins = len(admins)
			for _, a := range admins {
				if strings.EqualFold(a.RoleType, "SUPER_ADMIN") {
					out.SuperAdmins++
				}
			}
		}
	}

	if deps.APITokens != nil {
		tokens, err := deps.APITokens.List(ctx)
		if err != nil {
			out.Errs = append(out.Errs, "tokens: "+truncate(err.Error(), 32))
		} else {
			out.TotalTokens = len(tokens)
			cutoff := now.AddDate(0, 0, 7)
			for _, t := range tokens {
				if !t.ExpiresAt.IsZero() && t.ExpiresAt.Before(cutoff) {
					out.ExpiringTokens7d++
				}
			}
		}
	}

	if deps.GroupRules != nil {
		it, err := deps.GroupRules.List(ctx, domain.GroupRulesQuery{})
		if err != nil {
			out.Errs = append(out.Errs, "rules: "+truncate(err.Error(), 32))
		} else {
			for {
				r, hasMore, err := it.Next(ctx)
				if err != nil {
					out.Errs = append(out.Errs, "rules: "+truncate(err.Error(), 32))
					break
				}
				if !hasMore {
					break
				}
				out.TotalGroupRules++
				if string(r.Status) == "INVALID" {
					out.InvalidGroupRules++
				}
			}
			it.Close()
		}
	}

	if deps.Authenticators != nil {
		auths, err := deps.Authenticators.List(ctx)
		if err != nil {
			out.Errs = append(out.Errs, "auth: "+truncate(err.Error(), 32))
		} else {
			out.TotalAuthenticators = len(auths)
			for _, a := range auths {
				if string(a.Status) == "INACTIVE" {
					out.InactiveAuthenticators++
				}
			}
		}
	}

	return out
}
