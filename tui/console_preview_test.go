package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConsolePreview drives the run builder through the preview flow headlessly
// (no TTY): set a target list + match, resolve, and render the preview screen.
func TestConsolePreview(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.txt")
	if err := os.WriteFile(hosts, []byte("web01\nweb02\ndb01\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewConsole()
	c.set("tasktype", "run")
	c.set("inventory", hosts)
	c.set("match", "web*")

	m, _ := c.preview()
	pc := m.(Console)
	if !pc.previewing {
		t.Fatal("expected previewing mode after preview()")
	}
	if len(pc.previewNames) != 2 {
		t.Fatalf("match web* should resolve 2 targets, got %d: %v", len(pc.previewNames), pc.previewNames)
	}

	view := pc.View()
	if !strings.Contains(view, "Preview — 2 target(s)") {
		t.Errorf("preview header missing:\n%s", view)
	}
	for _, want := range []string{"web01", "web02"} {
		if !strings.Contains(view, want) {
			t.Errorf("preview missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "db01") {
		t.Errorf("db01 should be filtered out by match:\n%s", view)
	}
}

// TestConsolePreviewNoMatch verifies the empty-result preview is informative
// rather than blank.
func TestConsolePreviewNoMatch(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.txt")
	if err := os.WriteFile(hosts, []byte("web01\nweb02\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewConsole()
	c.set("tasktype", "run")
	c.set("inventory", hosts)
	c.set("match", "nope*")

	m, _ := c.preview()
	view := m.(Console).View()
	if !strings.Contains(view, "Preview — 0 target(s)") || !strings.Contains(view, "No targets match") {
		t.Errorf("empty preview should say so:\n%s", view)
	}
}
