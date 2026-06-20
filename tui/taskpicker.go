package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// The task picker is a command-palette overlay for choosing the verb: every task
// with its one-line description, filterable by name or description. It replaces a
// 13-option cycle-select where you could only ever see the current value.

var verbDescriptions = map[string]string{
	"run":        "run any command (templated with {{.Name}})",
	"gather":     "run a query per box, tabulate it",
	"svc":        "start / stop / restart / status a service",
	"install":    "silent install an MSI / EXE / script",
	"push":       "copy files to each box",
	"reboot":     "reboot (guarded by --yes)",
	"proc":       "kill a process",
	"task":       "manage a scheduled task",
	"localgroup": "add / remove a local-group member",
	"firewall":   "add / delete a firewall rule",
	"regset":     "set a registry value",
	"deldir":     "delete a directory",
	"ldapset":    "set an attribute on every user in an AD/LDAP OU",
}

type verbDesc struct{ verb, desc string }

// taskVerbs returns the verb list (in the Task field's order) with descriptions.
func (c Console) taskVerbs() []verbDesc {
	var out []verbDesc
	for _, f := range c.fields {
		if f.key == "tasktype" {
			for _, o := range f.opts {
				out = append(out, verbDesc{o, verbDescriptions[o]})
			}
		}
	}
	return out
}

// taskMatches filters the verbs by the palette query (matching name or desc).
func (c Console) taskMatches() []verbDesc {
	q := strings.ToLower(strings.TrimSpace(c.taskFilter.Value()))
	all := c.taskVerbs()
	if q == "" {
		return all
	}
	out := make([]verbDesc, 0, len(all))
	for _, v := range all {
		if strings.Contains(v.verb, q) || strings.Contains(strings.ToLower(v.desc), q) {
			out = append(out, v)
		}
	}
	return out
}

func (c *Console) openTaskPicker() {
	c.taskPicking = true
	c.taskFilter.SetValue("")
	c.taskFilter.Focus()
	c.taskCursor = 0
	c.err = ""
}

// updateTaskPicker handles keys while the palette is open.
func (c Console) updateTaskPicker(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		c.taskPicking = false
		return c, nil
	case "enter":
		if m := c.taskMatches(); len(m) > 0 {
			if c.taskCursor >= len(m) {
				c.taskCursor = len(m) - 1
			}
			c.setSelect("tasktype", m[c.taskCursor].verb)
			c.applyQuery() // refresh the gather prefill if they picked gather
		}
		c.taskPicking = false
		return c, nil
	case "up", "ctrl+k":
		if c.taskCursor > 0 {
			c.taskCursor--
		}
		return c, nil
	case "down", "ctrl+j":
		if c.taskCursor < len(c.taskMatches())-1 {
			c.taskCursor++
		}
		return c, nil
	}
	var cmd tea.Cmd
	c.taskFilter, cmd = c.taskFilter.Update(k)
	if n := len(c.taskMatches()); c.taskCursor >= n {
		c.taskCursor = maxInt(0, n-1)
	}
	return c, cmd
}

func (c Console) taskPickerView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("fleet — pick a task") + "\n\n")
	b.WriteString(c.taskFilter.View() + "\n\n")

	matches := c.taskMatches()
	if len(matches) == 0 {
		b.WriteString(mutedStyle.Render("no match — esc to cancel") + "\n")
	}
	for i, v := range matches {
		if i == c.taskCursor {
			b.WriteString(selectedStyle.Render("▸ "+padRight(v.verb, 12)+v.desc) + "\n")
		} else {
			b.WriteString("  " + valueStyle.Render(padRight(v.verb, 12)) + mutedStyle.Render(v.desc) + "\n")
		}
	}

	b.WriteString("\n" + fmt.Sprintf("%s %s   %s %s   %s %s",
		keyStyle.Render("↑↓"), mutedStyle.Render("move"),
		keyStyle.Render("enter"), mutedStyle.Render("pick"),
		keyStyle.Render("esc"), mutedStyle.Render("cancel")))
	return boxStyle.Render(b.String()) + "\n"
}

// taskFilterInput builds the palette's filter textinput.
func taskFilterInput() textinput.Model {
	t := textinput.New()
	t.Placeholder = "filter tasks…"
	t.Prompt = "/ "
	return t
}
