package tui

import "strings"

// svcEntry pairs a Windows service's short name (what sc.exe / Get-Service want)
// with its display name (what the Services MMC shows). The two rarely match, which
// is exactly why the Service field offers type-ahead over both.
type svcEntry struct {
	Short   string
	Display string
}

// winServices is a curated list of common Windows services. Not exhaustive —
// free text always works — just the ones people reach for, so you can type either
// name and get the right short name.
var winServices = []svcEntry{
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

// matchServices returns up to max services whose short OR display name contains
// the query (case-insensitive). An empty query returns the most common few.
func matchServices(query string, max int) []svcEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]svcEntry, 0, max)
	for _, s := range winServices {
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
	for _, s := range winServices {
		if strings.ToLower(s.Short) == n {
			return true
		}
	}
	return false
}
