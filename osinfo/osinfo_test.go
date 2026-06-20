package osinfo

import "testing"

func TestClassifyRole(t *testing.T) {
	cases := []struct {
		name        string
		productType string
		want        Role
		isServer    bool
	}{
		{"workstation", "WinNT", Workstation, false},
		{"member server", "ServerNT", MemberServer, true},
		{"domain controller", "LanmanNT", DomainController, true},
		{"empty defaults to workstation", "", Workstation, false},
		{"case-insensitive", "servernt", MemberServer, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classify(raw{productType: c.productType})
			if got.Role != c.want {
				t.Errorf("Role = %v, want %v", got.Role, c.want)
			}
			if got.IsServer() != c.isServer {
				t.Errorf("IsServer() = %v, want %v", got.IsServer(), c.isServer)
			}
		})
	}
}

func TestClassifyRDSHost(t *testing.T) {
	// Server + Terminal Server suite + app-compat mode = RDS host.
	host := classify(raw{productType: "ServerNT", productSuite: []string{"Terminal Server"}, tsAppCompat: true})
	if !host.IsRDSHost() {
		t.Error("expected an RDS host")
	}
	// Same server in remote-admin mode (no app-compat) is NOT an RDS host.
	admin := classify(raw{productType: "ServerNT", productSuite: []string{"Terminal Server"}, tsAppCompat: false})
	if admin.IsRDSHost() {
		t.Error("remote-admin-mode TS should not count as an RDS host")
	}
	// A workstation is never an RDS host even with the flag set.
	ws := classify(raw{productType: "WinNT", productSuite: []string{"Terminal Server"}, tsAppCompat: true})
	if ws.IsRDSHost() {
		t.Error("workstation should never be an RDS host")
	}
}

func TestClassifyWin11Quirk(t *testing.T) {
	got := classify(raw{productType: "WinNT", productName: "Windows 10 Pro", build: 22631})
	if got.ProductName != "Windows 11 Pro" {
		t.Errorf("ProductName = %q, want %q (build 22631 is Windows 11)", got.ProductName, "Windows 11 Pro")
	}
	// Genuine Windows 10 is left alone.
	ten := classify(raw{productType: "WinNT", productName: "Windows 10 Pro", build: 19045})
	if ten.ProductName != "Windows 10 Pro" {
		t.Errorf("ProductName = %q, want unchanged", ten.ProductName)
	}
}

func TestNormalizeArch(t *testing.T) {
	for in, want := range map[string]string{"AMD64": "amd64", "x86": "x86", "ARM64": "arm64", "": ""} {
		if got := normalizeArch(in); got != want {
			t.Errorf("normalizeArch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVersionAndString(t *testing.T) {
	i := classify(raw{
		productType: "ServerNT", productName: "Windows Server 2019 Standard",
		major: 10, minor: 0, build: 17763, ubr: 4131, arch: "AMD64", displayVersion: "1809",
	})
	if i.Version() != "10.0.17763.4131" {
		t.Errorf("Version() = %q", i.Version())
	}
	if s := i.String(); s == "" || !containsAll(s, "Windows Server 2019", "10.0.17763.4131", "amd64", "server") {
		t.Errorf("String() = %q", s)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
