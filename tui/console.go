package tui

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jasondostal/winadmin/fleet"
)

type fieldKind int

const (
	fText fieldKind = iota
	fSelect
	fToggle
	fButton
)

type field struct {
	key   string
	label string
	kind  fieldKind
	input textinput.Model
	opts  []string
	sel   int
	on    bool
	show  func(c *Console) bool
}

func (f field) value() string {
	switch f.kind {
	case fSelect:
		return f.opts[f.sel]
	case fToggle:
		if f.on {
			return "on"
		}
		return "off"
	default:
		return f.input.Value()
	}
}

// Console is the interactive run builder. It speaks every verb the CLI does:
// pick a task, fill its fields, set transport + staging, then hand off to the
// live Watcher.
type Console struct {
	fields   []field
	focus    int
	width    int
	height   int
	err      string
	browsing bool
	fp       filepicker.Model
}

func always(*Console) bool { return true }

func ti(placeholder, val string) textinput.Model {
	m := textinput.New()
	m.Placeholder = placeholder
	m.SetValue(val)
	m.Prompt = ""
	return m
}

// whenTask shows a field only for the given task types.
func whenTask(wants ...string) func(*Console) bool {
	set := map[string]bool{}
	for _, w := range wants {
		set[w] = true
	}
	return func(c *Console) bool { return set[c.tasktype()] }
}

func notTask(notWant string) func(*Console) bool {
	return func(c *Console) bool { return c.tasktype() != notWant }
}

func whenSSH(c *Console) bool { return c.get("transport") == "ssh" && c.tasktype() != "ldapset" }

