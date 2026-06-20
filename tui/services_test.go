package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServicesFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "services.txt")
	if err := os.WriteFile(f, []byte("# my fleet's services\nMySvc|My Service\nBareSvc\n\n; comment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLEET_SERVICES", f)

	got := loadServices()
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}
	if got[0] != (svcEntry{Short: "MySvc", Display: "My Service"}) {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if got[1] != (svcEntry{Short: "BareSvc", Display: "BareSvc"}) {
		t.Errorf("bare short should default Display to Short: %+v", got[1])
	}
}

func TestLoadServicesFallsBackToDefaults(t *testing.T) {
	t.Setenv("FLEET_SERVICES", filepath.Join(t.TempDir(), "does-not-exist.txt"))
	if got := loadServices(); len(got) != len(defaultServices) {
		t.Errorf("missing file should fall back to %d defaults, got %d", len(defaultServices), len(got))
	}

	// An empty/comment-only file also falls back rather than yielding nothing.
	empty := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(empty, []byte("# nothing here\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLEET_SERVICES", empty)
	if got := loadServices(); len(got) != len(defaultServices) {
		t.Errorf("empty file should fall back to defaults, got %d", len(got))
	}
}
