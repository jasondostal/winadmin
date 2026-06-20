// Package groups answers the classic login-script question:
// "is this principal a member of group X?"
//
// The preferred path here does NOT hit a domain controller. The current
// process's access token already carries every group SID the user belongs to,
// resolved at logon — so CurrentUserInGroup is fast, offline-capable, and
// correct for the "should this code be allowed to run" gate.
//
// For enumerating groups of an *arbitrary* user (the directory-lookup case),
// query AD over LDAP from the caller; that intentionally lives outside this
// package so the dependency footprint here stays at golang.org/x/sys.
package groups

import (
	"errors"
	"strings"
)

// ErrUnsupportedPlatform is returned off Windows.
var ErrUnsupportedPlatform = errors.New("winadmin/groups: only supported on Windows")

// CurrentUserInGroup reports whether the current process token is a member of
// the named group. The name is resolved with LookupAccountName, so both
// "DOMAIN\\Group" and bare well-known names (e.g. "Administrators") work.
func CurrentUserInGroup(name string) (bool, error) {
	return currentUserInGroup(name)
}

// CurrentUserGroups returns the names of every group on the current process
// token, formatted "DOMAIN\\Group" (or bare for well-known/local groups). SIDs
// that do not resolve to a name (logon SIDs, deleted accounts) are skipped.
// This is the enumerating form of CurrentUserInGroup — the token-only,
// no-DC-round-trip answer to "what is this principal a member of?"
func CurrentUserGroups() ([]string, error) {
	return currentUserGroups()
}

// InGroupList reports whether group appears in groups, comparing on the leaf
// name only (any "DOMAIN\\" prefix is ignored) and case-insensitively.
//
// It is the dependency-free predicate behind a *computer object* membership
// check — the modern shape of a per-machine authorization gate. This package keeps LDAP
// out on purpose, so fetch the machine's group list however you already query AD
// (e.g. `fleet --inventory ad-group:`, an ldapsearch, or a go-ldap client in the
// caller) and gate on it here.
func InGroupList(group string, groups []string) bool {
	want := leafName(group)
	for _, g := range groups {
		if leafName(g) == want {
			return true
		}
	}
	return false
}

// leafName lowercases a group name and strips any "DOMAIN\" prefix.
func leafName(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, `\`); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}
