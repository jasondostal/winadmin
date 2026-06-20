// Package tui is the Bubble Tea front-end for the fleet engine: a live "btop'ed
// out" run watcher, and an interactive console for setting up and launching runs.
package tui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent = lipgloss.Color("63")  // periwinkle
	colOK     = lipgloss.Color("42")  // green
	colFail   = lipgloss.Color("203") // red
	colWarn   = lipgloss.Color("214") // amber
	colMuted  = lipgloss.Color("245") // grey
	colRun    = lipgloss.Color("39")  // cyan

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).
			Background(colAccent).Padding(0, 1)

	boxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).Padding(0, 1)

	labelStyle = lipgloss.NewStyle().Foreground(colMuted)
	valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("231"))

	okStyle    = lipgloss.NewStyle().Foreground(colOK)
	failStyle  = lipgloss.NewStyle().Foreground(colFail)
	warnStyle  = lipgloss.NewStyle().Foreground(colWarn)
	runStyle   = lipgloss.NewStyle().Foreground(colRun)
	mutedStyle = lipgloss.NewStyle().Foreground(colMuted)

	focusStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	keyStyle   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	// selectedStyle highlights the row/line under the cursor in list views.
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(colAccent)
)
