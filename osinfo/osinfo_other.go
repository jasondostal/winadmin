//go:build !windows

package osinfo

func detect() (Info, error) { return Info{}, ErrUnsupportedPlatform }
