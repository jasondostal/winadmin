package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasondostal/winadmin/fleet"
)

// GatherView is the read side of the fleet in the TUI: a spreadsheet over the
// per-target output. It's "just data" — sort or group by any column (synthetic
// TARGET/EXIT/OS or whatever the query parses into), collapse groups into a
// tree, and type-to-filter. Keys: / filter · s/S sort col/dir · g group · space
// collapse · ↑↓ move · q/esc quit.
type GatherView struct {
	tbl gatherTable

	sortCol   int // index into tbl.cols
	sortDesc  bool
	groupCol  int // index into tbl.cols, or -1 for no grouping
	collapsed map[string]bool

	filter    textinput.Model
	filtering bool

	lines  []gline // current visible lines (group headers + rows)
	cursor int     // index into lines
	offset int     // first visible line (scroll)

	width, height int

	// Self-loading path (run builder hand-off): when loaded is false the view
	// runs the plan itself on Init and populates the table on completion.
	plan   fleet.Plan
	opts   fleet.Options
	parse  string
	osMap  map[string]string
	loaded bool
}

// gatherDoneMsg carries the results back from the background gather run.
type gatherDoneMsg struct{ results []fleet.Result }

// gline is one rendered line: a group header or a data row.
type gline struct {
	group   bool
	key     string // group key (group lines)
	count   int    // rows in the group (group lines)
	row     gatherRow
	colVals []string // pre-rendered cell values aligned to tbl.cols (row lines)
}

// RunGather runs the plan, parses the output per the parse hint, then shows it in
// the interactive spreadsheet. osByTarget (may be nil) injects an OS column.
func RunGather(plan fleet.Plan, opts fleet.Options, parse string, osByTarget map[string]string) error {
	if opts.Logger == nil {
		opts.Logger = quietSlog()
	}
	_, results := fleet.Run(context.Background(), plan, opts, nil)
	_, err := tea.NewProgram(newGatherView(results, parse, osByTarget), tea.WithAltScreen()).Run()
	return err
}

func newGatherView(results []fleet.Result, parse string, osByTarget map[string]string) GatherView {
	g := blankGatherView()
	g.tbl = buildTable(results, parse, osByTarget)
	g.loaded = true
	g.rebuild()
	return g
}

// newGatherRun builds a view that runs the plan itself (used by the run builder),
// showing a "gathering…" state until the results arrive.
func newGatherRun(plan fleet.Plan, opts fleet.Options, parse string, osByTarget map[string]string) GatherView {
	if opts.Logger == nil {
		opts.Logger = quietSlog()
	}
	g := blankGatherView()
	g.plan, g.opts, g.parse, g.osMap = plan, opts, parse, osByTarget
	return g
}

func blankGatherView() GatherView {
	fi := textinput.New()
	fi.Placeholder = "filter…"
	fi.Prompt = "/ "
	return GatherView{
		groupCol:  -1,
		collapsed: map[string]bool{},
		filter:    fi,
		height:    24,
	}
}

// rebuild recomputes the visible lines from the current filter/sort/group state.
func (g *GatherView) rebuild() {
	rows := filterRows(g.tbl.rows, g.filter.Value())
	sortCol := g.tbl.cols[g.sortCol]
	sortRows(rows, sortCol, g.sortDesc)

	g.lines = g.lines[:0]
	if g.groupCol < 0 {
		for _, r := range rows {
			g.lines = append(g.lines, g.rowLine(r))
		}
	} else {
		for _, grp := range groupRows(rows, g.tbl.cols[g.groupCol]) {
			g.lines = append(g.lines, gline{group: true, key: grp.key, count: len(grp.rows)})
			if g.collapsed[grp.key] {
				continue
			}
			for _, r := range grp.rows {
				g.lines = append(g.lines, g.rowLine(r))
			}
		}
	}
	if g.cursor >= len(g.lines) {
		g.cursor = maxInt(0, len(g.lines)-1)
	}
	g.clampScroll()
}

func (g *GatherView) rowLine(r gatherRow) gline {
	vals := make([]string, len(g.tbl.cols))
	for i, c := range g.tbl.cols {
		vals[i] = humanizeCell(c, r.get(c)) // display only — sort/group use the raw cell
	}
	return gline{row: r, colVals: vals}
}

