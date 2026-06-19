//go:build windows

package secret

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32      = windows.NewLazySystemDLL("advapi32.dll")
	procCredReadW = advapi32.NewProc("CredReadW")
	procCredFree  = advapi32.NewProc("CredFree")
)

const credTypeGeneric = 1 // CRED_TYPE_GENERIC

// winCredential mirrors the Win32 CREDENTIALW struct.
type winCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// CredMan resolves a credential from the Windows Credential Manager. Store the
// install/service account once (cmdkey, the Control Panel UI, or any tool) under
// TargetName, and the binary itself carries no secret at all — the strongest of
// the bundled providers.
//
// If User is set it overrides the username stored in the vault; otherwise the
// vault's UserName is used.
type CredMan struct {
	TargetName string
	Domain     string
	User       string
}

// Credential implements Provider.
func (c CredMan) Credential() (Credential, error) {
	if c.TargetName == "" {
		return Credential{}, errors.New("winadmin/secret: CredMan provider has empty TargetName")
	}
	target, err := windows.UTF16PtrFromString(c.TargetName)
	if err != nil {
		return Credential{}, err
	}

	var pcred *winCredential
	r, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&pcred)),
	)
	if r == 0 {
		return Credential{}, callErr
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))

	// CredentialBlob is the UTF-16 password, length in bytes (not NUL-terminated).
	password := ""
	if pcred.CredentialBlobSize > 0 && pcred.CredentialBlob != nil {
		n := int(pcred.CredentialBlobSize) / 2
		u16 := unsafe.Slice((*uint16)(unsafe.Pointer(pcred.CredentialBlob)), n)
		password = windows.UTF16ToString(u16)
	}

	user := c.User
	if user == "" && pcred.UserName != nil {
		user = windows.UTF16PtrToString(pcred.UserName)
	}
	if user == "" {
		return Credential{}, errors.New("winadmin/secret: CredMan entry has no username and none was provided")
	}

	return Credential{User: user, Domain: c.Domain, Password: password}, nil
}
