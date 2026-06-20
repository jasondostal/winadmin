package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jasondostal/winadmin/fleet"
)

func TestRenderStatusBoard(t *testing.T) {
	now := time.Now()
	machines := []fleet.Machine{
		{Name: "web01", OS: "Windows Server 2022", LastStatus: "RUNNING", LastSeen: now.Add(-2 * time.Second)},
		{Name: "web02", OS: "Windows Server 2022", LastStatus: "RUNNING", LastSeen: now.Add(-3 * time.Second)},
		{Name: "ws-06", OS: "Windows 11", LastStatus: "UNREACHABLE"},
	}
	out := RenderStatusBoard(machines, now)

	for _, want := range []string{
		"fleet status", "2/3 agents up", "web01", "ws-06", "UNREACHABLE",
		"Windows 11", "Windows Server 2022",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status board missing %q:\n%s", want, out)
		}
	}
	// Problems sort to the top (btop-style): the UNREACHABLE box above the running ones.
	if strings.Index(out, "ws-06") > strings.Index(out, "web01") {
		t.Errorf("UNREACHABLE should sort above RUNNING:\n%s", out)
	}
}