// NewConsole builds the setup form. Verb-specific fields are hidden unless their
// task is selected, so the form only ever shows what's relevant.
func NewConsole() Console {
	c := Console{
		fields: []field{
			{key: "tasktype", label: "Task", kind: fSelect, show: always,
				opts: []string{"run", "svc", "install", "push", "reboot", "proc", "regset", "deldir", "task", "localgroup", "firewall", "ldapset"}},

			// inventory (every verb except ldapset, which queries LDAP)
			{key: "inventory", label: "Target list", kind: fText, input: ti("path to hosts.txt", ""), show: notTask("ldapset")},

			// run
			{key: "cmd", label: "Command", kind: fText, input: ti("echo {{.Name}}", ""), show: whenTask("run")},

			// svc
			{key: "svc_name", label: "Service", kind: fText, input: ti("nginx / Spooler", ""), show: whenTask("svc")},
			{key: "svc_action", label: "Action", kind: fSelect, opts: []string{"status", "start", "stop", "restart"}, show: whenTask("svc")},
			{key: "svc_backend", label: "Backend", kind: fSelect, opts: []string{"systemctl", "sc"}, show: whenTask("svc")},
			{key: "svc_sudo", label: "Sudo (systemctl)", kind: fToggle, show: whenTask("svc")},

			// install
			{key: "inst_pkg", label: "Package", kind: fText, input: ti(`\\dist\App.msi`, ""), show: whenTask("install")},
			{key: "inst_args", label: "Args", kind: fText, input: ti("ALLUSERS=1", ""), show: whenTask("install")},
			{key: "inst_kind", label: "Kind", kind: fSelect, opts: []string{"msi", "exe", "sh"}, show: whenTask("install")},

			// push
			{key: "push_src", label: "Source", kind: fText, input: ti(`\\dist\payload`, ""), show: whenTask("push")},
			{key: "push_dst", label: "Dest", kind: fText, input: ti(`C$\App`, ""), show: whenTask("push")},
			{key: "push_backend", label: "Backend", kind: fSelect, opts: []string{"robocopy", "scp", "rsync"}, show: whenTask("push")},
			{key: "push_mirror", label: "Mirror (/MIR)", kind: fToggle, show: whenTask("push")},

			// reboot
			{key: "rb_backend", label: "Backend", kind: fSelect, opts: []string{"win", "linux"}, show: whenTask("reboot")},
			{key: "rb_delay", label: "Delay (sec)", kind: fText, input: ti("30", "30"), show: whenTask("reboot")},

			// proc
			{key: "proc_image", label: "Process", kind: fText, input: ti("stuck.exe", ""), show: whenTask("proc")},
			{key: "proc_backend", label: "Backend", kind: fSelect, opts: []string{"taskkill", "pkill"}, show: whenTask("proc")},
			{key: "proc_force", label: "Force", kind: fToggle, show: whenTask("proc")},

			// regset
			{key: "hive", label: "Hive", kind: fText, input: ti("HKLM", "HKLM"), show: whenTask("regset")},
			{key: "key", label: "Key", kind: fText, input: ti(`Software\Acme\App`, ""), show: whenTask("regset")},
			{key: "name", label: "Value name", kind: fText, input: ti("Enabled", ""), show: whenTask("regset")},
			{key: "rtype", label: "Value type", kind: fText, input: ti("REG_DWORD", "REG_DWORD"), show: whenTask("regset")},
			{key: "data", label: "Value data", kind: fText, input: ti("1", ""), show: whenTask("regset")},

			// deldir
			{key: "path", label: "Directory", kind: fText, input: ti(`C$\Temp\junk`, ""), show: whenTask("deldir")},

			// task (schtasks)
			{key: "tk_name", label: "Task name", kind: fText, input: ti("NightlyJob", ""), show: whenTask("task")},
			{key: "tk_action", label: "Action", kind: fSelect, opts: []string{"query", "run", "delete", "create"}, show: whenTask("task")},
			{key: "tk_program", label: "Program", kind: fText, input: ti(`C:\j.exe`, ""), show: whenTask("task")},
			{key: "tk_schedule", label: "Schedule", kind: fText, input: ti("ONLOGON", "ONLOGON"), show: whenTask("task")},

			// localgroup
			{key: "lg_group", label: "Group", kind: fText, input: ti("Administrators", "Administrators"), show: whenTask("localgroup")},
			{key: "lg_member", label: "Member", kind: fText, input: ti(`CORP\jdoe`, ""), show: whenTask("localgroup")},
			{key: "lg_action", label: "Action", kind: fSelect, opts: []string{"add", "remove"}, show: whenTask("localgroup")},

			// firewall
			{key: "fw_name", label: "Rule name", kind: fText, input: ti("Block SMB", ""), show: whenTask("firewall")},
			{key: "fw_action", label: "Action", kind: fSelect, opts: []string{"add", "delete"}, show: whenTask("firewall")},
			{key: "fw_dir", label: "Direction", kind: fSelect, opts: []string{"in", "out"}, show: whenTask("firewall")},
			{key: "fw_fwaction", label: "Allow/Block", kind: fSelect, opts: []string{"allow", "block"}, show: whenTask("firewall")},
			{key: "fw_proto", label: "Protocol", kind: fSelect, opts: []string{"tcp", "udp"}, show: whenTask("firewall")},
			{key: "fw_port", label: "Port", kind: fText, input: ti("445", ""), show: whenTask("firewall")},

			// ldapset
			{key: "ld_url", label: "LDAP URL", kind: fText, input: ti("ldap://dc01.corp.com", ""), show: whenTask("ldapset")},
			{key: "ld_binddn", label: "Bind DN", kind: fText, input: ti("CN=svc,DC=corp,DC=com", ""), show: whenTask("ldapset")},
			{key: "ld_bindpw", label: "Bind password", kind: fText, input: ti("", ""), show: whenTask("ldapset")},
			{key: "ld_base", label: "Base / OU", kind: fText, input: ti("OU=Tellers,DC=corp,DC=com", ""), show: whenTask("ldapset")},
			{key: "ld_filter", label: "Filter", kind: fText, input: ti("(objectClass=user)", ""), show: whenTask("ldapset")},
			{key: "ld_attr", label: "Attribute", kind: fText, input: ti("department", ""), show: whenTask("ldapset")},
			{key: "ld_value", label: "Value", kind: fText, input: ti("Retail", ""), show: whenTask("ldapset")},
			{key: "ld_op", label: "Op", kind: fSelect, opts: []string{"replace", "add", "delete"}, show: whenTask("ldapset")},

			// transport
			{key: "transport", label: "Transport", kind: fSelect, opts: []string{"local", "ssh"}, show: notTask("ldapset")},
			{key: "ssh_user", label: "SSH user", kind: fText, input: ti("ec2-user", ""), show: whenSSH},
			{key: "ssh_key", label: "SSH key", kind: fText, input: ti("~/.ssh/id_ed25519", "~/.ssh/id_ed25519"), show: whenSSH},

			// shared run options
			{key: "parallel", label: "Parallel", kind: fText, input: ti("15", "15"), show: always},
			{key: "whatif", label: "What-if (dry run)", kind: fToggle, on: true, show: always},
			{key: "shuffle", label: "Shuffle order", kind: fToggle, show: always},

			// staged rollout
			{key: "canary", label: "Canary (N first)", kind: fText, input: ti("0", "0"), show: always},
			{key: "wave", label: "Wave size", kind: fText, input: ti("0", "0"), show: always},
			{key: "health", label: "Health check cmd", kind: fText, input: ti("(optional)", ""), show: always},

			{key: "launch", label: "▶ Launch run", kind: fButton, show: always},
		},
	}
	fp := filepicker.New()
	if wd, err := os.Getwd(); err == nil {
		fp.CurrentDirectory = wd
	}
	fp.Height = 14
	c.fp = fp
	c.syncFocus()
	return c
}

