package main

// `ota -check` runs a single probe against /api/v1/users?limit=1 and prints a
// plain-text diagnostic. Used by operators to identify why the TUI is
// reporting an error without having to interpret the chrome'd ErrorPanel.
//
// Output format (one block, stable):
//
//   ota -check (v0.1.0)
//   profile:    prod
//   org url:    https://acme.okta.com
//   token env:  OKTA_API_TOKEN  (***  16 chars)
//   request:    GET /api/v1/users?limit=1
//   ──────────────────────────────────────
//   result:     OK   (200, 312ms)
//   users seen: 1
//
// On failure the tail is replaced with status code, errorCode (if Okta-shaped),
// errorSummary, and the actionable hint from errormap.UserMessage.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/config"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/logger"
	"github.com/tedilabs/ota/internal/okta"
	"github.com/tedilabs/ota/internal/okta/errormap"
	"github.com/tedilabs/ota/internal/version"
)

// runCheck performs the diagnostic probe. Returns the process exit code: 0
// for healthy, 1 for any failure (network, auth, or unexpected response).
func runCheck(ctx context.Context, in WireInput, stdout, stderr io.Writer) int {
	header(stdout)

	// 1. Config / profile resolution — same path Wire uses, surfaced step by step.
	res, cfgErr := config.LoadWithWarnings(config.LoadOptions{ExplicitPath: in.ConfigPath})
	cfg := res.Config
	if cfgErr != nil && !errors.Is(cfgErr, errors.Unwrap(cfgErr)) {
		// Best-effort surface, then continue — env-only path may still resolve.
		fmt.Fprintln(stdout, "config:    ", scrub(cfgErr.Error()))
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(stdout, "warning:   ", w)
	}

	profileName, profile, err := pickProfile(cfg, in.Profile)
	if err != nil {
		failBlock(stdout, "profile resolve failed", err)
		return 1
	}
	fmt.Fprintln(stdout, "profile:   ", profileName)
	fmt.Fprintln(stdout, "org url:   ", profile.OrgURL)

	token, source, err := app.ResolveToken(app.ResolveTokenInput{
		CLITokenEnv: in.TokenEnv,
		Profile:     profile,
	})
	if err != nil {
		failBlock(stdout, "token resolve failed", err)
		return 1
	}
	tokenLen := len(token)
	fmt.Fprintf(stdout, "token env: %s  (***  %d chars)\n", source, tokenLen)

	// 2. Construct an Okta client + perform single-page probe.
	cli, err := okta.NewClient(ctx, okta.Config{
		OrgURL:   profile.OrgURL,
		APIToken: token,
	}, okta.WithClock(clock.Real()), okta.WithMaxRetries(0))
	if err != nil {
		failBlock(stdout, "okta client init failed", err)
		return 1
	}

	fmt.Fprintln(stdout, "request:   ", "GET /api/v1/users?limit=1")
	fmt.Fprintln(stdout, strings.Repeat("─", 40))

	start := time.Now()
	iter, listErr := cli.Users().List(ctx, domain.UsersQuery{Limit: 1})
	elapsed := time.Since(start)

	if listErr != nil {
		printError(stdout, listErr, elapsed)
		return 1
	}
	defer iter.Close()

	user, hasMore, nextErr := iter.Next(ctx)
	elapsed = time.Since(start)
	if nextErr != nil {
		printError(stdout, nextErr, elapsed)
		return 1
	}
	if !hasMore {
		fmt.Fprintf(stdout, "result:     OK   (200, %s)\n", elapsed.Round(time.Millisecond))
		fmt.Fprintln(stdout, "users seen: 0   (tenant has no users — probe succeeded)")
		return 0
	}
	fmt.Fprintf(stdout, "result:     OK   (200, %s)\n", elapsed.Round(time.Millisecond))
	fmt.Fprintln(stdout, "users seen: 1+ (first =", user.Profile.Login+")")
	return 0
}

