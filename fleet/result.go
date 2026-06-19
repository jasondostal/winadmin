package fleet

import "time"

// Result is the outcome of running a task against one target.
type Result struct {
	Target   string
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error // transport-level failure (couldn't reach/start); nil on a clean run even if ExitCode != 0
	Skipped  bool  // run was cancelled before this target started (e.g. StopOnError tripped)
	DryRun   bool
	Started  time.Time
	Finished time.Time
}

// OK reports whether the task ran and the command succeeded.
func (r Result) OK() bool {
	return !r.Skipped && r.Err == nil && r.ExitCode == 0
}

// Duration is the wall-clock time the target took.
func (r Result) Duration() time.Duration {
	if r.Started.IsZero() || r.Finished.IsZero() {
		return 0
	}
	return r.Finished.Sub(r.Started)
}

// Summary is the completion screen: counts and timing across the whole run.
type Summary struct {
	Total     int
	Succeeded int
	Failed    int
	Skipped   int
	Started   time.Time
	Finished  time.Time
}

// Elapsed is total wall-clock time for the run.
func (s Summary) Elapsed() time.Duration {
	if s.Started.IsZero() || s.Finished.IsZero() {
		return 0
	}
	return s.Finished.Sub(s.Started)
}

func summarize(results []Result, started, finished time.Time) Summary {
	s := Summary{Total: len(results), Started: started, Finished: finished}
	for _, r := range results {
		switch {
		case r.Skipped:
			s.Skipped++
		case r.OK():
			s.Succeeded++
		default:
			s.Failed++
		}
	}
	return s
}
