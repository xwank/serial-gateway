//go:build windows

package device

import (
	"strings"

	"go.bug.st/serial"
)

// filterPresentPorts drops registry "ghost" COM ports left after USB unplug.
func filterPresentPorts(devices []Info) []Info {
	out := make([]Info, 0, len(devices))
	for _, d := range devices {
		if isComPortPresent(d.ComName) {
			out = append(out, d)
		}
	}
	return out
}

func isComPortPresent(comName string) bool {
	mode := &serial.Mode{BaudRate: 115200}
	p, err := serial.Open(comName, mode)
	if err == nil {
		_ = p.Close()
		return true
	}
	// Port exists but is opened exclusively by another process (e.g. gateway).
	return isPortBusyError(err)
}

func isPortBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "being used") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "busy")
}
