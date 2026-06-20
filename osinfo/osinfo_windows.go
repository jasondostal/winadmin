//go:build windows

package osinfo

import (
	"strconv"
	"strings"

	"github.com/jasondostal/winadmin/reg"
)

const (
	keyCurrentVersion = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	keyProductOptions = `SYSTEM\CurrentControlSet\Control\ProductOptions`
	keyTerminalServer = `SYSTEM\CurrentControlSet\Control\Terminal Server`
	keyEnvironment    = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	valProcessorArch  = "PROCESSOR_ARCHITECTURE"
)

func detect() (Info, error) {
	// Force the 64-bit view so a 32-bit build still reads the real values — the
	// WOW64 choice the old 32-bit scripts never had.
	v := reg.Force64

	r := raw{
		productName:    regStr(keyCurrentVersion, "ProductName", v),
		editionID:      regStr(keyCurrentVersion, "EditionID", v),
		displayVersion: regStr(keyCurrentVersion, "DisplayVersion", v),
		build:          atoi(regStr(keyCurrentVersion, "CurrentBuildNumber", v)),
		major:          int(regDWord(keyCurrentVersion, "CurrentMajorVersionNumber", v)),
		minor:          int(regDWord(keyCurrentVersion, "CurrentMinorVersionNumber", v)),
		ubr:            int(regDWord(keyCurrentVersion, "UBR", v)),
		productType:    regStr(keyProductOptions, "ProductType", v),
		productSuite:   regStrs(keyProductOptions, "ProductSuite", v),
		tsAppCompat:    regDWord(keyTerminalServer, "TSAppCompat", v) != 0,
		arch:           regStr(keyEnvironment, valProcessorArch, v),
	}
	// DisplayVersion is absent before 20H2; fall back to the older ReleaseId.
	if r.displayVersion == "" {
		r.displayVersion = regStr(keyCurrentVersion, "ReleaseId", v)
	}
	return classify(r), nil
}

// The reg helpers below treat any read error (including a missing value) as the
// zero value: a partial-but-useful Info beats failing the whole detect.
func regStr(path, name string, v reg.View) string {
	s, err := reg.GetString(reg.LocalMachine, path, name, v)
	if err != nil {
		return ""
	}
	return s
}

func regStrs(path, name string, v reg.View) []string {
	ss, err := reg.GetStrings(reg.LocalMachine, path, name, v)
	if err != nil {
		return nil
	}
	return ss
}

func regDWord(path, name string, v reg.View) uint32 {
	n, err := reg.GetDWord(reg.LocalMachine, path, name, v)
	if err != nil {
		return 0
	}
	return n
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
