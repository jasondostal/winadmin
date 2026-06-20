package fleet

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")

	// Missing file loads empty.
	r, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Machines) != 0 {
		t.Fatalf("missing registry should be empty, got %d", len(r.Machines))
	}

	prov := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	r.Upsert(Machine{Name: "web02", OS: "windows", AgentVersion: "v0.3.0", ProvisionedAt: prov})
	r.Upsert(Machine{Name: "web01", OS: "windows", AgentVersion: "v0.3.0", ProvisionedAt: prov})
	if err := r.Save(path); err != nil {
		t.Fatal(err)
	}

	r2, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Machines) != 2 {
		t.Fatalf("got %d machines, want 2", len(r2.Machines))
	}
	// Saved sorted by name.
	if r2.Machines[0].Name != "web01" || r2.Machines[1].Name != "web02" {
		t.Errorf("registry not sorted: %v", r2.Names())
	}
}

func TestRegistryUpsertPreservesProvisionTime(t *testing.T) {
	prov := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	r := &Registry{}
	r.Upsert(Machine{Name: "dc1", ProvisionedAt: prov})

	// A status refresh (zero ProvisionedAt) must not wipe the original time.
	seen := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	r.Upsert(Machine{Name: "DC1", LastStatus: "RUNNING", LastSeen: seen})

	m, ok := r.Find("dc1")
	if !ok {
		t.Fatal("dc1 missing after upsert")
	}
	if !m.ProvisionedAt.Equal(prov) {
		t.Errorf("ProvisionedAt = %v, want preserved %v", m.ProvisionedAt, prov)
	}
	if m.LastStatus != "RUNNING" {
		t.Errorf("LastStatus = %q, want RUNNING", m.LastStatus)
	}
	if len(r.Machines) != 1 {
		t.Errorf("case-insensitive upsert should not duplicate, got %d", len(r.Machines))
	}
}

func TestRegistryRemoveAndInventory(t *testing.T) {
	r := &Registry{}
	r.Upsert(Machine{Name: "a"})
	r.Upsert(Machine{Name: "b"})
	if !r.Remove("A") {
		t.Error("Remove should be case-insensitive")
	}
	if inv := r.Inventory(); inv.Len() != 1 || inv.Targets[0].Name != "b" {
		t.Errorf("inventory after remove = %v", r.Names())
	}
}
