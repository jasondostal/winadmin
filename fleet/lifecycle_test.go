package fleet

import (
	"testing"
	"time"
)

func TestNextClockTime(t *testing.T) {
	// Tuesday 2026-06-23 14:30:00 local.
	now := time.Date(2026, 6, 23, 14, 30, 0, 0, time.Local)

	// A time later today stays today.
	got, err := nextClockTime(now, "18:00")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hour() != 18 || got.Day() != 23 {
		t.Errorf("18:00 -> %v, want today 18:00", got)
	}

	// A time already past rolls to tomorrow.
	got, _ = nextClockTime(now, "09:00")
	if got.Day() != 24 || got.Hour() != 9 {
		t.Errorf("09:00 -> %v, want tomorrow 09:00", got)
	}

	// Seconds are honored.
	got, _ = nextClockTime(now, "14:30:45")
	if got.Second() != 45 || got.Day() != 23 {
		t.Errorf("14:30:45 -> %v, want today with :45", got)
	}
}

func TestNextClockTimeErrors(t *testing.T) {
	now := time.Date(2026, 6, 23, 14, 30, 0, 0, time.Local)
	for _, bad := range []string{"", "1400", "25:00", "12:60", "noon", "1:2:3:4"} {
		if _, err := nextClockTime(now, bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestLifecycleActiveAndRuns(t *testing.T) {
	if (LifecycleOptions{}).Active() {
		t.Error("zero value should be inactive (run once now)")
	}
	if (LifecycleOptions{}).TotalRuns() != 1 {
		t.Error("zero value should be one run")
	}
	if !(LifecycleOptions{Pre: "echo hi"}).Active() {
		t.Error("a pre-command is active")
	}
	if !(LifecycleOptions{Forever: true}).Active() {
		t.Error("forever is active")
	}
	if (LifecycleOptions{Loops: 5}).TotalRuns() != 5 {
		t.Error("Loops=5 -> 5 runs")
	}
}

func TestWaitForStartNoop(t *testing.T) {
	// No delay / no start time returns immediately.
	if err := (LifecycleOptions{}).WaitForStart(nil); err != nil {
		t.Fatalf("no-op wait should not error or block: %v", err)
	}
}

func TestStartTimeWithDelay(t *testing.T) {
	now := time.Date(2026, 6, 23, 14, 30, 0, 0, time.Local)
	got, err := (LifecycleOptions{Delay: 90 * time.Minute}).StartTime(now)
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(90 * time.Minute); !got.Equal(want) {
		t.Errorf("StartTime = %v, want %v", got, want)
	}
}
