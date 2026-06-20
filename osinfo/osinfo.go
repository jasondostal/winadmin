// Package osinfo identifies the running Windows OS the way classic login scripts
// did — by reading a handful of registry values and classifying them — but
// returns one typed Info instead of a pile of ad-hoc string compares scattered
// across a script.
//
// The reads are Windows-only (Detect returns ErrUnsupportedPlatform elsewhere),
// but the classification logic is pure and platform-independent, so the
// interesting decisions are unit-tested everywhere.
package osinfo

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnsupportedPlatform is returned by Detect off Windows.
var ErrUnsupportedPlatform = errors.New("winadmin/osinfo: only supported on Windows")

// Role is the installation type, from the ProductType registry value.
type Role int

const (
	Workstation      Role = iota // WinNT   — a client OS
	MemberServer                 // ServerNT — a server that is not a DC
	DomainController             // LanmanNT — a domain controller
)

// String renders the role.
func (r Role) String() string {
	switch r {
	case MemberServer:
		return "server"
	case DomainController:
		return "domain-controller"
	default:
		return "workstation"
	}
}

// Info is a structured description of the OS.
type Info struct {
	ProductName    string // e.g. "Windows 10 Pro" / "Windows Server 2019 Standard"
	EditionID      string // e.g. "Professional", "ServerStandard"
	DisplayVersion string // e.g. "22H2" (ReleaseId on older builds)
	Major, Minor   int    // e.g. 10, 0  (0 on pre-Windows-10 builds that lack the value)
	Build          int    // e.g. 22621
	UBR            int    // update build revision (the ".1702" in 10.0.22621.1702)
	Arch           string // normalized: "amd64", "x86", "arm64"
	Role           Role
	TerminalServer bool // true when this is an RDS/Citrix application host (multi-user)
}

// IsServer reports whether this is any server SKU (member server or DC).
func (i Info) IsServer() bool { return i.Role != Workstation }

// IsDomainController reports whether this box is a DC.
func (i Info) IsDomainController() bool { return i.Role == DomainController }

// IsRDSHost reports whether this is a multi-user Remote Desktop Session Host —
// the modern read of the old session-host check (a server in TS application mode).
func (i Info) IsRDSHost() bool { return i.IsServer() && i.TerminalServer }

// Version is the dotted version string, e.g. "10.0.22621.1702".
func (i Info) Version() string {
	return fmt.Sprintf("%d.%d.%d.%d", i.Major, i.Minor, i.Build, i.UBR)
}

// String renders a one-line human summary.
func (i Info) String() string {
	parts := []string{i.ProductName}
	if i.DisplayVersion != "" {
		parts = append(parts, i.DisplayVersion)
	}
	parts = append(parts, "("+i.Version()+")")
	if i.Arch != "" {
		parts = append(parts, i.Arch)
	}
	role := i.Role.String()
	if i.IsRDSHost() {
		role += "/rds-host"
	}
	return strings.Join(parts, " ") + " [" + role + "]"
}

// Detect reads the OS information from the registry. Off Windows it returns
// ErrUnsupportedPlatform.
func Detect() (Info, error) { return detect() }

// raw is the bag of registry values classify() turns into an Info. Keeping it
// separate is what lets the classification be tested without a registry.
type raw struct {
	productType    string
	productSuite   []string
	productName    string
	editionID      string
	displayVersion string
	major, minor   int
	build, ubr     int
	tsAppCompat    bool
	arch           string
}

// classify is the pure heart of the package: registry values in, typed Info out.
func classify(r raw) Info {
	info := Info{
		ProductName:    strings.TrimSpace(r.productName),
		EditionID:      r.editionID,
		DisplayVersion: r.displayVersion,
		Major:          r.major,
		Minor:          r.minor,
		Build:          r.build,
		UBR:            r.ubr,
		Arch:           normalizeArch(r.arch),
	}

	switch strings.ToUpper(strings.TrimSpace(r.productType)) {
	case "SERVERNT":
		info.Role = MemberServer
	case "LANMANNT":
		info.Role = DomainController
	default: // "WINNT" or empty
		info.Role = Workstation
	}

	// A server is an RDS application host when its product suite advertises
	// Terminal Server *and* it is in application-compatibility (multi-user) mode
	// rather than remote-administration mode.
	for _, s := range r.productSuite {
		if strings.EqualFold(strings.TrimSpace(s), "Terminal Server") {
			info.TerminalServer = r.tsAppCompat
		}
	}

	// The well-known Windows 11 quirk: the registry ProductName still reads
	// "Windows 10" on 11. Correct it by build number so callers don't have to.
	if info.Role == Workstation && info.Build >= 22000 && strings.Contains(info.ProductName, "Windows 10") {
		info.ProductName = strings.Replace(info.ProductName, "Windows 10", "Windows 11", 1)
	}

	return info
}

func normalizeArch(a string) string {
	switch strings.ToUpper(strings.TrimSpace(a)) {
	case "AMD64":
		return "amd64"
	case "X86":
		return "x86"
	case "ARM64":
		return "arm64"
	case "":
		return ""
	default:
		return strings.ToLower(a)
	}
}
