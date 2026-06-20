package fleet

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// This file is the reporting side of the engine: turning a []Result into the
// artifacts an admin actually hands off — a .json/.csv export stapled to a
// change ticket, or a tabulated gather report. It used to live in the CLI; it
// belongs to anyone building on the engine.

// ResultRow is the flattened, serialization-friendly projection of a Result.
type ResultRow struct {
	Target   string `json:"target"`
	OK       bool   `json:"ok"`
	Exit     int    `json:"exit"`
	Skipped  bool   `json:"skipped"`
	Command  string `json:"command,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Err      string `json:"error,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// Rows projects results into ResultRows.
func Rows(results []Result) []ResultRow {
	rows := make([]ResultRow, len(results))
	for i, r := range results {
		errStr := ""
		if r.Err != nil {
			errStr = r.Err.Error()
		}
		rows[i] = ResultRow{
			Target: r.Target, OK: r.OK(), Exit: r.ExitCode, Skipped: r.Skipped,
			Command: r.Command, Stdout: strings.TrimSpace(r.Stdout), Stderr: strings.TrimSpace(r.Stderr),
			Err: errStr, Duration: r.Duration().String(),
		}
	}
	return rows
}

// ExportResults writes the full per-target result set to path. The format is
// chosen by the file extension (.csv → CSV, anything else → JSON) — the artifact
// you staple to the change ticket.
func ExportResults(results []Result, path string) error {
	rows := Rows(results)
	if strings.HasSuffix(strings.ToLower(path), ".csv") {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return writeResultCSV(f, rows)
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeResultCSV(w io.Writer, rows []ResultRow) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"target", "ok", "exit", "skipped", "command", "stdout", "stderr", "error", "duration"})
	for _, r := range rows {
		_ = cw.Write([]string{r.Target, fmt.Sprintf("%t", r.OK), fmt.Sprintf("%d", r.Exit),
			fmt.Sprintf("%t", r.Skipped), r.Command, oneLine(r.Stdout), oneLine(r.Stderr), r.Err, r.Duration})
	}
	cw.Flush()
	return cw.Error()
}

// FormatGather renders results as a "table", "csv", or "json" report — the read
// side that turns a fan-out into a fleet report. Unknown formats fall back to
// the table.
func FormatGather(results []Result, format string) string {
	type row struct {
		Target string `json:"target"`
		Exit   int    `json:"exit"`
		Output string `json:"output"`
		Err    string `json:"error,omitempty"`
	}
	rows := make([]row, 0, len(results))
	for _, r := range results {
		errStr := ""
		if r.Err != nil {
			errStr = r.Err.Error()
		}
		rows = append(rows, row{Target: r.Target, Exit: r.ExitCode, Output: oneLine(r.Stdout), Err: errStr})
	}

	switch format {
	case "json":
		b, _ := json.MarshalIndent(rows, "", "  ")
		return string(b)
	case "csv":
		var sb strings.Builder
		cw := csv.NewWriter(&sb)
		_ = cw.Write([]string{"target", "exit", "output", "error"})
		for _, r := range rows {
			_ = cw.Write([]string{r.Target, fmt.Sprintf("%d", r.Exit), r.Output, r.Err})
		}
		cw.Flush()
		return sb.String()
	default: // table
		width := 0
		for _, r := range rows {
			if len(r.Target) > width {
				width = len(r.Target)
			}
		}
		var sb strings.Builder
		for _, r := range rows {
			out := r.Output
			if r.Err != "" {
				out = "ERROR: " + r.Err
			}
			fmt.Fprintf(&sb, "%-*s  %s\n", width, r.Target, out)
		}
		return sb.String()
	}
}

// NonEmptyLines splits s on newlines, trims trailing CRs, and drops blank lines.
func NonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimRight(line, "\r"))
		}
	}
	return out
}

func oneLine(s string) string { return strings.Join(NonEmptyLines(s), " ") }
