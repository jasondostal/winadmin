package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jasondostal/winadmin/fleet"
)

// GatherView is a scrollable, filterable results table — the read side of the
// fleet in the TUI. Type to filter (by target or output); arrows scroll; q quits.
type GatherView struct {
	tbl    table.Model
	filter textinput.Model
	all    []fleet.Result
}

// RunGather runs the plan, then shows the results in an interactive table.
func RunGather(plan fleet.Plan, opts fleet.Options) error {
	if opts.Logger == nil {
		opts.Logger = quietSlog()
	}
	_, results := fleet.Run(context.Background(), plan, opts, nil)
	_, err := tea.NewProgram(newGatherView(results), tea.WithAltScreen()).Run()
	return err
}

func newGatherView(results []fleet.Result) GatherView {
	cols := []table.Column{
		{Title: "TARGET", Width: 28},
		{Title: "EXIT", Width: 5},
		{Title: "OUTPUT", Width: 70},
	}
	t := table.New(table.WithColumns(cols), table.WithFocused(true), table.WithHeight(20))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true).Foreground(colAccent)
	st.Selected = st.Selected.Foreground(lipgloss.Color("231")).Background(colAccent)
	t.SetStyles(st)

	fi := textinput.New()
	fi.Placeholder = "filter…"
	fi.Prompt = "/ "
	fi.Focus()

	gv := GatherView{tbl: t, filter: fi, all: results}
	gv.applyFilter()
	return gv
}

func (g *GatherView) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(g.filter.Value()))
	rows := make([]table.Row, 0, len(g.all))
	for _, r := range g.all {
		out := strings.Join(splitLines(r.Stdout), " ")
		if r.Err != nil {
			out = "ERROR: " + r.Err.Error()
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Target), q) && !strings.Contains(strings.ToLower(out), q) {
			continue
		}
		rows = append(rows, table.Row{r.Target, itoa(r.ExitCode), out})
	}
	g.tbl.SetRows(rows)
}

func (g GatherView) Init() tea.Cmd { return textinput.Blink }

func (g GatherView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.tbl.SetHeight(maxInt(5, msg.Height-5))
		return g, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return g, tea.Quit
		case "up", "down", "pgup", "pgdown", "home", "end":
			var cmd tea.Cmd
			g.tbl, cmd = g.tbl.Update(msg)
			return g, cmd
		default:
			var cmd tea.Cmd
			g.filter, cmd = g.filter.Update(msg)
			g.applyFilter()
			return g, cmd
		}
	}
	return g, nil
}

func (g GatherView) View() string {
	head := titleStyle.Render("fleet — gather") + "  " +
		mutedStyle.Render(itoa(len(g.tbl.Rows()))+"/"+itoa(len(g.all))+" rows")
	help := mutedStyle.Render("type to filter · ↑/↓ scroll · esc quit")
	return head + "\n" + g.filter.View() + "\n" + g.tbl.View() + "\n" + help + "\n"
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, "\r"))
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
