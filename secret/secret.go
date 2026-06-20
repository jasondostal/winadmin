// Package secret provides pluggable credential sources for run-as operations.
//
// The whole point of this package is one small seam:
//
//	type Provider interface { Credential() (Credential, error) }
//
// In the old days, the run-as account's password was a plaintext constant
// hardcoded into every script. That worked — and it was the single biggest
// liability in the codebase. Here, the call site that needs a credential depends
// only on the Provider interface; *where* the secret actually lives is a
// configuration decision, not a code change.
//
// Providers, weakest to strongest:
//
//	Plaintext  - literal password in source. Possible, but it announces itself.
//	Env        - read from environment variables (out of the binary).
//	DPAPI      - Windows DPAPI-encrypted blob on disk (no key to ship). [windows]
//	CredMan    - Windows Credential Manager entry (binary carries no secret). [windows]
//
// The lesson worth keeping: make the insecure path possible but obvious, and
// make the secure path the easy default.
package secret

import "errors"

// ErrUnsupportedPlatform is returned by OS-specific providers (DPAPI, CredMan)
// when built for a non-Windows target.
var ErrUnsupportedPlatform = errors.New("winadmin/secret: provider not supported on this platform")

// Credential is a resolved set of logon values.
//
// Domain may be a real domain ("ACME") or a machine name for a local
// account. An empty Domain is interpreted as "." (the local machine) by the
// runas package.
type Credential struct {
	User     string
	Domain   string
	Password string
}

// IsZero reports whether the credential carries no user and no password.
func (c Credential) IsZero() bool {
	return c.User == "" && c.Password == ""
}

// String redacts the password so credentials never leak into logs.
func (c Credential) String() string {
	pw := ""
	if c.Password != "" {
		pw = "********"
	}
	dom := c.Domain
	if dom == "" {
		dom = "."
	}
	return dom + `\` + c.User + ":" + pw
}

// Provider yields a Credential at runtime. Implementations decide where the
// secret physically lives.
type Provider interface {
	Credential() (Credential, error)
}

// Plaintext is the deliberately-insecure option: a literal credential compiled into the
// program. It exists on purpose — for bootstrapping, for parity with the old
// scripts, and so the insecure choice is a deliberate, visible one.
//
// Reach for Env, DPAPI, or CredMan the moment you can. Swapping is a one-line
// change because everything downstream only sees Provider.
type Plaintext struct {
	User     string
	Domain   string
	Password string
}

// Credential implements Provider.
func (p Plaintext) Credential() (Credential, error) {
	if p.User == "" {
		return Credential{}, errors.New("winadmin/secret: Plaintext provider has empty User")
	}
	return Credential{User: p.User, Domain: p.Domain, Password: p.Password}, nil
}
