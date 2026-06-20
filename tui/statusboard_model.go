package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasondostal/winadmin/fleet"
)

// StatusBoard is the live heartbeat dashboard: it re-polls the agent service on
// every registered machine every `every`, flips row states/colors in place, and
// stamps the registry. The poll IS the engine's fan-out.
type StatusBoard struct {
	plan         fleet.Plan
	opts         fleet.Options
	reg          *fleet.Registry
	registryPath string
	every        time.Duration

	machines      []fleet.Machine
	width, height int
	lastPoll      time.Time
	polling       bool
}

// NewStatusBoard builds the board for the registry's machines.
func NewStatusBoard(reg *fleet.Registry, registryPath string, tr fleet.Transport, opts fleet.Options, svcName string, every time.Duration) StatusBoard {
	plan := fleet.Plan{
		Inventory: reg.Inventory(),
		Task:      fleet.CommandTask{Template: "sc query " + svcName},
		Transport: tr,
	}
	return StatusBoard{
		plan: plan, opts: opts, reg: reg, registryPath: registryPath, every: every,
		machines: reg.Machines,
	}
}

type sbPollMsg struct{ states map[string]string }
type sbTickMsg time.Time

func (b StatusBoard) Init() tea.Cmd { return b.poll() }

// poll runs one status fan-out across the fleet in the background.
func (b StatusBoard) poll() tea.Cmd {
	plan, opts := b.plan, b.opts
	return func() tea.Msg {
		_, results := fleet.Run(context.Background(), plan, opts, nil)
		states := make(map[string]string, len(results))
		for _, r := range results {
			states[r.Target] = fleet.ServiceState(r)
		}
		return sbPollMsg{states}
	}
}

func (b StatusBoard) tick() tea.Cmd {
	return tea.Tick(b.every, func(t time.Time) tea.Msg { return sbTickMsg(t) })
}

func (b StatusBoard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width, b.height = msg.Width, msg.Height
		return b, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return b, tea.Quit
		case "r":
			if !b.polling {
				b.polling = true
				return b, b.poll()
			}
		}
		return b, nil

	case sbTickMsg:
		if b.polling {
			return b, nil
		}
		b.polling = true
		return b, b.poll()

	case sbPollMsg:
		now := time.Now()
		for i := range b.machines {
			if st, ok := msg.states[b.machines[i].Name]; ok {
				b.machines[i].LastStatus = st
				b.machines[i].LastSeen = now
			}
		}
		b.lastPoll = now
		b.polling = false
		_ = b.reg.Save(b.registryPath)
		return b, b.tick()
	}
	return b, nil
}

func (b StatusBoard) View() string {
	board := RenderStatusBoard(b.machines, time.Now())
	help := "  " + keyStyle.Render("[r]") + mutedStyle.Render(" refresh   ") +
		keyStyle.Render("[q]") + mutedStyle.Render(" quit")
	next := ""
	if b.every > 0 && !b.lastPoll.IsZero() {
		next = "   " + mutedStyle.Render("auto-refresh "+b.every.String())
	}
	return board + "\n" + help + next + "\n"
}