func (c *Console) setText(key, val string) {
	for i := range c.fields {
		if c.fields[i].key == key {
			c.fields[i].input.SetValue(val)
			return
		}
	}
}

func (c Console) tasktype() string {
	for _, f := range c.fields {
		if f.key == "tasktype" {
			return f.opts[f.sel]
		}
	}
	return "run"
}

func (c *Console) visible(i int) bool { return c.fields[i].show(c) }

func (c *Console) syncFocus() {
	for i := range c.fields {
		if c.fields[i].kind == fText {
			if i == c.focus {
				c.fields[i].input.Focus()
			} else {
				c.fields[i].input.Blur()
			}
		}
	}
}

func (c *Console) moveFocus(delta int) {
	n := len(c.fields)
	for k := 0; k < n; k++ {
		c.focus = (c.focus + delta + n) % n
		if c.visible(c.focus) {
			break
		}
	}
	c.syncFocus()
}

func (c Console) Init() tea.Cmd { return textinput.Blink }

func (c Console) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// File-browser sub-mode for the Target list field.
	if c.browsing {
		if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
			c.browsing = false
			return c, nil
		}
		var cmd tea.Cmd
		c.fp, cmd = c.fp.Update(msg)
		if did, path := c.fp.DidSelectFile(msg); did {
			c.setText("inventory", path)
			c.browsing = false
		}
		return c, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width, c.height = msg.Width, msg.Height
		c.fp.Height = maxInt(6, msg.Height-8)
		return c, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return c, tea.Quit
		case "tab", "down":
			c.moveFocus(1)
			return c, nil
		case "shift+tab", "up":
			c.moveFocus(-1)
			return c, nil
		case "ctrl+g":
			return c.launch()
		case "ctrl+o":
			if c.fields[c.focus].key == "inventory" {
				c.browsing = true
				return c, c.fp.Init()
			}
		case "ctrl+y":
			// Accept the top service-name type-ahead match (fills the short name).
			if c.fields[c.focus].key == "svc_name" {
				if m := matchServices(c.get("svc_name"), 1); len(m) > 0 {
					c.setText("svc_name", m[0].Short)
				}
				return c, nil
			}
		}

		f := &c.fields[c.focus]
		switch f.kind {
		case fSelect:
			switch msg.String() {
			case "left", "h":
				f.sel = (f.sel - 1 + len(f.opts)) % len(f.opts)
			case "right", "l", " ":
				f.sel = (f.sel + 1) % len(f.opts)
			}
			return c, nil
		case fToggle:
			if msg.String() == " " || msg.String() == "enter" {
				f.on = !f.on
			}
			return c, nil
		case fButton:
			if msg.String() == "enter" {
				return c.launch()
			}
			return c, nil
		case fText:
			if msg.String() == "enter" {
				c.moveFocus(1)
				return c, nil
			}
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return c, cmd
		}
	}
	return c, nil
}

