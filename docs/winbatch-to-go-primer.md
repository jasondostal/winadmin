# WinBatch → Go: A Sysadmin Translation Primer

*Porting the "common things" from a turn-of-the-millennium WinBatch desktop-automation
codebase into modern, idiomatic Go.*

WinBatch was the duct tape that held a lot of enterprise desktop fleets together in the
late '90s and 2000s: registry surgery, NT/AD group gating, NTFS ACL changes, and the
ever-present "run this as an admin account" trick. This primer maps those patterns onto
Go — and shows where Go lets you do the same job with materially less risk.

The whole thing leans on one dependency:

```
go get golang.org/x/sys/windows          # official Win32 wrappers
go get golang.org/x/sys/windows/registry # registry subpackage
```

That package is the modern equivalent of the WinBatch "extender" DLLs (`WWADS44I.DLL`,
`WWWNT44I.DLL`, etc.) — it's the same Win32 API surface, just typed and memory-safe.

---

## The transferable principle (read this part)

The point of porting old automation isn't nostalgia — it's that the *shape* of the
problem never changed. You still need to:

1. **Read/write machine state** (registry, ACLs).
2. **Decide who's allowed** (group membership).
3. **Act with elevated authority** (run as a service/install account).
4. **Hold a secret to do #3.**

The WinBatch era solved #4 by hardcoding a plaintext password in the script. That *worked*
— and it's also the single biggest liability in the whole codebase. The Go version keeps
the capability but lets you choose how the secret is stored, via one small interface. Same
behavior, opt-in to do it right. That's the through-line: **make the insecure path
possible but obvious, and make the secure path the easy default.**

---

## 1. Registry edits

The cleanest 1:1 mapping. WinBatch sprays `RegCreateKey` / `RegSetValue` / `RegQueryValue`
/ `RegExistValue` everywhere.

**WinBatch:**
```winbatch
if RegExistValue(@REGMACHINE,"SOFTWARE\Microsoft\Windows NT\CurrentVersion\[ProductName]")
  szProductName = RegQueryValue(@REGMACHINE,"SOFTWARE\Microsoft\Windows NT\CurrentVersion\[ProductName]")
endif

regkey = RegCreateKey(@REGMACHINE, "SOFTWARE\Acme, Inc.\App")
RegSetValue(regkey, "[ConnStr]", "database=APP;...")
```

**Go:**
```go
import "golang.org/x/sys/windows/registry"

// RegQueryValue + RegExistValue collapse into one call — the error IS the "exist" check.
k, err := registry.OpenKey(registry.LOCAL_MACHINE,
    `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
if err != nil { /* key missing */ }
defer k.Close()

productName, _, err := k.GetStringValue("ProductName")
if err == registry.ErrNotExist { /* the RegExistValue == false branch */ }

// RegCreateKey + RegSetValue
ck, _, err := registry.CreateKey(registry.LOCAL_MACHINE,
    `SOFTWARE\Acme, Inc.\App`, registry.SET_VALUE)
defer ck.Close()
ck.SetStringValue("ConnStr", "database=APP;...")      // REG_SZ
ck.SetDWordValue("bEnabled", 1)                        // RegQueryDword equivalent
ck.SetStringsValue("ProductSuite", []string{"A","B"})  // RegQueryMulSz / REG_MULTI_SZ
```

| WinBatch | Go (`registry`) |
|---|---|
| `@REGMACHINE` / `@REGCURRENT` | `registry.LOCAL_MACHINE` / `registry.CURRENT_USER` |
| `RegExistValue` | non-nil err / `registry.ErrNotExist` from a Get |
| `RegQueryValue` | `k.GetStringValue` |
| `RegQueryDword` | `k.GetIntegerValue` |
| `RegQueryMulSz` | `k.GetStringsValue` |
| `RegCreateKey` | `registry.CreateKey` |
| `RegSetValue` | `k.SetStringValue` / `SetDWordValue` / … |
| `RegDeleteValue` | `k.DeleteValue` |

**Gotcha — WOW64 redirection.** A 32-bit-compiled WinBatch saw `SOFTWARE\...` silently
redirected into `Wow6432Node` on 64-bit Windows. In Go you control the view explicitly by
OR-ing a flag into the access mask: `registry.QUERY_VALUE|registry.WOW64_64KEY` (or
`WOW64_32KEY`). No more guessing which hive you landed in — a genuine upgrade over the old
"detect AMD64 and branch" dance.

---

## 2. NT/AD group membership

The classic gate: bind to AD over LDAP, walk the user's group list, allow or deny. Two
ways to reproduce it in Go — pick based on the question you're actually asking.

### 2a. "Is the *current user* in group X?" → don't hit AD at all

If you only need to gate the running user, the groups are already resolved and sitting in
the process **access token**. No DC round-trip, works offline.

```go
import "golang.org/x/sys/windows"

