package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/local/serial-gateway/internal/appdir"
	"github.com/local/serial-gateway/internal/runner"
	"github.com/local/serial-gateway/internal/webui"
)

func main() {
	addr, err := webui.PickListenAddr()
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	gw := &runner.Gateway{}
	srv := webui.NewServer(addr, gw)

	url := "http://" + addr + "/"
	fmt.Printf("Serial Gateway UI: %s\n", url)
	fmt.Printf("Config/log directory: %s\n", appdir.Base())
	webui.OpenBrowser(url)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("web server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	gw.Stop()
	fmt.Println("已退出")
}
