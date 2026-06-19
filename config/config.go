// Package config turns declarative configuration into a secret.Provider.
//
// This is where the "make doing-it-right easy" principle becomes concrete: the
// provider is selected by a single string, and the default is NOT plaintext.
// Choosing the insecure path requires explicitly writing "plaintext" — it can
// never be the thing you fell into by leaving a field blank.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jasondostal/winadmin/secret"
)

// ProviderKind selects which secret provider to build.
type ProviderKind string

const (
	KindEnv       ProviderKind = "env"       // default
	KindCredMan   ProviderKind = "credman"   // Windows Credential Manager
	KindDPAPI     ProviderKind = "dpapi"     // DPAPI-encrypted blob on disk
	KindPlaintext ProviderKind = "plaintext" // opt-in only, never the default
)

// RunAs configures the credential used for run-as operations.
type RunAs struct {
	// Provider selects the backend. Empty defaults to KindEnv.
	Provider ProviderKind `json:"provider"`

	User   string `json:"user,omitempty"`
	Domain string `json:"domain,omitempty"`

	// Password is consulted ONLY when Provider == "plaintext".
	Password string `json:"password,omitempty"`

	// BlobPath is the DPAPI blob location (Provider == "dpapi").
	BlobPath string `json:"blob_path,omitempty"`

	// Target is the Credential Manager target name (Provider == "credman").
	Target string `json:"target,omitempty"`
}

// Config is the top-level configuration document.
type Config struct {
	RunAs RunAs `json:"runas"`
}

// Load reads and parses a JSON config file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("winadmin/config: parse %s: %w", path, err)
	}
	return &c, nil
}

// Build constructs a secret.Provider from the RunAs configuration. The default
// (empty Provider) is KindEnv, deliberately keeping secrets out of both source
// and config files unless the operator opts down to plaintext.
func (r RunAs) Build() (secret.Provider, error) {
	switch ProviderKind(strings.ToLower(string(r.Provider))) {
	case "", KindEnv:
		return secret.Env{}, nil

	case KindPlaintext:
		return secret.Plaintext{User: r.User, Domain: r.Domain, Password: r.Password}, nil

	case KindDPAPI:
		return secret.DPAPI{User: r.User, Domain: r.Domain, BlobPath: r.BlobPath}, nil

	case KindCredMan:
		return secret.CredMan{TargetName: r.Target, Domain: r.Domain, User: r.User}, nil

	default:
		return nil, fmt.Errorf("winadmin/config: unknown provider %q", r.Provider)
	}
}
