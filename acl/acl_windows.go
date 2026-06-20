//go:build windows

package acl

import (
	"fmt"
	"os/exec"
	"strings"
)

func grant(args []string) error  { return runICacls(args) }
func revoke(args []string) error { return runICacls(args) }

func runICacls(args []string) error {
	out, err := exec.Command("icacls", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("acl: icacls %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
