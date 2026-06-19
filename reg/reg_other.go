//go:build !windows

package reg

func getString(Hive, string, string, View) (string, error) { return "", ErrUnsupportedPlatform }
func setString(Hive, string, string, string, View) error   { return ErrUnsupportedPlatform }
func setDWord(Hive, string, string, uint32, View) error    { return ErrUnsupportedPlatform }
func exists(Hive, string, string, View) (bool, error)      { return false, ErrUnsupportedPlatform }
