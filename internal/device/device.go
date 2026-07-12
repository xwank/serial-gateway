package device

import "strings"

// Info describes one serial port device discovered on the system.
type Info struct {
	ComName      string
	Vid          string
	Pid          string
	PNPDeviceID  string
	LocationInfo string
	MatchKey     string
}

// Enumerate returns all serial-capable devices on the host OS.
func Enumerate(all bool) ([]Info, error) {
	return enumeratePlatform(all)
}

// MatchSlot finds a device for the given location substring and optional hub anchor.
func MatchSlot(devices []Info, matchLocation string, hubAnchor string) *Info {
	needle := strings.ToUpper(strings.TrimSpace(matchLocation))
	if needle == "" {
		return nil
	}
	anchor := strings.ToUpper(strings.TrimSpace(hubAnchor))

	for i := range devices {
		d := &devices[i]
		key := strings.ToUpper(d.MatchKey)
		if anchor != "" && !strings.Contains(key, anchor) {
			continue
		}
		if strings.Contains(key, needle) {
			return d
		}
	}
	return nil
}

// VerifySlots checks that each slot matches exactly one unique device.
func VerifySlots(devices []Info, slots []SlotMatch) []VerifyResult {
	results := make([]VerifyResult, len(slots))
	used := make(map[int]bool)

	for i, slot := range slots {
		r := VerifyResult{SlotID: slot.ID, MatchLocation: slot.MatchLocation}
		var found *Info
		var foundIdx = -1
		for j := range devices {
			d := &devices[j]
			if MatchSlot([]Info{*d}, slot.MatchLocation, slot.HubAnchor) != nil {
				if found != nil {
					r.Status = "conflict"
					r.Message = "multiple devices match"
					results[i] = r
					found = nil
					break
				}
				found = d
				foundIdx = j
			}
		}
		if r.Status == "conflict" {
			continue
		}
		if found == nil {
			r.Status = "missing"
			r.Message = "no device found"
		} else if used[foundIdx] {
			r.Status = "conflict"
			r.Message = "device already matched by another slot"
		} else {
			r.Status = "ok"
			r.ComName = found.ComName
			r.PNPDeviceID = found.PNPDeviceID
			r.LocationInfo = found.LocationInfo
			used[foundIdx] = true
		}
		results[i] = r
	}
	return results
}

// SlotMatch is input for verification.
type SlotMatch struct {
	ID            int
	MatchLocation string
	HubAnchor     string
}

// VerifyResult is the outcome for one slot.
type VerifyResult struct {
	SlotID        int
	MatchLocation string
	Status        string
	Message       string
	ComName       string
	PNPDeviceID   string
	LocationInfo  string
}
