package runtime

import (
	"context"
	"time"

	"github.com/local/serial-gateway/internal/config"
	"github.com/local/serial-gateway/internal/device"
	"github.com/local/serial-gateway/internal/log"
	"github.com/local/serial-gateway/internal/slot"
)

// WatchLoop periodically rescans devices and updates slot serial handles.
func WatchLoop(ctx context.Context, slots []*slot.Slot, cfg *config.Config) {
	interval := time.Duration(cfg.Server.ScanIntervalSec) * time.Second

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	scan := func() {
		devs, err := device.Enumerate(false)
		if err != nil {
			log.Error("device", "enumerate: %v", err)
			return
		}
		for _, s := range slots {
			dev := s.Match(devs)
			s.UpdateSerial(dev)
		}
	}

	scan()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		}
	}
}

// PrintStatusTable logs slot status periodically.
func PrintStatusTable(ctx context.Context, slots []*slot.Slot) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Info("status", "SLOT  PORT  COM      DESCRIPTION      ON  CLIENTS")
			for _, s := range slots {
				log.Info("status", s.StatusLine())
			}
		}
	}
}
