package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sendC(c Console, msgs ...tea.Msg) Console {
	var m tea.Model = c
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m.(Console)
}

func TestTaskPicker_OpenFilterSelect(t *testing.T) {
	c := NewConsole() // focus starts on the Task field (index 0)
	c = sendC(c, key("enter"))
	if !c.taskPicking {
		t.Fatal("enter on the Task field should open the palette")
	}
	c = sendC(c, key("f"), key("i"), key("r"), key("e"))
	if m := c.taskMatches(); len(m) != 1 || m[0].verb != "firewall" {
		t.Fatalf("filter 'fire' => %v, want [firewall]", m)
	}
	c = sendC(c, key("enter"))
	if c.taskPicking {
		t.Error("enter should close the palette")
	}
	if c.tasktype() != "firewall" {
		t.Errorf("picked %q, want firewall", c.tasktype())
	}
}

func TestTaskPicker_EscKeepsCurrent(t *testing.T) {
	c := NewConsole()
	start := c.tasktype()
	c = sendC(c, key("enter"), key("s"), key("v"), key("esc"))
	if c.taskPicking {
		t.Error("esc should close the palette")
	}
	if c.tasktype() != start {
		t.Errorf("esc changed the verb to %q (want unchanged %q)", c.tasktype(), start)
	}
}

func TestTaskPicker_PickGatherPrefillsCommand(t *testing.T) {
	c := NewConsole()
	c = sendC(c, key("enter"), key("g"), key("a"), key("t"), key("enter"))
	if c.tasktype() != "gather" {
		t.Fatalf("expected gather, got %q", c.tasktype())
	}
	if c.get("gather_cmd") == "" {
		t.Error("picking gather should prefill the command from the query library")
	}
}

func TestVerbDescriptionsComplete(t *testing.T) {
	// Every verb in the Task field must carry a palette description.
	for _, v := range NewConsole().taskVerbs() {
		if v.desc == "" {
			t.Errorf("verb %q has no description in verbDescriptions", v.verb)
		}
	}
}
