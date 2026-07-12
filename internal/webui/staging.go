package webui

import (
	"fmt"
	"strings"

	"github.com/local/serial-gateway/internal/device"
	"github.com/local/serial-gateway/internal/yamlgen"
)

const defaultTCPPort = 2001

// StagingSlot is one hub position saved for service startup (by match_location).
type StagingSlot struct {
	ID            int    `json:"id"`
	MatchLocation string `json:"match_location"`
	Com           string `json:"com"`
	Location      string `json:"location"`
	Description   string `json:"description"`
	TCPPort       int    `json:"tcp_port"`
}

func (s *Server) stagingList() []StagingSlot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StagingSlot, len(s.staging))
	copy(out, s.staging)
	return out
}

func (s *Server) stagingHasMatch(match string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stagingHasMatchLocked(match)
}

func (s *Server) addStagingSlots(items []StagingSlot) (added int, skipped []string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range items {
		match := strings.TrimSpace(item.MatchLocation)
		if match == "" {
			return added, skipped, fmt.Errorf("match_location 不能为空")
		}
		if s.stagingHasMatchLocked(match) {
			skipped = append(skipped, match)
			continue
		}
		port := item.TCPPort
		if port <= 0 {
			port = defaultTCPPort
		}
		desc := strings.TrimSpace(item.Description)
		if desc == "" {
			desc = fmt.Sprintf("%s %s", item.Com, item.Location)
		}
		id := len(s.staging) + 1
		s.staging = append(s.staging, StagingSlot{
			ID:            id,
			MatchLocation: match,
			Com:           item.Com,
			Location:      item.Location,
			Description:   desc,
			TCPPort:       port,
		})
		added++
	}
	return added, skipped, nil
}

func (s *Server) applyStagingPreserve(list []StagingSlot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range list {
		loc := strings.TrimSpace(item.MatchLocation)
		for i := range s.staging {
			match := item.ID > 0 && s.staging[i].ID == item.ID
			if !match && loc != "" {
				match = strings.EqualFold(s.staging[i].MatchLocation, loc)
			}
			if !match {
				continue
			}
			if item.TCPPort > 0 {
				s.staging[i].TCPPort = item.TCPPort
			}
			if strings.TrimSpace(item.Description) != "" {
				s.staging[i].Description = strings.TrimSpace(item.Description)
			}
			break
		}
	}
}

func (s *Server) stagingHasMatchLocked(match string) bool {
	needle := strings.ToUpper(strings.TrimSpace(match))
	for _, sl := range s.staging {
		if strings.Contains(strings.ToUpper(sl.MatchLocation), needle) ||
			strings.Contains(needle, strings.ToUpper(sl.MatchLocation)) {
			return true
		}
	}
	return false
}

func (s *Server) removeStagingID(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sl := range s.staging {
		if sl.ID == id {
			s.staging = append(s.staging[:i], s.staging[i+1:]...)
			s.reindexStagingLocked()
			return true
		}
	}
	return false
}

func (s *Server) clearStaging() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.staging = nil
}

func (s *Server) updateStagingSlot(id int, tcpPort int, description string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.staging {
		if s.staging[i].ID != id {
			continue
		}
		if tcpPort > 0 {
			s.staging[i].TCPPort = tcpPort
		}
		if description != "" {
			s.staging[i].Description = description
		}
		return nil
	}
	return fmt.Errorf("槽位 id=%d 不存在", id)
}

func (s *Server) reindexStagingLocked() {
	for i := range s.staging {
		s.staging[i].ID = i + 1
	}
}

func (s *Server) stagingToDrafts() ([]yamlgen.SlotDraft, error) {
	list := s.stagingList()
	if len(list) == 0 {
		return nil, fmt.Errorf("服务启动列表为空，请先加入 Hub 槽位")
	}
	drafts := make([]yamlgen.SlotDraft, 0, len(list))
	for _, sl := range list {
		loc := sl.Location
		if loc == "" {
			loc = sl.MatchLocation
		}
		info := device.Info{
			ComName:      sl.Com,
			LocationInfo: loc,
			PNPDeviceID:  sl.MatchLocation,
			MatchKey:     strings.ToUpper(sl.MatchLocation + "|" + loc),
		}
		drafts = append(drafts, yamlgen.SlotDraft{
			Device:      info,
			TCPPort:     sl.TCPPort,
			Description: sl.Description,
		})
	}
	return drafts, nil
}
