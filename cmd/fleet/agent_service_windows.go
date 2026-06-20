//go:build windows

package main

import (
	"time"

	"golang.org/x/sys/windows/svc"
)

// runAsService runs the agent poll loop under the Windows Service Control
// Manager when the process was started as a service (so `sc create` yields a
// real, SCM-managed service). It returns (true, err) when it handled the service
// lifecycle, or (false, nil) when not running as a service — in which case the
// caller runs the normal interactive loop.
func runAsService(name string, interval time.Duration, poll func()) (bool, error) {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return false, err
	}
	if !isSvc {
		return false, nil
	}
	return true, svc.Run(name, &agentService{interval: interval, poll: poll})
}

type agentService struct {
	interval time.Duration
	poll     func()
}

// Execute is the SCM entry point: report Running, poll on the interval, and stop
// cleanly on Stop/Shutdown.
func (a *agentService) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}
	status <- svc.Status{State: svc.Running, Accepts: accepted}

	interval := a.interval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	a.poll() // run once immediately on start

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.poll()
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				return false, 0
			default:
			}
		}
	}
}
