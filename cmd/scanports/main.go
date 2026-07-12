package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/local/serial-gateway/internal/config"
	"github.com/local/serial-gateway/internal/device"
)

func main() {
	all := flag.Bool("all", false, "show all known USB serial devices, not only common VID/PID")
	genYAML := flag.Bool("gen-yaml", false, "print yaml snippet for currently connected devices")
	doVerify := flag.Bool("verify", false, "verify config file against current devices")
	cfgPath := flag.String("c", "configs/gateway.yaml", "config file path")
	flag.Parse()

	devs, err := device.Enumerate(*all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enumerate error: %v\n", err)
		os.Exit(1)
	}

	if *doVerify {
		runVerify(*cfgPath, devs)
		return
	}

	if *genYAML {
		printYAML(devs)
		return
	}

	printTable(devs)
	pauseIfInteractive("list")
}

func printTable(devs []device.Info) {
	fmt.Printf("%-4s %-8s %-9s %-12s %-30s %s\n",
		"NO", "COM", "VID:PID", "LOCATION", "INSTANCE_TAIL", "PNP_DEVICE_ID")
	for i, d := range devs {
		tail := instanceTail(d.PNPDeviceID)
		loc := d.LocationInfo
		if loc == "" {
			loc = "-"
		}
		fmt.Printf("%-4d %-8s %-9s %-12s %-30s %s\n",
			i+1, d.ComName, d.Vid+":"+d.Pid, loc, tail, d.PNPDeviceID)
	}
	if len(devs) == 0 {
		fmt.Println("(no devices found)")
	}
	fmt.Println()
	fmt.Println("Use LOCATION / INSTANCE_TAIL / PNP path substring in match_location.")
	fmt.Println("Calibration guide: docs/calibration.md")
}

func instanceTail(pnp string) string {
	parts := strings.Split(pnp, `\`)
	if len(parts) == 0 {
		return pnp
	}
	return parts[len(parts)-1]
}

func printYAML(devs []device.Info) {
	fmt.Println("# Generated draft — assign slot id/tcp_port and description manually")
	for i, d := range devs {
		tail := instanceTail(d.PNPDeviceID)
		match := tail
		if d.LocationInfo != "" {
			match = d.LocationInfo
		}
		desc := fmt.Sprintf("COM%s %s", strings.TrimPrefix(d.ComName, "COM"), d.LocationInfo)
		fmt.Printf(`  - id: %d
    tcp_port: %d
    match_location: "%s"
    description: "%s"
    baud: 115200
    data_bits: 8
    parity: "N"
    stop_bits: 1

`, i+1, 2000+i+1, match, desc)
	}
}

func runVerify(path string, devs []device.Info) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	anchor := ""
	if cfg.HubAnchor.Enabled {
		anchor = cfg.HubAnchor.LocationContains
	}

	matches := make([]device.SlotMatch, 0, len(cfg.Slots))
	for _, s := range cfg.Slots {
		matches = append(matches, device.SlotMatch{
			ID:            s.ID,
			MatchLocation: s.MatchLocation,
			HubAnchor:     anchor,
		})
	}

	results := device.VerifySlots(devs, matches)
	descByID := make(map[int]string, len(cfg.Slots))
	for _, s := range cfg.Slots {
		descByID[s.ID] = s.Label()
	}

	ok := true
	for _, r := range results {
		desc := descByID[r.SlotID]
		if desc != "" {
			fmt.Printf("slot %d [%s] match=%q status=%s com=%s msg=%s\n",
				r.SlotID, desc, r.MatchLocation, r.Status, r.ComName, r.Message)
		} else {
			fmt.Printf("slot %d match=%q status=%s com=%s msg=%s\n",
				r.SlotID, r.MatchLocation, r.Status, r.ComName, r.Message)
		}
		if r.Status != "ok" {
			ok = false
		}
	}
	if !ok {
		os.Exit(1)
	}
	fmt.Println("VERIFY OK")
}
