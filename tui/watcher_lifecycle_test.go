package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jasondostal/winadmin/fleet"
)

// TestWatcherLifecycleRender drives the lifecycle event messages through the
// watcher headlessly (no TTY) and checks the dashboard reflects them: a start
// countdown, a loop badge with a board reset, and a pre/post phase note.
func TestWatcherLifecycleRender(t *testing.T) {
	inv := &fleet.Inventory{Targets: []fleet.Target{{Name: "alpha"}, {Name: "beta"}}}
	plan := fleet.Plan{
		Inventory: inv,
		Task:      fleet.CommandTask{Template: "echo {{.Name}}"},
		Transport: fleet.LocalTransport{},
	}
	w := NewWatcher(plan, fleet.Options{Parallelism: 2}, fleet.StageOptions{},
		fleet.LifecycleOptions{Loops: 3, Delay: time.Minute})
	w.width, w.height = 90, 20

	// Countdown banner while waiting for the scheduled start.
	m, _ := w.Update(waitMsg{until: time.Now().Add(90 * time.Second)})
	w = m.(Watcher)
	if !strings.Contains(w.View(), "starting at") {
		t.Errorf("expected a start countdown banner:\n%s", w.View())
	}

	// Mark a target running, then a new loop must reset the board and clear the wait.
	m, _ = w.Update(startMsg{name: "alpha"})
	w = m.(Watcher)
	m, _ = w.Update(resultMsg{r: fleet.Result{Target: "alpha", ExitCode: 0}})
	w = m.(Watcher)
	if w.done != 1 {
		t.Fatalf("expected done=1 before loop reset, got %d", w.done)
	}

	m, _ = w.Update(loopMsg{num: 2, total: 3})
	w = m.(Watcher)
	if w.done != 0 {
		t.Errorf("loop should reset the done counter, got %d", w.done)
	}
	if !w.waitUntil.IsZero() {
		t.Error("loop should clear the wait countdown")
	}
	if !strings.Contains(w.View(), "loop 2/3") {
		t.Errorf("expected a loop badge:\n%s", w.View())
	}

	// A pre/post phase note surfaces in the status line.
	m, _ = w.Update(phaseMsg{note: "running pre-command: echo hi"})
	w = m.(Watcher)
	if !strings.Contains(w.View(), "running pre-command") {
		t.Errorf("expected the phase note in the view:\n%s", w.View())
	}
}

// TestWatcherForeverBadge checks the infinite-loop badge renders.
func TestWatcherForeverBadge(t *testing.T) {
	inv := &fleet.Inventory{Targets: []fleet.Target{{Name: "a"}}}
	plan := fleet.Plan{Inventory: inv, Task: fleet.CommandTask{Template: "echo {{.Name}}"}, Transport: fleet.LocalTransport{}}
	w := NewWatcher(plan, fleet.Options{Parallelism: 1}, fleet.StageOptions{}, fleet.LifecycleOptions{Forever: true})
	w.width, w.height = 90, 20

	m, _ := w.Update(loopMsg{num: 7, total: 0})
	w = m.(Watcher)
	if !strings.Contains(w.View(), "loop 7/∞") {
		t.Errorf("expected an infinite loop badge:\n%s", w.View())
	}
}
