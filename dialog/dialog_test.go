package dialog

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Every rendered line must have the same display (rune) width, or the box looks
// crooked in a terminal. This is the one invariant worth guarding.
func TestRenderIsRectangular(t *testing.T) {
	got := Dialog{
		Title:   "WINBATCH",
		Icon:    IconWarning,
		Message: "Your network password expires in 0 days. Please contact your system administrator.",
		Buttons: []string{"OK", "Cancel"},
		Default: 0,
	}.Render()

	lines := strings.Split(got, "\n")
	if len(lines) < 5 {
		t.Fatalf("dialog too short: %d lines", len(lines))
	}
	want := utf8.RuneCountInString(lines[0])
	for i, ln := range lines {
		if n := utf8.RuneCountInString(ln); n != want {
			t.Errorf("line %d width = %d, want %d:\n%q", i, n, want, ln)
		}
	}
}

func TestRenderContents(t *testing.T) {
	got := Dialog{Title: "Hello", Message: "world", Buttons: []string{"Yes", "No"}, Default: 1}.Render()
	for _, sub := range []string{"Hello", "world", "[ Yes ]", "< No >", "[_][#][X]"} {
		if !strings.Contains(got, sub) {
			t.Errorf("rendered dialog missing %q:\n%s", sub, got)
		}
	}
}

func TestSplashBox(t *testing.T) {
	got := SplashBox("ABC INSTALLER", "Installing ACME Toolbar")

	// Splash mode has no buttons...
	for _, bad := range []string{"< OK >", "[ OK ]"} {
		if strings.Contains(got, bad) {
			t.Errorf("splash should have no buttons, found %q", bad)
		}
	}
	// ...but keeps the title-bar controls and the wait line.
	for _, want := range []string{"[_][#][X]", "Installing ACME Toolbar", "Please wait..."} {
		if !strings.Contains(got, want) {
			t.Errorf("splash missing %q:\n%s", want, got)
		}
	}
	// Still rectangular.
	lines := strings.Split(got, "\n")
	width := utf8.RuneCountInString(lines[0])
	for i, ln := range lines {
		if n := utf8.RuneCountInString(ln); n != width {
			t.Errorf("splash line %d width = %d, want %d:\n%q", i, n, width, ln)
		}
	}
}

func TestZeroValueRenders(t *testing.T) {
	got := Dialog{}.Render()
	if !strings.Contains(got, "WinAdmin") || !strings.Contains(got, "< OK >") {
		t.Errorf("zero-value dialog should default to a WinAdmin OK box:\n%s", got)
	}
}
