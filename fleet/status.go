package fleet

import "strings"

// ServiceState extracts the Windows service state from an `sc query` Result —
// "RUNNING", "STOPPED", etc. A transport-level failure reads as "UNREACHABLE";
// a clean run with no STATE line and a non-zero exit reads as "NOT-INSTALLED".
// Shared by the CLI status command and the TUI status board so they agree.
func ServiceState(r Result) string {
	if r.Err != nil {
		return "UNREACHABLE"
	}
	for _, line := range NonEmptyLines(r.Stdout) {
		if strings.Contains(line, "STATE") {
			f := strings.Fields(line)
			return f[len(f)-1]
		}
	}
	if r.ExitCode != 0 {
		return "NOT-INSTALLED"
	}
	return "UNKNOWN"
}
