// Package dialog is a small, dependency-free homage to the chunky gray message
// boxes that classic Windows admin tools flung all over Windows 9x — the kind an
// overnight script loved to leave waiting on a locked workstation at 3 a.m. It
// renders that look in the terminal: a double-ruled window, a fake title bar
// with [_][#][X] controls, an icon gutter, and chunky buttons.
//
// It draws a dialog; it does not pop a real GUI window. That is the joke — and
// also why it works the same on every platform with zero dependencies. Render()
// returns the box as a string (so it is trivially testable); the Message /
// Warn / ErrorBox / AskYesNo helpers print it for you.
package dialog

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// Icon selects the little glyph in the dialog's left gutter, mirroring the
// classic MB_ICON* family.
type Icon int

const (
	IconNone Icon = iota
	IconInfo
	IconWarning
	IconError
	IconQuestion
)

func (ic Icon) glyph() string {
	switch ic {
	case IconInfo:
		return "(i)"
	case IconWarning:
		return "/!\\"
	case IconError:
		return "(x)"
	case IconQuestion:
		return "(?)"
	default:
		return ""
	}
}

// Dialog is a single message box. The zero value renders an empty OK box titled
// "WinAdmin"; set the fields you care about.
type Dialog struct {
	Title   string   // title-bar text (default "WinAdmin")
	Message string   // body text; '\n' forces a line break, the rest word-wraps
	Buttons []string // button labels (default ["OK"])
	Default int      // index of the focused/default button
	Icon    Icon     // gutter glyph
	Width   int      // body wrap width hint; 0 = a sensible default

	hideButtons bool // splash mode: render no button row
	centerBody  bool // center body lines instead of left-justifying
}

// Render draws the dialog and returns it as a multi-line string. Every line is
// padded to the same display width, so it stays aligned wherever it lands.
func (d Dialog) Render() string {
	rl := utf8.RuneCountInString

	title := d.Title
	if title == "" {
		title = "WinAdmin"
	}
	buttons := d.Buttons
	if len(buttons) == 0 {
		buttons = []string{"OK"}
	}
	const controls = "[_][#][X]"

	wrapW := d.Width
	if wrapW <= 0 {
		wrapW = 44
	}

	// Body lines, indented under a 4-wide icon gutter when an icon is set.
	icon := d.Icon.glyph()
	const gutter = "    "
	var body []string
	for i, ln := range wrap(d.Message, wrapW) {
		switch {
		case i == 0 && d.Icon != IconNone:
			body = append(body, icon+" "+ln)
		case d.Icon != IconNone:
			body = append(body, gutter+ln)
		default:
			body = append(body, ln)
		}
	}

	// Button bar (omitted in splash mode): the default button gets <angle>
	// emphasis, others [square].
	btnLine := ""
	if !d.hideButtons {
		var bs []string
		for i, b := range buttons {
			if i == d.Default {
				bs = append(bs, "< "+b+" >")
			} else {
				bs = append(bs, "[ "+b+" ]")
			}
		}
		btnLine = strings.Join(bs, "   ")
	}

	// Inner content width: the widest of the title bar, body, and button row.
	w := rl(title) + 2 + rl(controls)
	for _, ln := range body {
		if n := rl(ln); n > w {
			w = n
		}
	}
	if n := rl(btnLine); n > w {
		w = n
	}
	if w < 38 {
		w = 38
	}

	var sb strings.Builder
	rule := func(l, r string) { sb.WriteString(l + strings.Repeat("═", w+2) + r + "\n") }
	line := func(content string) {
		pad := w - rl(content)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString("║ " + content + strings.Repeat(" ", pad) + " ║\n")
	}

	gap := w - rl(title) - rl(controls)
	if gap < 1 {
		gap = 1
	}

	rule("╔", "╗")
	line(title + strings.Repeat(" ", gap) + controls)
	rule("╠", "╣")
	line("")
	for _, ln := range body {
		if d.centerBody {
			if pad := (w - rl(ln)) / 2; pad > 0 {
				ln = strings.Repeat(" ", pad) + ln
			}
		}
		line(ln)
	}
	line("")
	if !d.hideButtons {
		bpad := (w - rl(btnLine)) / 2
		if bpad < 0 {
			bpad = 0
		}
		line(strings.Repeat(" ", bpad) + btnLine)
		line("")
	}
	// bottom border, no trailing newline
	sb.WriteString("╚" + strings.Repeat("═", w+2) + "╝")
	return sb.String()
}

// wrap word-wraps s to width, honoring explicit '\n' as a hard break. Words
// longer than width are left intact rather than chopped.
func wrap(s string, width int) []string {
	rl := utf8.RuneCountInString
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := words[0]
		for _, wd := range words[1:] {
			if rl(cur)+1+rl(wd) <= width {
				cur += " " + wd
			} else {
				lines = append(lines, cur)
				cur = wd
			}
		}
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// Message prints an informational OK box.
func Message(title, msg string) {
	fmt.Println(Dialog{Title: title, Message: msg, Icon: IconInfo}.Render())
}

// Warn prints a warning OK box.
func Warn(title, msg string) {
	fmt.Println(Dialog{Title: title, Message: msg, Icon: IconWarning}.Render())
}

// ErrorBox prints an error OK box.
func ErrorBox(title, msg string) {
	fmt.Println(Dialog{Title: title, Message: msg, Icon: IconError}.Render())
}

// AskYesNo renders a Yes/No question and reads the answer from stdin. Anything
// starting with 'y' is yes; the default (bare Enter / EOF) is no.
func AskYesNo(title, msg string) bool {
	d := Dialog{Title: title, Message: msg, Icon: IconQuestion, Buttons: []string{"Yes", "No"}, Default: 0}
	fmt.Println(d.Render())
	fmt.Print("  [Y]es / [N]o > ")
	sc := bufio.NewScanner(os.Stdin)
	if sc.Scan() {
		a := strings.ToLower(strings.TrimSpace(sc.Text()))
		return strings.HasPrefix(a, "y")
	}
	return false
}

// Splash prints a button-less "loading" box — the terminal descendant of the
// splash/status boxes overnight installers threw up while they worked. status is
// the action line, e.g. "Installing Acme Toolbar"; an empty status falls back to
// "Working".
func Splash(title, status string) { fmt.Println(SplashBox(title, status)) }

// SplashBox returns the splash box as a string (Splash prints it).
func SplashBox(title, status string) string {
	if status == "" {
		status = "Working"
	}
	return Dialog{
		Title:       title,
		Message:     status + "\n\n( Please wait... )",
		Icon:        IconNone,
		Width:       40,
		hideButtons: true,
		centerBody:  true,
	}.Render()
}

// Classic is the easter egg: the exact flavor of dialog overnight installer jobs
// used to fling at half the office every morning — drawn here in loving,
// slightly cursed ASCII.
func Classic() {
	fmt.Println(Dialog{
		Title: "WINBATCH",
		Icon:  IconWarning,
		Message: "Your network password expires in 0 days.\n" +
			"Please contact your system administrator.\n\n" +
			"(There is no system administrator.)",
		Buttons: []string{"OK", "OK", "OK"},
		Default: 0,
	}.Render())
}
