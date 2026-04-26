package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// LoadOptions parameterize Load.
type LoadOptions struct {
	// ExplicitPath, if non-empty, overrides XDG resolution (--config flag).
	ExplicitPath string
}

// LoadResult bundles the parsed Config plus non-fatal warnings the caller
// (cmd/ota) prints to stderr without aborting startup. Currently the only
// warning is loose file permissions (REQ-C05 / QA-012).
type LoadResult struct {
	Path     string
	Config   Config
	Warnings []string
}

// Load resolves the config path (XDG), merges default → file via koanf, and
// validates. Returns the resolved path (for `:about`) and the Config.
//
// Backwards-compatible signature for existing callers; use LoadWithWarnings
// when surfacing warnings is required.
func Load(opts LoadOptions) (string, Config, error) {
	res, err := LoadWithWarnings(opts)
	return res.Path, res.Config, err
}

// LoadWithWarnings is the same as Load but returns a LoadResult so callers
// can surface non-fatal warnings (e.g., loose file permissions).
func LoadWithWarnings(opts LoadOptions) (LoadResult, error) {
	path := opts.ExplicitPath
	if path == "" {
		p, err := ResolvePath()
		if err != nil {
			return LoadResult{Config: Default()}, err
		}
		path = p
	}

	res := LoadResult{Path: path, Config: Default()}

	// Permission check is a soft warning — boot continues.
	if w := checkFilePermissions(path); w != "" {
		res.Warnings = append(res.Warnings, w)
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return res, fmt.Errorf("config: load %s: %w", path, err)
	}

	cfg := Default()
	if err := k.Unmarshal("", &cfg); err != nil {
		return res, fmt.Errorf("config: unmarshal: %w", err)
	}
	res.Config = cfg

	if err := Validate(cfg); err != nil {
		return res, err
	}
	return res, nil
}

// Validate checks structural invariants (profile URL scheme, key ids, ...).
// Called by Load; exposed for tests of synthetic configs.
func Validate(cfg Config) error {
	for name, p := range cfg.Profiles {
		if p.OrgURL == "" {
			return fmt.Errorf("config: profile %q: org_url is required", name)
		}
		if !strings.HasPrefix(p.OrgURL, "https://") {
			return fmt.Errorf("config: profile %q: org_url must use https (got %s)", name, p.OrgURL)
		}
	}
	return nil
}

// checkFilePermissions returns a non-empty warning string when the config
// file is world- or group-readable beyond 0600 (REQ-C05 / QA-012). Symlinks
// and missing files are ignored — Load will surface those separately.
func checkFilePermissions(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	mode := info.Mode().Perm()
	if mode&0o077 == 0 {
		return ""
	}
	return fmt.Sprintf(
		"config file %s has loose permissions (%o); recommend chmod 0600",
		path, mode,
	)
}
