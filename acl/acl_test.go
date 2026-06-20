package acl

import (
	"strings"
	"testing"
)

func TestGrantArgs(t *testing.T) {
	got := grantArgs(`C:\Share\Tellers`, `CORP\Tellers`, Modify, Options{Recurse: true})
	want := []string{`C:\Share\Tellers`, "/grant", `CORP\Tellers:(OI)(CI)M`, "/T"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("grantArgs = %v, want %v", got, want)
	}
}

func TestGrantArgsNoInherit(t *testing.T) {
	got := grantArgs(`C:\app.exe`, "Users", ReadExecute, Options{NoInherit: true})
	want := []string{`C:\app.exe`, "/grant", "Users:RX"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("grantArgs = %v, want %v", got, want)
	}
}

func TestRevokeArgs(t *testing.T) {
	got := revokeArgs(`C:\Share`, `CORP\Tellers`, Options{})
	want := []string{`C:\Share`, "/remove", `CORP\Tellers`}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("revokeArgs = %v, want %v", got, want)
	}
}

func TestGrantValidates(t *testing.T) {
	if err := Grant("", "Users", Read, Options{}); err == nil {
		t.Error("expected error for empty path")
	}
	if err := Grant(`C:\x`, "", Read, Options{}); err == nil {
		t.Error("expected error for empty principal")
	}
}
