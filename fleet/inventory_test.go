package fleet

import (
	"context"
	"strings"
	"testing"
)

func TestInventoryFromLines(t *testing.T) {
	in := "dc01\n# a comment\n\n  dc02  \n; another\nDC01\n"
	inv, err := InventoryFromLines(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	// dc01 (deduped case-insensitively), dc02
	if inv.Len() != 2 {
		t.Fatalf("got %d targets, want 2: %+v", inv.Len(), inv.Targets)
	}
	if inv.Targets[0].Name != "dc01" || inv.Targets[1].Name != "dc02" {
		t.Fatalf("unexpected targets: %+v", inv.Targets)
	}
}

func TestInventoryFromCommand(t *testing.T) {
	// Simulates an ldapsearch | sed pipeline producing one DN per line.
	inv, err := InventoryFromCommand(context.Background(),
		`printf 'CN=Ann,OU=T,DC=corp\nCN=Bob,OU=T,DC=corp\n'`)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Len() != 2 || inv.Targets[0].Name != "CN=Ann,OU=T,DC=corp" {
		t.Fatalf("unexpected inventory: %+v", inv.Targets)
	}
}

func TestInventoryFromCommandError(t *testing.T) {
	if _, err := InventoryFromCommand(context.Background(), "exit 3"); err == nil {
		t.Fatal("expected error from failing inventory command")
	}
}

func TestInventoryMatch(t *testing.T) {
	inv := &Inventory{Targets: []Target{
		{Name: "web01.corp.com"}, {Name: "web02.corp.com"}, {Name: "db01.corp.com"}, {Name: "DC1"},
	}}
	if err := inv.Match([]string{"web*", " dc? "}); err != nil { // case-insensitive, trims
		t.Fatal(err)
	}
	if inv.Len() != 3 {
		t.Fatalf("got %d targets, want 3: %+v", inv.Len(), inv.Targets)
	}
	got := map[string]bool{}
	for _, tg := range inv.Targets {
		got[tg.Name] = true
	}
	for _, want := range []string{"web01.corp.com", "web02.corp.com", "DC1"} {
		if !got[want] {
			t.Errorf("expected %q kept", want)
		}
	}
	if got["db01.corp.com"] {
		t.Error("db01 should have been filtered out")
	}
}

func TestInventoryMatchEmptyIsNoop(t *testing.T) {
	inv := &Inventory{Targets: []Target{{Name: "a"}, {Name: "b"}}}
	if err := inv.Match([]string{"", "  "}); err != nil {
		t.Fatal(err)
	}
	if inv.Len() != 2 {
		t.Fatal("blank patterns should be a no-op")
	}
}

func TestInventoryMatchBadPattern(t *testing.T) {
	inv := &Inventory{Targets: []Target{{Name: "a"}}}
	if err := inv.Match([]string{"[bad"}); err == nil {
		t.Fatal("expected error for malformed pattern")
	}
}
