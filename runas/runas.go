// Package runas launches a process as a different user, given a credential.
//
// This is the modern, in-process replacement for the WinBatch run-as trick:
//
//	cmd /c echo <password> | su.exe svc-install "cmd" -l
//
// Every variant of that — su.exe, a keyed userexec.exe, runas piped a password —
// is doing one Win32 thing: CreateProcessWithLogonW. This package calls it
// directly, so there is no helper EXE, no echo pipe, and the password never
// lands on a command line or in the process list.
package runas

import (
	"errors"

	"github.com/jasondostal/winadmin/secret"
)

// ErrUnsupportedPlatform is returned when run-as is invoked off Windows.
var ErrUnsupportedPlatform = errors.New("winadmin/runas: only supported on Windows")

// Options controls how the command is launched.
type Options struct {
	// CommandLine is the full command line to execute, e.g.
	//   `C:\Windows\System32\cmd.exe /c icacls C:\foo /grant grp:M`.
	CommandLine string

	// WorkingDir is the starting directory ("" -> inherit).
	WorkingDir string

	// Wait blocks until the launched process exits (the WinBatch @WAIT flag).
	// When false, returns as soon as the process is created (@NOWAIT).
	Wait bool

	// LoadProfile loads the target user's profile (runas /profile). Needed when
	// the command reads HKCU or the user's environment.
	LoadProfile bool
}

// Run resolves a credential from p and launches Options.CommandLine as that
// user. On non-Windows builds it returns ErrUnsupportedPlatform.
func Run(p secret.Provider, opts Options) error {
	cred, err := p.Credential()
	if err != nil {
		return err
	}
	return run(cred, opts)
}
