// Package reboot covers two Windows admin needs the old install scripts handled
// with a delayed-move primitive and a scatter of registry probes:
//
//   - Scheduling a file to be replaced or deleted at the next boot, for files
//     that are locked/in-use right now. This is MoveFileEx with
//     MOVEFILE_DELAY_UNTIL_REBOOT.
//   - Detecting whether the machine is already waiting on a reboot, so a rollout
//     can hold off (or a health gate can fail) instead of stacking changes on a
//     box that is half-patched.
//
// Both are Windows-only; off Windows every function returns ErrUnsupportedPlatform.
package reboot

import "errors"

// ErrUnsupportedPlatform is returned off Windows.
var ErrUnsupportedPlatform = errors.New("winadmin/reboot: only supported on Windows")

// ReplaceOnReboot schedules src to take the place of dst at the next reboot —
// the way to update a file that is locked and in use now. dst is replaced if it
// already exists. If dst is empty, src is scheduled for deletion instead (see
// DeleteOnReboot).
func ReplaceOnReboot(src, dst string) error { return replaceOnReboot(src, dst) }

// DeleteOnReboot schedules path for deletion at the next reboot — for a locked
// file you cannot remove now.
func DeleteOnReboot(path string) error { return replaceOnReboot(path, "") }

// Pending reports whether any "a reboot is pending" signal is set on the machine.
func Pending() (bool, error) {
	reasons, err := pendingReasons()
	return len(reasons) > 0, err
}

// PendingReasons returns the specific reboot-pending signals that are set (empty
// when none are) — e.g. "pending file rename operations", "windows update".
func PendingReasons() ([]string, error) { return pendingReasons() }
