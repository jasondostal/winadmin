//go:build windows

package reg

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

func hiveKey(h Hive) registry.Key {
	switch h {
	case CurrentUser:
		return registry.CURRENT_USER
	case ClassesRoot:
		return registry.CLASSES_ROOT
	case Users:
		return registry.USERS
	default:
		return registry.LOCAL_MACHINE
	}
}

func viewFlag(v View) uint32 {
	switch v {
	case Force32:
		return registry.WOW64_32KEY
	case Force64:
		return registry.WOW64_64KEY
	default:
		return 0
	}
}

func getString(h Hive, path, name string, v View) (string, error) {
	k, err := registry.OpenKey(hiveKey(h), path, registry.QUERY_VALUE|viewFlag(v))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", ErrNotExist
		}
		return "", err
	}
	defer k.Close()

	s, _, err := k.GetStringValue(name)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", ErrNotExist
		}
		return "", err
	}
	return s, nil
}

func setString(h Hive, path, name, value string, v View) error {
	k, _, err := registry.CreateKey(hiveKey(h), path, registry.SET_VALUE|viewFlag(v))
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(name, value)
}

func setDWord(h Hive, path, name string, value uint32, v View) error {
	k, _, err := registry.CreateKey(hiveKey(h), path, registry.SET_VALUE|viewFlag(v))
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetDWordValue(name, value)
}

func getDWord(h Hive, path, name string, v View) (uint32, error) {
	k, err := registry.OpenKey(hiveKey(h), path, registry.QUERY_VALUE|viewFlag(v))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return 0, ErrNotExist
		}
		return 0, err
	}
	defer k.Close()

	n, _, err := k.GetIntegerValue(name)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return 0, ErrNotExist
		}
		return 0, err
	}
	return uint32(n), nil
}

func getStrings(h Hive, path, name string, v View) ([]string, error) {
	k, err := registry.OpenKey(hiveKey(h), path, registry.QUERY_VALUE|viewFlag(v))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, ErrNotExist
		}
		return nil, err
	}
	defer k.Close()

	ss, _, err := k.GetStringsValue(name)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, ErrNotExist
		}
		return nil, err
	}
	return ss, nil
}

func keyExists(h Hive, path string, v View) (bool, error) {
	k, err := registry.OpenKey(hiveKey(h), path, registry.QUERY_VALUE|viewFlag(v))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	k.Close()
	return true, nil
}

func exists(h Hive, path, name string, v View) (bool, error) {
	k, err := registry.OpenKey(hiveKey(h), path, registry.QUERY_VALUE|viewFlag(v))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer k.Close()

	if _, _, err := k.GetValue(name, nil); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		// ERROR_MORE_DATA means the value exists but our nil buffer was too small.
		if errors.Is(err, registry.ErrShortBuffer) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}
