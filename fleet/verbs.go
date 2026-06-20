package fleet

import (
	"fmt"
	"strings"
)

// This file holds the "verb" tasks beyond run/regset/deldir/ldapset: the common
// fleet actions an admin reaches for — service control, silent install, file
// push, reboot, process kill. Each renders a command per target; the backend
// field picks the Windows (remote-admin) or Unix/SSH (on-the-box) form.

// ServiceTask controls a service across the fleet. Backend "sc" targets Windows
// remotely (sc \\host); "systemctl" runs on the box (over ssh).
//
// The fleet-wide version of restarting a stuck service everywhere.
type ServiceTask struct {
	Service string
	Action  string // start | stop | restart | status
	Backend string // sc | systemctl
	Local   bool   // sc: run on the box instead of \\host
	Sudo    bool   // systemctl: prefix sudo (start/stop/restart need root over ssh)
}

// Command implements Task.
func (s ServiceTask) Command(t Target) (string, error) {
	action := strings.ToLower(s.Action)
	switch s.Backend {
	case "systemctl", "":
		sudo := ""
		if s.Sudo {
			sudo = "sudo "
		}
		switch action {
		case "start", "stop", "restart":
			return fmt.Sprintf(`%ssystemctl %s %s`, sudo, action, shquote(s.Service)), nil
		case "status":
			return fmt.Sprintf(`systemctl is-active %s`, shquote(s.Service)), nil
		default:
			return "", fmt.Errorf("fleet: unknown service action %q", s.Action)
		}
	case "sc":
		host := ""
		if !s.Local {
			host = `\\` + t.Name + ` `
		}
		switch action {
		case "start", "stop":
			return fmt.Sprintf(`sc %s%s "%s"`, host, action, s.Service), nil
		case "status":
			return fmt.Sprintf(`sc %squery "%s"`, host, s.Service), nil
		case "restart":
			// sc has no restart; stop, brief pause, start.
			return fmt.Sprintf(`cmd /c sc %sstop "%s" & ping -n 4 127.0.0.1 >NUL & sc %sstart "%s"`,
				host, s.Service, host, s.Service), nil
		default:
			return "", fmt.Errorf("fleet: unknown service action %q", s.Action)
		}
	default:
		return "", fmt.Errorf("fleet: unknown service backend %q", s.Backend)
	}
}

// Describe implements Task.
func (s ServiceTask) Describe() string {
	return fmt.Sprintf("svc %s %s (%s)", s.Action, s.Service, backendOr(s.Backend, "systemctl"))
}

// InstallTask runs a silent install. Kind selects the form:
//
//	msi -> msiexec /i "<pkg>" /qn <args>
//	exe -> "<pkg>" <args>           (silent flags go in args, e.g. /S)
//	sh  -> <pkg> <args>             (generic; e.g. "dnf install -y htop")
//
// The classic silent-install pattern (`msiexec /qn ALLUSERS=1`). Installs run ON the
// box, so pair this with the ssh (or winrm) transport.
type InstallTask struct {
	Package string
	Args    string
	Kind    string // msi | exe | sh
}

// Command implements Task.
func (i InstallTask) Command(_ Target) (string, error) {
	if i.Package == "" {
		return "", fmt.Errorf("fleet: InstallTask requires a Package")
	}
	args := strings.TrimSpace(i.Args)
	switch i.Kind {
	case "msi", "":
		cmd := fmt.Sprintf(`msiexec /i "%s" /qn`, i.Package)
		if args != "" {
			cmd += " " + args
		}
		return cmd, nil
	case "exe":
		cmd := fmt.Sprintf(`"%s"`, i.Package)
		if args != "" {
			cmd += " " + args
		}
		return cmd, nil
	case "sh":
		if args != "" {
			return i.Package + " " + args, nil
		}
		return i.Package, nil
	default:
		return "", fmt.Errorf("fleet: unknown install kind %q", i.Kind)
	}
}

// Describe implements Task.
func (i InstallTask) Describe() string {
	return fmt.Sprintf("install %s (%s)", i.Package, backendOr(i.Kind, "msi"))
}

