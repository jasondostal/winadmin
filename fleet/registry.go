package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// The fleet registry is the file-based record of machines we've provisioned and
// own — "who has the agent installed, what version, when, and what we last saw."
// It is deliberately a JSON FILE, not a database: keep it local, or point it at a
// path on a network share to make it fleet-wide. Provisioning writes it; the
// status board reads it and stamps LastStatus/LastSeen.

// Machine is one provisioned box.
type Machine struct {
	Name          string    `json:"name"`
	OS            string    `json:"os,omitempty"` // "windows" | "linux"
	AgentVersion  string    `json:"agent_version,omitempty"`
	ProvisionedAt time.Time `json:"provisioned_at,omitempty"`
	LastStatus    string    `json:"last_status,omitempty"` // free-form, set by the status board
	LastSeen      time.Time `json:"last_seen,omitempty"`
}

// Registry is a set of provisioned machines persisted as JSON.
type Registry struct {
	Machines []Machine `json:"machines"`
}

// LoadRegistry reads the registry file. A missing file is not an error — it
// returns an empty registry, so the first provision run can create it.
func LoadRegistry(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, err
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("fleet: parse registry %s: %w", path, err)
	}
	return &r, nil
}

// Save writes the registry to path, sorted by name for stable diffs.
func (r *Registry) Save(path string) error {
	sort.Slice(r.Machines, func(i, j int) bool {
		return strings.ToLower(r.Machines[i].Name) < strings.ToLower(r.Machines[j].Name)
	})
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Upsert adds m, or updates the existing machine with the same name
// (case-insensitive). A zero ProvisionedAt preserves the existing one, so a
// status-board refresh doesn't wipe the original provision time.
func (r *Registry) Upsert(m Machine) {
	for i := range r.Machines {
		if strings.EqualFold(r.Machines[i].Name, m.Name) {
			if m.ProvisionedAt.IsZero() {
				m.ProvisionedAt = r.Machines[i].ProvisionedAt
			}
			r.Machines[i] = m
			return
		}
	}
	r.Machines = append(r.Machines, m)
}

// Find returns the machine with the given name (case-insensitive) and whether it
// was present.
func (r *Registry) Find(name string) (Machine, bool) {
	for _, m := range r.Machines {
		if strings.EqualFold(m.Name, name) {
			return m, true
		}
	}
	return Machine{}, false
}

// Remove deletes a machine by name (case-insensitive); reports whether it existed.
func (r *Registry) Remove(name string) bool {
	for i := range r.Machines {
		if strings.EqualFold(r.Machines[i].Name, name) {
			r.Machines = append(r.Machines[:i], r.Machines[i+1:]...)
			return true
		}
	}
	return false
}

// Names returns the machine names, in stored order.
func (r *Registry) Names() []string {
	out := make([]string, len(r.Machines))
	for i, m := range r.Machines {
		out[i] = m.Name
	}
	return out
}

// Inventory builds a fleet Inventory from the registry — so the status board can
// poll exactly the machines we own.
func (r *Registry) Inventory() *Inventory {
	inv := &Inventory{Targets: make([]Target, len(r.Machines))}
	for i, m := range r.Machines {
		inv.Targets[i] = Target{Name: m.Name}
	}
	return inv
}
