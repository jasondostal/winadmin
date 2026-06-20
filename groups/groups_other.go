//go:build !windows

package groups

func currentUserInGroup(string) (bool, error) { return false, ErrUnsupportedPlatform }
func currentUserGroups() ([]string, error)    { return nil, ErrUnsupportedPlatform }
