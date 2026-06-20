//go:build windows

package reboot

import (
	"github.com/jasondostal/winadmin/reg"
	"golang.org/x/sys/windows"
)

func replaceOnReboot(src, dst string) error {
	from, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	flags := uint32(windows.MOVEFILE_DELAY_UNTIL_REBOOT)
	var to *uint16
	if dst != "" {
		to, err = windows.UTF16PtrFromString(dst)
		if err != nil {
			return err
		}
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	return windows.MoveFileEx(from, to, flags)
}

// signal is one "reboot pending" marker and how to test for it.
type signal struct {
	reason string
	check  func() (bool, error)
}

func pendingReasons() ([]string, error) {
	checks := []signal{
		{"pending file rename operations", func() (bool, error) {
			return reg.Exists(reg.LocalMachine,
				`SYSTEM\CurrentControlSet\Control\Session Manager`, "PendingFileRenameOperations", reg.Force64)
		}},
		{"component based servicing", func() (bool, error) {
			return reg.KeyExists(reg.LocalMachine,
				`SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`, reg.Force64)
		}},
		{"windows update", func() (bool, error) {
			return reg.KeyExists(reg.LocalMachine,
				`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`, reg.Force64)
		}},
		{"pending domain join/rename", func() (bool, error) {
			return reg.Exists(reg.LocalMachine,
				`SYSTEM\CurrentControlSet\Services\Netlogon`, "JoinDomain", reg.Force64)
		}},
	}

	var reasons []string
	for _, c := range checks {
		ok, err := c.check()
		if err != nil {
			return nil, err
		}
		if ok {
			reasons = append(reasons, c.reason)
		}
	}
	return reasons, nil
}
