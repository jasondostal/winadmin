package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasondostal/winadmin/fleet"
)

// TestStatusBoardPoll drives a poll result through the model and checks the board
// reflects the new states (headless — no TTY, no real fleet).
func TestStatusBoardPoll(t *testing.T) {
	reg := &fleet.Registry{Machines: []fleet.Machine{{Name: "web01"}, {Name: "web02"}}}
	b := StatusBoard{
		machines:     reg.Machines,
		reg:          reg,
		registryPath: filepath.Join(t.TempDir(), "reg.json"),
		every:        time.Second,
		polling:      true,
	}

	m, _ := b.Update(sbPollMsg{states: map[string]string{"web01": "RUNNING", "web02": "STOPPED"}})
	bb := m.(StatusBoard)

	if bb.machines[0].LastStatus != "RUNNING" || bb.machines[1].LastStatus != "STOPPED" {
		t.Fatalf("poll states not applied: %+v", bb.machines)
	}
	if bb.polling {
		t.Error("polling flag should clear after a poll result")
	}
	v := bb.View()
	for _, want := range []string{"RUNNING", "STOPPED", "[q]", "[r]"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}
