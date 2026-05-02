package config

// Config is ota's top-level configuration (see docs/CONVENTIONS.md §7).
type Config struct {
	Profiles    map[string]Profile `koanf:"profiles"`
	UI          UI                 `koanf:"ui"`
	Keybindings map[string]string  `koanf:"keybindings"`
	Logs        LogsConfig         `koanf:"logs"`
	Refresh     RefreshConfig      `koanf:"refresh"`
	Debug       bool               `koanf:"debug"`
}

// RefreshConfig tunes the periodic auto-refresh cadence per resource
// list (issue #177 v0.1.16). LogsSeconds drives the Logs tail loop;
// DefaultSeconds drives every other list (Users / Groups / Group
// Rules / Apps / Policies). Either value can be zero to disable
// auto-refresh on that surface.
type RefreshConfig struct {
	LogsSeconds    int `koanf:"logs_seconds"`
	DefaultSeconds int `koanf:"default_seconds"`
}

// Profile captures a single tenant profile. API token is never stored in the
// file; only the env variable name is referenced (REQ-C05 AC-4).
type Profile struct {
	OrgURL           string `koanf:"org_url"`
	APITokenEnv      string `koanf:"api_token_env"`
	DefaultLogFilter string `koanf:"default_log_filter"`
}

// UI groups presentation-layer settings.
type UI struct {
	Theme       string      `koanf:"theme"` // dark | high_contrast | monochrome
	PIIMasking  PIIMasking  `koanf:"pii_masking"`
}

// PIIMasking configures the mask layer (TUI_DESIGN §7.3).
type PIIMasking struct {
	Enabled              bool `koanf:"enabled"`
	DefaultUnmaskOnCopy  bool `koanf:"default_unmask_on_copy"`
	LogsActorEmail       bool `koanf:"logs_actor_email"` // reserved — Logs actor.alternateId masking
}

// LogsConfig carries Logs tail tuning.
type LogsConfig struct {
	PollIntervalSeconds int `koanf:"poll_interval_seconds"`
	// LimitPerFetch caps how many events ota requests per /api/v1/logs
	// call. Default 100 — small enough to keep the screen readable,
	// big enough to cover most active orgs' last-30-min volume.
	// Operators investigating a specific actor / time range can raise
	// this; ota always uses Okta's `since`/`until` to scope the
	// window so the cap doesn't truncate context. (#F3 v0.2.5)
	LimitPerFetch int `koanf:"limit_per_fetch"`
}

// Default returns the in-memory default Config used when no file is present.
func Default() Config {
	return Config{
		Profiles: map[string]Profile{},
		UI: UI{
			Theme: "dark",
			PIIMasking: PIIMasking{
				Enabled: true,
			},
		},
		Keybindings: map[string]string{},
		Logs:        LogsConfig{PollIntervalSeconds: 7, LimitPerFetch: 100},
		Refresh:     RefreshConfig{LogsSeconds: 10, DefaultSeconds: 10},
		Debug:       false,
	}
}
