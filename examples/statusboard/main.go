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
	rows := []struct {
		host, os, state string
		latencyMS       int // poll response time
	}{
		{"DC01", "Windows Server 2022 Datacenter", "RUNNING", 88},
		{"DC02", "Windows Server 2022 Datacenter", "RUNNING", 102},
		{"WEB01", "Windows Server 2022 Standard", "RUNNING", 71},
		{"WEB02", "Windows Server 2019 Standard", "RUNNING", 240},
		{"WEB03", "Windows Server 2019 Standard", "RUNNING", 96},
		{"APP01", "Windows Server 2019 Standard", "RUNNING", 410}, // a slow one
		{"APP02", "Windows Server 2016 Standard", "RUNNING", 133},
		{"DB01", "Windows Server 2016 Standard", "RUNNING", 119},
		{"DB02", "Windows Server 2022 Datacenter", "RUNNING", 84},
		{"WKS-1042", "Windows 11 Enterprise", "RUNNING", 152},
		{"WKS-1043", "Windows 11 Enterprise", "RUNNING", 168},
		{"WKS-1044", "Windows 11 Enterprise", "RUNNING", 145},
		{"WKS-1051", "Windows 11 Enterprise", "STOPPED", 130},
		{"WKS-1077", "Windows 11 Pro", "UNREACHABLE", 0},
	}

	var machines []fleet.Machine
	for _, r := range rows {
		machines = append(machines, fleet.Machine{
			Hostname: r.host, OS: r.os, LastStatus: r.state, LastSeen: now,
			Latency: time.Duration(r.latencyMS) * time.Millisecond,
		})
	}
	fmt.Println(tui.RenderStatusBoard(machines, now))
}
