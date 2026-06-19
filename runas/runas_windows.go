//go:build windows

package runas

import (
	"unsafe"

	"github.com/jasondostal/winadmin/secret"
	"golang.org/x/sys/windows"
)

var (
	advapi32                    = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithLogonW = advapi32.NewProc("CreateProcessWithLogonW")
)

const (
	logonWithProfile = 0x00000001 // LOGON_WITH_PROFILE
	createUnicodeEnv = 0x00000400 // CREATE_UNICODE_ENVIRONMENT
)

func run(cred secret.Credential, opts Options) error {
	domain := cred.Domain
	if domain == "" {
		domain = "." // local machine
	}

	user, err := windows.UTF16PtrFromString(cred.User)
	if err != nil {
		return err
	}
	dom, err := windows.UTF16PtrFromString(domain)
	if err != nil {
		return err
	}
	pass, err := windows.UTF16PtrFromString(cred.Password)
	if err != nil {
		return err
	}
	cmd, err := windows.UTF16PtrFromString(opts.CommandLine)
	if err != nil {
		return err
	}
	var dirPtr *uint16
	if opts.WorkingDir != "" {
		dirPtr, err = windows.UTF16PtrFromString(opts.WorkingDir)
		if err != nil {
			return err
		}
	}

	var logonFlags uintptr
	if opts.LoadProfile {
		logonFlags = logonWithProfile
	}

	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation

	r, _, callErr := procCreateProcessWithLogonW.Call(
		uintptr(unsafe.Pointer(user)),
		uintptr(unsafe.Pointer(dom)),
		uintptr(unsafe.Pointer(pass)),
		logonFlags,
		0, // lpApplicationName (NULL -> parsed from command line)
		uintptr(unsafe.Pointer(cmd)),
		createUnicodeEnv,
		0, // lpEnvironment (NULL -> derived from the new user)
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r == 0 {
		return callErr
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)

	if opts.Wait {
		if _, err := windows.WaitForSingleObject(pi.Process, windows.INFINITE); err != nil {
			return err
		}
	}
	return nil
}
