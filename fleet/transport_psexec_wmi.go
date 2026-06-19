package fleet

import (
	"context"
	"fmt"
	"strings"

	"github.com/jasondostal/winadmin/secret"
)

// PsExecTransport and WMITransport both run FROM a Windows control host (they
// shell out to psexec.exe / wmic.exe) and reach the target over SMB / DCOM —
// the paths to use when a box has neither SSH nor WinRM enabled. The command
// runs on the target, so verbs should use their local form.

// PsExecTransport runs a command on the target via SysInternals PsExec
// (`psexec \\host -u U -p P cmd /c "<command>"`). Requires psexec.exe on PATH
// (or set Path) and a Windows control host.
type PsExecTransport struct {
	User             string
	PasswordProvider secret.Provider
	Path             string // psexec.exe path; default "psexec"
}

// Describe implements Transport.
func (p PsExecTransport) Describe() string { return fmt.Sprintf("psexec (user=%s)", p.User) }

// Exec implements Transport.
func (p PsExecTransport) Exec(ctx context.Context, target Target, command string) (Outcome, error) {
	pw, err := providerPassword(p.PasswordProvider)
	if err != nil {
		return Outcome{ExitCode: -1}, err
	}
	exe := p.Path
	if exe == "" {
		exe = "psexec"
	}
	return LocalTransport{}.Exec(ctx, target, psexecCmd(exe, target.Name, p.User, pw, command))
}

func psexecCmd(exe, host, user, pw, command string) string {
	creds := ""
	if user != "" {
		creds = fmt.Sprintf(` -u %s -p %s`, user, pw)
	}
	return fmt.Sprintf(`%s \\%s%s -accepteula -nobanner cmd /c %s`, exe, host, creds, winQuote(command))
}

// WMITransport runs a command on the target via WMI
// (`wmic /node:host /user:U /password:P process call create "<command>"`).
// WMI process-create is fire-and-forget: it returns a ProcessId/ReturnValue, not
// the command's stdout or exit code — good for kicking off work, not for output.
// Requires wmic.exe and a Windows control host.
type WMITransport struct {
	User             string
	PasswordProvider secret.Provider
}

// Describe implements Transport.
func (w WMITransport) Describe() string { return fmt.Sprintf("wmi (user=%s)", w.User) }

// Exec implements Transport.
func (w WMITransport) Exec(ctx context.Context, target Target, command string) (Outcome, error) {
	pw, err := providerPassword(w.PasswordProvider)
	if err != nil {
		return Outcome{ExitCode: -1}, err
	}
	return LocalTransport{}.Exec(ctx, target, wmiCmd(target.Name, w.User, pw, command))
}

func wmiCmd(host, user, pw, command string) string {
	creds := ""
	if user != "" {
		creds = fmt.Sprintf(` /user:"%s" /password:"%s"`, user, pw)
	}
	return fmt.Sprintf(`wmic /node:"%s"%s process call create %s`, host, creds, winQuote(command))
}

func providerPassword(p secret.Provider) (string, error) {
	if p == nil {
		return "", nil
	}
	cred, err := p.Credential()
	if err != nil {
		return "", err
	}
	return cred.Password, nil
}

// winQuote wraps a command for `cmd /c "<...>"`, escaping embedded quotes.
func winQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
