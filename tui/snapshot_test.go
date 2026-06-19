package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasondostal/winadmin/fleet"
	"github.com/muesli/termenv"
)

func seedInv(names ...string) *fleet.Inventory {
	inv := &fleet.Inventory{}
	for _, n := range names {
		inv.Targets = append(inv.Targets, fleet.Target{Name: n})
	}
	return inv
}

// TestSnapshotWatcher renders a representative mid-run frame so the dashboard
// layout can be eyeballed without a TTY.
func TestSnapshotWatcher(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	inv := seedInv("BR01-DC01", "BR02-DC01", "BR03-DC01", "BR04-DC01",
		"BR05-DC01", "BR06-DC01", "BR07-DC01", "BR08-DC01")
	plan := fleet.Plan{
		Inventory: inv,
		Task:      fleet.RegSetTask{Hive: "HKLM", Key: `SYSTEM\...\W32Time\Config`, Name: "MaxPollInterval", Data: "10"},
		Transport: fleet.LocalTransport{},
	}
	w := NewWatcher(plan, fleet.Options{Parallelism: 5}, fleet.StageOptions{})
	w.width, w.height = 92, 22
	w.progress.Width = 46
	w.start = time.Now().Add(-42 * time.Second)

	set := func(name string, st targetState, dur time.Duration, note string) {
		i := w.index[name]
		w.rows[i] = row{name: name, state: st, dur: dur, note: note}
	}
	set("BR01-DC01", stOK, 12*time.Millisecond, "ok")
	set("BR02-DC01", stOK, 9*time.Millisecond, "ok")
	set("BR03-DC01", stRunning, 0, "")
	set("BR04-DC01", stRunning, 0, "")
	set("BR05-DC01", stFail, 21*time.Millisecond, "exit 5")
	set("BR06-DC01", stOK, 15*time.Millisecond, "ok")
	set("BR07-DC01", stQueued, 0, "")
	set("BR08-DC01", stQueued, 0, "")
	w.done, w.okN, w.failN = 4, 3, 1

	out := w.View()
	if out == "" {
		t.Fatal("empty watcher view")
	}
	fmt.Printf("\n===== WATCHER (TUI #1) =====\n%s\n", out)
}

// TestSnapshotConsole renders the run-builder form.
func TestSnapshotConsole(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	c := NewConsole()
	c.width, c.height = 92, 30
	// Select the "svc" task + ssh transport so the verb-specific and ssh fields show.
	for i := range c.fields {
		switch c.fields[i].key {
		case "tasktype":
			c.fields[i].sel = 1 // svc
		case "transport":
			c.fields[i].sel = 1 // ssh
		}
	}
	c.focus = 0
	c.syncFocus()

	out := c.View()
	if out == "" {
		t.Fatal("empty console view")
	}
	fmt.Printf("\n===== CONSOLE — svc verb selected (TUI #2) =====\n%s\n", out)
}
