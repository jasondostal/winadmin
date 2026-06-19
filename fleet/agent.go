package fleet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// The agent/pull model: instead of pushing from a control box, each machine runs
// `fleet agent`, which periodically PULLS a job (a command) from a shared source
// and runs it only when it has changed — the same version-gated convergence as
// a robocopy /MIR-style pull. Right for locked-down networks where nothing can
// connect inbound to the fleet.

// FetchJob reads the job command from source: an http(s) URL or a file path.
func FetchJob(ctx context.Context, source string) (string, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("agent: fetch %s: status %d", source, resp.StatusCode)
		}
		b, err := io.ReadAll(resp.Body)
		return strings.TrimSpace(string(b)), err
	}
	b, err := os.ReadFile(source)
	return strings.TrimSpace(string(b)), err
}

// JobVersion is the content hash used to decide whether the job changed.
func JobVersion(job string) string {
	sum := sha256.Sum256([]byte(job))
	return hex.EncodeToString(sum[:8])
}

// AgentResult reports one poll's outcome.
type AgentResult struct {
	Version string
	Ran     bool
	Outcome Outcome
	Err     error
}

// AgentPoll fetches the job, and if its version differs from lastVersion, runs it
// once via the local shell. Returns the (possibly unchanged) version so the caller
// can persist it across polls.
func AgentPoll(ctx context.Context, source, lastVersion string) AgentResult {
	job, err := FetchJob(ctx, source)
	if err != nil {
		return AgentResult{Err: err}
	}
	v := JobVersion(job)
	if v == lastVersion || job == "" {
		return AgentResult{Version: v, Ran: false}
	}
	out, execErr := LocalTransport{}.Exec(ctx, Target{Name: "localhost"}, job)
	return AgentResult{Version: v, Ran: true, Outcome: out, Err: execErr}
}

// ReadState / WriteState persist the last-run version between agent invocations
// (so a one-shot `fleet agent --once` from cron/Task Scheduler is idempotent).
func ReadState(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func WriteState(path, version string) error {
	return os.WriteFile(path, []byte(version), 0o644)
}
