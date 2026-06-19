package fleet

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPsexecCmd(t *testing.T) {
	got := psexecCmd("psexec", "WS01", "CORP\\adm", "p@ss", "ipconfig")
	want := `psexec \\WS01 -u CORP\adm -p p@ss -accepteula -nobanner cmd /c "ipconfig"`
	if got != want {
		t.Fatalf("got:  %s\nwant: %s", got, want)
	}
	// no creds when user empty
	nc := psexecCmd("psexec", "WS01", "", "", "hostname")
	if strings.Contains(nc, "-u ") {
		t.Fatalf("unexpected creds: %s", nc)
	}
}

func TestWmiCmd(t *testing.T) {
	got := wmiCmd("WS01", "adm", "pw", "ipconfig")
	want := `wmic /node:"WS01" /user:"adm" /password:"pw" process call create "ipconfig"`
	if got != want {
		t.Fatalf("got:  %s\nwant: %s", got, want)
	}
}

func TestWinQuoteEscapes(t *testing.T) {
	if got := winQuote(`echo "hi"`); got != `"echo \"hi\""` {
		t.Fatalf("got %s", got)
	}
}

func TestAgentJobVersionStable(t *testing.T) {
	a := JobVersion("echo hi")
	b := JobVersion("echo hi")
	c := JobVersion("echo bye")
	if a != b {
		t.Fatal("same job hashed differently")
	}
	if a == c {
		t.Fatal("different jobs hashed the same")
	}
}

func TestAgentPollGatesOnVersion(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "job.sh")
	if err := os.WriteFile(jobFile, []byte("echo agent-ran"), 0o644); err != nil {
		t.Fatal(err)
	}
	// first poll with empty lastVersion -> runs
	r1 := AgentPoll(context.Background(), jobFile, "")
	if !r1.Ran || r1.Err != nil {
		t.Fatalf("expected run, got %+v", r1)
	}
	// second poll with the same version -> does not run
	r2 := AgentPoll(context.Background(), jobFile, r1.Version)
	if r2.Ran {
		t.Fatalf("expected gated (no run), got %+v", r2)
	}
}

func TestFetchJobFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "j")
	_ = os.WriteFile(f, []byte("  do-thing  \n"), 0o644)
	got, err := FetchJob(context.Background(), f)
	if err != nil || got != "do-thing" {
		t.Fatalf("got %q err %v", got, err)
	}
}
