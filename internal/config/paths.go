package config

import (
	"errors"
	"os"
	"path/filepath"
)

// ResolvePath returns the XDG-compliant path ota reads by default.
// Precedence: $XDG_CONFIG_HOME/ota/config.yaml → ~/.config/ota/config.yaml.
// Returns an error only when neither HOME nor XDG_CONFIG_HOME is set;
// non-existent files are fine (caller treats absence as "use defaults").
func ResolvePath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ota", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("config: cannot resolve home directory")
	}
	return filepath.Join(home, ".config", "ota", "config.yaml"), nil
}
