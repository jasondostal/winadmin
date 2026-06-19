// Package groups answers the question the WinBatch the membership gate function answered:
// "is this principal a member of group X?"
//
// The preferred path here does NOT hit a domain controller. The current
// process's access token already carries every group SID the user belongs to,
// resolved at logon — so CurrentUserInGroup is fast, offline-capable, and
// correct for the "should this code be allowed to run" gate.
//
// For enumerating groups of an *arbitrary* user (the dsGetUsersGrps case),
// query AD over LDAP from the caller; that intentionally lives outside this
// package so the dependency footprint here stays at golang.org/x/sys.
package groups

import "errors"

// ErrUnsupportedPlatform is returned off Windows.
var ErrUnsupportedPlatform = errors.New("winadmin/groups: only supported on Windows")

// CurrentUserInGroup reports whether the current process token is a member of
// the named group. The name is resolved with LookupAccountName, so both
// "DOMAIN\\Group" and bare well-known names (e.g. "Administrators") work.
func CurrentUserInGroup(name string) (bool, error) {
	return currentUserInGroup(name)
}