func (c Console) get(key string) string {
	for _, f := range c.fields {
		if f.key == key {
			return f.value()
		}
	}
	return ""
}

func (c Console) launch() (tea.Model, tea.Cmd) {
	tt := c.tasktype()

	// Build the task and inventory for the chosen verb.
	var task fleet.Task
	var inv *fleet.Inventory
	var err error

	if tt == "ldapset" {
		inv, task, err = c.buildLdap()
		if err != nil {
			c.err = err.Error()
			return c, nil
		}
	} else {
		invPath := strings.TrimSpace(c.get("inventory"))
		if invPath == "" {
			c.err = "Target list is required"
			return c, nil
		}
		inv, err = fleet.LoadInventory(invPath)
		if err != nil {
			c.err = "inventory: " + err.Error()
			return c, nil
		}
		task, err = c.buildTask(tt)
		if err != nil {
			c.err = err.Error()
			return c, nil
		}
	}
	if inv.Len() == 0 {
		c.err = "no targets"
		return c, nil
	}

	parallel := 15
	if n, e := strconv.Atoi(strings.TrimSpace(c.get("parallel"))); e == nil && n > 0 {
		parallel = n
	}
	if c.get("shuffle") == "on" {
		inv.Shuffle(rand.Shuffle)
	}

	var tr fleet.Transport = fleet.LocalTransport{}
	if tt != "ldapset" && c.get("transport") == "ssh" {
		tr = fleet.SSHTransport{User: c.get("ssh_user"), KeyPath: expandHome(c.get("ssh_key")), UseAgent: true}
	}

	plan := fleet.Plan{Inventory: inv, Task: task, Transport: tr}
	opts := fleet.Options{Parallelism: parallel, DryRun: c.get("whatif") == "on", Logger: quietSlog()}
	stage := fleet.StageOptions{
		Canary:    atoi(c.get("canary")),
		Wave:      atoi(c.get("wave")),
		HealthCmd: strings.TrimSpace(c.get("health")),
	}

	w := NewWatcher(plan, opts, stage)
	return w, w.Init()
}

