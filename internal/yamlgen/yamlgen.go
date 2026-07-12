package yamlgen

import (
	"fmt"
	"strings"

	"github.com/local/serial-gateway/internal/appdir"
	"github.com/local/serial-gateway/internal/config"
	"github.com/local/serial-gateway/internal/device"
)

// SlotDraft is one GUI row before saving yaml.
type SlotDraft struct {
	Device      device.Info
	TCPPort     int
	Description string
}

// MatchLocation picks the best match string for a device.
func MatchLocation(d device.Info) string {
	if strings.TrimSpace(d.LocationInfo) != "" {
		return d.LocationInfo
	}
	parts := strings.Split(d.PNPDeviceID, `\`)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return d.PNPDeviceID
}

// DefaultDescription builds a human-readable label.
func DefaultDescription(d device.Info) string {
	loc := d.LocationInfo
	if loc == "" {
		loc = MatchLocation(d)
	}
	return fmt.Sprintf("%s %s", d.ComName, loc)
}

// BuildConfig creates a Config from GUI selections.
func BuildConfig(lanIP string, drafts []SlotDraft) (*config.Config, error) {
	if len(drafts) == 0 {
		return nil, fmt.Errorf("no devices to save")
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			BindAddr:        "0.0.0.0",
			LanIP:           strings.TrimSpace(lanIP),
			LogLevel:        "info",
			LogFile:         "gateway.log",
			ScanIntervalSec: 2,
		},
		HubAnchor: config.HubAnchor{Enabled: false},
	}

	ports := make(map[int]bool)
	for i, d := range drafts {
		if d.TCPPort <= 0 || d.TCPPort > 65535 {
			return nil, fmt.Errorf("row %d: invalid tcp port %d", i+1, d.TCPPort)
		}
		if ports[d.TCPPort] {
			return nil, fmt.Errorf("duplicate tcp port %d", d.TCPPort)
		}
		ports[d.TCPPort] = true

		desc := strings.TrimSpace(d.Description)
		if desc == "" {
			desc = DefaultDescription(d.Device)
		}

		cfg.Slots = append(cfg.Slots, config.SlotConfig{
			ID:            i + 1,
			TCPPort:       d.TCPPort,
			MatchLocation: MatchLocation(d.Device),
			Description:   desc,
			Baud:          115200,
			DataBits:      8,
			Parity:        "N",
			StopBits:      1,
		})
	}
	return cfg, nil
}

// SaveDefaultPath writes gateway.yaml beside the executable.
func SaveDefaultPath(lanIP string, drafts []SlotDraft) (string, error) {
	cfg, err := BuildConfig(lanIP, drafts)
	if err != nil {
		return "", err
	}
	path := appdir.ConfigPath()
	if err := config.Save(path, cfg); err != nil {
		return "", err
	}
	return path, nil
}
