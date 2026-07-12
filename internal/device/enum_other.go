//go:build !windows

package device

import "fmt"

func enumeratePlatform(all bool) ([]Info, error) {
	return nil, fmt.Errorf("device enumeration is only supported on Windows")
}
