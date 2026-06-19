package fleet

import (
	"context"
	"strings"
	"testing"
)

func cmdOf(t *testing.T, task Task, name string) string {
	t.Helper()
	c, err := task.Command(Target{Name: name})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return c
}

func TestServiceTaskSystemctl(t *testing.T) {
	got := cmdOf(t, ServiceTask{Service: "nginx", Action: "restart", Backend: "systemctl"}, "box")
	if got != "systemctl restart 'nginx'" {
		t.Fatalf("got %q", got)
	}
}

func TestServiceTaskScRemote(t *testing.T) {
	got := cmdOf(t, ServiceTask{Service: "Spooler", Action: "stop", Backend: "sc"}, "PRINTSRV")
	if got != `sc \\PRINTSRV stop "Spooler"` {
		t.Fatalf("got %q", got)
	}
	restart := cmdOf(t, ServiceTask{Service: "Spooler", Action: "restart", Backend: "sc"}, "PRINTSRV")
	if !strings.Contains(restart, `sc \\PRINTSRV stop "Spooler"`) || !strings.Contains(restart, `start "Spooler"`) {
		t.Fatalf("restart not stop+start: %q", restart)
	}
}

func TestInstallTask(t *testing.T) {
	msi := cmdOf(t, InstallTask{Package: `\\dist\App.msi`, Args: "ALLUSERS=1", Kind: "msi"}, "x")
	if msi != `msiexec /i "\\dist\App.msi" /qn ALLUSERS=1` {
		t.Fatalf("msi: %q", msi)
	}
	sh := cmdOf(t, InstallTask{Package: "dnf install -y", Args: "htop", Kind: "sh"}, "x")
	if sh != "dnf install -y htop" {
		t.Fatalf("sh: %q", sh)
	}
}

func TestCopyTaskRobocopyMirror(t *testing.T) {
	got := cmdOf(t, CopyTask{Src: `C:\payload`, Dst: `C$\App`, Backend: "robocopy", Mirror: true}, "BR01")
	if got != `robocopy "C:\payload" "\\BR01\C$\App" /MIR` {
		t.Fatalf("got %q", got)
	}
}

func TestProcKillTaskkill(t *testing.T) {
	got := cmdOf(t, ProcKillTask{Image: "stuck.exe", Backend: "taskkill", Force: true}, "WS42")
	if got != `taskkill /s WS42 /im "stuck.exe" /f` {
		t.Fatalf("got %q", got)
	}
}

func TestRebootTaskWin(t *testing.T) {
	got := cmdOf(t, RebootTask{Backend: "win", DelaySec: 30}, "WS42")
	if !strings.HasPrefix(got, `shutdown /r /m \\WS42 /t 30`) {
		t.Fatalf("got %q", got)
	}
}

func TestSchTask(t *testing.T) {
	run := cmdOf(t, SchTask{Name: "Nightly", Action: "run"}, "WS1")
	if run != `schtasks /s WS1 /run /tn "Nightly"` {
		t.Fatalf("run: %q", run)
	}
	create := cmdOf(t, SchTask{Name: "Nightly", Action: "create", Program: `C:\j.exe`, Schedule: "DAILY"}, "WS1")
	if !strings.Contains(create, `/create /tn "Nightly" /tr "C:\j.exe" /sc DAILY`) {
		t.Fatalf("create: %q", create)
	}
}

func TestLocalGroupTask(t *testing.T) {
	got := cmdOf(t, LocalGroupTask{Group: "Administrators", Member: `CORP\jdoe`, Action: "add"}, "WS1")
	if got != `net localgroup "Administrators" "CORP\jdoe" /add` {
		t.Fatalf("got %q", got)
	}
}

func TestFirewallTask(t *testing.T) {
	add := cmdOf(t, FirewallTask{Action: "add", Name: "Block SMB", FWAction: "block", Port: "445"}, "WS1")
	if !strings.Contains(add, `name="Block SMB" dir=in action=block protocol=tcp localport=445`) {
		t.Fatalf("add: %q", add)
	}
	del := cmdOf(t, FirewallTask{Action: "delete", Name: "Block SMB"}, "WS1")
	if del != `netsh advfirewall firewall delete rule name="Block SMB"` {
		t.Fatalf("del: %q", del)
	}
}

func TestSplitBatches(t *testing.T) {
	ts := []Target{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}}
	b := splitBatches(ts, 1, 2) // canary 1, then waves of 2
	if len(b) != 4 || len(b[0]) != 1 || len(b[1]) != 2 || len(b[3]) != 1 {
		t.Fatalf("unexpected batches: %v", b)
	}
	one := splitBatches(ts, 0, 0) // no staging => single batch
	if len(one) != 1 || len(one[0]) != 6 {
		t.Fatalf("expected single batch of 6, got %v", one)
	}
}

func TestRunWavesHealthGateAborts(t *testing.T) {
	rt := &recordingTransport{}
	plan := Plan{Inventory: invOf("a", "b", "c", "d"), Task: CommandTask{Template: "x"}, Transport: rt}
	// canary of 1, waves of 1, health check always fails -> abort after canary.
	stage := StageOptions{Canary: 1, Wave: 1, HealthCmd: "false"}
	sum, _, err := RunWaves(context.Background(), plan, Options{Parallelism: 2, Logger: quietLogger()}, stage, nil, nil)
	if err == nil {
		t.Fatal("expected abort error from failed health gate")
	}
	if sum.Skipped == 0 {
		t.Fatalf("expected remaining targets skipped, got %+v", sum)
	}
}

func TestRunWavesHealthGatePasses(t *testing.T) {
	rt := &recordingTransport{}
	plan := Plan{Inventory: invOf("a", "b", "c", "d"), Task: CommandTask{Template: "x"}, Transport: rt}
	stage := StageOptions{Canary: 1, Wave: 2, HealthCmd: "true"}
	sum, _, err := RunWaves(context.Background(), plan, Options{Parallelism: 2, Logger: quietLogger()}, stage, nil, nil)
	if err != nil {
		t.Fatalf("unexpected abort: %v", err)
	}
	if sum.Succeeded != 4 {
		t.Fatalf("expected all 4 to run, got %+v", sum)
	}
}
