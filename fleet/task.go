package fleet

import (
	"fmt"
	"strings"
	"text/template"
)

// Task renders the command line to run for a given target. Keeping it a
// per-target render (rather than one fixed string) is what lets a task reach
// "\\{{.Name}}\..." in the local-transport model, or run plainly over SSH.
type Task interface {
	Command(t Target) (string, error)
	Describe() string
}

// CommandTask runs an arbitrary command. The template is text/template over the
// Target, so "{{.Name}}" expands to the machine name — e.g.
//
//	ping -n 1 {{.Name}}
//	robocopy \\fileserver\payload \\{{.Name}}\C$\App\ /MIR
type CommandTask struct {
	Template string
}

// Command implements Task.
func (c CommandTask) Command(t Target) (string, error) {
	return renderTemplate(c.Template, t)
}

// Describe implements Task.
func (c CommandTask) Describe() string { return "run: " + c.Template }

// RegSetTask sets a registry value. By default it targets the remote machine
// over the network (reg.exe's \\COMPUTER syntax) — the overnight-against-350-DCs
// model. Set Local to run it on the box itself (e.g. over SSH).
//
// This is the fleet-scale version of winadmin/reg.SetString.
type RegSetTask struct {
	Hive  string // HKLM, HKCU, HKCR, HKU
	Key   string // e.g. Software\Acme\App
	Name  string // value name ("" for the default value)
	Type  string // REG_SZ, REG_DWORD, REG_MULTI_SZ, ...
	Data  string
	Local bool // run on the box (no \\host prefix) instead of remotely
}

// Command implements Task.
func (r RegSetTask) Command(t Target) (string, error) {
	if r.Hive == "" || r.Key == "" {
		return "", fmt.Errorf("fleet: RegSetTask requires Hive and Key")
	}
	root := r.Hive + `\` + r.Key
	if !r.Local {
		root = `\\` + t.Name + `\` + root
	}
	var b strings.Builder
	fmt.Fprintf(&b, `reg add "%s"`, root)
	if r.Name != "" {
		fmt.Fprintf(&b, ` /v "%s"`, r.Name)
	} else {
		b.WriteString(" /ve")
	}
	if r.Type != "" {
		fmt.Fprintf(&b, ` /t %s`, r.Type)
	}
	fmt.Fprintf(&b, ` /d "%s" /f`, r.Data)
	return b.String(), nil
}

// Describe implements Task.
func (r RegSetTask) Describe() string {
	return fmt.Sprintf("regset: %s\\%s\\%s = %s", r.Hive, r.Key, r.Name, r.Data)
}

// DeleteDirTask removes a directory and its contents. By default Path is taken
// as a share-relative path on the remote machine (\\host\Path), matching the
// remote-admin model; set Local to delete a path on the box itself.
//
// Example remote Path: "C$\Temp\junk"  ->  rd /s /q "\\SERVER\C$\Temp\junk"
type DeleteDirTask struct {
	Path  string
	Local bool
}

// Command implements Task.
func (d DeleteDirTask) Command(t Target) (string, error) {
	if strings.TrimSpace(d.Path) == "" {
		return "", fmt.Errorf("fleet: DeleteDirTask requires a Path")
	}
	target := d.Path
	if !d.Local {
		target = `\\` + t.Name + `\` + d.Path
	}
	// rd /s /q: recursive, quiet. cmd /c so it works through any shell.
	return fmt.Sprintf(`cmd /c rd /s /q "%s"`, target), nil
}

// Describe implements Task.
func (d DeleteDirTask) Describe() string { return "deldir: " + d.Path }

func renderTemplate(tmpl string, t Target) (string, error) {
	parsed, err := template.New("cmd").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("fleet: bad command template: %w", err)
	}
	var b strings.Builder
	if err := parsed.Execute(&b, t.templateData()); err != nil {
		return "", fmt.Errorf("fleet: rendering command for %q: %w", t.Name, err)
	}
	return b.String(), nil
}
