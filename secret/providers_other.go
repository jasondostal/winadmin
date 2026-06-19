//go:build !windows

package secret

// On non-Windows targets the OS-backed providers compile but refuse at runtime,
// so the package builds and vets everywhere (handy for editing on macOS/Linux).
// The cross-platform Plaintext and Env providers above work on every platform.

// DPAPI is a non-functional stub off Windows.
type DPAPI struct {
	User     string
	Domain   string
	BlobPath string
}

// Credential implements Provider; always returns ErrUnsupportedPlatform here.
func (DPAPI) Credential() (Credential, error) {
	return Credential{}, ErrUnsupportedPlatform
}

// EncryptDPAPI is a non-functional stub off Windows.
func EncryptDPAPI(string, bool) ([]byte, error) { return nil, ErrUnsupportedPlatform }

// WriteDPAPIBlob is a non-functional stub off Windows.
func WriteDPAPIBlob(string, string, bool) error { return ErrUnsupportedPlatform }

// CredMan is a non-functional stub off Windows.
type CredMan struct {
	TargetName string
	Domain     string
	User       string
}

// Credential implements Provider; always returns ErrUnsupportedPlatform here.
func (CredMan) Credential() (Credential, error) {
	return Credential{}, ErrUnsupportedPlatform
}
