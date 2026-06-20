package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Settings holds persisted run defaults — the "set it once" preferences an admin
// shouldn't have to retype every run. The file is the single source of truth;
// both the TUI settings screen and the CLI read it.
type Settings struct {
	Parallelism  int    `json:"parallelism,omitempty"`
	Transport    string `json:"transport,omitempty"`
	SSHUser      string `json:"ssh_user,omitempty"`
	SSHKey       string `json:"ssh_key,omitempty"`
	ServicesFile string `json:"services_file,omitempty"`
	DefaultHosts string `json:"default_hosts,omitempty"`
}

// SettingsPath is the config file location: $FLEET_CONFIG if set, else the
// per-user path (~/.config/fleet/config.json, %AppData%\fleet\config.json).
func SettingsPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("FLEET_CONFIG")); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "fleet", "config.json"), nil
}

// LoadSettings reads the settings file. A missing file is not an error — it
// returns the zero Settings, so callers can always use the result.
func LoadSettings() (Settings, error) {
	var s Settings
	path, err := SettingsPath()
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("winadmin/config: parse %s: %w", path, err)
	}
	return s, nil
}

// SaveSettings writes the settings file (creating the directory) and returns the
// path it wrote to.
func SaveSettings(s Settings) (string, error) {
	path, err := SettingsPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
