package runner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/local/serial-gateway/internal/appdir"
	"github.com/local/serial-gateway/internal/config"
	"github.com/local/serial-gateway/internal/log"
	"github.com/local/serial-gateway/internal/runtime"
	"github.com/local/serial-gateway/internal/slot"
)

// Gateway runs the TCP serial bridge service.
type Gateway struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
}

// Running reports whether the gateway is active.
func (g *Gateway) Running() bool {
	return g.running.Load()
}

// Start loads config and runs listeners until Stop is called.
func (g *Gateway) Start(cfgPath string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running.Load() {
		return fmt.Errorf("gateway already running")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	logPath := cfg.Server.LogFile
	if logPath == "" {
		logPath = appdir.LogPath()
	} else if !isAbs(logPath) {
		logPath = appdir.Join(logPath)
	}

	if err := log.InitWithOptions(log.InitOptions{
		Level:    cfg.Server.LogLevel,
		FilePath: logPath,
		MaxBytes: 1 << 20,
		Console:  false,
	}); err != nil {
		return err
	}

	anchor := ""
	if cfg.HubAnchor.Enabled {
		anchor = cfg.HubAnchor.LocationContains
	}

	slots := make([]*slot.Slot, 0, len(cfg.Slots))
	for _, sc := range cfg.Slots {
		scCopy := sc
		slots = append(slots, slot.New(scCopy, anchor))
	}

	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.running.Store(true)

	go runtime.WatchLoop(ctx, slots, cfg)
	go runtime.PrintStatusTable(ctx, slots)

	for _, s := range slots {
		g.wg.Add(1)
		go func(sl *slot.Slot) {
			defer g.wg.Done()
			sl.Run(ctx, cfg.Server.BindAddr)
		}(s)
	}

	go func() {
		g.wg.Wait()
		g.running.Store(false)
	}()

	log.Info("gateway", "started, %d slots, config=%s", len(slots), cfgPath)
	return nil
}

// Stop shuts down a running gateway.
func (g *Gateway) Stop() {
	g.mu.Lock()
	cancel := g.cancel
	g.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func isAbs(path string) bool {
	if len(path) >= 2 && path[1] == ':' {
		return true
	}
	if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		return true
	}
	return false
}
