package fleet

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// LifecycleOptions wraps a fan-out with the overnight-job ergonomics the old
// fleet-runners had: a control-host command before/after the run, repeats, and a
// delayed or clock-scheduled start. The zero value is "run once, right now".
type LifecycleOptions struct {
	// Pre and Post run once on the CONTROL host (not per target) — before and
	// after each run's fan-out. A non-zero exit aborts.
	Pre  string
	Post string

	// Loops is the total number of times to run the whole job. 0 or 1 means once;
	// N>1 repeats. See Forever for an unbounded loop.
	Loops int

	// Forever repeats the job until interrupted (overrides Loops).
	Forever bool

	// Delay waits this long before the first run.
	Delay time.Duration

	// StartAt holds the first run until a wall-clock time, "HH:MM" or "HH:MM:SS"
	// (today if still ahead, else tomorrow).
	StartAt string
}

// Active reports whether any lifecycle behavior is configured (so the plain
// single-run path can stay untouched when it is not).
func (l LifecycleOptions) Active() bool {
	return l.Pre != "" || l.Post != "" || l.Loops > 1 || l.Forever || l.Delay > 0 || l.StartAt != ""
}

// TotalRuns is the bounded run count (1 when not looping). Meaningless when
// Forever is set.
func (l LifecycleOptions) TotalRuns() int {
	if l.Loops < 1 {
		return 1
	}
	return l.Loops
}

// StartTime is the absolute instant the first run will begin, given now — for a
// UI that wants to show a countdown. Returns an error for a malformed StartAt.
func (l LifecycleOptions) StartTime(now time.Time) (time.Time, error) {
	target := now
	if l.StartAt != "" {
		t, err := nextClockTime(now, l.StartAt)
		if err != nil {
			return time.Time{}, err
		}
		target = t
	}
	return target.Add(l.Delay), nil
}

// WaitForStart blocks until the configured start time (Delay and/or StartAt),
// or until ctx is cancelled. A no-op when neither is set.
func (l LifecycleOptions) WaitForStart(ctx context.Context) error {
	return l.waitForStart(ctx, time.Now())
}

func (l LifecycleOptions) waitForStart(ctx context.Context, now time.Time) error {
	target, err := l.StartTime(now)
	if err != nil {
		return err
	}
	d := target.Sub(now)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunControlCommand runs a one-shot command on the control host through the
// local shell (so pipes and redirection work) — the Pre/Post bracketing command.
func RunControlCommand(ctx context.Context, command string) error {
	shell, flag := localShell()
	out, err := exec.CommandContext(ctx, shell, flag, command).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// nextClockTime resolves "HH:MM" / "HH:MM:SS" to the next instant matching it on
// or after now (today if still ahead, otherwise tomorrow).
func nextClockTime(now time.Time, clock string) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(clock), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, fmt.Errorf("fleet: bad start time %q (want HH:MM or HH:MM:SS)", clock)
	}
	var hms [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return time.Time{}, fmt.Errorf("fleet: bad start time %q: %w", clock, err)
		}
		hms[i] = n
	}
	h, m, s := hms[0], hms[1], hms[2]
	if h < 0 || h > 23 || m < 0 || m > 59 || s < 0 || s > 59 {
		return time.Time{}, fmt.Errorf("fleet: start time %q out of range", clock)
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, now.Location())
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target, nil
}
