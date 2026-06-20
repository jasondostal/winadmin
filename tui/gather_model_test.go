package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasondostal/winadmin/fleet"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func send(m tea.Model, msgs ...tea.Msg) GatherView {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m.(GatherView)
}

// sampleDiskResults: two hosts, each reporting two filesystems as CSV.
func sampleDiskResults() []fleet.Result {
	return []fleet.Result{
		{Target: "web01", ExitCode: 0, Stdout: "Mount,Size,Used\n/,20G,5G\n/home,100G,80G"},
		{Target: "db01", ExitCode: 0, Stdout: "Mount,Size,Used\n/,50G,40G\n/home,200G,10G"},
	}
}

func TestGatherView_ColumnsFromCSV(t *testing.T) {
	g := newGatherView(sampleDiskResults(), "csv", nil)
	want := []string{colTarget, colExit, "Mount", "Size", "Used"}
	for _, c := range want {
		found := false
		for _, have := range g.tbl.cols {
			if have == c {
				found = true
			}
		}
		if !found {
			t.Errorf("missing column %q in %v", c, g.tbl.cols)
		}
	}
	if len(g.tbl.rows) != 4 {
		t.Fatalf("want 4 rows (2 hosts × 2 filesystems), got %d", len(g.tbl.rows))
	}
}

func TestGatherView_SortCyclesAndReverses(t *testing.T) {
	g := newGatherView(sampleDiskResults(), "csv", nil)
	// Sort by Size: find the Size column index, cycle sort to it.
	sizeIdx := -1
	for i, c := range g.tbl.cols {
		if c == "Size" {
			sizeIdx = i
		}
	}
	if sizeIdx < 0 {
		t.Fatal("no Size column")
	}
	g.sortCol = sizeIdx
	g.sortDesc = true
	g.rebuild()
	// Largest Size first (200G), numeric-aware.
	first := g.lines[0]
	if first.group {
		t.Fatal("unexpected group line")
	}
	if got := first.row.get("Size"); got != "200G" {
		t.Errorf("desc sort by Size: first = %q, want 200G", got)
	}
}

func TestGatherView_GroupAndCollapse(t *testing.T) {
	g := newGatherView(sampleDiskResults(), "csv", nil)
	// Group by Mount → two groups (/ and /home), each with 2 rows.
	mountIdx := -1
	for i, c := range g.tbl.cols {
		if c == "Mount" {
			mountIdx = i
		}
	}
	g.groupCol = mountIdx
	g.rebuild()

	groupLines, rowLines := 0, 0
	for _, ln := range g.lines {
		if ln.group {
			groupLines++
		} else {
			rowLines++
		}
	}
	if groupLines != 2 || rowLines != 4 {
		t.Fatalf("grouped view: %d groups / %d rows, want 2 / 4", groupLines, rowLines)
	}

	// Collapsing the first group hides its rows.
	g.cursor = 0
	g.toggleGroup()
	rowLines = 0
	for _, ln := range g.lines {
		if !ln.group {
			rowLines++
		}
	}
	if rowLines != 2 {
		t.Errorf("after collapsing one group, want 2 visible rows, got %d", rowLines)
	}
}

func TestGatherView_FilterMode(t *testing.T) {
	g := newGatherView(sampleDiskResults(), "csv", nil)
	// Enter filter mode, type "web", and only web01's rows survive.
	g = send(g, key("/"), key("w"), key("e"), key("b"))
	if !g.filtering {
		t.Fatal("expected to be in filter mode")
	}
	for _, ln := range g.lines {
		if !ln.group && ln.row.get(colTarget) != "web01" {
			t.Errorf("filter 'web' leaked row for %q", ln.row.get(colTarget))
		}
	}
	// esc leaves filter mode but keeps the filter applied.
	g = send(g, key("esc"))
	if g.filtering {
		t.Error("esc should exit filter mode")
	}
	if strings.TrimSpace(g.filter.Value()) != "web" {
		t.Errorf("filter value should persist, got %q", g.filter.Value())
	}
}

func TestGatherView_SelfLoading(t *testing.T) {
	plan := fleet.Plan{
		Inventory: &fleet.Inventory{Targets: []fleet.Target{{Name: "h1"}}},
		Task:      fleet.CommandTask{Template: "echo hi"},
		Transport: fleet.LocalTransport{},
	}
	g := newGatherRun(plan, fleet.Options{}, "", nil)
	if g.loaded {
		t.Fatal("runner should start unloaded")
	}
	if !strings.Contains(g.View(), "gathering") {
		t.Error("unloaded view should show a gathering state")
	}
	// Simulate results arriving.
	g = send(g, gatherDoneMsg{results: []fleet.Result{{Target: "h1", Stdout: "ok"}}})
	if !g.loaded {
		t.Error("view should be loaded after results arrive")
	}
}
