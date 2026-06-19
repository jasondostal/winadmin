package secret

import (
	"errors"
	"os"
)

// Env reads a credential from environment variables. This gets the secret out
// of the binary and into the deployment surface (a systemd unit, a CI secret,
// a launchd plist, etc.) — the first meaningful step up from Plaintext.
//
// Zero-value field names fall back to sane defaults:
//
//	UserVar     -> WINADMIN_USER
//	DomainVar   -> WINADMIN_DOMAIN
//	PasswordVar -> WINADMIN_PASSWORD
type Env struct {
	UserVar     string
	DomainVar   string
	PasswordVar string
}

func orDefault(name, def string) string {
	if name == "" {
		return def
	}
	return name
}

// Credential implements Provider.
func (e Env) Credential() (Credential, error) {
	userVar := orDefault(e.UserVar, "WINADMIN_USER")
	domVar := orDefault(e.DomainVar, "WINADMIN_DOMAIN")
	pwVar := orDefault(e.PasswordVar, "WINADMIN_PASSWORD")

	user := os.Getenv(userVar)
	if user == "" {
		return Credential{}, errors.New("winadmin/secret: " + userVar + " is empty")
	}
	return Credential{
		User:     user,
		Domain:   os.Getenv(domVar),
		Password: os.Getenv(pwVar),
	}, nil
}
