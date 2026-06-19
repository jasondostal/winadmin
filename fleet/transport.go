package fleet

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
)

// Outcome is what a Transport returns for one executed command. A non-zero
// ExitCode is NOT an error — it's a result; err is reserved for the transport
// failing to run the command at all (couldn't connect, couldn't spawn).
type Outcome struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Transport executes a rendered command "for" a target and reports the outcome.
//
// Two models, both valid:
//
//   - LocalTransport runs the command on THIS machine. The command itself
//     reaches the target — `reg add \\SERVER\HKLM\...`, `robocopy src \\SERVER\...`.
//     The classic remote-admin pattern: shell a command locally and let it reach
//     out over admin shares. One control box, hundreds of servers.
//
//   - SSHTransport connects to the target and runs the command ON the box.
//     The modern model; the command needs no \\SERVER prefix.
type Transport interface {
	Exec(ctx context.Context, target Target, command string) (Outcome, error)
	Describe() string
}

// LocalTransport runs each command through the local shell. It is the
// dependency-free, runs-anywhere default — shell a command per target.
type LocalTransport struct{}

// Describe implements Transport.
func (LocalTransport) Describe() string { return "local shell" }

// Exec implements Transport.
func (LocalTransport) Exec(ctx context.Context, _ Target, command string) (Outcome, error) {
	shell, flag := localShell()
	cmd := exec.CommandContext(ctx, shell, flag, command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := Outcome{Stdout: stdout.String(), Stderr: stderr.String()}

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		out.ExitCode = 0
	case errors.As(err, &exitErr):
		// The command ran and exited non-zero: a result, not a transport error.
		out.ExitCode = exitErr.ExitCode()
		err = nil
	default:
		// Couldn't start the command (shell missing, context cancelled, etc.).
		out.ExitCode = -1
	}
	return out, err
}

func localShell() (shell, flag string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", "/c"
	}
	return "/bin/sh", "-c"
}
