package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/local/serial-gateway/internal/appdir"
	"github.com/local/serial-gateway/internal/config"
	"github.com/local/serial-gateway/internal/device"
	"github.com/local/serial-gateway/internal/netlocal"
	"github.com/local/serial-gateway/internal/runner"
	"github.com/local/serial-gateway/internal/yamlgen"
)

// Server is the local HTTP management UI.
type Server struct {
	addr    string
	gateway *runner.Gateway
	mu      sync.Mutex
	lanIP   string
	staging []StagingSlot
}

// NewServer creates a web UI server bound to listenAddr (e.g. 127.0.0.1:17888).
func NewServer(listenAddr string, gw *runner.Gateway) *Server {
	s := &Server{addr: listenAddr, gateway: gw}
	s.loadStagingFromConfig()
	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	if err := MountStatic(mux); err != nil {
		return err
	}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/ips", s.handleIPs)
	mux.HandleFunc("/api/scan", s.handleScan)
	mux.HandleFunc("/api/staging", s.handleStaging)
	mux.HandleFunc("/api/staging/add", s.handleStagingAdd)
	mux.HandleFunc("/api/staging/remove", s.handleStagingRemove)
	mux.HandleFunc("/api/staging/clear", s.handleStagingClear)
	mux.HandleFunc("/api/staging/update", s.handleStagingUpdate)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/gateway/start", s.handleStart)
	mux.HandleFunc("/api/gateway/stop", s.handleStop)
	mux.HandleFunc("/api/status", s.handleStatus)

	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleIPs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ips, err := netlocal.IPv4List()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	selected := s.lanIP
	if selected == "" {
		if cfg, err := config.Load(appdir.ConfigPath()); err == nil {
			selected = cfg.Server.LanIP
		}
	}
	if selected == "" && len(ips) > 0 {
		selected = ips[0]
	}
	writeJSON(w, 200, map[string]interface{}{
		"ips":      ips,
		"selected": selected,
	})
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	devs, err := device.Enumerate(false)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	type row struct {
		Com           string `json:"com"`
		Location      string `json:"location"`
		MatchLocation string `json:"match_location"`
		Description   string `json:"description"`
		TCPPort       int    `json:"tcp_port"`
		VIDPID        string `json:"vid_pid"`
		InStaging     bool   `json:"in_staging"`
	}

	staging := s.stagingList()
	stagingMatch := func(match string) bool {
		m := strings.ToUpper(strings.TrimSpace(match))
		for _, sl := range staging {
			if strings.ToUpper(sl.MatchLocation) == m ||
				strings.Contains(strings.ToUpper(sl.MatchLocation), m) {
				return true
			}
		}
		return false
	}

	rows := make([]row, 0, len(devs))
	for _, d := range devs {
		loc := d.LocationInfo
		if loc == "" {
			loc = yamlgen.MatchLocation(d)
		}
		match := yamlgen.MatchLocation(d)
		rows = append(rows, row{
			Com:           d.ComName,
			Location:      loc,
			MatchLocation: match,
			Description:   yamlgen.DefaultDescription(d),
			TCPPort:       defaultTCPPort,
			VIDPID:        d.Vid + ":" + d.Pid,
			InStaging:     stagingMatch(match),
		})
	}

	msg := fmt.Sprintf("扫描到 %d 个在线设备。勾选后点击「加入服务列表」。", len(rows))
	if len(rows) == 0 {
		msg = "未发现在线 USB 串口。可拔掉板子换 Hub 下一口后再扫描。"
	}
	writeJSON(w, 200, map[string]interface{}{
		"devices": rows,
		"message": msg,
		"staging_count": len(staging),
	})
}

func (s *Server) handleStaging(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"slots":   s.stagingList(),
		"count":   len(s.stagingList()),
		"message": "服务启动列表：按 Hub 槽位逐个加入，全部完成后保存并启动。",
	})
}

type stagingAddRequest struct {
	Preserve []StagingSlot `json:"preserve"`
	Slots    []StagingSlot `json:"slots"`
}

func (s *Server) handleStagingAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req stagingAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if len(req.Slots) == 0 {
		writeJSON(w, 400, map[string]string{"error": "未选择设备"})
		return
	}

	if len(req.Preserve) > 0 {
		s.applyStagingPreserve(req.Preserve)
	}
	// only allow adding devices that are currently online
	online, err := device.Enumerate(false)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	toAdd := make([]StagingSlot, 0, len(req.Slots))
	for _, item := range req.Slots {
		match := strings.TrimSpace(item.MatchLocation)
		if match == "" {
			writeJSON(w, 400, map[string]string{"error": "match_location 缺失"})
			return
		}
		found := device.MatchSlot(online, match, "")
		if found == nil {
			writeJSON(w, 400, map[string]string{
				"error": fmt.Sprintf("设备 %s (%s) 当前不在线，无法加入。请确认板子已插入并重新扫描。", item.Com, match),
			})
			return
		}
		loc := found.LocationInfo
		if loc == "" {
			loc = yamlgen.MatchLocation(*found)
		}
		toAdd = append(toAdd, StagingSlot{
			MatchLocation: yamlgen.MatchLocation(*found),
			Com:           found.ComName,
			Location:      loc,
			Description:   item.Description,
			TCPPort:       defaultTCPPort,
		})
	}

	added, skipped, err := s.addStagingSlots(toAdd)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	msg := fmt.Sprintf("已加入 %d 项到服务列表（共 %d 项）", added, len(s.stagingList()))
	if len(skipped) > 0 {
		msg += fmt.Sprintf("\n跳过重复: %v", skipped)
	}
	writeJSON(w, 200, map[string]interface{}{
		"message": msg,
		"slots":   s.stagingList(),
	})
}

