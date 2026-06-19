package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jasondostal/winadmin/fleet"
)

type targetState int

const (
	stQueued targetState = iota
	stRunning
	stOK
	stFail
	stSkip
)

type row struct {
	name  string
	state targetState
	dur   time.Duration
	note  string
}

// engine event messages
type batchMsg struct {
	num, total, size int
	label            string
}
type startMsg struct{ name string }
type resultMsg struct{ r fleet.Result }
type doneMsg struct{ s fleet.Summary }
type tickMsg time.Time

// Watcher is the live dashboard for a single run. It drives fleet.Run in a
// goroutine and renders progress as start/result events stream in.
type Watcher struct {
	plan  fleet.Plan
	opts  fleet.Options
	stage fleet.StageOptions

	rows  []row
	index map[string]int

	total, done       int
	okN, failN, skipN int

	spinner  spinner.Model
	progress progress.Model

	events chan tea.Msg
	ctx    context.Context
	cancel context.CancelFunc

	start    time.Time
	finished bool
	summary  fleet.Summary

	filterFails bool
	batchLabel  string
	batchNum    int
	batchTotal  int
	width       int
	height      int
}

// NewWatcher builds a watcher for a plan. The caller provides the plan and
// options; the watcher installs its own OnStart/OnResult hooks.
func NewWatcher(plan fleet.Plan, opts fleet.Options, stage fleet.StageOptions) Watcher {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = runStyle

	pr := progress.New(progress.WithDefaultGradient(), progress.WithoutPercentage())

	rows := make([]row, len(plan.Inventory.Targets))
	idx := make(map[string]int, len(rows))
	for i, t := range plan.Inventory.Targets {
		rows[i] = row{name: t.Name, state: stQueued}
		idx[t.Name] = i
	}

	ctx, cancel := context.WithCancel(context.Background())
	return Watcher{
		plan:     plan,
		opts:     opts,
		stage:    stage,
		rows:     rows,
		index:    idx,
		total:    len(rows),
		spinner:  sp,
		progress: pr,
		events:   make(chan tea.Msg, 256),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Init starts the engine and the render loops.
func (w Watcher) Init() tea.Cmd {
	return tea.Batch(w.spinner.Tick, w.runEngine(), w.listen(), w.tick())
}

func (w Watcher) tick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// listen pulls the next engine event off the channel.
func (w Watcher) listen() tea.Cmd {
	return func() tea.Msg { return <-w.events }
}

// runEngine launches fleet.Run in the background, funnelling its hooks into the
// event channel. It returns immediately.
func (w Watcher) runEngine() tea.Cmd {
	return func() tea.Msg {
		go func() {
			opts := w.opts
			opts.OnStart = func(t fleet.Target) { w.events <- startMsg{t.Name} }
			onResult := func(done, total int, r fleet.Result) { w.events <- resultMsg{r} }
			var summary fleet.Summary
			if w.stage.Active() {
				onBatch := func(num, total, size int, label string) {
					w.events <- batchMsg{num: num, total: total, size: size, label: label}
				}
				summary, _, _ = fleet.RunWaves(w.ctx, w.plan, opts, w.stage, onBatch, onResult)
			} else {
				summary, _ = fleet.Run(w.ctx, w.plan, opts, onResult)
			}
			w.events <- doneMsg{summary}
		}()
		return nil
	}
}

func (w Watcher) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width, w.height = msg.Width, msg.Height
		w.progress.Width = max(20, msg.Width-30)
		return w, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if w.cancel != nil {
				w.cancel()
			}
			return w, tea.Quit
		case "f":
			w.filterFails = !w.filterFails
			return w, nil
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		w.spinner, cmd = w.spinner.Update(msg)
		return w, cmd

	case tickMsg:
		if w.finished {
			return w, nil
		}
		return w, w.tick()

	case batchMsg:
		w.batchLabel, w.batchNum, w.batchTotal = msg.label, msg.num, msg.total
		return w, w.listen()

	case startMsg:
		if w.start.IsZero() {
			w.start = time.Now()
		}
		if i, ok := w.index[msg.name]; ok && w.rows[i].state == stQueued {
			w.rows[i].state = stRunning
		}
		return w, w.listen()

	case resultMsg:
		w.applyResult(msg.r)
		w.done++
		return w, w.listen()

	case doneMsg:
		w.finished = true
		w.summary = msg.s
		return w, nil
	}
	return w, nil
}

func (w *Watcher) applyResult(r fleet.Result) {
	i, ok := w.index[r.Target]
	if !ok {
		return
	}
	w.rows[i].dur = r.Duration()
	switch {
	case r.Skipped:
		w.rows[i].state = stSkip
		w.rows[i].note = "skipped"
		w.skipN++
	case r.DryRun:
		w.rows[i].state = stOK
		w.rows[i].note = "would-run"
		w.okN++
	case r.Err != nil:
		w.rows[i].state = stFail
		w.rows[i].note = trunc(r.Err.Error(), 40)
		w.failN++
	case r.ExitCode != 0:
		w.rows[i].state = stFail
		w.rows[i].note = fmt.Sprintf("exit %d", r.ExitCode)
		w.failN++
	default:
		w.rows[i].state = stOK
		w.rows[i].note = "ok"
		w.okN++
	}
}