func currentUserInGroup(groupSAM string) (bool, error) {
    var tok windows.Token
    if err := windows.OpenProcessToken(windows.CurrentProcess(),
        windows.TOKEN_QUERY, &tok); err != nil {
        return false, err
    }
    defer tok.Close()

    groups, err := tok.GetTokenGroups()
    if err != nil {
        return false, err
    }
    want, _, _, err := windows.LookupSID("", groupSAM) // "DOMAIN\\Some Group"
    if err != nil {
        return false, err
    }
    for _, g := range groups.AllGroups() {
        if g.Sid.Equals(want) {
            return true, nil
        }
    }
    return false, nil
}
```

This replaces the typical `the membership gate(USERNAME, "Some Group", exitIfNot)`. The "exit if not
a member" behavior becomes an `if !ok { warn(); os.Exit(1) }` at the call site.

### 2b. "Enumerate groups for an *arbitrary* user/computer" → LDAP, like the original

When you're looking up *other* objects you do need to query the directory. Use
`github.com/go-ldap/ldap/v3`. The old `dsFindPath` + `dsGetUsersGrps` pattern becomes a
search filtered on `memberOf` (or `tokenGroups` for transitive/nested resolution — which is
what the WinBatch helper was quietly giving you).

```go
import "github.com/go-ldap/ldap/v3"

func adGroups(sAMAccountName string) ([]string, error) {
    c, err := ldap.DialURL("ldap://example.com")
    if err != nil {
        return nil, err
    }
    defer c.Close()
    if err := c.GSSAPIBindCCacheWithAPIOptions(nil, nil); err != nil { // Kerberos/SSO
        return nil, err                                                 // or c.Bind(user, pass)
    }

    // rootDSE → defaultNamingContext, exactly like the old the domain-root lookup()
    root, err := c.Search(ldap.NewSearchRequest("", ldap.ScopeBaseObject,
        ldap.NeverDerefAliases, 0, 0, false,
        "(objectClass=*)", []string{"defaultNamingContext"}, nil))
    if err != nil {
        return nil, err
    }
    baseDN := root.Entries[0].GetAttributeValue("defaultNamingContext")

    res, err := c.Search(ldap.NewSearchRequest(baseDN, ldap.ScopeWholeSubtree,
        ldap.NeverDerefAliases, 0, 0, false,
        "(&(objectCategory=person)(sAMAccountName="+ldap.EscapeFilter(sAMAccountName)+"))",
        []string{"memberOf"}, nil))
    if err != nil {
        return nil, err
    }
    var groups []string
    for _, e := range res.Entries {
        for _, dn := range e.GetAttributeValues("memberOf") {
            groups = append(groups, parseCN(dn)) // pull "Foo" out of "CN=Foo,OU=...,DC=..."
        }
    }
    return groups, nil
}
```

The old two-step `ItemExtract(1,...,",")` then `ItemExtract(2,...,"=")` to peel `CN=Foo`
out of a DN becomes `ldap.ParseDN(dn)` (read `.RDNs[0].Attributes[0].Value`) — or a one-line
`strings.SplitN` if you want to keep it familiar.

For *local* machine groups (`net localgroup`, not domain), the syscall is
`NetLocalGroupGetMembers` — but for the "am I allowed" case, 2a's token approach beats
shelling out in every way.

---

## 3. Run-as with a service/install account

The trick every WinBatch shop reinvented: launch a process as a different user given a
password. The old implementations did it by piping a plaintext password into a helper:

```winbatch
; cmd /c echo <password> | su.exe svc-install "cmd" -l
```

Every variant — `su.exe`, a keyed `userexec.exe`, `runas /user:... | <password>` — is doing
*one* Win32 thing: **CreateProcessWithLogonW**. Go calls it directly. No helper EXE, no
`echo` pipe, no version-matched dependency.

```go
import (
    "syscall"
    "unsafe"
    "golang.org/x/sys/windows"
)

