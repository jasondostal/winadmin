//go:build !windows

package groups

func currentUserInGroup(string) (bool, error) { return false, ErrUnsupportedPlatform }