func (c Console) buildTask(tt string) (fleet.Task, error) {
	switch tt {
	case "run":
		if strings.TrimSpace(c.get("cmd")) == "" {
			return nil, fmt.Errorf("Command is required")
		}
		return fleet.CommandTask{Template: c.get("cmd")}, nil
	case "svc":
		if strings.TrimSpace(c.get("svc_name")) == "" {
			return nil, fmt.Errorf("Service name is required")
		}
		return fleet.ServiceTask{Service: c.get("svc_name"), Action: c.get("svc_action"),
			Backend: c.get("svc_backend"), Sudo: c.get("svc_sudo") == "on"}, nil
	case "install":
		if strings.TrimSpace(c.get("inst_pkg")) == "" {
			return nil, fmt.Errorf("Package is required")
		}
		return fleet.InstallTask{Package: c.get("inst_pkg"), Args: c.get("inst_args"), Kind: c.get("inst_kind")}, nil
	case "push":
		return fleet.CopyTask{Src: c.get("push_src"), Dst: c.get("push_dst"),
			Backend: c.get("push_backend"), Mirror: c.get("push_mirror") == "on", SSHUser: c.get("ssh_user")}, nil
	case "reboot":
		return fleet.RebootTask{Backend: c.get("rb_backend"), DelaySec: atoi(c.get("rb_delay"))}, nil
	case "proc":
		if strings.TrimSpace(c.get("proc_image")) == "" {
			return nil, fmt.Errorf("Process image is required")
		}
		return fleet.ProcKillTask{Image: c.get("proc_image"), Backend: c.get("proc_backend"), Force: c.get("proc_force") == "on"}, nil
	case "regset":
		if strings.TrimSpace(c.get("key")) == "" {
			return nil, fmt.Errorf("Key is required")
		}
		return fleet.RegSetTask{Hive: c.get("hive"), Key: c.get("key"), Name: c.get("name"),
			Type: c.get("rtype"), Data: c.get("data")}, nil
	case "deldir":
		if strings.TrimSpace(c.get("path")) == "" {
			return nil, fmt.Errorf("Directory is required")
		}
		return fleet.DeleteDirTask{Path: c.get("path")}, nil
	case "task":
		if strings.TrimSpace(c.get("tk_name")) == "" {
			return nil, fmt.Errorf("Task name is required")
		}
		return fleet.SchTask{Name: c.get("tk_name"), Action: c.get("tk_action"),
			Program: c.get("tk_program"), Schedule: c.get("tk_schedule")}, nil
	case "localgroup":
		if strings.TrimSpace(c.get("lg_member")) == "" {
			return nil, fmt.Errorf("Member is required")
		}
		return fleet.LocalGroupTask{Group: c.get("lg_group"), Member: c.get("lg_member"), Action: c.get("lg_action")}, nil
	case "firewall":
		if strings.TrimSpace(c.get("fw_name")) == "" {
			return nil, fmt.Errorf("Rule name is required")
		}
		return fleet.FirewallTask{Action: c.get("fw_action"), Name: c.get("fw_name"), Dir: c.get("fw_dir"),
			FWAction: c.get("fw_fwaction"), Protocol: c.get("fw_proto"), Port: c.get("fw_port")}, nil
	}
	return nil, fmt.Errorf("unknown task %q", tt)
}

// buildLdap mirrors the CLI ldapset: an ldapsearch dynamic inventory + an
// ldapmodify task, with the bind password kept in a 0600 temp file (via -y).
func (c Console) buildLdap() (*fleet.Inventory, fleet.Task, error) {
	url := strings.TrimSpace(c.get("ld_url"))
	binddn := strings.TrimSpace(c.get("ld_binddn"))
	base := strings.TrimSpace(c.get("ld_base"))
	attr := strings.TrimSpace(c.get("ld_attr"))
	if url == "" || binddn == "" || base == "" || attr == "" {
		return nil, nil, fmt.Errorf("ldapset needs URL, bind DN, base and attribute")
	}
	filter := c.get("ld_filter")
	if strings.TrimSpace(filter) == "" {
		filter = "(&(objectCategory=person)(objectClass=user))"
	}
	pwFile, err := os.CreateTemp("", "fleet-ldap-*.secret")
	if err != nil {
		return nil, nil, err
	}
	_ = pwFile.Chmod(0o600)
	_, _ = pwFile.WriteString(c.get("ld_bindpw"))
	_ = pwFile.Close()

	conn := fmt.Sprintf(`-H %s -D %s -y %s`, shq(url), shq(binddn), shq(pwFile.Name()))
	invCmd := fmt.Sprintf(`ldapsearch -x -LLL -o ldif-wrap=no %s -b %s %s dn | sed -n 's/^dn: //p'`,
		conn, shq(base), shq(filter))
	ldif := fmt.Sprintf("dn: {{.Name}}\\nchangetype: modify\\n%s: %s\\n%s: %s\\n",
		c.get("ld_op"), attr, attr, c.get("ld_value"))
	taskCmd := fmt.Sprintf(`printf '%s' | ldapmodify -x %s`, ldif, conn)

	inv, err := fleet.InventoryFromCommand(context.Background(), invCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("ldapsearch: %w", err)
	}
	return inv, fleet.CommandTask{Template: taskCmd}, nil
}

