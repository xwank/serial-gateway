package main

import (
	"bufio"
	"fmt"
	"os"
)

func pauseIfInteractive(mode string) {
	if mode == "verify" || mode == "gen-yaml" {
		return
	}
	if isStdoutRedirected() {
		return
	}
	fmt.Println()
	fmt.Print("Press Enter to exit...")
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func isStdoutRedirected() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}