// humanizeCell prettifies a byte-ish column's raw integer for display (e.g.
// 123273396224 → "114.8G"), leaving everything else — and already-human values
// like "20G" — untouched. The table still sorts/filters on the raw cell.
func humanizeCell(col, val string) string {
	if !looksLikeBytes(col) {
		return val
	}
	n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if err != nil || n < 1024 {
		return val
	}
	return humanBytes(float64(n))
}

func looksLikeBytes(col string) bool {
	c := strings.ToLower(col)
	for _, k := range []string{"size", "free", "avail", "used", "space", "bytes", "capacity"} {
		if strings.Contains(c, k) {
			return true
		}
	}
	return false
}

func humanBytes(n float64) string {
	units := []string{"B", "K", "M", "G", "T", "P"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f%s", n, units[i])
	}
	return fmt.Sprintf("%.1f%s", n, units[i])
}

func (g GatherView) Init() tea.Cmd {
	if !g.loaded {
		return g.runGather()
	}
	return textinput.Blink
}

// runGather executes the plan in the background and reports the results.
func (g GatherView) runGather() tea.Cmd {
	plan, opts := g.plan, g.opts
	return func() tea.Msg {
		_, results := fleet.Run(context.Background(), plan, opts, nil)
		return gatherDoneMsg{results: results}
	}
}

func (g GatherView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.width, g.height = msg.Width, msg.Height
		g.clampScroll()
		return g, nil

	case gatherDoneMsg:
		g.tbl = buildTable(msg.results, g.parse, g.osMap)
		g.loaded = true
		g.rebuild()
		return g, textinput.Blink

	case tea.KeyMsg:
		// Before results land, only allow quitting.
		if !g.loaded {
			switch msg.String() {
			case "q", "esc", "ctrl+c":
				return g, tea.Quit
			}
			return g, nil
		}

		// Filter sub-mode: every keystroke feeds the filter; esc/enter exit.
		if g.filtering {
			switch msg.String() {
			case "esc", "enter":
				g.filtering = false
				g.filter.Blur()
				return g, nil
			case "ctrl+c":
				return g, tea.Quit
			}
			var cmd tea.Cmd
			g.filter, cmd = g.filter.Update(msg)
			g.rebuild()
			return g, cmd
		}

		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return g, tea.Quit
		case "/":
			g.filtering = true
			g.filter.Focus()
			return g, textinput.Blink
		case "s":
			g.sortCol = (g.sortCol + 1) % len(g.tbl.cols)
			g.rebuild()
		case "S", "r":
			g.sortDesc = !g.sortDesc
			g.rebuild()
		case "g":
			// none → first non-TARGET col → … → none
			g.groupCol++
			if g.groupCol == 0 {
				g.groupCol = 1 // skip TARGET as a grouping (it's unique)
			}
			if g.groupCol >= len(g.tbl.cols) {
				g.groupCol = -1
			}
			g.rebuild()
		case "up", "k":
			g.move(-1)
		case "down", "j":
			g.move(1)
		case "pgup":
			g.move(-g.bodyHeight())
		case "pgdown":
			g.move(g.bodyHeight())
		case "home":
			g.cursor, g.offset = 0, 0
		case "end":
			g.cursor = maxInt(0, len(g.lines)-1)
			g.clampScroll()
		case " ", "enter", "left", "right":
			g.toggleGroup()
		}
		return g, nil
	}
	return g, nil
}

func (g *GatherView) move(delta int) {
	if len(g.lines) == 0 {
		return
	}
	g.cursor += delta
	if g.cursor < 0 {
		g.cursor = 0
	}
	if g.cursor >= len(g.lines) {
		g.cursor = len(g.lines) - 1
	}
	g.clampScroll()
}

// toggleGroup collapses/expands the group under the cursor (or the group a row
// belongs to), then keeps the cursor on that header.
func (g *GatherView) toggleGroup() {
	if g.groupCol < 0 || len(g.lines) == 0 {
		return
	}
	// Walk up to the nearest group header at/above the cursor.
	i := g.cursor
	for i > 0 && !g.lines[i].group {
		i--
	}
	if !g.lines[i].group {
		return
	}
	key := g.lines[i].key
	g.collapsed[key] = !g.collapsed[key]
	g.cursor = i
	g.rebuild()
	// rebuild may shift indices; re-find the header to anchor the cursor.
	for j, ln := range g.lines {
		if ln.group && ln.key == key {
			g.cursor = j
			break
		}
	}
	g.clampScroll()
}

func (g *GatherView) bodyHeight() int { return maxInt(3, g.height-4) }

