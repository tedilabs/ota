package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/config"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/logger"
	"github.com/tedilabs/ota/internal/okta"
	"github.com/tedilabs/ota/internal/service"
)

// WireInput bundles inputs needed by Wire.
type WireInput struct {
	ConfigPath string
	Profile    string
	TokenEnv   string
	Debug      bool
	PollSec    int
}

// Wire is the single explicit dependency-assembly point. It loads config,
// resolves the active profile and token, constructs the Okta client, wires
// services, and returns the App Shell ready for tea.Program.
func Wire(ctx context.Context, in WireInput) (app.Model, config.Config, error) {
	// 1. Load config (XDG or explicit --config).
	res, err := config.LoadWithWarnings(config.LoadOptions{ExplicitPath: in.ConfigPath})
	cfg := res.Config
	if err != nil && !os.IsNotExist(errors.Unwrap(err)) {
		// Missing file is acceptable (we operate on defaults + env), but any
		// other load error is fatal so users see the misconfiguration.
		return app.Model{}, cfg, err
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "ota: warning:", w)
	}

	// 2. Pick profile.
	profileName, profile, err := pickProfile(cfg, in.Profile)
	if err != nil {
		return app.Model{}, cfg, err
	}

	// 3. Resolve token via the app's precedence rules (env only; REQ-C05 AC-1).
	token, _, err := app.ResolveToken(app.ResolveTokenInput{
		CLITokenEnv: in.TokenEnv,
		Profile:     profile,
	})
	if err != nil {
		return app.Model{}, cfg, err
	}

	// 4. Configure logger with a session_id.
	logPath, _ := defaultDebugLogPath()
	sessionID := uuid.NewString()
	lg, err := logger.New(logger.Options{
		FilePath:  logPath,
		Debug:     in.Debug || cfg.Debug,
		SessionID: sessionID,
	})
	if err != nil {
		return app.Model{}, cfg, err
	}

	// 5. Construct the Okta client.
	clk := clock.Real()
	oktaClient, err := okta.NewClient(ctx, okta.Config{
		OrgURL:   profile.OrgURL,
		APIToken: token,
	}, okta.WithClock(clk), okta.WithLogger(lg))
	if err != nil {
		return app.Model{}, cfg, err
	}

	// 6. Assemble services.
	bundle := &service.Bundle{
		Users:    service.NewUsersService(oktaClient.Users(), service.WithClock(clk), service.WithLogger(lg)),
		Groups:   service.NewGroupsService(oktaClient.Groups(), oktaClient.GroupRules(), service.WithClock(clk), service.WithLogger(lg)),
		Rules:    service.NewGroupRulesService(oktaClient.GroupRules(), oktaClient.Groups(), service.WithClock(clk), service.WithLogger(lg)),
		Policies: service.NewPoliciesService(oktaClient.Policies(), service.WithClock(clk), service.WithLogger(lg)),
		Logs:     service.NewLogsService(oktaClient.Logs(), service.WithClock(clk), service.WithLogger(lg)),
		LogsTail: service.NewLogsTail(oktaClient.Logs()),
	}

	// 7. Keybindings.
	keymap, _, err := keys.Resolve(cfg.Keybindings)
	if err != nil {
		return app.Model{}, cfg, err
	}

	// 8. Build the App Shell.
	model := app.New(app.Deps{
		Services:       bundle,
		RateLimit:      oktaClient.RateLimitMonitor(),
		Keys:           keymap,
		Clock:          clk,
		Logger:         lg,
		Profile:        profileName,
		OrgURL:         profile.OrgURL,
		UsersPort:      oktaClient.Users(),
		GroupsPort:     oktaClient.Groups(),
		GroupRulesPort: oktaClient.GroupRules(),
		PoliciesPort:   oktaClient.Policies(),
		LogsPort:       oktaClient.Logs(),
	})
	return model, cfg, nil
}

func pickProfile(cfg config.Config, override string) (string, config.Profile, error) {
	if override != "" {
		p, ok := cfg.Profiles[override]
		if !ok {
			return "", config.Profile{}, fmt.Errorf("ota: profile %q not found in config", override)
		}
		return override, p, nil
	}
	// Auto-select when exactly one profile exists.
	if len(cfg.Profiles) == 1 {
		for n, p := range cfg.Profiles {
			return n, p, nil
		}
	}
	if len(cfg.Profiles) == 0 {
		// Environment-only path: synthesize a default profile from OKTA_ORG_URL.
		if orgURL := os.Getenv("OKTA_ORG_URL"); orgURL != "" {
			return "env", config.Profile{OrgURL: orgURL, APITokenEnv: "OKTA_API_TOKEN"}, nil
		}
		return "", config.Profile{}, errors.New("ota: no profile available — configure one in ~/.config/ota/config.yaml or set OKTA_ORG_URL")
	}
	return "", config.Profile{}, errors.New("ota: multiple profiles configured — pass --profile <name>")
}

func defaultDebugLogPath() (string, error) {
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cache = filepath.Join(home, ".cache")
	}
	return filepath.Join(cache, "ota", "debug.log"), nil
}
