package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Save writes configuration to a YAML file.
func Save(path string, cfg *Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