func (w Watcher) View() string {
	var b strings.Builder

	// ---- header ----
	elapsed := time.Duration(0)
	if !w.start.IsZero() {
		end := time.Now()
		if w.finished {
			end = w.summary.Finished
		}
		elapsed = end.Sub(w.start).Truncate(time.Millisecond)
	}
	header := fmt.Sprintf("%s   %s %s",
		titleStyle.Render("fleet"),
		labelStyle.Render("elapsed"),
		valueStyle.Render(fmtDuration(elapsed)))
	if w.batchTotal > 0 {
		header += "   " + runStyle.Render(fmt.Sprintf("▶ %s %d/%d", strings.ToUpper(w.batchLabel), w.batchNum, w.batchTotal))
	}

	mode := "execute"
	if w.opts.DryRun {
		mode = "what-if"
	}
	meta := fmt.Sprintf("%s %s   %s %s   %s %s   %s %s",
		labelStyle.Render("task"), valueStyle.Render(trunc(w.plan.Task.Describe(), 46)),
		labelStyle.Render("transport"), valueStyle.Render(w.plan.Transport.Describe()),
		labelStyle.Render("parallel"), valueStyle.Render(fmt.Sprintf("%d", w.opts.Parallelism)),
		labelStyle.Render("mode"), valueStyle.Render(mode))

	// ---- progress ----
	frac := 0.0
	if w.total > 0 {
		frac = float64(w.done) / float64(w.total)
	}
	bar := fmt.Sprintf("%s  %s",
		w.progress.ViewAs(frac),
		valueStyle.Render(fmt.Sprintf("%d/%d", w.done, w.total)))

	// ---- rows ----
	rowsView := w.renderRows()

	// ---- footer ----
	footer := fmt.Sprintf("%s %s   %s %s   %s %s      %s%s   %s%s",
		okStyle.Render("ok"), valueStyle.Render(fmt.Sprintf("%d", w.okN)),
		failStyle.Render("fail"), valueStyle.Render(fmt.Sprintf("%d", w.failN)),
		mutedStyle.Render("skip"), valueStyle.Render(fmt.Sprintf("%d", w.skipN)),
		keyStyle.Render("[f]"), mutedStyle.Render("ilter fails"),
		keyStyle.Render("[q]"), mutedStyle.Render("uit"))
	if w.finished {
		footer = okStyle.Render("✓ run complete  ") + footer
	}

	body := lipgloss.JoinVertical(lipgloss.Left, meta, "", bar, "", rowsView, "", footer)
	b.WriteString(header + "\n")
	b.WriteString(boxStyle.Render(body))
	b.WriteString("\n")
	return b.String()
}

func (w Watcher) renderRows() string {
	// Build display order: running first, then failures, then the rest — so the
	// action is always at the top, btop-style.
	display := make([]row, 0, len(w.rows))
	for _, r := range w.rows {
		if w.filterFails && r.state != stFail {
			continue
		}
		display = append(display, r)
	}
	sort.SliceStable(display, func(i, j int) bool {
		return statePriority(display[i].state) < statePriority(display[j].state)
	})

	// Cap the visible rows so a 350-host run doesn't blow past the viewport.
	maxRows := 16
	if w.height > 12 {
		maxRows = w.height - 12
	}
	more := 0
	if len(display) > maxRows {
		more = len(display) - maxRows
		display = display[:maxRows]
	}

	var lines []string
	for _, r := range display {
		lines = append(lines, w.renderRow(r))
	}
	if more > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("  … %d more …", more)))
	}
	if len(lines) == 0 {
		lines = append(lines, mutedStyle.Render("  (no targets)"))
	}
	return strings.Join(lines, "\n")
}

func (w Watcher) renderRow(r row) string {
	var icon, name, note string
	name = valueStyle.Render(padRight(r.name, 20))
	switch r.state {
	case stRunning:
		icon = w.spinner.View()
		note = runStyle.Render("running")
	case stOK:
		icon = okStyle.Render("✓")
		note = okStyle.Render(r.note)
	case stFail:
		icon = failStyle.Render("✗")
		note = failStyle.Render(r.note)
	case stSkip:
		icon = mutedStyle.Render("⏸")
		note = mutedStyle.Render(r.note)
	default:
		icon = mutedStyle.Render("·")
		note = mutedStyle.Render("queued")
	}
	dur := ""
	if r.dur > 0 {
		dur = mutedStyle.Render(r.dur.Truncate(time.Millisecond).String())
	}
	return fmt.Sprintf(" %s %s %-10s %s", icon, name, note, dur)
}

func statePriority(s targetState) int {
	switch s {
	case stRunning:
		return 0
	case stFail:
		return 1
	case stQueued:
		return 2
	case stOK:
		return 3
	default:
		return 4
	}
}
