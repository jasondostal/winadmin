package secret

import "testing"

func TestPlaintextProvider(t *testing.T) {
	p := Plaintext{User: "svc-install", Domain: "ACME", Password: "hunter2"}
	c, err := p.Credential()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.User != "svc-install" || c.Domain != "ACME" || c.Password != "hunter2" {
		t.Fatalf("credential round-trip mismatch: %+v", c)
	}
}

func TestPlaintextRejectsEmptyUser(t *testing.T) {
	if _, err := (Plaintext{Password: "x"}).Credential(); err == nil {
		t.Fatal("expected error for empty user, got nil")
	}
}

func TestCredentialStringRedactsPassword(t *testing.T) {
	c := Credential{User: "svc", Domain: "ACME", Password: "topsecret"}
	got := c.String()
	if got != `ACME\svc:********` {
		t.Fatalf("got %q, want redacted form", got)
	}
	if contains(got, "topsecret") {
		t.Fatal("password leaked into String()")
	}
}

func TestCredentialStringEmptyDomain(t *testing.T) {
	c := Credential{User: "svc"}
	if got := c.String(); got != `.\svc:` {
		t.Fatalf("got %q, want .\\svc:", got)
	}
}

func TestEnvProvider(t *testing.T) {
	t.Setenv("WINADMIN_USER", "envuser")
	t.Setenv("WINADMIN_DOMAIN", "ENVDOM")
	t.Setenv("WINADMIN_PASSWORD", "envpass")

	c, err := Env{}.Credential()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.User != "envuser" || c.Domain != "ENVDOM" || c.Password != "envpass" {
		t.Fatalf("env provider mismatch: %+v", c)
	}
}

func TestEnvProviderCustomVars(t *testing.T) {
	t.Setenv("MY_USER", "custom")
	c, err := Env{UserVar: "MY_USER"}.Credential()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.User != "custom" {
		t.Fatalf("got user %q, want custom", c.User)
	}
}

func TestEnvProviderMissingUser(t *testing.T) {
	t.Setenv("WINADMIN_USER", "")
	if _, err := (Env{}).Credential(); err == nil {
		t.Fatal("expected error for missing user, got nil")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
