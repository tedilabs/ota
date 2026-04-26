package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/tedilabs/ota/internal/config"
)

// ResolveTokenInput controls token resolution (REQ-C04 AC-1).
//
// Precedence (highest first):
//  1. CLITokenEnv — env variable name from --token-env flag
//  2. Profile.APITokenEnv — env variable name from the active profile
//  3. Interactive prompt (only if Interactive=true) — returned from memory only
type ResolveTokenInput struct {
	CLITokenEnv string
	Profile     config.Profile
	// Interactive enables the terminal prompt fallback (REQ-C04 AC-1 step 3).
	// Tests leave this false to keep Resolve deterministic.
	Interactive bool
}

// ResolveToken applies the precedence in ResolveTokenInput. It returns the
// token value and a human-readable source string for :about ("env OKTA_API_TOKEN"
// or "interactive prompt").
//
// Error messages never contain the token itself (REQ-C05 AC-2) — they refer
// to the env variable name only. Whitespace-only env values are treated as
// missing so placeholders like "   " do not leak.
func ResolveToken(in ResolveTokenInput) (token, source string, err error) {
	candidates := []string{in.CLITokenEnv, in.Profile.APITokenEnv}
	for _, name := range candidates {
		if name == "" {
			continue
		}
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			continue
		}
		return v, "env " + name, nil
	}

	// No env-backed token found. Interactive prompt is production-only; tests
	// leave Interactive=false, so this falls through to an error.
	if !in.Interactive {
		name := firstNonEmpty(in.CLITokenEnv, in.Profile.APITokenEnv, "OKTA_API_TOKEN")
		return "", "", fmt.Errorf("no API token found — set env %s", name)
	}
	// Interactive path would read from tty; keep MVP simple and return an
	// explicit error for now. v0.2 can add huh.Input(EchoMode=Password).
	return "", "", fmt.Errorf("interactive token prompt not implemented")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
