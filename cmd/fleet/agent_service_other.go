//go:build !windows

package main

import "time"

// runAsService is a no-op off Windows: there is no SCM, so the agent always runs
// its normal interactive/loop form.
func runAsService(string, time.Duration, func()) (bool, error) { return false, nil }
