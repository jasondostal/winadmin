//go:build windows

package secret

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	crypt32                  = windows.NewLazySystemDLL("crypt32.dll")
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procCryptProtect         = crypt32.NewProc("CryptProtectData")
	procCryptUnprot          = crypt32.NewProc("CryptUnprotectData")
	procLocalFree            = kernel32.NewProc("LocalFree")
	cryptprotectLocalMachine = uint32(0x4) // CRYPTPROTECT_LOCAL_MACHINE
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(b []byte) dataBlob {
	if len(b) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

func (b dataBlob) bytes() []byte {
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

// EncryptDPAPI protects plaintext with Windows DPAPI. When machineScope is true
// the blob is tied to the machine (any user/service on the box can decrypt it);
// otherwise it is tied to the current user profile. Use the result with
// WriteDPAPIBlob, then point a DPAPI provider at the file.
func EncryptDPAPI(plaintext string, machineScope bool) ([]byte, error) {
	in := newBlob([]byte(plaintext))
	var out dataBlob
	var flags uint32
	if machineScope {
		flags = cryptprotectLocalMachine
	}
	r, _, err := procCryptProtect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		uintptr(flags),
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}

func decryptDPAPI(blob []byte) (string, error) {
	in := newBlob(blob)
	var out dataBlob
	r, _, err := procCryptUnprot.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return string(out.bytes()), nil
}

// WriteDPAPIBlob encrypts plaintext and writes the resulting blob to path.
func WriteDPAPIBlob(path, plaintext string, machineScope bool) error {
	blob, err := EncryptDPAPI(plaintext, machineScope)
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o600)
}

// DPAPI resolves a password from a DPAPI-encrypted blob on disk. Only the
// password is encrypted; User/Domain are non-sensitive and held in the clear.
// No key ships with the binary — the OS holds it.
type DPAPI struct {
	User     string
	Domain   string
	BlobPath string
}

// Credential implements Provider.
func (d DPAPI) Credential() (Credential, error) {
	if d.User == "" {
		return Credential{}, errors.New("winadmin/secret: DPAPI provider has empty User")
	}
	if d.BlobPath == "" {
		return Credential{}, errors.New("winadmin/secret: DPAPI provider has empty BlobPath")
	}
	blob, err := os.ReadFile(d.BlobPath)
	if err != nil {
		return Credential{}, err
	}
	pw, err := decryptDPAPI(blob)
	if err != nil {
		return Credential{}, err
	}
	return Credential{User: d.User, Domain: d.Domain, Password: pw}, nil
}
