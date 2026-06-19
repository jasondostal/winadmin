//go:build !windows

package runas

import "github.com/jasondostal/winadmin/secret"

func run(secret.Credential, Options) error {
	return ErrUnsupportedPlatform
}