func (c Console) View() string {
	if c.browsing {
		body := titleStyle.Render("Select target list") + "\n\n" +
			labelStyle.Render(c.fp.CurrentDirectory) + "\n" +
			c.fp.View() + "\n" +
			mutedStyle.Render("↑/↓ move · enter open/select · esc cancel")
		return boxStyle.Render(body) + "\n"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("fleet — run builder") + "\n\n")

	for i, f := range c.fields {
		if !c.visible(i) {
			continue
		}
		b.WriteString(c.renderField(f, i == c.focus) + "\n")
		// Service-name type-ahead suggestions, shown while the field is focused.
		if f.key == "svc_name" && i == c.focus {
			b.WriteString(c.serviceSuggestions())
		}
	}

	help := fmt.Sprintf("\n%s %s   %s %s   %s %s   %s %s",
		keyStyle.Render("tab"), mutedStyle.Render("next"),
		keyStyle.Render("←/→/space"), mutedStyle.Render("change"),
		keyStyle.Render("ctrl+g"), mutedStyle.Render("launch"),
		keyStyle.Render("esc"), mutedStyle.Render("quit"))
	b.WriteString(help + "\n")

	if c.err != "" {
		b.WriteString("\n" + failStyle.Render("✗ "+c.err) + "\n")
	}
	return boxStyle.Render(b.String()) + "\n"
}

// serviceSuggestions renders the type-ahead match list under the Service field.
func (c Console) serviceSuggestions() string {
	q := c.get("svc_name")
	if exactService(q) {
		return "        " + okStyle.Render("✓ "+q) + " " + mutedStyle.Render("(known service)") + "\n"
	}
	matches := matchServices(q, 5)
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	for i, s := range matches {
		marker := mutedStyle.Render("  ")
		short := mutedStyle.Render(padRight(s.Short, 18))
		if i == 0 {
			marker = focusStyle.Render("▸ ")
			short = focusStyle.Render(padRight(s.Short, 18))
		}
		b.WriteString("        " + marker + short + mutedStyle.Render(s.Display) + "\n")
	}
	b.WriteString("        " + keyStyle.Render("ctrl+y") + mutedStyle.Render(" use top match") + "\n")
	return b.String()
}

func (c Console) renderField(f field, focused bool) string {
	label := labelStyle.Render(padRight(f.label, 18))
	if focused {
		label = focusStyle.Render(padRight("▸ "+f.label, 18))
	}

	var val string
	switch f.kind {
	case fSelect:
		parts := make([]string, len(f.opts))
		for i, o := range f.opts {
			if i == f.sel {
				parts[i] = focusStyle.Render("(" + o + ")")
			} else {
				parts[i] = mutedStyle.Render(o)
			}
		}
		val = strings.Join(parts, " ")
	case fToggle:
		if f.on {
			val = okStyle.Render("[x] on")
		} else {
			val = mutedStyle.Render("[ ] off")
		}
	case fButton:
		if focused {
			return "  " + lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render(f.label)
		}
		return "  " + valueStyle.Render(f.label)
	case fText:
		val = f.input.View()
		if !focused && f.input.Value() == "" {
			val = mutedStyle.Render(f.input.Placeholder)
		}
		if f.key == "inventory" && focused {
			val += "  " + keyStyle.Render("(ctrl+o to browse)")
		}
	}
	return "  " + label + "  " + val
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + p[1:]
		}
	}
	return p
}
