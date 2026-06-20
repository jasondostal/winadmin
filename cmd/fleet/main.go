// Command fleet is a parallel fleet task runner: point it at a list of machines
// and a task, and it fans out across them with a bounded pool of concurrent
// workers — the overnight "run this against hundreds of servers" job, with a
// dry-run, staged rollouts, and an audit trail.
//
// Usage:
//
//	fleet run    -L hosts.txt -c "ping -n 1 {{.Name}}" [-P 15] [--shuffle] [--what-if]
//	fleet regset -L hosts.txt --hive HKLM --key "Software\Acme" --name Enabled --type REG_DWORD --data 1
//	fleet deldir -L hosts.txt --path "C$\Temp\junk"
//
// Common flags (all subcommands):
//
//	-L  target list file          -P  max parallel (worker-pool cap)
//	-E  exclude list file         --shuffle  randomize order
//	--what-if  dry-run            --timeout  per-target timeout (e.g. 30s)
//	--stop-on-error              --transport local|ssh   --ssh-user USER
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/jasondostal/winadmin/config"
	"github.com/jasondostal/winadmin/dialog"
	"github.com/jasondostal/winadmin/fleet"
	"github.com/jasondostal/winadmin/secret"
	"github.com/jasondostal/winadmin/tui"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "run":
		runCmd(args)
	case "regset":
		regsetCmd(args)
	case "deldir":
		deldirCmd(args)
	case "ldapset":
		ldapsetCmd(args)
	case "svc":
		svcCmd(args)
	case "install":
		installCmd(args)
	case "push":
		pushCmd(args)
	case "reboot":
		rebootCmd(args)
	case "proc":
		procCmd(args)
	case "task":
		taskCmd(args)
	case "localgroup":
		localgroupCmd(args)
	case "firewall":
		firewallCmd(args)
	case "gather":
		gatherCmd(args)
	case "agent":
		agentCmd(args)
	case "provision":
		provisionCmd(args)
	case "status":
		statusCmd(args)
	case "tui":
		if err := tui.RunConsole(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "egg": // undocumented: the retro 3 a.m. installer dialog
		dialog.Classic()
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", sub)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `fleet - parallel fleet task runner

Subcommands:
  run     run an arbitrary command per target   (-c "cmd {{.Name}}")
  regset  set a registry value per target       (--hive --key --name --type --data)
  deldir  delete a directory per target         (--path)
  svc     control a service                      (--name --action start|stop|restart|status)
  install silent install an MSI/EXE/script       (--package --args --kind msi|exe|sh)
  push    copy files to each target              (--src --dst --backend robocopy|scp|rsync)
  reboot  reboot each target (needs --yes)       (--backend win|linux --delay)
  proc    kill a process                         (--image --backend taskkill|pkill)
  task    manage a scheduled task                (--name --action run|delete|query|create)
  localgroup add/remove a local group member     (--group --member --action add|remove)
  firewall manage a firewall rule                (--name --action add|delete --port)
  ldapset set an LDAP/AD attribute on every entry in an OU
                                                (--ldap-url --bind-dn --base --attr --value)
  gather  run a query per target, tabulate it    (-c "cmd" --format table|csv|json)
  provision install the agent as a service + register them  (--agent-url --job-source)
  status  poll the registered fleet's agent service (--registry --loop 0 --every 10s)
  tui     interactive run builder + live dashboard

Targets come from -L <file> or --inventory-cmd "<shell producing one target per line>".
Filter/preview: --match 'web*,db0?' keeps matching targets; --preview prints the
resolved target list and exits without running.
Staged rollout (most verbs): --canary N --wave M --health-cmd '<check>' --pause 10s.
Lifecycle: --loop N (0=forever), --wait 30s / --start-at HH:MM, --pre/--post '<cmd>'.
Add --tui to a verb to watch that run in the live dashboard.
Run "fleet <subcommand> -h" for subcommand flags.
`)
}

// commonFlags holds the switches every subcommand shares.
type commonFlags struct {
	list          string
	inventoryCmd  string
	inventorySpec string
	exclude       string
	match         string
	preview       bool
	parallelism   int
	shuffle       bool
	dryRun        bool
	timeout       time.Duration
	stopOnError   bool
	transport     string
	sshUser       string
	sshKey        string
	sshKnown      string
	sshPort       int
	sshInsecure   bool
	showOutput    bool
	tui           bool
	canary        int
	wave          int
	healthCmd     string
	pause         time.Duration
	retries       int
	retryBackoff  time.Duration
	sshPwEnv      string
	export        string
	yes           bool
	pre           string
	post          string
	loop          int
	wait          time.Duration
	startAt       string
	winrmUser     string
	winrmPwEnv    string
	winrmPort     int
	winrmHTTPS    bool
	winrmInsecure bool
}

func registerCommon(fs *flag.FlagSet) *commonFlags {
	c := &commonFlags{}
	// Persisted defaults (set via `fleet tui` → settings) seed the flags; an
	// explicit flag on the command line still wins.
	s, _ := config.LoadSettings()
	pDefault := 15
	if s.Parallelism > 0 {
		pDefault = s.Parallelism
	}
	transportDefault := "local"
	if s.Transport != "" {
		transportDefault = s.Transport
	}

	fs.StringVar(&c.list, "L", s.DefaultHosts, "target list file (one machine per line)")
	fs.StringVar(&c.inventoryCmd, "inventory-cmd", "", "shell command whose stdout lines are the targets (dynamic inventory)")
	fs.StringVar(&c.inventorySpec, "inventory", "", "inventory plugin: file:<p> | cmd:<sh> | aws:<filter> | ad-ou:<dn> | ad-group:<dn>")
	fs.StringVar(&c.exclude, "E", "", "exclude list file")
	fs.StringVar(&c.match, "match", "", "keep only targets matching these comma-separated globs (e.g. 'web*,db0?')")
	fs.BoolVar(&c.preview, "preview", false, "resolve and print the target list (after exclude/match) and exit, without running")
	fs.IntVar(&c.parallelism, "P", pDefault, "max targets in flight (worker-pool cap); 1 = sequential")
	fs.BoolVar(&c.shuffle, "shuffle", false, "randomize target order")
	fs.BoolVar(&c.dryRun, "what-if", false, "render commands without executing them")
	fs.DurationVar(&c.timeout, "timeout", 0, "per-target timeout, e.g. 30s (0 = none)")
	fs.BoolVar(&c.stopOnError, "stop-on-error", false, "cancel remaining targets on first failure")
	fs.StringVar(&c.transport, "transport", transportDefault, "transport: local | ssh | winrm | psexec | wmi")
	fs.StringVar(&c.sshUser, "ssh-user", s.SSHUser, "ssh username (transport=ssh)")
	fs.StringVar(&c.sshKey, "ssh-key", s.SSHKey, "ssh private key file (transport=ssh)")
	fs.StringVar(&c.sshKnown, "ssh-known-hosts", "", "ssh known_hosts file (default ~/.ssh/known_hosts)")
	fs.IntVar(&c.sshPort, "ssh-port", 22, "ssh port")
	fs.BoolVar(&c.sshInsecure, "ssh-insecure-hostkey", false, "skip ssh host-key verification (POC only)")
	fs.BoolVar(&c.tui, "tui", false, "watch the run in a live full-screen dashboard")
	fs.BoolVar(&c.showOutput, "v", false, "print each target's command output")
	fs.IntVar(&c.canary, "canary", 0, "staged rollout: hit this many targets first, then health-gate")
	fs.IntVar(&c.wave, "wave", 0, "staged rollout: batch size for the rest (0 = all at once)")
	fs.StringVar(&c.healthCmd, "health-cmd", "", "staged rollout: command run between batches; non-zero aborts")
	fs.DurationVar(&c.pause, "pause", 0, "staged rollout: pause between batches, e.g. 10s")
	fs.IntVar(&c.retries, "retries", 0, "extra attempts per target on failure")
	fs.DurationVar(&c.retryBackoff, "retry-backoff", 0, "wait between attempts, e.g. 2s")
	fs.StringVar(&c.sshPwEnv, "ssh-pw-env", "", "env var holding the ssh password (transport=ssh)")
	fs.StringVar(&c.export, "export", "", "write full per-target results to a .json or .csv file")
	fs.BoolVar(&c.yes, "yes", false, "confirm a destructive action across many targets")
	fs.StringVar(&c.pre, "pre", "", "control-host command to run once before the fan-out")
	fs.StringVar(&c.post, "post", "", "control-host command to run once after the fan-out")
	fs.IntVar(&c.loop, "loop", 1, "repeat the whole run this many times (0 = forever)")
	fs.DurationVar(&c.wait, "wait", 0, "wait this long before starting, e.g. 30s")
	fs.StringVar(&c.startAt, "start-at", "", "hold until a wall-clock time, HH:MM or HH:MM:SS")
	fs.StringVar(&c.winrmUser, "winrm-user", "", "winrm username (transport=winrm)")
	fs.StringVar(&c.winrmPwEnv, "winrm-pw-env", "", "env var holding the winrm password")
	fs.IntVar(&c.winrmPort, "winrm-port", 0, "winrm port (default 5985 http / 5986 https)")
	fs.BoolVar(&c.winrmHTTPS, "winrm-https", false, "use winrm over https (5986)")
	fs.BoolVar(&c.winrmInsecure, "winrm-insecure", false, "winrm https: skip TLS verification")
	return c
}

func (c *commonFlags) stageOptions() fleet.StageOptions {
	return fleet.StageOptions{
		Canary:    c.canary,
		Wave:      c.wave,
		HealthCmd: c.healthCmd,
		Pause:     c.pause,
	}
}

func (c *commonFlags) buildPlan(task fleet.Task) (fleet.Plan, error) {
	var inv *fleet.Inventory
	var err error
	switch {
	case c.inventorySpec != "":
		inv, err = fleet.ResolveInventory(context.Background(), c.inventorySpec)
	case c.inventoryCmd != "":
		inv, err = fleet.InventoryFromCommand(context.Background(), c.inventoryCmd)
	case c.list != "":
		inv, err = fleet.LoadInventory(c.list)
	default:
		return fleet.Plan{}, fmt.Errorf("provide -L <file>, --inventory <spec>, or --inventory-cmd <command>")
	}
	if err != nil {
		return fleet.Plan{}, err
	}
	if c.exclude != "" {
		if err := inv.ExcludeFromFile(c.exclude); err != nil {
			return fleet.Plan{}, err
		}
	}
	if c.match != "" {
		if err := inv.Match(strings.Split(c.match, ",")); err != nil {
			return fleet.Plan{}, err
		}
	}
	if c.shuffle {
		inv.Shuffle(rand.Shuffle)
	}

	tr, err := c.buildTransport()
	if err != nil {
		return fleet.Plan{}, err
	}
	return fleet.Plan{Inventory: inv, Task: task, Transport: tr}, nil
}

// buildTransport constructs the transport from the flags — shared by buildPlan
// and the registry-driven commands (provision/status) that supply their own
// inventory.
func (c *commonFlags) buildTransport() (fleet.Transport, error) {
	switch c.transport {
	case "local", "":
		return fleet.LocalTransport{}, nil
	case "ssh":
		var pwProvider secret.Provider
		if c.sshPwEnv != "" {
			user := c.sshUser
			if user == "" {
				user = "ssh"
			}
			pwProvider = secret.Plaintext{User: user, Password: os.Getenv(c.sshPwEnv)}
		}
		return fleet.SSHTransport{
			User:                  c.sshUser,
			Port:                  c.sshPort,
			KeyPath:               c.sshKey,
			UseAgent:              true, // fall back to ssh-agent if present
			PasswordProvider:      pwProvider,
			KnownHostsPath:        c.sshKnown,
			InsecureIgnoreHostKey: c.sshInsecure,
		}, nil
	case "winrm":
		user := c.winrmUser
		if user == "" {
			user = "Administrator"
		}
		var pw secret.Provider
		if c.winrmPwEnv != "" {
			pw = secret.Plaintext{User: user, Password: os.Getenv(c.winrmPwEnv)}
		}
		return fleet.WinRMTransport{
			User:             user,
			PasswordProvider: pw,
			Port:             c.winrmPort,
			HTTPS:            c.winrmHTTPS,
			Insecure:         c.winrmInsecure,
		}, nil
	case "psexec":
		var pw secret.Provider
		if c.winrmPwEnv != "" {
			pw = secret.Plaintext{User: c.winrmUser, Password: os.Getenv(c.winrmPwEnv)}
		}
		return fleet.PsExecTransport{User: c.winrmUser, PasswordProvider: pw}, nil
	case "wmi":
		var pw secret.Provider
		if c.winrmPwEnv != "" {
			pw = secret.Plaintext{User: c.winrmUser, Password: os.Getenv(c.winrmPwEnv)}
		}
		return fleet.WMITransport{User: c.winrmUser, PasswordProvider: pw}, nil
	default:
		return nil, fmt.Errorf("unknown transport %q", c.transport)
	}
}

func (c *commonFlags) options() fleet.Options {
	return fleet.Options{
		Parallelism:  c.parallelism,
		Timeout:      c.timeout,
		DryRun:       c.dryRun,
		StopOnError:  c.stopOnError,
		Retries:      c.retries,
		RetryBackoff: c.retryBackoff,
	}
}

func (c *commonFlags) lifecycleOptions() fleet.LifecycleOptions {
	return fleet.LifecycleOptions{
		Pre:     c.pre,
		Post:    c.post,
		Loops:   c.loop,
		Forever: c.loop == 0,
		Delay:   c.wait,
		StartAt: c.startAt,
	}
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	common := registerCommon(fs)
	cmd := fs.String("c", "", `command template, e.g. "ping -n 1 {{.Name}}" [required]`)
	_ = fs.Parse(args)

	if *cmd == "" {
		fmt.Fprintln(os.Stderr, "run: -c command is required")
		os.Exit(2)
	}
	execute(common, fleet.CommandTask{Template: *cmd})
}

func regsetCmd(args []string) {
	fs := flag.NewFlagSet("regset", flag.ExitOnError)
	common := registerCommon(fs)
	hive := fs.String("hive", "HKLM", "registry hive (HKLM, HKCU, ...)")
	key := fs.String("key", "", `key path, e.g. Software\Acme\App [required]`)
	name := fs.String("name", "", "value name (empty = default value)")
	typ := fs.String("type", "REG_SZ", "value type (REG_SZ, REG_DWORD, ...)")
	data := fs.String("data", "", "value data")
	local := fs.Bool("local", false, "run on the box (over ssh) instead of remotely via \\\\host")
	_ = fs.Parse(args)

	if *key == "" {
		fmt.Fprintln(os.Stderr, "regset: --key is required")
		os.Exit(2)
	}
	execute(common, fleet.RegSetTask{
		Hive: *hive, Key: *key, Name: *name, Type: *typ, Data: *data, Local: *local,
	})
}

func deldirCmd(args []string) {
	fs := flag.NewFlagSet("deldir", flag.ExitOnError)
	common := registerCommon(fs)
	path := fs.String("path", "", `directory to delete, e.g. C$\Temp\junk [required]`)
	local := fs.Bool("local", false, "delete a path on the box (over ssh) instead of \\\\host\\path")
	_ = fs.Parse(args)

	if *path == "" {
		fmt.Fprintln(os.Stderr, "deldir: --path is required")
		os.Exit(2)
	}
	guardDestructive(common, "deldir")
	execute(common, fleet.DeleteDirTask{Path: *path, Local: *local})
}

// guardDestructive refuses a destructive verb that would hit more than one
// target unless --yes (or --what-if) is given — confirm-on-blast.
func guardDestructive(c *commonFlags, verb string) {
	if c.dryRun || c.yes || c.preview {
		return
	}
	n := 2 // unknown until inventory loads; treat as "many" to be safe
	if c.list != "" {
		if inv, err := fleet.LoadInventory(c.list); err == nil {
			n = inv.Len()
		}
	}
	if n > 1 {
		fmt.Fprintf(os.Stderr, "%s: refusing to hit %d targets without --yes (or preview with --what-if)\n", verb, n)
		os.Exit(2)
	}
}

func execute(common *commonFlags, task fleet.Task) {
	plan, err := common.buildPlan(task)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if common.preview {
		previewTargets(plan)
		return
	}
	runPlan(plan, common.options(), common.stageOptions(), common.lifecycleOptions(), common.tui, common.showOutput, common.export)
}

// previewTargets prints the resolved target list (post inventory/exclude/match)
// without running anything — "show me exactly who I'm about to hit."
func previewTargets(plan fleet.Plan) {
	fmt.Printf("fleet :: %s\n", plan.Task.Describe())
	fmt.Printf("%d target(s) after inventory/exclude/match:\n\n", plan.Inventory.Len())
	for _, t := range plan.Inventory.Targets {
		fmt.Printf("  %s\n", t.Name)
	}
}

// runPlan executes a built plan: the lifecycle wait/loop/pre-post wrapper around
// the live dashboard or stdout progress. Shared by every subcommand so they all
// behave identically (TUI, dry-run, -v, staging, lifecycle, exit codes).
func runPlan(plan fleet.Plan, opts fleet.Options, stage fleet.StageOptions, life fleet.LifecycleOptions, useTUI, showOutput bool, exportPath string) {
	if plan.Inventory.Len() == 0 {
		fmt.Fprintln(os.Stderr, "no targets after inventory/exclude/match — nothing to do")
		os.Exit(1)
	}

	// The full-screen dashboard owns the whole lifecycle itself (wait/pre/post/loop).
	if useTUI {
		if err := tui.RunWatcher(plan, opts, stage, life); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	ctx := context.Background()

	// Delayed / clock-scheduled start.
	if life.StartAt != "" || life.Delay > 0 {
		when, err := life.StartTime(time.Now())
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		fmt.Printf("waiting until %s (in %s) ...\n",
			when.Format("2006-01-02 15:04:05"), time.Until(when).Truncate(time.Second))
		if err := life.WaitForStart(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}

	anyFailed := false
	looping := life.Forever || life.TotalRuns() > 1
	for n := 1; life.Forever || n <= life.TotalRuns(); n++ {
		if looping {
			label := fmt.Sprintf("%d/%d", n, life.TotalRuns())
			if life.Forever {
				label = fmt.Sprintf("%d (forever — ctrl-c to stop)", n)
			}
			fmt.Printf("\n======== loop %s ========\n", label)
		}
		if life.Pre != "" {
			fmt.Printf(":: pre-command: %s\n", life.Pre)
			if err := fleet.RunControlCommand(ctx, life.Pre); err != nil {
				fmt.Fprintln(os.Stderr, "pre-command failed:", err)
				os.Exit(1)
			}
		}
		if runOnce(ctx, plan, opts, stage, showOutput, exportPath) {
			anyFailed = true
		}
		if life.Post != "" {
			fmt.Printf(":: post-command: %s\n", life.Post)
			if err := fleet.RunControlCommand(ctx, life.Post); err != nil {
				fmt.Fprintln(os.Stderr, "post-command failed:", err)
				os.Exit(1)
			}
		}
	}

	if anyFailed {
		os.Exit(1)
	}
}

// runOnce performs one fan-out (staged or plain), prints per-target progress and
// the run summary, and reports whether any target failed.
func runOnce(ctx context.Context, plan fleet.Plan, opts fleet.Options, stage fleet.StageOptions, showOutput bool, exportPath string) (failed bool) {
	mode := "EXECUTE"
	if opts.DryRun {
		mode = "WHAT-IF (dry run)"
	}
	stageNote := ""
	if stage.Active() {
		stageNote = fmt.Sprintf("  rollout=canary:%d/wave:%d", stage.Canary, stage.Wave)
	}
	fmt.Printf("fleet :: %s\n", plan.Task.Describe())
	fmt.Printf("targets=%d  parallel=%d  transport=%s  mode=%s%s\n\n",
		plan.Inventory.Len(), opts.Parallelism, plan.Transport.Describe(), mode, stageNote)

	// Live per-target progress to stdout; structured audit goes to slog (stderr).
	onResult := func(done, total int, r fleet.Result) {
		status := "ok"
		switch {
		case r.Skipped:
			status = "skip"
		case r.DryRun:
			status = "would-run"
		case r.Err != nil:
			status = "ERROR: " + r.Err.Error()
		case r.ExitCode != 0:
			status = fmt.Sprintf("exit %d", r.ExitCode)
		}
		fmt.Printf("[%d/%d] %-20s %s\n", done, total, r.Target, status)
		if showOutput {
			for _, line := range fleet.NonEmptyLines(r.Stdout) {
				fmt.Printf("        | %s\n", line)
			}
			if r.ExitCode != 0 || r.Err != nil {
				for _, line := range fleet.NonEmptyLines(r.Stderr) {
					fmt.Printf("        ! %s\n", line)
				}
			}
		}
	}

	var summary fleet.Summary
	var results []fleet.Result
	if stage.Active() {
		onBatch := func(num, totalB, size int, label string) {
			fmt.Printf("\n== %s: batch %d/%d (%d target(s)) ==\n", strings.ToUpper(label), num, totalB, size)
		}
		s, res, err := fleet.RunWaves(ctx, plan, opts, stage, onBatch, onResult)
		summary, results = s, res
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n!! %s\n", err)
		}
	} else {
		summary, results = fleet.Run(ctx, plan, opts, onResult)
	}

	if exportPath != "" {
		if err := fleet.ExportResults(results, exportPath); err != nil {
			fmt.Fprintln(os.Stderr, "export error:", err)
		} else {
			fmt.Printf("results written to %s\n", exportPath)
		}
	}

	fmt.Printf("\n---- run summary ----\n")
	fmt.Printf("total=%d  succeeded=%d  failed=%d  skipped=%d\n",
		summary.Total, summary.Succeeded, summary.Failed, summary.Skipped)
	fmt.Printf("started=%s\nfinished=%s\nelapsed=%s\n",
		summary.Started.Format(time.RFC3339), summary.Finished.Format(time.RFC3339), summary.Elapsed())

	return summary.Failed > 0
}

func svcCmd(args []string) {
	fs := flag.NewFlagSet("svc", flag.ExitOnError)
	common := registerCommon(fs)
	name := fs.String("name", "", "service name [required]")
	action := fs.String("action", "status", "start | stop | restart | status")
	backend := fs.String("backend", "systemctl", "systemctl (ssh/linux) | sc (windows)")
	local := fs.Bool("local", false, "sc: run on the box instead of \\\\host")
	sudo := fs.Bool("sudo", false, "systemctl: prefix sudo (start/stop/restart over ssh)")
	_ = fs.Parse(args)
	if *name == "" {
		fmt.Fprintln(os.Stderr, "svc: --name is required")
		os.Exit(2)
	}
	execute(common, fleet.ServiceTask{Service: *name, Action: *action, Backend: *backend, Local: *local, Sudo: *sudo})
}

func installCmd(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	common := registerCommon(fs)
	pkg := fs.String("package", "", "installer path or command [required]")
	iargs := fs.String("args", "", "installer arguments")
	kind := fs.String("kind", "msi", "msi | exe | sh")
	_ = fs.Parse(args)
	if *pkg == "" {
		fmt.Fprintln(os.Stderr, "install: --package is required")
		os.Exit(2)
	}
	execute(common, fleet.InstallTask{Package: *pkg, Args: *iargs, Kind: *kind})
}

func pushCmd(args []string) {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	common := registerCommon(fs)
	src := fs.String("src", "", "source path [required]")
	dst := fs.String("dst", "", "destination path/share [required]")
	backend := fs.String("backend", "robocopy", "robocopy | scp | rsync")
	mirror := fs.Bool("mirror", false, "robocopy /MIR")
	user := fs.String("scp-user", "", "scp/rsync user (user@host)")
	_ = fs.Parse(args)
	if *src == "" || *dst == "" {
		fmt.Fprintln(os.Stderr, "push: --src and --dst are required")
		os.Exit(2)
	}
	execute(common, fleet.CopyTask{Src: *src, Dst: *dst, Backend: *backend, Mirror: *mirror, SSHUser: *user})
}

func rebootCmd(args []string) {
	fs := flag.NewFlagSet("reboot", flag.ExitOnError)
	common := registerCommon(fs)
	backend := fs.String("backend", "win", "win | linux")
	delay := fs.Int("delay", 30, "delay in seconds before reboot")
	msg := fs.String("message", "", "reboot message (windows)")
	sudo := fs.Bool("sudo", false, "linux: prefix sudo")
	_ = fs.Parse(args)
	guardDestructive(common, "reboot")
	execute(common, fleet.RebootTask{Backend: *backend, DelaySec: *delay, Message: *msg, Sudo: *sudo})
}

func procCmd(args []string) {
	fs := flag.NewFlagSet("proc", flag.ExitOnError)
	common := registerCommon(fs)
	image := fs.String("image", "", "process image/name to kill [required]")
	backend := fs.String("backend", "taskkill", "taskkill (windows) | pkill (ssh/linux)")
	force := fs.Bool("force", false, "force kill")
	_ = fs.Parse(args)
	if *image == "" {
		fmt.Fprintln(os.Stderr, "proc: --image is required")
		os.Exit(2)
	}
	guardDestructive(common, "proc")
	execute(common, fleet.ProcKillTask{Image: *image, Backend: *backend, Force: *force})
}

func taskCmd(args []string) {
	fs := flag.NewFlagSet("task", flag.ExitOnError)
	common := registerCommon(fs)
	name := fs.String("name", "", "scheduled task name [required]")
	action := fs.String("action", "query", "run | delete | query | create")
	cmd := fs.String("command", "", "create: program to run")
	sched := fs.String("schedule", "ONLOGON", "create: ONLOGON|DAILY|HOURLY|ONSTART|ONCE")
	_ = fs.Parse(args)
	if *name == "" {
		fmt.Fprintln(os.Stderr, "task: --name is required")
		os.Exit(2)
	}
	execute(common, fleet.SchTask{Name: *name, Action: *action, Program: *cmd, Schedule: *sched})
}

func localgroupCmd(args []string) {
	fs := flag.NewFlagSet("localgroup", flag.ExitOnError)
	common := registerCommon(fs)
	group := fs.String("group", "Administrators", "local group name")
	member := fs.String("member", "", "member to add/remove [required]")
	action := fs.String("action", "add", "add | remove")
	_ = fs.Parse(args)
	if *member == "" {
		fmt.Fprintln(os.Stderr, "localgroup: --member is required")
		os.Exit(2)
	}
	execute(common, fleet.LocalGroupTask{Group: *group, Member: *member, Action: *action})
}

func firewallCmd(args []string) {
	fs := flag.NewFlagSet("firewall", flag.ExitOnError)
	common := registerCommon(fs)
	action := fs.String("action", "add", "add | delete")
	name := fs.String("name", "", "rule name [required]")
	dir := fs.String("dir", "in", "in | out")
	fwAction := fs.String("fw-action", "allow", "allow | block")
	proto := fs.String("protocol", "tcp", "tcp | udp")
	port := fs.String("port", "", "local port(s)")
	_ = fs.Parse(args)
	if *name == "" {
		fmt.Fprintln(os.Stderr, "firewall: --name is required")
		os.Exit(2)
	}
	execute(common, fleet.FirewallTask{Action: *action, Name: *name, Dir: *dir, FWAction: *fwAction, Protocol: *proto, Port: *port})
}

// gatherCmd runs a query per target and aggregates the output into a table / CSV
// / JSON — the read side that turns the tool into a fleet reporting engine.
func gatherCmd(args []string) {
	fs := flag.NewFlagSet("gather", flag.ExitOnError)
	common := registerCommon(fs)
	cmd := fs.String("c", "", `query command, e.g. "cat /etc/os-release | grep VERSION" [required]`)
	format := fs.String("format", "table", "table | csv | json")
	_ = fs.Parse(args)
	if *cmd == "" {
		fmt.Fprintln(os.Stderr, "gather: -c command is required")
		os.Exit(2)
	}
	plan, err := common.buildPlan(fleet.CommandTask{Template: *cmd})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if plan.Inventory.Len() == 0 {
		fmt.Fprintln(os.Stderr, "no targets — nothing to gather")
		os.Exit(1)
	}
	if common.preview {
		previewTargets(plan)
		return
	}
	if common.tui {
		if err := tui.RunGather(plan, common.options()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	_, results := fleet.Run(context.Background(), plan, common.options(), nil)
	fmt.Print(fleet.FormatGather(results, *format))
}

// ldapsetCmd sets one attribute on every entry returned by an LDAP/AD search —
// e.g. "set department on every user in this OU". Dynamic inventory comes from
// ldapsearch; the per-entry change is an ldapmodify run via the local transport,
// so it reuses the whole engine (pool, dry-run, audit, TUI). No new dependency:
// it shells out to ldap-utils, the modern equivalent of the old dsmod/Set-ADUser
// the legacy scripts called.
func ldapsetCmd(args []string) {
	fs := flag.NewFlagSet("ldapset", flag.ExitOnError)
	url := fs.String("ldap-url", "", "LDAP URI, e.g. ldap://dc01.corp.com [required]")
	bindDN := fs.String("bind-dn", "", "bind DN, e.g. CN=svc,OU=Svc,DC=corp,DC=com [required]")
	bindPw := fs.String("bind-pw", "", "bind password (plaintext; prefer --bind-pw-env)")
	bindPwEnv := fs.String("bind-pw-env", "", "env var holding the bind password")
	base := fs.String("base", "", "search base / OU, e.g. OU=Tellers,DC=corp,DC=com [required]")
	filter := fs.String("filter", "(&(objectCategory=person)(objectClass=user))", "LDAP search filter")
	attr := fs.String("attr", "", "attribute to set [required]")
	value := fs.String("value", "", "value to set the attribute to")
	op := fs.String("op", "replace", "modify op: replace | add | delete")
	parallel := fs.Int("P", 10, "max entries modified in flight")
	dryRun := fs.Bool("what-if", false, "show what would change without modifying")
	showOutput := fs.Bool("v", false, "print ldapmodify output per entry")
	_ = fs.Parse(args)

	for name, v := range map[string]string{"--ldap-url": *url, "--bind-dn": *bindDN, "--base": *base, "--attr": *attr} {
		if v == "" {
			fmt.Fprintf(os.Stderr, "ldapset: %s is required\n", name)
			os.Exit(2)
		}
	}

	// Resolve the bind password and stash it in a 0600 temp file, so it is passed
	// to ldapsearch/ldapmodify via -y (never on a command line or in the audit log).
	pw := *bindPw
	if *bindPwEnv != "" {
		pw = os.Getenv(*bindPwEnv)
	}
	pwFile, err := os.CreateTemp("", "fleet-ldap-*.secret")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer os.Remove(pwFile.Name())
	_ = pwFile.Chmod(0o600)
	_, _ = pwFile.WriteString(pw)
	_ = pwFile.Close()

	conn := fmt.Sprintf(`-H %s -D %s -y %s`, fleet.ShellQuote(*url), fleet.ShellQuote(*bindDN), fleet.ShellQuote(pwFile.Name()))

	// Dynamic inventory: every DN under the base matching the filter.
	invCmd := fmt.Sprintf(`ldapsearch -x -LLL -o ldif-wrap=no %s -b %s %s dn | sed -n 's/^dn: //p'`,
		conn, fleet.ShellQuote(*base), fleet.ShellQuote(*filter))

	// Per-entry change: an LDIF modify piped into ldapmodify. {{.Name}} is the DN.
	ldif := fmt.Sprintf("dn: {{.Name}}\\nchangetype: modify\\n%s: %s\\n%s: %s\\n",
		*op, *attr, *attr, *value)
	taskCmd := fmt.Sprintf(`printf '%s' | ldapmodify -x %s`, ldif, conn)

	common := &commonFlags{
		inventoryCmd: invCmd,
		parallelism:  *parallel,
		dryRun:       *dryRun,
		transport:    "local",
		showOutput:   *showOutput,
		loop:         1, // once; lifecycleOptions() treats loop==0 as "forever"
	}
	plan, err := common.buildPlan(fleet.CommandTask{Template: taskCmd})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("ldapset :: %s %s=%s under %s\n", *op, *attr, *value, *base)
	runPlan(plan, common.options(), common.stageOptions(), common.lifecycleOptions(), false, *showOutput, common.export)
}

// agentCmd is the pull side: each box runs this to fetch a job from a shared
// source and run it when it changes (version-gated, pull-agent style).
func agentCmd(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	source := fs.String("source", "", "job source: a file path or http(s) URL [required]")
	state := fs.String("state", "", "state file for the last-run version (default: <source-hash> in temp)")
	interval := fs.Duration("interval", 0, "poll interval, e.g. 60s (0 = run once)")
	once := fs.Bool("once", false, "poll exactly once and exit")
	_ = fs.Parse(args)
	if *source == "" {
		fmt.Fprintln(os.Stderr, "agent: --source is required")
		os.Exit(2)
	}
	statePath := *state
	if statePath == "" {
		statePath = filepathJoinTemp("fleet-agent-" + fleet.JobVersion(*source) + ".state")
	}

	poll := func() {
		last := fleet.ReadState(statePath)
		res := fleet.AgentPoll(context.Background(), *source, last)
		switch {
		case res.Err != nil && !res.Ran:
			fmt.Fprintln(os.Stderr, "agent: fetch error:", res.Err)
		case !res.Ran:
			fmt.Printf("agent: up to date (version %s)\n", res.Version)
		default:
			_ = fleet.WriteState(statePath, res.Version)
			status := "ok"
			if res.Err != nil || res.Outcome.ExitCode != 0 {
				status = fmt.Sprintf("exit %d", res.Outcome.ExitCode)
			}
			fmt.Printf("agent: ran job version %s -> %s\n", res.Version, status)
			for _, l := range fleet.NonEmptyLines(res.Outcome.Stdout) {
				fmt.Printf("  | %s\n", l)
			}
		}
	}

	// If the Windows SCM started us, run under the service control protocol
	// (this is what `fleet provision` installs via `sc create`).
	if handled, err := runAsService("fleetagent", *interval, poll); err != nil {
		fmt.Fprintln(os.Stderr, "agent service:", err)
		os.Exit(1)
	} else if handled {
		return
	}

	if *interval <= 0 || *once {
		poll()
		return
	}
	for {
		poll()
		if !sleepCtx(*interval) {
			return
		}
	}
}

// provisionCmd installs the agent as a service across a fleet and records each
// box in the registry. Each target downloads fleet.exe from --agent-url, then
// `sc create`s + starts the service (driven over winrm). The whole install is a
// base64-encoded PowerShell payload, so there's no cmd/sc.exe quoting to fight.
func provisionCmd(args []string) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	common := registerCommon(fs)
	agentURL := fs.String("agent-url", "", "URL the box downloads fleet.exe from [required]")
	jobSource := fs.String("job-source", "", "agent job source (fleet agent --source) [required]")
	installDir := fs.String("install-dir", `C:\fleet`, "install directory on the target")
	interval := fs.String("interval", "60s", "agent poll interval")
	svcName := fs.String("service", "fleetagent", "Windows service name")
	registryPath := fs.String("registry", "fleet-registry.json", "registry file to create/update")
	_ = fs.Parse(args)
	if *agentURL == "" || *jobSource == "" {
		fmt.Fprintln(os.Stderr, "provision: --agent-url and --job-source are required")
		os.Exit(2)
	}

	script := provisionScript(*installDir, *agentURL, *jobSource, *interval, *svcName)
	cmd := "powershell -NoProfile -ExecutionPolicy Bypass -EncodedCommand " + psEncoded(script)
	plan, err := common.buildPlan(fleet.CommandTask{Template: cmd})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if plan.Inventory.Len() == 0 {
		fmt.Fprintln(os.Stderr, "no targets — nothing to provision")
		os.Exit(1)
	}
	if common.preview {
		previewTargets(plan)
		return
	}

	fmt.Printf("provision :: %d target(s) over %s — install service %q from %s\n\n",
		plan.Inventory.Len(), plan.Transport.Describe(), *svcName, *agentURL)
	onResult := func(done, total int, r fleet.Result) {
		status := "ok"
		switch {
		case r.Err != nil:
			status = "ERROR: " + r.Err.Error()
		case r.ExitCode != 0:
			status = fmt.Sprintf("exit %d", r.ExitCode)
		}
		fmt.Printf("[%d/%d] %-24s %s\n", done, total, r.Target, status)
		if common.showOutput {
			for _, l := range fleet.NonEmptyLines(r.Stdout) {
				fmt.Printf("        | %s\n", l)
			}
		}
	}
	_, results := fleet.Run(context.Background(), plan, common.options(), onResult)

	reg, err := fleet.LoadRegistry(*registryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry:", err)
		os.Exit(1)
	}
	ok := 0
	for _, r := range results {
		if r.OK() {
			reg.Upsert(fleet.Machine{Name: r.Target, OS: "windows", ProvisionedAt: time.Now(), LastStatus: "PROVISIONED"})
			ok++
		}
	}
	if err := reg.Save(*registryPath); err != nil {
		fmt.Fprintln(os.Stderr, "registry save:", err)
		os.Exit(1)
	}
	fmt.Printf("\nprovisioned %d/%d — registry: %s\n", ok, len(results), *registryPath)
	if ok < len(results) {
		os.Exit(1)
	}
}

// provisionScript is the PowerShell that runs on each box: fetch the agent,
// (re)create the service, start it, and report its state. Wrapped in try/catch so
// a failure comes back as a readable message, with the progress bar silenced
// (it hangs Invoke-WebRequest in non-interactive WinRM sessions).
func provisionScript(dir, url, jobSrc, interval, svc string) string {
	binPath := fmt.Sprintf(`%s\fleet.exe agent --source %s --interval %s --state %s\agent.state`, dir, jobSrc, interval, dir)
	body := strings.Join([]string{
		fmt.Sprintf(`New-Item -ItemType Directory -Force -Path '%s' | Out-Null`, dir),
		// Stop + delete the old service FIRST so it releases the lock on fleet.exe
		// (a re-provision overwriting the running binary otherwise fails).
		fmt.Sprintf(`& sc.exe stop %s | Out-Null`, svc),
		fmt.Sprintf(`& sc.exe delete %s | Out-Null`, svc),
		fmt.Sprintf(`$deadline=(Get-Date).AddSeconds(20); while((Get-Date) -lt $deadline){ & sc.exe query %s *>$null; if($LASTEXITCODE -ne 0){break}; Start-Sleep -Milliseconds 400 }`, svc),
		fmt.Sprintf(`Invoke-WebRequest -UseBasicParsing -Uri '%s' -OutFile '%s\fleet.exe'`, url, dir),
		fmt.Sprintf(`& sc.exe create %s binPath= '%s' start= auto | Out-Null`, svc, binPath),
		fmt.Sprintf(`& sc.exe start %s | Out-Null`, svc),
		`Start-Sleep -Seconds 1`,
		fmt.Sprintf(`$q = (& sc.exe query %s | Out-String)`, svc),
		`Write-Output ('STATE: ' + (($q -split "` + "`n" + `") | Where-Object { $_ -match 'STATE' }))`,
	}, "; ")
	return `$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; ` +
		`[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; ` +
		`try { ` + body + ` } catch { Write-Output ('PROVISION-ERROR: ' + $_.Exception.Message); exit 1 }`
}

// psEncoded UTF-16LE-base64-encodes a script for `powershell -EncodedCommand`.
func psEncoded(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, r := range u {
		b[2*i] = byte(r)
		b[2*i+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// statusCmd is the live status board: poll the agent service on every registered
// machine on a cycle, tabulate, and stamp last_status/last_seen in the registry.
// The heartbeat-by-polling model — the engine's fan-out IS the poller.
func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	common := registerCommon(fs)
	registryPath := fs.String("registry", "fleet-registry.json", "registry file to read/update")
	svcName := fs.String("service", "fleetagent", "agent service to query")
	every := fs.Duration("every", 10*time.Second, "delay between cycles")
	_ = fs.Parse(args)
	cycles := common.loop // --loop N (0 = forever); reuses the common lifecycle flag

	reg, err := fleet.LoadRegistry(*registryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry:", err)
		os.Exit(1)
	}
	if len(reg.Machines) == 0 {
		fmt.Fprintln(os.Stderr, "no machines in registry", *registryPath)
		os.Exit(1)
	}
	tr, err := common.buildTransport()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// Live full-screen dashboard.
	if common.tui {
		if err := tui.RunStatusBoard(reg, *registryPath, tr, common.options(), *svcName, *every); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	plan := fleet.Plan{Inventory: reg.Inventory(), Task: fleet.CommandTask{Template: fmt.Sprintf("sc query %s", *svcName)}, Transport: tr}
	for n := 1; cycles == 0 || n <= cycles; n++ {
		_, results := fleet.Run(context.Background(), plan, common.options(), nil)
		fmt.Printf("\n==== fleet status @ %s ====\n", time.Now().Format("15:04:05"))
		running := 0
		for _, r := range results {
			st := fleet.ServiceState(r)
			if st == "RUNNING" {
				running++
			}
			fmt.Printf("  %-24s %s\n", r.Target, st)
			reg.Upsert(fleet.Machine{Name: r.Target, LastStatus: st, LastSeen: time.Now()})
		}
		fmt.Printf("  ---- %d/%d running ----\n", running, len(results))
		_ = reg.Save(*registryPath)
		if cycles != 0 && n >= cycles {
			break
		}
		if !sleepCtx(*every) {
			break
		}
	}
}

func sleepCtx(d time.Duration) bool { time.Sleep(d); return true }

func filepathJoinTemp(name string) string { return os.TempDir() + string(os.PathSeparator) + name }
