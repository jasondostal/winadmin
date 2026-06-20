package fleet

import "testing"

func TestTemplateColumns_PositionalWhitespace(t *testing.T) {
	// With no explicit split, columns come from whitespace — the classic
	// "whole row becomes $1 $2 $3" behavior, here as {{.F1}} {{.F2}} {{.F3}}.
	tgt := Target{Name: "jdoe Tellers logon.bat"}
	got, err := renderTemplate("user={{.F1}} grp={{.F2}} script={{.F3}} all={{.Name}}", tgt)
	if err != nil {
		t.Fatal(err)
	}
	want := "user=jdoe grp=Tellers script=logon.bat all=jdoe Tellers logon.bat"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSplitFields_CSVNamed(t *testing.T) {
	inv := &Inventory{Targets: []Target{{Name: "jdoe,Domain-Admins,logon_v2.bat"}}}
	inv.SplitFields(",", []string{"user", "grp", "script"})
	got, err := renderTemplate("net localgroup {{.grp}} {{.user}} /add  # {{.script}}", inv.Targets[0])
	if err != nil {
		t.Fatal(err)
	}
	want := "net localgroup Domain-Admins jdoe /add  # logon_v2.bat"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCommandTask_Columns(t *testing.T) {
	got, err := CommandTask{Template: "process {{.F1}} into {{.F2}}"}.Command(Target{Name: "src dst"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "process src into dst"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTemplate_MissingColumnErrors(t *testing.T) {
	if _, err := renderTemplate("{{.nope}}", Target{Name: "x"}); err == nil {
		t.Error("referencing an undefined column should error (missingkey=error)")
	}
}

func TestSplitFields_WhitespaceDefault(t *testing.T) {
	inv := &Inventory{Targets: []Target{{Name: "alpha   beta\tgamma"}}}
	inv.SplitFields("", nil) // empty delim = whitespace, collapses runs
	if got := inv.Targets[0].Fields; len(got) != 3 || got[0] != "alpha" || got[2] != "gamma" {
		t.Errorf("whitespace split = %v", inv.Targets[0].Fields)
	}
}
