package fleet

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingTransport counts concurrency and lets us assert the pool cap.
type recordingTransport struct {
	mu       sync.Mutex
	inFlight int32
	maxSeen  int32
	commands []string
	delay    time.Duration
	failOn   map[string]bool
}

func (rt *recordingTransport) Describe() string { return "recording" }

func (rt *recordingTransport) Exec(ctx context.Context, t Target, command string) (Outcome, error) {
	n := atomic.AddInt32(&rt.inFlight, 1)
	for {
		old := atomic.LoadInt32(&rt.maxSeen)
		if n <= old || atomic.CompareAndSwapInt32(&rt.maxSeen, old, n) {
			break
		}
	}
	defer atomic.AddInt32(&rt.inFlight, -1)

	rt.mu.Lock()
	rt.commands = append(rt.commands, command)
	rt.mu.Unlock()

	if rt.delay > 0 {
		select {
		case <-time.After(rt.delay):
		case <-ctx.Done():
			return Outcome{ExitCode: -1}, ctx.Err()
		}
	}
	if rt.failOn[t.Name] {
		return Outcome{ExitCode: 1, Stderr: "boom"}, nil
	}
	return Outcome{ExitCode: 0, Stdout: "ok"}, nil
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func invOf(names ...string) *Inventory {
	inv := &Inventory{}
	for _, n := range names {
		inv.Targets = append(inv.Targets, Target{Name: n})
	}
	return inv
}

func TestRunAllSucceed(t *testing.T) {
	rt := &recordingTransport{}
	plan := Plan{Inventory: invOf("a", "b", "c", "d"), Task: CommandTask{Template: "echo {{.Name}}"}, Transport: rt}

	sum, results := Run(context.Background(), plan, Options{Parallelism: 2, Logger: quietLogger()}, nil)

	if sum.Total != 4 || sum.Succeeded != 4 || sum.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	if results[0].Command != "echo a" {
		t.Fatalf("template not rendered per target: %q", results[0].Command)
	}
}

func TestParallelismIsBounded(t *testing.T) {
	rt := &recordingTransport{delay: 20 * time.Millisecond}
	plan := Plan{Inventory: invOf("1", "2", "3", "4", "5", "6", "7", "8"), Task: CommandTask{Template: "x"}, Transport: rt}

	Run(context.Background(), plan, Options{Parallelism: 3, Logger: quietLogger()}, nil)

	if got := atomic.LoadInt32(&rt.maxSeen); got > 3 {
		t.Fatalf("pool cap violated: saw %d concurrent, limit was 3", got)
	}
}

func TestDryRunExecutesNothing(t *testing.T) {
	rt := &recordingTransport{}
	plan := Plan{Inventory: invOf("a", "b"), Task: CommandTask{Template: "echo {{.Name}}"}, Transport: rt}

	_, results := Run(context.Background(), plan, Options{DryRun: true, Logger: quietLogger()}, nil)

	rt.mu.Lock()
	executed := len(rt.commands)
	rt.mu.Unlock()
	if executed != 0 {
		t.Fatalf("dry-run executed %d commands, want 0", executed)
	}
	if !results[0].DryRun || results[0].Command != "echo a" {
		t.Fatalf("dry-run result missing command: %+v", results[0])
	}
}

func TestStopOnErrorSkipsRemainder(t *testing.T) {
	// First target fails; with Parallelism 1 the rest must be skipped.
	rt := &recordingTransport{failOn: map[string]bool{"a": true}}
	plan := Plan{Inventory: invOf("a", "b", "c", "d"), Task: CommandTask{Template: "x"}, Transport: rt}

	sum, results := Run(context.Background(), plan, Options{Parallelism: 1, StopOnError: true, Logger: quietLogger()}, nil)

	if sum.Failed != 1 {
		t.Fatalf("expected 1 failure, got %d (%+v)", sum.Failed, sum)
	}
	if sum.Skipped == 0 {
		t.Fatalf("expected some skipped targets, got %+v", sum)
	}
	if results[0].OK() {
		t.Fatal("results[0] (target a) should have failed")
	}
}

func TestOnResultCallbackFires(t *testing.T) {
	rt := &recordingTransport{}
	plan := Plan{Inventory: invOf("a", "b", "c"), Task: CommandTask{Template: "x"}, Transport: rt}

	var count int32
	Run(context.Background(), plan, Options{Parallelism: 2, Logger: quietLogger()},
		func(done, total int, r Result) { atomic.AddInt32(&count, 1) })

	if got := atomic.LoadInt32(&count); got != 3 {
		t.Fatalf("expected 3 callbacks, got %d", got)
	}
}

func TestRegSetTaskRendersRemote(t *testing.T) {
	task := RegSetTask{Hive: "HKLM", Key: `Software\Acme`, Name: "Enabled", Type: "REG_DWORD", Data: "1"}
	got, err := task.Command(Target{Name: "DC07"})
	if err != nil {
		t.Fatal(err)
	}
	want := `reg add "\\DC07\HKLM\Software\Acme" /v "Enabled" /t REG_DWORD /d "1" /f`
	if got != want {
		t.Fatalf("got:  %s\nwant: %s", got, want)
	}
}

func TestRegSetTaskLocal(t *testing.T) {
	task := RegSetTask{Hive: "HKLM", Key: `Software\Acme`, Name: "Enabled", Type: "REG_DWORD", Data: "1", Local: true}
	got, _ := task.Command(Target{Name: "DC07"})
	want := `reg add "HKLM\Software\Acme" /v "Enabled" /t REG_DWORD /d "1" /f`
	if got != want {
		t.Fatalf("got:  %s\nwant: %s", got, want)
	}
}

func TestDeleteDirTaskRendersRemote(t *testing.T) {
	task := DeleteDirTask{Path: `C$\Temp\junk`}
	got, _ := task.Command(Target{Name: "BR12FS"})
	want := `cmd /c rd /s /q "\\BR12FS\C$\Temp\junk"`
	if got != want {
		t.Fatalf("got:  %s\nwant: %s", got, want)
	}
}

func TestCommandTaskBadTemplate(t *testing.T) {
	if _, err := (CommandTask{Template: "{{.Nope}}"}).Command(Target{Name: "x"}); err == nil {
		t.Fatal("expected error for unknown template field")
	}
}

func TestLocalTransportRunsAndCaptures(t *testing.T) {
	// LocalTransport is cross-platform via the shell; echo works everywhere.
	out, err := LocalTransport{}.Exec(context.Background(), Target{Name: "x"}, "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit %d, want 0", out.ExitCode)
	}
	if got := fmt.Sprintf("%s", out.Stdout); len(got) == 0 {
		t.Fatal("expected stdout from echo")
	}
}

func TestInventoryExclude(t *testing.T) {
	inv := invOf("dc01", "dc02", "dc03")
	inv.Exclude([]string{"DC02"}) // case-insensitive
	if inv.Len() != 2 {
		t.Fatalf("expected 2 after exclude, got %d", inv.Len())
	}
	for _, tg := range inv.Targets {
		if tg.Name == "dc02" {
			t.Fatal("dc02 should have been excluded")
		}
	}
}