const LOGON_WITH_PROFILE = 0x00000001 // == runas /profile

func RunAs(user, domain, pass, cmdline, startDir string) error {
    advapi32 := windows.NewLazySystemDLL("advapi32.dll")
    proc := advapi32.NewProc("CreateProcessWithLogonW")

    u, _ := syscall.UTF16PtrFromString(user)
    d, _ := syscall.UTF16PtrFromString(domain)   // "DOMAIN", or COMPUTERNAME for a local acct
    p, _ := syscall.UTF16PtrFromString(pass)
    cl, _ := syscall.UTF16PtrFromString(cmdline)
    dir, _ := syscall.UTF16PtrFromString(startDir)

    si := new(windows.StartupInfo)
    si.Cb = uint32(unsafe.Sizeof(*si))
    pi := new(windows.ProcessInformation)

    r, _, err := proc.Call(
        uintptr(unsafe.Pointer(u)),
        uintptr(unsafe.Pointer(d)),
        uintptr(unsafe.Pointer(p)),
        LOGON_WITH_PROFILE,
        0,                                  // lpApplicationName (NULL → parse from cmdline)
        uintptr(unsafe.Pointer(cl)),
        windows.CREATE_UNICODE_ENVIRONMENT,
        0,
        uintptr(unsafe.Pointer(dir)),
        uintptr(unsafe.Pointer(si)),
        uintptr(unsafe.Pointer(pi)),
    )
    if r == 0 {
        return err
    }
    windows.WaitForSingleObject(pi.Process, windows.INFINITE) // == the WinBatch @WAIT flag
    windows.CloseHandle(pi.Thread)
    windows.CloseHandle(pi.Process)
    return nil
}
```

What *disappeared* versus the WinBatch version: no `cmd.exe /c`, no `echo … |` pipe (which
leaked the password onto the command line / process list — the genuinely dangerous part),
no dependency on a helper EXE being present and the right build. The credential goes
straight into LSA via the API and never appears on a command line. Same capability,
materially less exposure. Drop the `WaitForSingleObject` for fire-and-forget (`@NOWAIT`).

---

## 4. The secret: from hardcoded to configurable

The plaintext password constant — not the API call — is the real liability. The thing the
WinBatch era *couldn't* easily do is get the secret out of the source. Modern Windows gives
you drop-in options, and the clean way to express the choice is a tiny interface:

```go
// A CredentialProvider yields a username/domain/password at runtime.
// The implementation decides where the secret actually lives.
type CredentialProvider interface {
    Credential() (user, domain, password string, err error)
}
```

Implementations, weakest to strongest:

- **Plaintext / literal** — the "just-hardcode-it" escape hatch, kept deliberately for bootstrapping and
  parity with the old scripts. Possible, but it announces itself.
- **Environment / config file** — secret out of the binary, into the deployment.
- **DPAPI** (`windows.CryptProtectData` / `CryptUnprotectData`) — encrypt once,
  machine- or user-scoped; no key to ship.
- **Windows Credential Manager** (`CredRead` via `advapi32`) — store the install-account
  creds under a target name, read at runtime. The binary carries *no* secret.

Same `RunAs` call site; you swap the provider via config. That's the whole lesson in one
seam: the insecure path still exists for the journey, but doing it right is a one-line
configuration change, not a rewrite.

---

## TL;DR mapping

| The "common thing" | WinBatch | Go |
|---|---|---|
| Read/write registry | `RegQueryValue` / `RegSetValue` / `RegCreateKey` | `golang.org/x/sys/windows/registry` |
| Am I in a group? | `the membership gate` → AD/LDAP | token groups (`GetTokenGroups`) — no DC needed |
| Enumerate a user's groups | `dsGetUsersGrps` | `go-ldap/ldap/v3`, filter `memberOf`/`tokenGroups` |
| Run as another user w/ password | `echo pw \| su.exe` / `userexec` | `CreateProcessWithLogonW` directly |
| Hold the password | hardcoded plaintext | `CredentialProvider`: plaintext → env → DPAPI → CredMan |
| NTFS perms (`cacls`) | `cacls` run as SU | `os/exec` `icacls` under `RunAs`, or `SetNamedSecurityInfo` |

Everything ports. The run-as is the part that gets *cleaner*, not harder — and the secret
handling is the part that finally gets to grow up.
