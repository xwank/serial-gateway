//go:build !windows

package webui

import "fmt"

func openBrowserWindows(url string) error {
	return fmt.Errorf("open browser not implemented on this OS")
}
