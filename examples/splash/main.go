// Command splash shows the winadmin dialog package's ass-ugly retro splash
// screen — the terminal descendant of the splash screens overnight installers
// loved. Run it:
//
//	go run ./examples/splash
package main

import (
	"time"

	"github.com/jasondostal/winadmin/dialog"
)

func main() {
	steps := []string{
		"Installing ACME Teller Toolbar 4.0",
		"Copying files to C:\\ABC",
		"Updating registry (do not power off)",
	}
	for _, s := range steps {
		dialog.Splash("ABC SOFTWARE INSTALLER", s)
		time.Sleep(600 * time.Millisecond)
	}
	dialog.Message("ABC SOFTWARE INSTALLER", "Installation complete. A reboot is required.")
}