type stagingRemoveRequest struct {
	ID       int           `json:"id"`
	Preserve []StagingSlot `json:"preserve"`
}

func (s *Server) handleStagingRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req stagingRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if len(req.Preserve) > 0 {
		s.applyStagingPreserve(req.Preserve)
	}
	if !s.removeStagingID(req.ID) {
		writeJSON(w, 404, map[string]string{"error": "槽位不存在"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"message": "已移除",
		"slots":   s.stagingList(),
	})
}

func (s *Server) handleStagingClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.clearStaging()
	writeJSON(w, 200, map[string]interface{}{
		"message": "服务列表已清空",
		"slots":   []StagingSlot{},
	})
}

type stagingUpdateRequest struct {
	ID          int    `json:"id"`
	TCPPort     int    `json:"tcp_port"`
	Description string `json:"description"`
}

func (s *Server) handleStagingUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req stagingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.updateStagingSlot(req.ID, req.TCPPort, req.Description); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"slots": s.stagingList(),
	})
}

func (s *Server) loadStagingFromConfig() {
	cfg, err := config.Load(appdir.ConfigPath())
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.staging = nil
	if cfg.Server.LanIP != "" {
		s.lanIP = cfg.Server.LanIP
	}
	for _, sl := range cfg.Slots {
		loc := sl.MatchLocation
		s.staging = append(s.staging, StagingSlot{
			ID:            sl.ID,
			MatchLocation: sl.MatchLocation,
			Com:           sl.Label(),
			Location:      loc,
			Description:   sl.Description,
			TCPPort:       sl.TCPPort,
		})
	}
}

type configRequest struct {
	LanIP string `json:"lan_ip"`
	Slots []struct {
		MatchLocation string `json:"match_location"`
		Com           string `json:"com"`
		Description   string `json:"description"`
		TCPPort       int    `json:"tcp_port"`
	} `json:"slots"`
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req configRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.LanIP) == "" {
		writeJSON(w, 400, map[string]string{"error": "请选择 Server IP"})
		return
	}

	drafts, err := s.draftsFromRequest(req)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	s.mu.Lock()
	s.lanIP = req.LanIP
	s.mu.Unlock()

	path, err := yamlgen.SaveDefaultPath(req.LanIP, drafts)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	msg := fmt.Sprintf("配置已保存: %s\n服务列表 %d 个槽位\n客户端 Telnet: %s:<端口>\n日志: %s",
		path, len(drafts), req.LanIP, appdir.LogPath())
	writeJSON(w, 200, map[string]string{"message": msg})
}

func (s *Server) draftsFromRequest(req configRequest) ([]yamlgen.SlotDraft, error) {
	if len(req.Slots) == 0 {
		return s.stagingToDrafts()
	}
	drafts := make([]yamlgen.SlotDraft, 0, len(req.Slots))
	ports := make(map[int]bool)
	for i, sl := range req.Slots {
		if sl.TCPPort <= 0 {
			return nil, fmt.Errorf("第 %d 行端口无效", i+1)
		}
		if ports[sl.TCPPort] {
			return nil, fmt.Errorf("TCP 端口 %d 重复", sl.TCPPort)
		}
		ports[sl.TCPPort] = true
		match := strings.TrimSpace(sl.MatchLocation)
		if match == "" {
			return nil, fmt.Errorf("第 %d 行缺少 match_location", i+1)
		}
		info := device.Info{
			ComName:      sl.Com,
			LocationInfo: match,
			PNPDeviceID:  match,
			MatchKey:     strings.ToUpper(match),
		}
		drafts = append(drafts, yamlgen.SlotDraft{
			Device:      info,
			TCPPort:     sl.TCPPort,
			Description: sl.Description,
		})
	}
	return drafts, nil
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.gateway.Running() {
		writeJSON(w, 400, map[string]string{"error": "网关已在运行"})
		return
	}

	var req configRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.LanIP) == "" {
		writeJSON(w, 400, map[string]string{"error": "请选择 Server IP"})
		return
	}

	drafts, err := s.draftsFromRequest(req)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	s.mu.Lock()
	s.lanIP = req.LanIP
	s.mu.Unlock()

	if _, err := yamlgen.SaveDefaultPath(req.LanIP, drafts); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	if err := s.gateway.Start(appdir.ConfigPath()); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	msg := fmt.Sprintf("网关已启动（%d 个槽位）。Telnet: %s:<端口>\n日志: %s",
		len(drafts), req.LanIP, appdir.LogPath())
	writeJSON(w, 200, map[string]string{"message": msg})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.gateway.Stop()
	writeJSON(w, 200, map[string]string{"message": "网关已停止"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"running":       s.gateway.Running(),
		"log_tail":      tailLog(appdir.LogPath(), 64*1024),
		"staging_count": len(s.stagingList()),
	})
}

func tailLog(path string, max int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	size := st.Size()
	if size > max {
		_, _ = f.Seek(size-max, io.SeekStart)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	return string(b)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// PickListenAddr finds a free localhost port starting from 17888.
func PickListenAddr() (string, error) {
	for port := 17888; port < 17988; port++ {
		addr := "127.0.0.1:" + strconv.Itoa(port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		_ = ln.Close()
		return addr, nil
	}
	return "", fmt.Errorf("no free port for web UI")
}

// OpenBrowser opens the default browser on Windows.
func OpenBrowser(url string) {
	if url == "" {
		return
	}
	// best-effort; ignore errors
	_ = openBrowserPlatform(url)
}

func openBrowserPlatform(url string) error {
	return openBrowserWindows(url)
}
