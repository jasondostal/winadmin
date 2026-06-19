package fleet

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/jasondostal/winadmin/secret"
	"github.com/masterzen/winrm"
)

// WinRMTransport runs commands on a Windows target over WinRM (WS-Man /
// PowerShell Remoting's channel) — the Windows-native remote-exec path, no SSH
// required. Commands run on the box, so verbs should use their *local* form
// (e.g. `sc start "X"`, `reg add HKLM\...`), not the \\host remote-admin form.
//
// Go speaks WinRM from any OS, so a Linux/macOS control host can drive a Windows
// fleet. Credentials come from the secret ladder via PasswordProvider.
type WinRMTransport struct {
	User             string
	PasswordProvider secret.Provider
	Port             int           // default 5985 (http) or 5986 (https)
	HTTPS            bool          // use 5986/TLS
	Insecure         bool          // https: skip TLS verification (lab)
	Timeout          time.Duration // default 30s
}

// Describe implements Transport.
func (w WinRMTransport) Describe() string {
	scheme := "http"
	if w.HTTPS {
		scheme = "https"
	}
	return fmt.Sprintf("winrm (user=%s, %s:%d)", w.User, scheme, w.portOrDefault())
}

func (w WinRMTransport) portOrDefault() int {
	if w.Port != 0 {
		return w.Port
	}
	if w.HTTPS {
		return 5986
	}
	return 5985
}

// Exec implements Transport.
func (w WinRMTransport) Exec(ctx context.Context, target Target, command string) (Outcome, error) {
	password := ""
	if w.PasswordProvider != nil {
		cred, err := w.PasswordProvider.Credential()
		if err != nil {
			return Outcome{ExitCode: -1}, err
		}
		password = cred.Password
	}

	timeout := w.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	endpoint := winrm.NewEndpoint(target.Name, w.portOrDefault(), w.HTTPS, w.Insecure, nil, nil, nil, timeout)
	client, err := winrm.NewClient(endpoint, w.User, password)
	if err != nil {
		return Outcome{ExitCode: -1}, fmt.Errorf("winrm connect %s: %w", target.Name, err)
	}

	var stdout, stderr bytes.Buffer
	code, err := client.RunWithContext(ctx, command, &stdout, &stderr)
	out := Outcome{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
	if err != nil {
		out.ExitCode = -1
		return out, fmt.Errorf("winrm exec %s: %w", target.Name, err)
	}
	return out, nil
}
