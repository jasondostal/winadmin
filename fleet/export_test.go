package fleet

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleResults() []Result {
	t0 := time.Unix(0, 0)
	return []Result{
		{Target: "web01", Command: "uptime", Stdout: "up 3 days\n", ExitCode: 0, Started: t0, Finished: t0.Add(time.Second)},
		{Target: "web02", Command: "uptime", Stderr: "boom\n", ExitCode: 1, Started: t0, Finished: t0.Add(2 * time.Second)},
	}
}

func TestExportResultsJSONAndCSV(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".json", ".csv"} {
		path := filepath.Join(dir, "out"+ext)
		if err := ExportResults(sampleResults(), path); err != nil {
			t.Fatalf("ExportResults(%s): %v", ext, err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out := string(b)
		if !strings.Contains(out, "web01") || !strings.Contains(out, "web02") {
			t.Errorf("%s export missing targets:\n%s", ext, out)
		}
	}
}

func TestFormatGather(t *testing.T) {
	if got := FormatGather(sampleResults(), "table"); !strings.Contains(got, "web01") || !strings.Contains(got, "up 3 days") {
		t.Errorf("table gather:\n%s", got)
	}
	if got := FormatGather(sampleResults(), "csv"); !strings.HasPrefix(got, "target,exit,output,error") {
		t.Errorf("csv gather header:\n%s", got)
	}
	if got := FormatGather(sampleResults(), "json"); !strings.Contains(got, `"target": "web01"`) {
		t.Errorf("json gather:\n%s", got)
	}
}

func TestResolveInventory(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "hosts.txt")
	if err := os.WriteFile(list, []byte("a\nb\n# skip\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := ResolveInventory(context.Background(), "file:"+list)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Len() != 2 { // de-duped, comment skipped
		t.Errorf("file: inventory len = %d, want 2", inv.Len())
	}
	if _, err := ResolveInventory(context.Background(), "bogus:x"); err == nil {
		t.Error("expected error for unknown inventory spec")
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("a'b"); got != `'a'\''b'` {
		t.Errorf("ShellQuote = %q", got)
	}
}