// CopyTask pushes files to each target. Backend "robocopy" mirrors to a remote
// admin share (\\host); "scp"/"rsync" push over ssh from the control host.
//
// The classic mirror-push pattern (`robocopy ... /MIR`).
type CopyTask struct {
	Src     string
	Dst     string
	Backend string // robocopy | scp | rsync
	Mirror  bool   // robocopy /MIR
	SSHUser string // scp/rsync: user@host
}

// Command implements Task.
func (c CopyTask) Command(t Target) (string, error) {
	if c.Src == "" || c.Dst == "" {
		return "", fmt.Errorf("fleet: CopyTask requires Src and Dst")
	}
	switch c.Backend {
	case "robocopy", "":
		flags := ""
		if c.Mirror {
			flags = " /MIR"
		}
		return fmt.Sprintf(`robocopy "%s" "\\%s\%s"%s`, c.Src, t.Name, c.Dst, flags), nil
	case "scp":
		return fmt.Sprintf(`scp -r %s %s:%s`, shquote(c.Src), userAt(c.SSHUser, t.Name), shquote(c.Dst)), nil
	case "rsync":
		return fmt.Sprintf(`rsync -a %s %s:%s`, shquote(c.Src), userAt(c.SSHUser, t.Name), shquote(c.Dst)), nil
	default:
		return "", fmt.Errorf("fleet: unknown copy backend %q", c.Backend)
	}
}

// Describe implements Task.
func (c CopyTask) Describe() string {
	return fmt.Sprintf("push %s -> %s (%s)", c.Src, c.Dst, backendOr(c.Backend, "robocopy"))
}

// RebootTask reboots each target. Backend "win" -> shutdown /r /m \\host;
// "linux" -> shutdown -r on the box (over ssh).
type RebootTask struct {
	Backend  string // win | linux
	DelaySec int
	Message  string
	Sudo     bool // linux: prefix sudo
}

// Command implements Task.
func (r RebootTask) Command(t Target) (string, error) {
	switch r.Backend {
	case "win", "":
		msg := r.Message
		if msg == "" {
			msg = "winadmin reboot"
		}
		return fmt.Sprintf(`shutdown /r /m \\%s /t %d /c "%s"`, t.Name, r.DelaySec, msg), nil
	case "linux":
		mins := (r.DelaySec + 59) / 60
		prefix := ""
		if r.Sudo {
			prefix = "sudo "
		}
		return fmt.Sprintf(`%sshutdown -r +%d`, prefix, mins), nil
	default:
		return "", fmt.Errorf("fleet: unknown reboot backend %q", r.Backend)
	}
}

// Describe implements Task.
func (r RebootTask) Describe() string { return "reboot (" + backendOr(r.Backend, "win") + ")" }

// ProcKillTask terminates a process by image/name. Backend "taskkill" targets
// Windows remotely; "pkill" runs on the box (over ssh).
type ProcKillTask struct {
	Image   string
	Backend string // taskkill | pkill
	Force   bool
}

// Command implements Task.
func (p ProcKillTask) Command(t Target) (string, error) {
	if p.Image == "" {
		return "", fmt.Errorf("fleet: ProcKillTask requires an Image")
	}
	switch p.Backend {
	case "taskkill", "":
		force := ""
		if p.Force {
			force = " /f"
		}
		return fmt.Sprintf(`taskkill /s %s /im "%s"%s`, t.Name, p.Image, force), nil
	case "pkill":
		sig := ""
		if p.Force {
			sig = "-9 "
		}
		return fmt.Sprintf(`pkill %s-f %s`, sig, shquote(p.Image)), nil
	default:
		return "", fmt.Errorf("fleet: unknown proc backend %q", p.Backend)
	}
}

// Describe implements Task.
func (p ProcKillTask) Describe() string {
	return fmt.Sprintf("proc kill %s (%s)", p.Image, backendOr(p.Backend, "taskkill"))
}

