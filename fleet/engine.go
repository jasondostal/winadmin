package fleet

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Options tunes a fan-out run.
type Options struct {
	// Parallelism is the max number of targets in flight at once (the worker-pool cap).
	// Values < 1 mean 1 (sequential).
	Parallelism int

	// Timeout bounds each individual target. 0 means no per-target timeout.
	Timeout time.Duration

	// DryRun renders and logs the command for every target without executing it
	// — the --what-if you always wished you had before hitting 350 servers.
	DryRun bool

	// StopOnError cancels not-yet-started targets after the first failure.
	// Off by default: overnight fleet jobs usually want to push through.
	StopOnError bool

	// Logger receives a structured audit record per target. Defaults to
	// slog.Default(). This is the audit trail the old scripts never had.
	Logger *slog.Logger

	// OnStart, if set, fires just before a target's command executes. It is
	// called from worker goroutines (keep it cheap/thread-safe) and exists so a
	// live UI can show in-flight targets, not just completed ones.
	OnStart func(t Target)

	// Retries is the number of extra attempts per target on failure (transport
	// error or non-zero exit). 0 = a single attempt. Useful for flaky hosts.
	Retries int

	// RetryBackoff waits between attempts. 0 = retry immediately.
	RetryBackoff time.Duration
}

// Plan is the immutable description of a run: who, what, and how to reach them.
type Plan struct {
	Inventory *Inventory
	Task      Task
	Transport Transport
}

// OnResult is an optional callback fired as each target finishes (for live
// progress). It is called from multiple goroutines; keep it cheap and
// thread-safe, or nil it out.
type OnResult func(done, total int, r Result)

// Run fans Plan.Task across Plan.Inventory over Plan.Transport, honouring the
// concurrency cap. It always returns one Result per target, in inventory order,
// plus a Summary of counts and timing. It does not return an error: per-target failures
// live in the Results.
func Run(ctx context.Context, plan Plan, opts Options, onResult OnResult) (Summary, []Result) {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	targets := plan.Inventory.Targets
	results := make([]Result, len(targets))

	parallelism := opts.Parallelism
	if parallelism < 1 {
		parallelism = 1
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	started := time.Now()
	log.Info("fleet run starting",
		"targets", len(targets),
		"parallelism", parallelism,
		"task", plan.Task.Describe(),
		"transport", plan.Transport.Describe(),
		"dry_run", opts.DryRun,
	)

	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var doneMu sync.Mutex
	done := 0

	for i := range targets {
		// Acquire a slot BEFORE spawning: this is the worker-pool cap.
		sem <- struct{}{}

		// If we've been cancelled, mark the rest skipped without launching.
		if runCtx.Err() != nil {
			<-sem
			results[i] = Result{Target: targets[i].Name, Skipped: true}
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			r := runOne(runCtx, plan, targets[idx], opts, log)
			results[idx] = r

			if !r.OK() && !r.Skipped && opts.StopOnError {
				cancel()
			}

			doneMu.Lock()
			done++
			d := done
			doneMu.Unlock()
			if onResult != nil {
				onResult(d, len(targets), r)
			}
		}(i)
	}

	wg.Wait()
	finished := time.Now()

	summary := summarize(results, started, finished)
	log.Info("fleet run complete",
		"total", summary.Total,
		"succeeded", summary.Succeeded,
		"failed", summary.Failed,
		"skipped", summary.Skipped,
		"elapsed", summary.Elapsed().String(),
	)
	return summary, results
}

func runOne(ctx context.Context, plan Plan, t Target, opts Options, log *slog.Logger) Result {
	r := Result{Target: t.Name, Started: time.Now()}

	cmd, err := plan.Task.Command(t)
	if err != nil {
		r.Err = err
		r.Finished = time.Now()
		log.Error("task render failed", "target", t.Name, "err", err)
		return r
	}
	r.Command = cmd

	if opts.DryRun {
		r.DryRun = true
		r.Finished = time.Now()
		log.Info("would run", "target", t.Name, "command", cmd)
		return r
	}

	if opts.OnStart != nil {
		opts.OnStart(t)
	}

	attempts := opts.Retries + 1
	var outcome Outcome
	var execErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		execCtx := ctx
		var cancel context.CancelFunc
		if opts.Timeout > 0 {
			execCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		}
		outcome, execErr = plan.Transport.Exec(execCtx, t, cmd)
		if cancel != nil {
			cancel()
		}
		if execErr == nil && outcome.ExitCode == 0 {
			break // success
		}
		if attempt < attempts && ctx.Err() == nil {
			log.Warn("attempt failed, retrying", "target", t.Name, "attempt", attempt, "of", attempts)
			if opts.RetryBackoff > 0 {
				select {
				case <-time.After(opts.RetryBackoff):
				case <-ctx.Done():
				}
			}
		}
	}

	r.Stdout = outcome.Stdout
	r.Stderr = outcome.Stderr
	r.ExitCode = outcome.ExitCode
	r.Err = execErr
	r.Finished = time.Now()

	switch {
	case execErr != nil:
		log.Error("target failed", "target", t.Name, "command", cmd, "err", execErr, "dur", r.Duration().String())
	case r.ExitCode != 0:
		log.Warn("non-zero exit", "target", t.Name, "command", cmd, "exit", r.ExitCode, "dur", r.Duration().String())
	default:
		log.Info("target ok", "target", t.Name, "exit", 0, "dur", r.Duration().String())
	}
	return r
}
