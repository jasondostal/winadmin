//go:build !windows

package reboot

func replaceOnReboot(string, string) error { return ErrUnsupportedPlatform }
func pendingReasons() ([]string, error)    { return nil, ErrUnsupportedPlatform }
