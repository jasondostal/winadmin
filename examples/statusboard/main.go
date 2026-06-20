// Command statusboard renders the fleet status dashboard with a representative
// heterogeneous fleet, so you can see the `fleet status --tui` view without a
// live fleet. Run it:
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
	ago := func(ms int) time.Time { return now.Add(-time.Duration(ms) * time.Millisecond) }

	rows := []struct {
		host, os, state string
		ms              int
	}{
		{"DC01", "Windows Server 2022 Datacenter", "RUNNING", 180},
		{"DC02", "Windows Server 2022 Datacenter", "RUNNING", 210},
		{"WEB01", "Windows Server 2022 Standard", "RUNNING", 160},
		{"WEB02", "Windows Server 2019 Standard", "RUNNING", 240},
		{"WEB03", "Windows Server 2019 Standard", "RUNNING", 190},
		{"APP01", "Windows Server 2019 Standard", "RUNNING", 170},
		{"APP02", "Windows Server 2016 Standard", "RUNNING", 220},
		{"DB01", "Windows Server 2016 Standard", "RUNNING", 200},
		{"DB02", "Windows Server 2022 Datacenter", "RUNNING", 150},
		{"WKS-1042", "Windows 11 Enterprise", "RUNNING", 320},
		{"WKS-1043", "Windows 11 Enterprise", "RUNNING", 280},
		{"WKS-1044", "Windows 11 Enterprise", "RUNNING", 300},
		{"WKS-1051", "Windows 11 Enterprise", "STOPPED", 4200},
		{"WKS-1077", "Windows 11 Pro", "UNREACHABLE", 0},
	}

	var machines []fleet.Machine
	for _, r := range rows {
		m := fleet.Machine{Hostname: r.host, OS: r.os, LastStatus: r.state}
		if r.ms > 0 {
			m.LastSeen = ago(r.ms)
		}
		machines = append(machines, m)
	}
	fmt.Println(tui.RenderStatusBoard(machines, now))
}