func (g *GatherView) clampScroll() {
	h := g.bodyHeight()
	if g.cursor < g.offset {
		g.offset = g.cursor
	}
	if g.cursor >= g.offset+h {
		g.offset = g.cursor - h + 1
	}
	if g.offset < 0 {
		g.offset = 0
	}
}

// colWidths computes a display width per column, capped, from header + cells.
func (g GatherView) colWidths() []int {
	w := make([]int, len(g.tbl.cols))
	for i, c := range g.tbl.cols {
		w[i] = len(c)
	}
	for _, ln := range g.lines {
		if ln.group {
			continue
		}
		for i, v := range ln.colVals {
			if len(v) > w[i] {
				w[i] = len(v)
			}
		}
	}
	for i := range w {
		if w[i] > 32 {
			w[i] = 32
		}
	}
	if g.sortCol >= 0 && g.sortCol < len(w) {
		w[g.sortCol]++ // room for the ▲/▼ sort marker
	}
	return w
}

func (g GatherView) View() string {
	if !g.loaded {
		n := 0
		if g.plan.Inventory != nil {
			n = g.plan.Inventory.Len()
		}
		return "\n  " + titleStyle.Render("fleet — gather") + "\n\n  " +
			runStyle.Render("◌ gathering across "+itoa(n)+" targets…") + "\n\n  " +
			mutedStyle.Render("q to cancel") + "\n"
	}

	var b strings.Builder

	// Title + sort/group status.
	sortMark := "▲"
	if g.sortDesc {
		sortMark = "▼"
	}
	status := "sort " + g.tbl.cols[g.sortCol] + sortMark
	if g.groupCol >= 0 {
		status += "  ·  group " + g.tbl.cols[g.groupCol]
	}
	rowCount := 0
	for _, ln := range g.lines {
		if !ln.group {
			rowCount++
		}
	}
	b.WriteString(titleStyle.Render("fleet — gather") + "  " +
		mutedStyle.Render(itoa(rowCount)+"/"+itoa(len(g.tbl.rows))+" rows") + "  " +
		labelStyle.Render(status) + "\n")

	// Filter line (only meaningful when active or set).
	if g.filtering || strings.TrimSpace(g.filter.Value()) != "" {
		b.WriteString(g.filter.View() + "\n")
	} else {
		b.WriteString(mutedStyle.Render("/ to filter") + "\n")
	}

	widths := g.colWidths()

	// Header row. The sort marker is appended *after* padding so its multi-byte
	// rune can't be truncated mid-column. Indent to match grouped rows.
	var hdr strings.Builder
	if g.groupCol >= 0 {
		hdr.WriteString("  ")
	}
	for i, c := range g.tbl.cols {
		cell := c
		if i == g.sortCol {
			cell += sortMark
		}
		hdr.WriteString(padRight(cell, widths[i]+1))
	}
	b.WriteString(labelStyle.Bold(true).Render(strings.TrimRight(hdr.String(), " ")) + "\n")

	// Body (windowed by offset/bodyHeight).
	h := g.bodyHeight()
	end := minInt(len(g.lines), g.offset+h)
	for i := g.offset; i < end; i++ {
		b.WriteString(g.renderLine(i, widths) + "\n")
	}
	// Pad to a stable height so the help line doesn't jump.
	for i := end - g.offset; i < h; i++ {
		b.WriteString("\n")
	}

	help := "/ filter · s sort col · S/r flip ▲▼ · g group · space fold · ↑↓ move · q quit"
	b.WriteString(mutedStyle.Render(help))
	return b.String()
}

func (g GatherView) renderLine(i int, widths []int) string {
	ln := g.lines[i]
	sel := i == g.cursor

	if ln.group {
		marker := "▾"
		if g.collapsed[ln.key] {
			marker = "▸"
		}
		text := marker + " " + ln.key + "  " + mutedStyle.Render("("+itoa(ln.count)+")")
		if sel {
			return focusStyle.Render(marker+" "+ln.key) + "  " + mutedStyle.Render("("+itoa(ln.count)+")")
		}
		return valueStyle.Render(text)
	}

	var row strings.Builder
	if g.groupCol >= 0 {
		row.WriteString("  ") // indent rows under a group
	}
	for c, v := range ln.colVals {
		row.WriteString(padRight(v, widths[c]+1))
	}
	line := strings.TrimRight(row.String(), " ")
	if sel {
		return selectedStyle.Render(line)
	}
	return line
}
