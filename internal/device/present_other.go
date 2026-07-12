//go:build !windows

package device

func filterPresentPorts(devices []Info) []Info {
	return devices
}
