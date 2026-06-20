// Package acl grants and revokes NTFS permissions on a file or directory — the
// modern, icacls-backed replacement for the WinBatch the NTFS-permission helper helpers,
// which shelled out to cacls through an SU'd helper EXE just to elevate.
//
// Here the elevation story is separate: run the process elevated (or hand the
// command to runas) and call Grant/Revoke directly. The command assembly is pure
// and unit-tested; the execution is Windows-only.
package acl

import (
	"errors"
	"fmt"
)

// ErrUnsupportedPlatform is returned off Windows.
var ErrUnsupportedPlatform = errors.New("winadmin/acl: only supported on Windows")

// Permission is an icacls simple-rights token.
type Permission string

const (
	Read        Permission = "R"  // read
	ReadExecute Permission = "RX" // read & execute
	Modify      Permission = "M"  // modify (read/write/delete)
	Write       Permission = "W"  // write
	FullControl Permission = "F"  // full control
)

// Options tunes how the change is applied.
type Options struct {
	// Recurse applies the change to the directory and everything under it
	// (icacls /T).
	Recurse bool

	// NoInherit grants the right WITHOUT the (OI)(CI) object/container inherit
	// flags. The default (false) makes a directory grant inheritable, which is
	// what "give this group access to this folder" almost always means.
	NoInherit bool
}

// Grant gives principal (e.g. "CORP\\Tellers" or "Users") the permission on path.
func Grant(path, principal string, perm Permission, opts Options) error {
	if path == "" || principal == "" || perm == "" {
		return fmt.Errorf("acl: path, principal, and permission are required")
	}
	return grant(grantArgs(path, principal, perm, opts))
}

// Revoke removes all granted ACEs for principal on path (icacls /remove).
func Revoke(path, principal string, opts Options) error {
	if path == "" || principal == "" {
		return fmt.Errorf("acl: path and principal are required")
	}
	return revoke(revokeArgs(path, principal, opts))
}

func grantArgs(path, principal string, perm Permission, opts Options) []string {
	inherit := "(OI)(CI)"
	if opts.NoInherit {
		inherit = ""
	}
	spec := fmt.Sprintf("%s:%s%s", principal, inherit, perm)
	args := []string{path, "/grant", spec}
	if opts.Recurse {
		args = append(args, "/T")
	}
	return args
}

func revokeArgs(path, principal string, opts Options) []string {
	args := []string{path, "/remove", principal}
	if opts.Recurse {
		args = append(args, "/T")
	}
	return args
}
