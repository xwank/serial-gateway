package netlocal

import (
	"fmt"
	"net"
	"sort"
)

// IPv4List returns non-loopback IPv4 addresses on up interfaces.
func IPv4List() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{})
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			set[ip4.String()] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for ip := range set {
		out = append(out, ip)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no local IPv4 address found")
	}
	return out, nil
}
