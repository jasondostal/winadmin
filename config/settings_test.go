package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("FLEET_CONFIG", path)

	// A missing file loads as the zero value, not an error.
	got, err := LoadSettings()
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if !reflect.DeepEqual(got, Settings{}) {
		t.Errorf("missing file should be zero Settings, got %+v", got)
	}

	want := Settings{Parallelism: 25, Transport: "ssh", SSHUser: "ec2-user", DefaultHosts: "/etc/fleet/hosts.txt"}
	wrote, err := SaveSettings(want)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if wrote != path {
		t.Errorf("saved to %q, want %q", wrote, path)
	}

	got, err = LoadSettings()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}

func TestGatherLibraryFallback(t *testing.T) {
	// No configured queries → the built-in defaults back the picker.
	if got := (Settings{}).GatherLibrary(); len(got) == 0 {
		t.Fatal("empty settings should fall back to DefaultGatherQueries")
	}
	custom := []NamedQuery{{Name: "mine", Command: "echo hi"}}
	got := Settings{GatherQueries: custom}.GatherLibrary()
	if !reflect.DeepEqual(got, custom) {
		t.Errorf("configured queries should win, got %+v", got)
	}
}
