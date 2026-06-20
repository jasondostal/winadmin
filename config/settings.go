package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NamedQuery is a reusable gather query: a friendly name, the command to run per
// target, and an optional parse hint ("kv", "columns", "csv", or "" for raw
// output) that tells the gather view how to split the output into columns.
type NamedQuery struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Parse   string `json:"parse,omitempty"`
}

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

	// GatherQueries is the library of canned gather queries surfaced in the run
	// builder and resolvable by name on the CLI. Empty falls back to the
	// built-in DefaultGatherQueries.
	GatherQueries []NamedQuery `json:"gather_queries,omitempty"`
}

// DefaultGatherQueries is the built-in query library — the everyday "what's the
// state of the fleet?" questions, with a parse hint so each comes back as real,
// sortable/groupable columns. Linux forms run as-is; the Windows forms use wmic.
// Users override or extend this via the gather_queries config key.
func DefaultGatherQueries() []NamedQuery {
	return []NamedQuery{
		{Name: "os version (linux)", Command: "cat /etc/os-release", Parse: "kv"},
		{Name: "free disk (linux)", Command: `df -hP | awk 'BEGIN{print "Mount,Size,Used,Avail,Usepct"} NR>1{print $6","$2","$3","$4","$5}'`, Parse: "csv"},
		{Name: "uptime (linux)", Command: "uptime -p", Parse: ""},
		{Name: "last reboot (linux)", Command: `who -b | sed 's/^[[:space:]]*system boot[[:space:]]*//'`, Parse: ""},
		{Name: "who's logged on (linux)", Command: `who | awk 'BEGIN{print "User,TTY,Since"} {print $1","$2","$3" "$4}'`, Parse: "csv"},
		{Name: "kernel (linux)", Command: "uname -r", Parse: ""},
		// Windows queries use PowerShell/CIM, not wmic — wmic is absent on
		// Server 2025 (deprecated), where these still work. ConvertTo-Csv gives
		// clean, parseable columns across 2016→2025.
		{Name: "os version (windows)", Command: `powershell -NoProfile -Command "Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,BuildNumber | ConvertTo-Csv -NoTypeInformation"`, Parse: "csv"},
		{Name: "free disk (windows)", Command: `powershell -NoProfile -Command "Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | Select-Object DeviceID,@{n='FreeBytes';e={$_.FreeSpace}},@{n='SizeBytes';e={$_.Size}} | ConvertTo-Csv -NoTypeInformation"`, Parse: "csv"},
		{Name: "services running (windows)", Command: `powershell -NoProfile -Command "Get-Service | Where-Object Status -eq Running | Select-Object Name,StartType,Status | ConvertTo-Csv -NoTypeInformation"`, Parse: "csv"},
		{Name: "pending reboot? (windows)", Command: `powershell -NoProfile -Command "Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending'"`, Parse: ""},
	}
}

// GatherLibrary returns the configured queries, or the built-in defaults when
// none are set — so the picker is never empty.
func (s Settings) GatherLibrary() []NamedQuery {
	if len(s.GatherQueries) > 0 {
		return s.GatherQueries
	}
	return DefaultGatherQueries()
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
