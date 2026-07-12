package session

import (
	"net"
	"sync"

	"github.com/local/serial-gateway/internal/log"
)

const maxClients = 16

// WriteReq is one client write request queued for the serial writer.
type WriteReq struct {
	ClientID int
	Data     []byte
}

// Session manages multiple TCP clients sharing one serial port.
type Session struct {
	slotID int

	mu      sync.Mutex
	clients map[int]*clientEntry
	nextID  int

	writeCh chan WriteReq
}

type clientEntry struct {
	id   int
	conn net.Conn
}

// New creates a session for one slot.
func New(slotID int) *Session {
	return &Session{
		slotID:  slotID,
		clients: make(map[int]*clientEntry),
		writeCh: make(chan WriteReq, 4096),
	}
}

// WriteChannel returns the queue consumed by the serial writer goroutine.
func (s *Session) WriteChannel() <-chan WriteReq {
	return s.writeCh
}

// AddClient registers a new TCP connection.
func (s *Session) AddClient(conn net.Conn) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.clients) >= maxClients {
		return -1
	}

	s.nextID++
	id := s.nextID
	s.clients[id] = &clientEntry{id: id, conn: conn}
	log.Info(log.SlotTag(s.slotID), "client=%d connect %s", id, conn.RemoteAddr())
	return id
}

// RemoveClient unregisters a TCP connection.
func (s *Session) RemoveClient(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[id]; ok {
		delete(s.clients, id)
		log.Info(log.SlotTag(s.slotID), "client=%d disconnect", id)
	}
}

// ClientCount returns active client count.
func (s *Session) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

// EnqueueWrite queues client data for serialized serial output.
func (s *Session) EnqueueWrite(req WriteReq) {
	select {
	case s.writeCh <- req:
	default:
		log.Warn(log.SlotTag(s.slotID), "write queue full, drop %d bytes from client=%d", len(req.Data), req.ClientID)
	}
}

// Broadcast sends data to all connected clients.
func (s *Session) Broadcast(data []byte) {
	s.mu.Lock()
	list := make([]*clientEntry, 0, len(s.clients))
	for _, c := range s.clients {
		list = append(list, c)
	}
	s.mu.Unlock()

	for _, c := range list {
		if len(data) == 0 {
			continue
		}
		if _, err := c.conn.Write(data); err != nil {
			log.Warn(log.SlotTag(s.slotID), "client=%d write failed: %v", c.id, err)
			s.RemoveClient(c.id)
			_ = c.conn.Close()
		}
	}
}

// Notify sends a short system message to all clients.
func (s *Session) Notify(msg string) {
	s.Broadcast([]byte(msg))
}

// OfflineMessage returns the serial offline banner.
func OfflineMessage() []byte { return []byte("\r\n[GW] serial offline\r\n") }

// ReadyMessage returns the serial ready banner.
func ReadyMessage() []byte { return []byte("\r\n[GW] serial ready\r\n") }

// ReconnectMessage returns the serial reconnect banner.
func ReconnectMessage() []byte { return []byte("\r\n[GW] serial reconnecting...\r\n") }
