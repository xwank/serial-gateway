package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds global server settings.
type ServerConfig struct {
	BindAddr        string `yaml:"bind_addr"`
	LanIP           string `yaml:"lan_ip"` // selected LAN IP for client connections (display)
	LogLevel        string `yaml:"log_level"`
	LogFile         string `yaml:"log_file"`
	ScanIntervalSec int    `yaml:"scan_interval_sec"`
}

// HubAnchor optionally restricts devices to a specific hub tree.
type HubAnchor struct {
	Enabled           bool   `yaml:"enabled"`
	LocationContains  string `yaml:"location_contains"`
}

// SlotConfig maps one hub slot to a TCP port and serial parameters.
type SlotConfig struct {
	ID             int    `yaml:"id"`
	TCPPort        int    `yaml:"tcp_port"`
	MatchLocation  string `yaml:"match_location"`
	Description    string `yaml:"description"` // human-readable label, e.g. "Hub2口1 COM4"
	Baud           int    `yaml:"baud"`
	DataBits       int    `yaml:"data_bits"`
	Parity         string `yaml:"parity"`
	StopBits       int    `yaml:"stop_bits"`
}

// Label returns description for logs/UI, or a short fallback.
func (s *SlotConfig) Label() string {
	if strings.TrimSpace(s.Description) != "" {
		return strings.TrimSpace(s.Description)
	}
	return fmt.Sprintf("slot-%d", s.ID)
}

// Config is the root configuration object.
type Config struct {
	Server    ServerConfig `yaml:"server"`
	HubAnchor HubAnchor    `yaml:"hub_anchor"`
	Slots     []SlotConfig `yaml:"slots"`
}

// Load reads and validates configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Server.BindAddr == "" {
		c.Server.BindAddr = "0.0.0.0"
	}
	if c.Server.LogLevel == "" {
		c.Server.LogLevel = "info"
	}
	if c.Server.ScanIntervalSec <= 0 {
		c.Server.ScanIntervalSec = 2
	}
	if len(c.Slots) == 0 {
		return fmt.Errorf("config: at least one slot required")
	}

	ports := make(map[int]int)
	for _, s := range c.Slots {
		if s.ID <= 0 {
			return fmt.Errorf("slot: invalid id %d", s.ID)
		}
		if s.TCPPort <= 0 || s.TCPPort > 65535 {
			return fmt.Errorf("slot %d: invalid tcp_port %d", s.ID, s.TCPPort)
		}
		if strings.TrimSpace(s.MatchLocation) == "" {
			return fmt.Errorf("slot %d: match_location is required", s.ID)
		}
		if s.Baud <= 0 {
			return fmt.Errorf("slot %d: invalid baud %d", s.ID, s.Baud)
		}
		if s.DataBits == 0 {
			return fmt.Errorf("slot %d: data_bits required", s.ID)
		}
		if s.StopBits == 0 {
			return fmt.Errorf("slot %d: stop_bits required", s.ID)
		}
		if other, ok := ports[s.TCPPort]; ok {
			return fmt.Errorf("duplicate tcp_port %d (slot %d and %d)", s.TCPPort, other, s.ID)
		}
		ports[s.TCPPort] = s.ID
	}
	return nil
}

// FindSlot returns slot config by id, or nil.
func (c *Config) FindSlot(id int) *SlotConfig {
	for i := range c.Slots {
		if c.Slots[i].ID == id {
			return &c.Slots[i]
		}
	}
	return nil
}
