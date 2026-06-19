// Command winadmin is a small example that wires the packages together: load a
// config, build a credential provider from it, and run a command as that user.
//
// It is intentionally thin — the interesting code is in the library packages.
// On non-Windows hosts the run-as call reports that it is unsupported, which is
// exactly how the cross-platform seams are meant to behave.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jasondostal/winadmin/config"
	"github.com/jasondostal/winadmin/runas"
)

func main() {
	cfgPath := flag.String("config", "", "path to JSON config (optional; defaults to env provider)")
	cmdLine := flag.String("cmd", "", "command line to run as the configured user")
	workDir := flag.String("dir", "", "working directory for the command")
	wait := flag.Bool("wait", true, "wait for the command to exit")
	profile := flag.Bool("profile", false, "load the target user's profile")
	flag.Parse()

	if *cmdLine == "" {
		fmt.Fprintln(os.Stderr, "error: -cmd is required")
		flag.Usage()
		os.Exit(2)
	}

	var runAsCfg config.RunAs
	if *cfgPath != "" {
		c, err := config.Load(*cfgPath)
		if err != nil {
			fatal(err)
		}
		runAsCfg = c.RunAs
	}

	provider, err := runAsCfg.Build()
	if err != nil {
		fatal(err)
	}

	// Credentials never get printed; the redacting String() is all we log.
	if cred, err := provider.Credential(); err == nil {
		fmt.Fprintf(os.Stderr, "running as %s\n", cred)
	}

	err = runas.Run(provider, runas.Options{
		CommandLine: *cmdLine,
		WorkingDir:  *workDir,
		Wait:        *wait,
		LoadProfile: *profile,
	})
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
