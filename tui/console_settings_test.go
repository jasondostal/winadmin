package tui

import (
	"path/filepath"
	"testing"

	"github.com/jasondostal/winadmin/config"
)

// setSetting assigns a value to a settings-form field by key.
func (c *Console) setSetting(key, val string) {
	for i := range c.settingFields {
		if c.settingFields[i].key != key {
			continue
		}
		if c.settingFields[i].kind == fSelect {
			for j, o := range c.settingFields[i].opts {
				if o == val {
					c.settingFields[i].sel = j
				}
			}
		} else {
			c.settingFields[i].input.SetValue(val)
		}
		return
	}
}

func TestConsoleSettingsSaveAndApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("FLEET_CONFIG", path)

	c := NewConsole()
	c.enterSettings()
	if !c.settingsMode {
		t.Fatal("ctrl+e should enter settings mode")
	}

	c.setSetting("s_parallel", "30")
	c.setSetting("s_transport", "ssh")
	c.setSetting("s_ssh_user", "deploy")
	c.setSetting("s_hosts", "/etc/fleet/hosts.txt")

	m, _ := c.saveSettings()
	saved := m.(Console)

	// Persisted to the config file.
	s, err := config.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.Parallelism != 30 || s.Transport != "ssh" || s.SSHUser != "deploy" || s.DefaultHosts != "/etc/fleet/hosts.txt" {
		t.Errorf("persisted settings = %+v", s)
	}

	// Applied live to the run builder.
	if saved.get("parallel") != "30" {
		t.Errorf("builder parallel = %q, want 30", saved.get("parallel"))
	}
	if saved.get("transport") != "ssh" {
		t.Errorf("builder transport = %q, want ssh", saved.get("transport"))
	}
	if saved.get("ssh_user") != "deploy" {
		t.Errorf("builder ssh_user = %q, want deploy", saved.get("ssh_user"))
	}
	if saved.get("inventory") != "/etc/fleet/hosts.txt" {
		t.Errorf("builder inventory = %q, want the default hosts path", saved.get("inventory"))
	}
}

// TestConsolePrefillsFromSettings verifies a saved config seeds a fresh builder.
func TestConsolePrefillsFromSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("FLEET_CONFIG", path)
	if _, err := config.SaveSettings(config.Settings{Parallelism: 50, SSHUser: "ops"}); err != nil {
		t.Fatal(err)
	}

	c := NewConsole()
	if c.get("parallel") != "50" {
		t.Errorf("fresh builder parallel = %q, want 50 from settings", c.get("parallel"))
	}
	if c.get("ssh_user") != "ops" {
		t.Errorf("fresh builder ssh_user = %q, want ops from settings", c.get("ssh_user"))
	}
}
