package fleet

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// This file is the inventory-plugin layer behind fleet's --inventory flag. It
// lived in the CLI, which meant a library user calling LoadInventory got none of
// the cloud/AD plugins the README advertises. Now any tool on the engine can
// resolve the same specs.

// ShellQuote single-quotes a string for safe inclusion in a /bin/sh command.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ResolveInventory turns an inventory spec into an Inventory:
//
//	file:<path>     a static target-list file (same format as LoadInventory)
//	cmd:<shell>     dynamic: the command's stdout lines are the targets
//	aws:<filters>   EC2 private IPs matching `aws ec2 describe-instances --filters`
//	ad-ou:<dn>      every user DN under an OU
//	ad-group:<dn>   every member of a group
//
// The ad-* plugins read LDAP_URL / LDAP_BIND_DN / LDAP_PW (and LDAP_BASE for the
// ad-group search base) from the environment. $LDAP_PW is expanded by the shell
// at run time, so the password is never baked into the rendered command or the
// audit log.
func ResolveInventory(ctx context.Context, spec string) (*Inventory, error) {
	switch {
	case strings.HasPrefix(spec, "file:"):
		return LoadInventory(strings.TrimPrefix(spec, "file:"))
	case strings.HasPrefix(spec, "cmd:"):
		return InventoryFromCommand(ctx, strings.TrimPrefix(spec, "cmd:"))
	case strings.HasPrefix(spec, "aws:"):
		filter := strings.TrimPrefix(spec, "aws:")
		cmd := fmt.Sprintf(`aws ec2 describe-instances --filters %s `+
			`--query 'Reservations[].Instances[].PrivateIpAddress' --output text | tr '\t' '\n'`,
			ShellQuote(filter))
		return InventoryFromCommand(ctx, cmd)
	case strings.HasPrefix(spec, "ad-ou:"):
		dn := strings.TrimPrefix(spec, "ad-ou:")
		return InventoryFromCommand(ctx, ldapInventoryCmd(dn, "(&(objectCategory=person)(objectClass=user))"))
	case strings.HasPrefix(spec, "ad-group:"):
		group := strings.TrimPrefix(spec, "ad-group:")
		return InventoryFromCommand(ctx, ldapInventoryCmd(os.Getenv("LDAP_BASE"), fmt.Sprintf("(memberOf=%s)", group)))
	default:
		return nil, fmt.Errorf("fleet: unknown inventory spec %q (use file:/cmd:/aws:/ad-ou:/ad-group:)", spec)
	}
}

func ldapInventoryCmd(base, filter string) string {
	return fmt.Sprintf(`ldapsearch -x -LLL -o ldif-wrap=no -H %s -D %s -w "$LDAP_PW" -b %s %s dn | sed -n 's/^dn: //p'`,
		ShellQuote(os.Getenv("LDAP_URL")), ShellQuote(os.Getenv("LDAP_BIND_DN")), ShellQuote(base), ShellQuote(filter))
}
