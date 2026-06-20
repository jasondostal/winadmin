package tui

import (
	"testing"

	"github.com/jasondostal/winadmin/fleet"
)

// set assigns a value to a console field by key (text value or select option).
func (c *Console) set(key, val string) {
	for i := range c.fields {
		if c.fields[i].key != key {
			continue
		}
		if c.fields[i].kind == fSelect {
			for j, o := range c.fields[i].opts {
				if o == val {
					c.fields[i].sel = j
				}
			}
		} else {
			c.fields[i].input.SetValue(val)
		}
		return
	}
}

// TestConsoleBuildsEveryVerb verifies the run builder constructs the correct
// task type for each CLI verb — i.e. the TUI is at parity with the util.
func TestConsoleBuildsEveryVerb(t *testing.T) {
	cases := []struct {
		task   string
		set    map[string]string
		expect fleet.Task
	}{
		{"run", map[string]string{"cmd": "uptime"}, fleet.CommandTask{}},
		{"gather", map[string]string{"gather_cmd": "df -h"}, fleet.CommandTask{}},
		{"svc", map[string]string{"svc_name": "nginx"}, fleet.ServiceTask{}},
		{"install", map[string]string{"inst_pkg": "a.msi"}, fleet.InstallTask{}},
		{"push", map[string]string{"push_src": "a", "push_dst": "b"}, fleet.CopyTask{}},
		{"reboot", map[string]string{}, fleet.RebootTask{}},
		{"proc", map[string]string{"proc_image": "x.exe"}, fleet.ProcKillTask{}},
		{"regset", map[string]string{"key": "Software\\X"}, fleet.RegSetTask{}},
		{"deldir", map[string]string{"path": "C$\\tmp"}, fleet.DeleteDirTask{}},
		{"task", map[string]string{"tk_name": "Job"}, fleet.SchTask{}},
		{"localgroup", map[string]string{"lg_member": "jdoe"}, fleet.LocalGroupTask{}},
		{"firewall", map[string]string{"fw_name": "Rule"}, fleet.FirewallTask{}},
	}
	for _, tc := range cases {
		c := NewConsole()
		c.set("tasktype", tc.task)
		for k, v := range tc.set {
			c.set(k, v)
		}
		got, err := c.buildTask(tc.task)
		if err != nil {
			t.Fatalf("%s: %v", tc.task, err)
		}
		if gotT, wantT := sprintType(got), sprintType(tc.expect); gotT != wantT {
			t.Fatalf("%s: built %s, want %s", tc.task, gotT, wantT)
		}
		// the rendered command must be non-empty for a representative target
		if cmd, err := got.Command(fleet.Target{Name: "HOST"}); err != nil || cmd == "" {
			t.Fatalf("%s: empty/invalid command: %q (%v)", tc.task, cmd, err)
		}
	}
}

func sprintType(t fleet.Task) string {
	switch t.(type) {
	case fleet.CommandTask:
		return "Command"
	case fleet.ServiceTask:
		return "Service"
	case fleet.InstallTask:
		return "Install"
	case fleet.CopyTask:
		return "Copy"
	case fleet.RebootTask:
		return "Reboot"
	case fleet.ProcKillTask:
		return "ProcKill"
	case fleet.RegSetTask:
		return "RegSet"
	case fleet.DeleteDirTask:
		return "DeleteDir"
	case fleet.SchTask:
		return "Sch"
	case fleet.LocalGroupTask:
		return "LocalGroup"
	case fleet.FirewallTask:
		return "Firewall"
	}
	return "Unknown"
}
