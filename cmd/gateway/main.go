package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/local/serial-gateway/internal/appdir"
	"github.com/local/serial-gateway/internal/log"
	"github.com/local/serial-gateway/internal/runner"
)

func main() {
	cfgPath := flag.String("c", appdir.ConfigPath(), "config file path")
	flag.Parse()

	gw := &runner.Gateway{}
	if err := gw.Start(*cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "gateway error: %v\n", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	gw.Stop()
	log.Info("main", "serial-gateway stopped")
}
