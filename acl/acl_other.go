//go:build !windows

package acl

func grant([]string) error  { return ErrUnsupportedPlatform }
func revoke([]string) error { return ErrUnsupportedPlatform }
