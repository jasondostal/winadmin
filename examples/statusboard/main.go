// Command statusboard renders the fleet status dashboard with a representative
// mixed fleet (Windows Servers + Windows 11 workstations), so you can see the
// `fleet status --tui` view without a live fleet. Run it:
//
//	go run ./examples/statusboard
package main

import (
	"fmt"
	"time"

	"github.com/jasondostal/winadmin/fleet"
	"github.com/jasondostal/winadmin/tui"
)

func main() {
	now := time.Now()
	ago := func(s int) time.Time { return now.Add(-time.Duration(s) * time.Second) }

	var fleetMachines []fleet.Machine
	// 12 Windows Servers — all agents up.
	servers := []string{"dc01", "dc02", "web01", "web02", "web03", "web04", "app01", "app02", "app03", "db01", "db02", "db03"}
	for i, n := range servers {
		fleetMachines = append(fleetMachines, fleet.Machine{
			Name: n + ".corp.local", OS: "Windows Server 2022", LastStatus: "RUNNING", LastSeen: ago(2 + i%3),
		})
	}
	// 6 Windows 11 workstations — mostly up, one stopped, one unreachable.
	win11 := []struct {
		name, state string
		seen        int
	}{
		{"ws-01", "RUNNING", 3}, {"ws-02", "RUNNING", 2}, {"ws-03", "RUNNING", 4},
		{"ws-04", "RUNNING", 3}, {"ws-05", "STOPPED", 5}, {"ws-06", "UNREACHABLE", 0},
	}
	for _, w := range win11 {
		m := fleet.Machine{Name: w.name + ".corp.local", OS: "Windows 11", LastStatus: w.state}
		if w.seen > 0 {
			m.LastSeen = ago(w.seen)
		}
		fleetMachines = append(fleetMachines, m)
	}

	fmt.Println(tui.RenderStatusBoard(fleetMachines, now))
}
