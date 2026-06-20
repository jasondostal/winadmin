package tui

import (
	"os"
	"path/filepath"
	"strings"
)

// svcEntry pairs a Windows service's short name (what sc.exe / Get-Service want)
// with its display name (what the Services MMC shows). The two rarely match, which
// is exactly why the Service field offers type-ahead over both.
type svcEntry struct {
	Short   string
	Display string
}

// defaultServices is the built-in fallback list of common Windows services. Not
// exhaustive — free text always works — just the ones people reach for. The
// single source of truth, when present, is the services file (see serviceList).
var defaultServices = []svcEntry{
	{"Spooler", "Print Spooler"},
	{"W32Time", "Windows Time"},
	{"WinRM", "Windows Remote Management (WS-Management)"},
	{"TermService", "Remote Desktop Services"},
	{"LanmanServer", "Server"},
	{"LanmanWorkstation", "Workstation"},
	{"Dhcp", "DHCP Client"},
	{"Dnscache", "DNS Client"},
	{"BITS", "Background Intelligent Transfer Service"},
	{"wuauserv", "Windows Update"},
	{"WinDefend", "Microsoft Defender Antivirus Service"},
	{"MpsSvc", "Windows Defender Firewall"},
	{"EventLog", "Windows Event Log"},
	{"Schedule", "Task Scheduler"},
	{"gpsvc", "Group Policy Client"},
	{"Netlogon", "Netlogon"},
	{"NTDS", "Active Directory Domain Services"},
	{"DNS", "DNS Server"},
	{"DFSR", "DFS Replication"},
	{"W3SVC", "World Wide Web Publishing Service"},
	{"MSSQLSERVER", "SQL Server (Database Engine)"},
	{"RpcSs", "Remote Procedure Call (RPC)"},
	{"WSearch", "Windows Search"},
	{"sshd", "OpenSSH SSH Server"},
	{"VSS", "Volume Shadow Copy"},
	{"Audiosrv", "Windows Audio"},
	{"ProfSvc", "User Profile Service"},
	{"WlanSvc", "WLAN AutoConfig"},
	{"AppInfo", "Application Information (UAC)"},
	{"Themes", "Themes"},
}

// loadedServices caches the resolved list for the process lifetime.
var loadedServices []svcEntry

// serviceList returns the curated services, loading them once. The file (when
// present) is the single source of truth; otherwise the built-in defaults apply.
func serviceList() []svcEntry {
	if loadedServices == nil {
		loadedServices = loadServices()
	}
	return loadedServices
}

// loadServices reads the services file, falling back to defaultServices when no
// file is configured, it can't be read, or it's empty. Format: one service per
// line, "Short" or "Short|Display Name"; blank lines and #/; comments ignored.
func loadServices() []svcEntry {
	path := servicesFilePath()
	if path == "" {
		return defaultServices
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return defaultServices
	}
	var out []svcEntry
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		short, display, _ := strings.Cut(line, "|")
		short = strings.TrimSpace(short)
		display = strings.TrimSpace(display)
		if display == "" {
			display = short
		}
		if short != "" {
			out = append(out, svcEntry{Short: short, Display: display})
		}
	}
	if len(out) == 0 {
		return defaultServices
	}
	return out
}

// servicesFilePath resolves the services list file: $FLEET_SERVICES wins; else
// the per-user config path (~/.config/fleet/services.txt, %AppData%\fleet on
// Windows) when it exists. Empty means "use the built-in defaults".
func servicesFilePath() string {
	if p := strings.TrimSpace(os.Getenv("FLEET_SERVICES")); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(dir, "fleet", "services.txt")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// matchServices returns up to max services whose short OR display name contains
// the query (case-insensitive). An empty query returns the most common few.
func matchServices(query string, max int) []svcEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]svcEntry, 0, max)
	for _, s := range serviceList() {
		if q == "" || strings.Contains(strings.ToLower(s.Short), q) || strings.Contains(strings.ToLower(s.Display), q) {
			out = append(out, s)
			if len(out) >= max {
				break
			}
		}
	}
	return out
}

// exactService reports whether name is already a known short name (case-insensitive).
func exactService(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, s := range serviceList() {
		if strings.ToLower(s.Short) == n {
			return true
		}
	}
	return false
}