func header(w io.Writer) {
	fmt.Fprintf(w, "ota -check (%s)\n", version.Tag)
}

// failBlock is used for failures that happen before the HTTP probe (config,
// profile, token). They short-circuit with a plain error and the same
// remediation hint the BootErrorModel uses.
func failBlock(w io.Writer, label string, err error) {
	fmt.Fprintln(w, strings.Repeat("─", 40))
	fmt.Fprintf(w, "result:     FAIL — %s\n", label)
	fmt.Fprintln(w, "error:     ", scrub(err.Error()))
	fmt.Fprintln(w, "hint:      ", "set OKTA_ORG_URL + OKTA_API_TOKEN, or use --config / --profile")
}

// printError unpacks a domain error into status / errorCode / errorSummary /
// actionable hint, so the operator sees enough context to fix it.
func printError(w io.Writer, err error, elapsed time.Duration) {
	fmt.Fprintf(w, "result:     FAIL  (%s)\n", elapsed.Round(time.Millisecond))

	var rl *domain.RateLimitedError
	var bad *domain.BadRequestError
	switch {
	case errors.Is(err, domain.ErrTokenInvalid):
		fmt.Fprintln(w, "status:    ", "401 Unauthorized")
	case errors.Is(err, domain.ErrForbidden):
		fmt.Fprintln(w, "status:    ", "403 Forbidden")
	case errors.Is(err, domain.ErrNotFound):
		fmt.Fprintln(w, "status:    ", "404 Not Found")
	case errors.As(err, &rl):
		fmt.Fprintln(w, "status:    ", "429 Too Many Requests")
		if rl.RetryAfter > 0 {
			fmt.Fprintln(w, "retry-after:", rl.RetryAfter)
		}
	case errors.As(err, &bad):
		fmt.Fprintln(w, "status:    ", "400 Bad Request")
		for _, c := range bad.Causes {
			fmt.Fprintln(w, "  cause:   ", scrub(c.Field+": "+c.Summary))
		}
	case errors.Is(err, domain.ErrNetwork):
		fmt.Fprintln(w, "status:    ", "network failure (DNS, TLS, or connect)")
	case errors.Is(err, domain.ErrOktaServer):
		fmt.Fprintln(w, "status:    ", "5xx — Okta upstream issue")
	default:
		fmt.Fprintln(w, "status:    ", "unexpected error")
	}

	if msg := errormap.UserMessage(err); msg != "" {
		fmt.Fprintln(w, "message:   ", msg)
	}
	fmt.Fprintln(w, "raw:       ", scrub(err.Error()))
	fmt.Fprintln(w, "hint:      ", actionHint(err))
}

func actionHint(err error) string {
	switch {
	case errors.Is(err, domain.ErrTokenInvalid):
		return "Token rejected. Generate a new SSWS token in Okta Admin → Security → API → Tokens, then re-export OKTA_API_TOKEN."
	case errors.Is(err, domain.ErrForbidden):
		return "Token authenticated but lacks permission. Use a Read-Only Administrator token (or higher) for the target tenant."
	case errors.Is(err, domain.ErrNotFound):
		return "Endpoint or org not found. Verify OKTA_ORG_URL points at your tenant (e.g. https://yourco.okta.com)."
	case errors.Is(err, domain.ErrNetwork):
		return "Could not reach Okta. Check the URL spelling, DNS, VPN, and TLS proxies."
	case errors.Is(err, domain.ErrOktaServer):
		return "Okta returned 5xx. Wait a moment and re-run; check https://status.okta.com if persistent."
	default:
		var rl *domain.RateLimitedError
		if errors.As(err, &rl) {
			return "Rate limited. ota will back off automatically in the TUI."
		}
		return "Unexpected error — re-run with --debug for a trace."
	}
}

// scrub strips token-like fragments from the diagnostic so a redirected log
// (e.g. ./ota -check > diag.txt) is safe to share.
func scrub(s string) string { return logger.ScrubText(s) }
