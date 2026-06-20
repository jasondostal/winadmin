package tui

import (
	"io"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasondostal/winadmin/fleet"
)

// quietSlog returns a logger that discards everything, so the engine's audit
// log doesn't scribble over the full-screen TUI.
func quietSlog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// RunConsole launches the interactive run builder (TUI #2). On launch it hands
// off to the live Watcher.
func RunConsole() error {
	_, err := tea.NewProgram(NewConsole(), tea.WithAltScreen()).Run()
	return err
}

// RunWatcher launches the live dashboard (TUI #1) for an already-built plan.
func RunWatcher(plan fleet.Plan, opts fleet.Options, stage fleet.StageOptions, life fleet.LifecycleOptions) error {
	if opts.Logger == nil {
		opts.Logger = quietSlog()
	}
	w := NewWatcher(plan, opts, stage, life)
	_, err := tea.NewProgram(w, tea.WithAltScreen()).Run()
	return err
}
