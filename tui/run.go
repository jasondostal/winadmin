package tui

import (
	"io"
	"log/slog"
	"time"

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

// RunStatusBoard launches the live fleet-status dashboard, polling the agent
// service on every registered machine every `every`.
func RunStatusBoard(reg *fleet.Registry, registryPath string, tr fleet.Transport, opts fleet.Options, svcName string, every time.Duration) error {
	if opts.Logger == nil {
		opts.Logger = quietSlog()
	}
	b := NewStatusBoard(reg, registryPath, tr, opts, svcName, every)
	_, err := tea.NewProgram(b, tea.WithAltScreen()).Run()
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
