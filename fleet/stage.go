package fleet

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// StageOptions describes a staged rollout — the layer that makes fleet ops
// humane instead of just powerful. Do a small canary, check health, and only
// then let the change ripple across the rest in waves.
type StageOptions struct {
	// Canary is how many targets to hit first, before anything else. 0 disables.
	Canary int

	// Wave is the batch size for the remaining targets after the canary. 0 means
	// "all remaining at once" (still gated by HealthCmd between canary and rest).
	Wave int

	// HealthCmd, if set, runs between batches; a non-zero exit aborts the rollout
	// and the not-yet-touched targets are marked skipped.
	HealthCmd string

	// Pause waits this long between batches (after any health check).
	Pause time.Duration
}

// Active reports whether any staging is configured.
func (s StageOptions) Active() bool {
	return s.Canary > 0 || s.Wave > 0 || s.HealthCmd != ""
}

// OnBatch is called when a batch is about to run (1-based index, total batches).
type OnBatch func(batchNum, totalBatches, size int, label string)

// RunWaves runs plan in batches: a canary, then Wave-sized waves, with a health
// gate between them. It returns the combined results, the summary, and a non-nil
// error if the rollout was aborted by a failed health check.
func RunWaves(ctx context.Context, plan Plan, opts Options, stage StageOptions, onBatch OnBatch, onResult OnResult) (Summary, []Result, error) {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	batches := splitBatches(plan.Inventory.Targets, stage.Canary, stage.Wave)
	var all []Result
	started := time.Now()

	for i, batch := range batches {
		label := fmt.Sprintf("wave %d", i+1)
		if stage.Canary > 0 && i == 0 {
			label = "canary"
		}
		if onBatch != nil {
			onBatch(i+1, len(batches), len(batch), label)
		}
		log.Info("rollout batch starting", "batch", i+1, "of", len(batches), "label", label, "size", len(batch))

		sub := plan
		sub.Inventory = &Inventory{Targets: batch}
		_, res := Run(ctx, sub, opts, onResult)
		all = append(all, res...)

		last := i == len(batches)-1
		if !last && stage.HealthCmd != "" {
			if err := runHealth(ctx, stage.HealthCmd); err != nil {
				log.Error("health gate failed — aborting rollout", "after_batch", i+1, "err", err)
				for _, b := range batches[i+1:] {
					for _, t := range b {
						all = append(all, Result{Target: t.Name, Skipped: true})
					}
				}
				return summarize(all, started, time.Now()), all, fmt.Errorf("rollout aborted: health check failed after %s: %w", label, err)
			}
			log.Info("health gate passed", "after_batch", i+1)
		}
		if !last && stage.Pause > 0 {
			select {
			case <-time.After(stage.Pause):
			case <-ctx.Done():
				return summarize(all, started, time.Now()), all, ctx.Err()
			}
		}
	}
	return summarize(all, started, time.Now()), all, nil
}

// splitBatches splits targets into a canary batch (size canary) followed by
// wave-sized batches of the remainder. wave <= 0 means the remainder is one batch.
func splitBatches(targets []Target, canary, wave int) [][]Target {
	var batches [][]Target
	rest := targets
	if canary > 0 && canary < len(rest) {
		batches = append(batches, rest[:canary])
		rest = rest[canary:]
	} else if canary > 0 {
		// canary covers everything
		return [][]Target{rest}
	}
	if wave <= 0 {
		if len(rest) > 0 {
			batches = append(batches, rest)
		}
		return batches
	}
	for len(rest) > 0 {
		n := wave
		if n > len(rest) {
			n = len(rest)
		}
		batches = append(batches, rest[:n])
		rest = rest[n:]
	}
	return batches
}

func runHealth(ctx context.Context, command string) error {
	shell, flag := localShell()
	cmd := exec.CommandContext(ctx, shell, flag, command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
