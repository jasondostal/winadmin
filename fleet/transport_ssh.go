package fleet

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/jasondostal/winadmin/secret"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHTransport connects to each target and runs the command ON that machine.
// This is the modern complement to LocalTransport — and a reminder that current
// Windows ships an OpenSSH server as an optional feature, so the same transport
// drives Linux and Windows fleets alike.
//
// Secure by default: host keys are verified against KnownHostsPath (defaulting
// to ~/.ssh/known_hosts). Skipping that check is possible but must be asked for
// explicitly via InsecureIgnoreHostKey — the same posture as the secret package.
type SSHTransport struct {
	User           string
	Port           int           // default 22
	KnownHostsPath string        // default ~/.ssh/known_hosts
	Timeout        time.Duration // dial timeout; default 15s

	// Auth, if set, is used verbatim. Otherwise auth is resolved from KeyPath
	// and/or UseAgent below (and any Password provider wired via Auth).
	Auth []ssh.AuthMethod

	// KeyPath is a private key file used for public-key auth (resolved lazily).
	KeyPath string
	// UseAgent adds the running ssh-agent (SSH_AUTH_SOCK) as an auth source.
	UseAgent bool
	// PasswordProvider, if set, adds password auth resolved from the secret
	// ladder (env -> DPAPI -> CredMan), wiring winadmin/secret into the transport.
	PasswordProvider secret.Provider

	// InsecureIgnoreHostKey disables host-key verification. POC / lab only.
	InsecureIgnoreHostKey bool
}

func (s SSHTransport) resolveAuth() ([]ssh.AuthMethod, error) {
	if len(s.Auth) > 0 {
		return s.Auth, nil
	}
	var methods []ssh.AuthMethod
	if s.KeyPath != "" {
		m, err := SSHKeyAuth(s.KeyPath)
		if err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}
	if s.UseAgent {
		if a, ok := SSHAgentAuth(); ok {
			methods = append(methods, a)
		}
	}
	if s.PasswordProvider != nil {
		methods = append(methods, SSHPasswordFromSecret(s.PasswordProvider))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("ssh: no auth method (set KeyPath, UseAgent, or Auth)")
	}
	return methods, nil
}

// SSHKeyAuth loads a private key file for public-key authentication.
func SSHKeyAuth(path string) (ssh.AuthMethod, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, fmt.Errorf("ssh: parse key %s: %w", path, err)
	}
	return ssh.PublicKeys(signer), nil
}

// SSHAgentAuth returns an auth method backed by the running ssh-agent, if one is
// reachable via SSH_AUTH_SOCK. ok is false when no agent is available.
func SSHAgentAuth() (ssh.AuthMethod, bool) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, false
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, false
	}
	ag := agent.NewClient(conn)
	return ssh.PublicKeysCallback(ag.Signers), true
}

// Describe implements Transport.
func (s SSHTransport) Describe() string {
	return fmt.Sprintf("ssh (user=%s, port=%d)", s.User, s.portOrDefault())
}

func (s SSHTransport) portOrDefault() int {
	if s.Port == 0 {
		return 22
	}
	return s.Port
}

// Exec implements Transport.
func (s SSHTransport) Exec(ctx context.Context, target Target, command string) (Outcome, error) {
	hostKeyCB, err := s.hostKeyCallback()
	if err != nil {
		return Outcome{ExitCode: -1}, err
	}

	timeout := s.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	authMethods, err := s.resolveAuth()
	if err != nil {
		return Outcome{ExitCode: -1}, err
	}
	cfg := &ssh.ClientConfig{
		User:            s.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCB,
		Timeout:         timeout,
	}

	addr := net.JoinHostPort(target.Name, fmt.Sprintf("%d", s.portOrDefault()))

	// Honour context cancellation around the dial.
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Outcome{ExitCode: -1}, fmt.Errorf("dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return Outcome{ExitCode: -1}, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return Outcome{ExitCode: -1}, fmt.Errorf("ssh session %s: %w", addr, err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	runErr := session.Run(command)
	out := Outcome{Stdout: stdout.String(), Stderr: stderr.String()}
	if runErr == nil {
		return out, nil
	}
	if exitErr, ok := runErr.(*ssh.ExitError); ok {
		out.ExitCode = exitErr.ExitStatus()
		return out, nil // non-zero exit is a result, not a transport failure
	}
	out.ExitCode = -1
	return out, runErr
}

func (s SSHTransport) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if s.InsecureIgnoreHostKey {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // opt-in, POC only
	}
	path := s.KnownHostsPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	return knownhosts.New(path)
}

// SSHPasswordFromSecret builds an SSH password auth method from a credential
// provider — wiring the winadmin/secret seam (Env, DPAPI, CredMan, ...) straight
// into the fleet transport. The password is resolved lazily, per connection.
func SSHPasswordFromSecret(p secret.Provider) ssh.AuthMethod {
	return ssh.PasswordCallback(func() (string, error) {
		cred, err := p.Credential()
		if err != nil {
			return "", err
		}
		return cred.Password, nil
	})
}
