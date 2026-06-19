//go:build windows

package groups

import "golang.org/x/sys/windows"

func currentUserInGroup(name string) (bool, error) {
	want, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return false, err
	}

	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false, err
	}
	defer token.Close()

	tg, err := token.GetTokenGroups()
	if err != nil {
		return false, err
	}
	for _, g := range tg.AllGroups() {
		if g.Sid.Equals(want) {
			return true, nil
		}
	}
	return false, nil
}
