// Package reg is a thin, WOW64-aware wrapper over the Windows registry for the
// common sysadmin reads and writes that WinBatch did with RegQueryValue /
// RegSetValue / RegCreateKey / RegExistValue.
//
// The cross-platform surface (the Hive and View types, the function signatures)
// compiles everywhere; the actual work is Windows-only.
package reg

import "errors"

// ErrUnsupportedPlatform is returned by every function off Windows.
var ErrUnsupportedPlatform = errors.New("winadmin/reg: only supported on Windows")

// Hive selects a root key.
type Hive int

const (
	LocalMachine Hive = iota // HKEY_LOCAL_MACHINE  (@REGMACHINE)
	CurrentUser              // HKEY_CURRENT_USER   (@REGCURRENT)
	ClassesRoot              // HKEY_CLASSES_ROOT
	Users                    // HKEY_USERS
)

// View selects the registry redirection view on 64-bit Windows. Default picks
// the natural view for the running process; Force32/Force64 are the explicit
// WOW64 overrides that the old 32-bit WinBatch interpreter could not choose.
type View int

const (
	Default View = iota
	Force32
	Force64
)

// GetString reads a REG_SZ value. It returns ("", nil-wrapped not-exist) you can
// test with errors.Is(err, ErrNotExist).
func GetString(h Hive, path, name string, v View) (string, error) {
	return getString(h, path, name, v)
}

// SetString writes a REG_SZ value, creating the key if needed.
func SetString(h Hive, path, name, value string, v View) error {
	return setString(h, path, name, value, v)
}

// SetDWord writes a REG_DWORD value, creating the key if needed.
func SetDWord(h Hive, path, name string, value uint32, v View) error {
	return setDWord(h, path, name, value, v)
}

// Exists reports whether a value exists (the RegExistValue equivalent).
func Exists(h Hive, path, name string, v View) (bool, error) {
	return exists(h, path, name, v)
}

// ErrNotExist signals a missing key or value.
var ErrNotExist = errors.New("winadmin/reg: key or value does not exist")
