package config

import (
	"testing"

	"github.com/jasondostal/winadmin/secret"
)

func TestDefaultProviderIsEnvNotPlaintext(t *testing.T) {
	// The whole safety argument: a blank config must NEVER yield a plaintext
	// provider. Leaving fields empty should fall through to env, not secrets.
	p, err := RunAs{}.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(secret.Env); !ok {
		t.Fatalf("default provider is %T, want secret.Env", p)
	}
}

func TestPlaintextRequiresExplicitOptIn(t *testing.T) {
	p, err := RunAs{Provider: KindPlaintext, User: "svc", Password: "pw"}.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pt, ok := p.(secret.Plaintext)
	if !ok {
		t.Fatalf("got %T, want secret.Plaintext", p)
	}
	if pt.User != "svc" || pt.Password != "pw" {
		t.Fatalf("plaintext fields not wired through: %+v", pt)
	}
}

func TestProviderKindIsCaseInsensitive(t *testing.T) {
	p, err := RunAs{Provider: "CredMan", Target: "svc-cred"}.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(secret.CredMan); !ok {
		t.Fatalf("got %T, want secret.CredMan", p)
	}
}

func TestUnknownProviderErrors(t *testing.T) {
	if _, err := (RunAs{Provider: "ldap-magic"}).Build(); err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}
