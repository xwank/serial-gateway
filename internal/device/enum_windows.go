//go:build windows

package device

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

var wchUSBSerialVIDPIDs = map[string]bool{
	"1A86:5523": true, // CH341A
	"1A86:5512": true, // CH341
	"1A86:7523": true, // CH340
	"1A86:7584": true, // CH340G
}

func enumeratePlatform(all bool) ([]Info, error) {
	var out []Info

	usbRoots := []string{
		`SYSTEM\CurrentControlSet\Enum\USB`,
		`SYSTEM\CurrentControlSet\Enum\FTDIBUS`,
	}

	seen := make(map[string]bool)
	for _, root := range usbRoots {
		items, err := walkUSBEnum(root, all)
		if err != nil {
			if root == `SYSTEM\CurrentControlSet\Enum\FTDIBUS` {
				continue
			}
			return nil, err
		}
		for _, item := range items {
			if seen[item.ComName] {
				continue
			}
			seen[item.ComName] = true
			out = append(out, item)
		}
	}
	return filterPresentPorts(out), nil
}

func walkUSBEnum(root string, all bool) ([]Info, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, root, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", root, err)
	}
	defer key.Close()

	vidKeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	var out []Info
	for _, vidKey := range vidKeys {
		vidPath := root + `\` + vidKey
		vid, pid := parseVidPid(vidKey)
		if !all && vid != "" && pid != "" {
			if !wchUSBSerialVIDPIDs[strings.ToUpper(vid)+":"+strings.ToUpper(pid)] {
				// still allow common CP210x / FTDI if all=false? For now WCH only unless --all
				if !isKnownSerialVIDPID(vid, pid) {
					continue
				}
			}
		}

		instKey, err := registry.OpenKey(registry.LOCAL_MACHINE, vidPath, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		instNames, _ := instKey.ReadSubKeyNames(-1)
		instKey.Close()

		for _, inst := range instNames {
			devPath := vidPath + `\` + inst
			info, ok := readDeviceInfo(devPath, vid, pid, inst)
			if ok {
				out = append(out, info)
			}
		}
	}
	return out, nil
}

func isKnownSerialVIDPID(vid, pid string) bool {
	key := strings.ToUpper(vid) + ":" + strings.ToUpper(pid)
	if wchUSBSerialVIDPIDs[key] {
		return true
	}
	// FTDI 0403:6001, Silicon Labs 10C4:EA60
	switch key {
	case "0403:6001", "0403:6015", "10C4:EA60", "10C4:EA70":
		return true
	default:
		return false
	}
}

func readDeviceInfo(devPath, vid, pid, instance string) (Info, bool) {
	paramsPath := devPath + `\Device Parameters`
	paramsKey, err := registry.OpenKey(registry.LOCAL_MACHINE, paramsPath, registry.QUERY_VALUE)
	if err != nil {
		return Info{}, false
	}
	defer paramsKey.Close()

	comName, _, err := paramsKey.GetStringValue("PortName")
	if err != nil || comName == "" {
		return Info{}, false
	}

	devKey, err := registry.OpenKey(registry.LOCAL_MACHINE, devPath, registry.QUERY_VALUE)
	if err != nil {
		return Info{}, false
	}
	defer devKey.Close()

	locationInfo, _, _ := devKey.GetStringValue("LocationInformation")
	friendly, _, _ := devKey.GetStringValue("FriendlyName")

	pnpID := strings.TrimPrefix(devPath, `SYSTEM\CurrentControlSet\Enum\`)

	// Canonical match key combines PNP path and location text for flexible substring matching.
	matchKey := strings.ToUpper(pnpID + "|" + locationInfo + "|" + instance + "|" + friendly)

	return Info{
		ComName:      comName,
		Vid:          strings.ToUpper(vid),
		Pid:          strings.ToUpper(pid),
		PNPDeviceID:  pnpID,
		LocationInfo: locationInfo,
		MatchKey:     matchKey,
	}, true
}

func parseVidPid(keyName string) (vid, pid string) {
	// USB\VID_1A86&PID_5523
	upper := strings.ToUpper(keyName)
	if !strings.HasPrefix(upper, "VID_") {
		return "", ""
	}
	parts := strings.Split(upper, "&")
	if len(parts) < 2 {
		return "", ""
	}
	vid = strings.TrimPrefix(parts[0], "VID_")
	pid = strings.TrimPrefix(parts[1], "PID_")
	return vid, pid
}