// SchTask manages a Windows Scheduled Task across machines (schtasks /s \\host).
type SchTask struct {
	Name     string
	Action   string // run | delete | query | create
	Program  string // create: program to run
	Schedule string // create: ONLOGON | DAILY | HOURLY | ONSTART | ONCE ...
}

// Command implements Task.
func (s SchTask) Command(t Target) (string, error) {
	if s.Name == "" {
		return "", fmt.Errorf("fleet: SchTask requires a Name")
	}
	switch strings.ToLower(s.Action) {
	case "run":
		return fmt.Sprintf(`schtasks /s %s /run /tn "%s"`, t.Name, s.Name), nil
	case "delete":
		return fmt.Sprintf(`schtasks /s %s /delete /tn "%s" /f`, t.Name, s.Name), nil
	case "query", "":
		return fmt.Sprintf(`schtasks /s %s /query /tn "%s"`, t.Name, s.Name), nil
	case "create":
		if s.Program == "" {
			return "", fmt.Errorf("fleet: SchTask create requires Program")
		}
		sc := s.Schedule
		if sc == "" {
			sc = "ONLOGON"
		}
		return fmt.Sprintf(`schtasks /s %s /create /tn "%s" /tr "%s" /sc %s /f`, t.Name, s.Name, s.Program, sc), nil
	default:
		return "", fmt.Errorf("fleet: unknown task action %q", s.Action)
	}
}

// Describe implements Task.
func (s SchTask) Describe() string { return fmt.Sprintf("task %s %s", s.Action, s.Name) }

// LocalGroupTask adds/removes a member of a local group. Runs ON the box (net
// localgroup has no \\host form), so pair with the ssh/winrm transport. Pairs
// with the groups package.
type LocalGroupTask struct {
	Group  string
	Member string
	Action string // add | remove
}

// Command implements Task.
func (l LocalGroupTask) Command(_ Target) (string, error) {
	if l.Group == "" || l.Member == "" {
		return "", fmt.Errorf("fleet: LocalGroupTask requires Group and Member")
	}
	verb := "/add"
	if strings.ToLower(l.Action) == "remove" {
		verb = "/delete"
	}
	return fmt.Sprintf(`net localgroup "%s" "%s" %s`, l.Group, l.Member, verb), nil
}

// Describe implements Task.
func (l LocalGroupTask) Describe() string {
	return fmt.Sprintf("localgroup %s %s -> %s", l.Action, l.Member, l.Group)
}

// FirewallTask adds or deletes a Windows firewall rule (netsh advfirewall). Runs
// on the box.
type FirewallTask struct {
	Action   string // add | delete
	Name     string
	Dir      string // in | out
	FWAction string // allow | block
	Protocol string // tcp | udp
	Port     string
}

// Command implements Task.
func (f FirewallTask) Command(_ Target) (string, error) {
	if f.Name == "" {
		return "", fmt.Errorf("fleet: FirewallTask requires a Name")
	}
	switch strings.ToLower(f.Action) {
	case "delete":
		return fmt.Sprintf(`netsh advfirewall firewall delete rule name="%s"`, f.Name), nil
	case "add", "":
		dir := orStr(f.Dir, "in")
		act := orStr(f.FWAction, "allow")
		proto := orStr(f.Protocol, "tcp")
		cmd := fmt.Sprintf(`netsh advfirewall firewall add rule name="%s" dir=%s action=%s protocol=%s`,
			f.Name, dir, act, proto)
		if f.Port != "" {
			cmd += " localport=" + f.Port
		}
		return cmd, nil
	default:
		return "", fmt.Errorf("fleet: unknown firewall action %q", f.Action)
	}
}

// Describe implements Task.
func (f FirewallTask) Describe() string { return fmt.Sprintf("firewall %s %s", f.Action, f.Name) }

func orStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func backendOr(b, def string) string {
	if b == "" {
		return def
	}
	return b
}

func userAt(user, host string) string {
	if user == "" {
		return host
	}
	return user + "@" + host
}

// shquote single-quotes a string for /bin/sh. See ShellQuote.
func shquote(s string) string { return ShellQuote(s) }
