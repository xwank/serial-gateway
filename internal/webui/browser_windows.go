//go:build windows

package webui

import (
	"os/exec"
)

func openBrowserWindows(url string) error {
	return exec.Command("cmd", "/c", "start", "", url).Start()
}
