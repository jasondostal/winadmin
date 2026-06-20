// Package fleet is a parallel fan-out engine for running a task across a list of
// machines — inspired by the classic overnight fleet-runners of the early-2000s
// Windows admin era, which fired a batch file at hundreds of servers with
// bounded concurrency.
//
// The shape is unchanged; the primitives are just better:
//
//	INVENTORY  ->  ENGINE (bounded pool)  ->  TRANSPORT  ->  N machines
//	who to hit     worker-pool cap          how to reach    do the thing
//
// The worker-pool cap is a semaphore; the target list is an Inventory; the work
// is a Task; "shell out per machine" is a Transport.
package fleet

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
)

// Target is a single item to act on. Name is whatever appeared in the list — a
// hostname, an IP, but just as well a folder, a username, any token — and is what
// tasks template against ({{.Name}}). A list row can carry columns: positional
// {{.F1}}, {{.F2}}, … (split from the row) and, when named, {{.<name>}}.
type Target struct {
	Name   string
	Fields []string          // positional columns split from the row (→ {{.F1}}…)
	Cols   map[string]string // named columns (→ {{.<name>}})
}

// templateData is the value a command template renders against: Name, the
// positional columns F1…Fn (whitespace-split from Name by default), and any
// named columns. A map (not the struct) so {{.F1}}/{{.user}} resolve dynamically.
func (t Target) templateData() map[string]any {
	fields := t.Fields
	if fields == nil {
		fields = strings.Fields(t.Name)
	}
	m := map[string]any{"Name": t.Name}
	for i, f := range fields {
		m[fmt.Sprintf("F%d", i+1)] = f
	}
	for k, v := range t.Cols {
		m[k] = v
	}
	return m
}

// SplitFields parses each row's columns for templating: split on delim (empty =
// whitespace), and — when cols is non-empty — bind those names to the columns in
// order, so {{.<name>}} works. The whole row remains {{.Name}}.
func (inv *Inventory) SplitFields(delim string, cols []string) {
	for i := range inv.Targets {
		var fields []string
		if delim == "" {
			fields = strings.Fields(inv.Targets[i].Name)
		} else {
			for _, p := range strings.Split(inv.Targets[i].Name, delim) {
				fields = append(fields, strings.TrimSpace(p))
			}
		}
		inv.Targets[i].Fields = fields
		if len(cols) > 0 {
			m := make(map[string]string, len(cols))
			for j, c := range cols {
				if j < len(fields) {
					m[strings.TrimSpace(c)] = fields[j]
				}
			}
			inv.Targets[i].Cols = m
		}
	}
}

// Inventory is an ordered set of targets.
type Inventory struct {
	Targets []Target
}

// LoadInventory reads a target list file: one machine per line. Blank lines and
// lines beginning with '#' or ';' (shell/INI comment style) are ignored, as is
// trailing whitespace. This is the modern /L: target list.
func LoadInventory(path string) (*Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var inv Inventory
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key := strings.ToLower(line)
		if seen[key] {
			continue // de-dup; nobody wants to hit a box twice
		}
		seen[key] = true
		inv.Targets = append(inv.Targets, Target{Name: line})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return &inv, nil
}

// InventoryFromLines builds an inventory from the lines of a reader, using the
// same rules as LoadInventory (trim, skip blank / '#' / ';' lines, de-dup).
func InventoryFromLines(r io.Reader) (*Inventory, error) {
	var inv Inventory
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20) // tolerate long lines (e.g. full DNs)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key := strings.ToLower(line)
		if seen[key] {
			continue
		}
		seen[key] = true
		inv.Targets = append(inv.Targets, Target{Name: line})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return &inv, nil
}

// InventoryFromCommand runs a shell command and builds an inventory from its
// stdout, one target per line. This is dynamic inventory: instead of a static
// host list, the fleet is whatever a query returns *right now* — e.g. every
// computer in an AD group, every instance from a cloud API, or every user DN in
// an OU:
//
//	ldapsearch -LLL -o ldif-wrap=no -b "OU=Tellers,DC=corp,DC=com" \
//	  "(objectClass=user)" dn | sed -n 's/^dn: //p'
//
// The command runs through the local shell, so pipes and redirection work.
func InventoryFromCommand(ctx context.Context, shellCommand string) (*Inventory, error) {
	shell, flag := localShell()
	cmd := exec.CommandContext(ctx, shell, flag, shellCommand)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("fleet: inventory command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return InventoryFromLines(&stdout)
}

// Exclude removes any targets whose name matches (case-insensitively) an entry
// in names — the exclude list every fleet runner eventually grows.
func (inv *Inventory) Exclude(names []string) {
	if len(names) == 0 {
		return
	}
	drop := make(map[string]bool, len(names))
	for _, n := range names {
		drop[strings.ToLower(strings.TrimSpace(n))] = true
	}
	kept := inv.Targets[:0]
	for _, t := range inv.Targets {
		if !drop[strings.ToLower(t.Name)] {
			kept = append(kept, t)
		}
	}
	inv.Targets = kept
}

// ExcludeFromFile excludes targets listed in an exclusion file (same format as
// LoadInventory).
func (inv *Inventory) ExcludeFromFile(path string) error {
	ex, err := LoadInventory(path)
	if err != nil {
		return err
	}
	names := make([]string, len(ex.Targets))
	for i, t := range ex.Targets {
		names[i] = t.Name
	}
	inv.Exclude(names)
	return nil
}

// Match keeps only targets whose name matches at least one of the glob patterns
// (case-insensitive; '*' and '?' wildcards, via path.Match). An empty/blank
// pattern set is a no-op. A malformed pattern returns an error. This is the
// "filter the list" knob — keep just the web boxes, just one site's DCs — the
// list-filtering wish the old fleet-runner never shipped.
func (inv *Inventory) Match(patterns []string) error {
	var globs []string
	for _, p := range patterns {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			globs = append(globs, p)
		}
	}
	if len(globs) == 0 {
		return nil
	}
	for _, g := range globs {
		if _, err := path.Match(g, ""); err != nil {
			return fmt.Errorf("fleet: bad match pattern %q: %w", g, err)
		}
	}
	kept := inv.Targets[:0]
	for _, t := range inv.Targets {
		name := strings.ToLower(t.Name)
		for _, g := range globs {
			if ok, _ := path.Match(g, name); ok {
				kept = append(kept, t)
				break
			}
		}
	}
	inv.Targets = kept
	return nil
}

// Shuffle reorders targets using the provided swap source — randomizing the list
// so a run doesn't hammer servers in list-order. Pass a shuffle func (e.g.
// math/rand's Shuffle); kept dependency-free so the caller owns the randomness.
func (inv *Inventory) Shuffle(shuffle func(n int, swap func(i, j int))) {
	shuffle(len(inv.Targets), func(i, j int) {
		inv.Targets[i], inv.Targets[j] = inv.Targets[j], inv.Targets[i]
	})
}

// Len reports the number of targets.
func (inv *Inventory) Len() int { return len(inv.Targets) }
