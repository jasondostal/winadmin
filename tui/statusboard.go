package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasondostal/winadmin/fleet"
)

// The status board is the heartbeat view: poll the agent service on every
// registered machine and render their live state, btop-style — problems on top,
// a per-OS roll-up at the bottom.

func stateRank(s string) int {
	switch strings.ToUpper(s) {
	case "UNREACHABLE":
		return 0
	case "NOT-INSTALLED":
		return 1
	case "STOPPED", "STOP_PENDING", "PAUSED":
		return 2
	case "START_PENDING":
		return 3
	case "RUNNING":
		return 4
	default:
		return 5
	}
}

func stateStyle(s string) lipgloss.Style {
	switch strings.ToUpper(s) {
	case "RUNNING":
		return okStyle
	case "STOPPED", "STOP_PENDING", "PAUSED":
		return warnStyle
	case "START_PENDING":
		return runStyle
	case "UNREACHABLE", "NOT-INSTALLED":
		return failStyle
	default:
		return mutedStyle
	}
}

func stateIcon(s string) string {
	switch strings.ToUpper(s) {
	case "RUNNING":
		return "●"
	case "STOPPED", "STOP_PENDING", "PAUSED":
		return "■"
	case "START_PENDING":
		return "◌"
	case "UNREACHABLE", "NOT-INSTALLED":
		return "✗"
	default:
		return "·"
	}
}

func isRunning(s string) bool { return strings.EqualFold(s, "RUNNING") }

func relTime(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

// RenderStatusBoard draws the live fleet-status dashboard for the given machines.
func RenderStatusBoard(machines []fleet.Machine, now time.Time) string {
	rows := append([]fleet.Machine(nil), machines...)
	sort.SliceStable(rows, func(i, j int) bool {
		if ri, rj := stateRank(rows[i].LastStatus), stateRank(rows[j].LastStatus); ri != rj {
			return ri < rj
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})

	running := 0
	byOS := map[string][2]int{} // os -> {running, total}
	nameW, osW := 4, 2
	for _, m := range machines {
		c := byOS[m.OS]
		c[1]++
		if isRunning(m.LastStatus) {
			running++
			c[0]++
		}
		byOS[m.OS] = c
		if len(m.Name) > nameW {
			nameW = len(m.Name)
		}
		if len(m.OS) > osW {
			osW = len(m.OS)
		}
	}
	if nameW > 32 {
		nameW = 32
	}
	if osW > 24 {
		osW = 24
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s   %s   %s\n\n",
		titleStyle.Render("fleet status"),
		valueStyle.Render(fmt.Sprintf("%d/%d agents up", running, len(machines))),
		mutedStyle.Render(now.Format("15:04:05"))))

	for _, m := range rows {
		b.WriteString(fmt.Sprintf(" %s  %s  %s  %s %s\n",
			stateStyle(m.LastStatus).Render(stateIcon(m.LastStatus)),
			valueStyle.Render(padRight(m.Name, nameW)),
			mutedStyle.Render(padRight(m.OS, osW)),
			stateStyle(m.LastStatus).Render(padRight(m.LastStatus, 12)),
			mutedStyle.Render(relTime(m.LastSeen, now))))
	}

	// per-OS roll-up
	oss := make([]string, 0, len(byOS))
	for k := range byOS {
		oss = append(oss, k)
	}
	sort.Strings(oss)
	parts := make([]string, 0, len(oss))
	for _, k := range oss {
		c := byOS[k]
		label := k
		if label == "" {
			label = "unknown"
		}
		style := okStyle
		if c[0] < c[1] {
			style = warnStyle
		}
		parts = append(parts, mutedStyle.Render(label+":")+" "+style.Render(fmt.Sprintf("%d/%d", c[0], c[1])))
	}
	b.WriteString("\n" + strings.Repeat("─", nameW+osW+24) + "\n")
	b.WriteString(strings.Join(parts, "    "))

	return boxStyle.Render(b.String())
}
