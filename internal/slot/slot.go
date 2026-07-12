package slot

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/local/serial-gateway/internal/config"
	"github.com/local/serial-gateway/internal/device"
	"github.com/local/serial-gateway/internal/log"
	"github.com/local/serial-gateway/internal/serialio"
	"github.com/local/serial-gateway/internal/session"
	"github.com/local/serial-gateway/internal/telnet"
)

// Slot owns one hub position: TCP listener + serial port + shared session.
type Slot struct {
	Cfg       config.SlotConfig
	HubAnchor string

	sess      *session.Session
	serialMu  sync.Mutex
	serial    *serialio.Port
	comName   string
	online    bool

	listener  net.Listener
}

// New creates a slot from configuration.
func New(cfg config.SlotConfig, hubAnchor string) *Slot {
	return &Slot{
		Cfg:       cfg,
		HubAnchor: hubAnchor,
		sess:      session.New(cfg.ID),
	}
}

// Run starts TCP listener and background loops until ctx is cancelled.
func (s *Slot) Run(ctx context.Context, bindAddr string) {
	tag := log.SlotTag(s.Cfg.ID)

	addr := fmt.Sprintf("%s:%d", bindAddr, s.Cfg.TCPPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error(tag, "listen %s failed: %v", addr, err)
		return
	}
	s.listener = ln
	log.Info(tag, "TCP listen %s (%s)", addr, s.Cfg.Label())

	go s.serialWriterLoop(ctx)
	go s.serialReaderLoop(ctx)

	defer func() {
		_ = ln.Close()
		s.closeSerial()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Warn(tag, "accept: %v", err)
				continue
			}
		}
		go s.handleClient(ctx, conn)
	}
}

// UpdateSerial opens/closes/reopens COM based on resolved device.
func (s *Slot) UpdateSerial(dev *device.Info) {
	tag := log.SlotTag(s.Cfg.ID)

	s.serialMu.Lock()
	defer s.serialMu.Unlock()

	if dev == nil {
		if s.serial != nil {
			log.Warn(tag, "serial offline (was %s)", s.comName)
			_ = s.serial.Close()
			s.serial = nil
			s.comName = ""
			s.online = false
			s.sess.Broadcast(session.OfflineMessage())
		}
		return
	}

	if s.serial != nil && s.comName == dev.ComName {
		s.online = true
		return
	}

	if s.serial != nil {
		log.Info(tag, "serial reconnect %s -> %s", s.comName, dev.ComName)
		s.sess.Broadcast(session.ReconnectMessage())
		_ = s.serial.Close()
		s.serial = nil
	}

	p, err := serialio.Open(dev.ComName, &s.Cfg)
	if err != nil {
		log.Error(tag, "open %s failed: %v", dev.ComName, err)
		s.online = false
		s.sess.Broadcast(session.OfflineMessage())
		return
	}

	s.serial = p
	s.comName = dev.ComName
	s.online = true
	log.Info(tag, "serial open %s (%s)", dev.ComName, s.Cfg.Label())
	s.sess.Broadcast(session.ReadyMessage())
}

func (s *Slot) closeSerial() {
	s.serialMu.Lock()
	defer s.serialMu.Unlock()
	if s.serial != nil {
		_ = s.serial.Close()
		s.serial = nil
	}
}

func (s *Slot) handleClient(ctx context.Context, conn net.Conn) {
	tag := log.SlotTag(s.Cfg.ID)
	defer conn.Close()

	cid := s.sess.AddClient(conn)
	if cid < 0 {
		log.Warn(tag, "reject client %s: max clients", conn.RemoteAddr())
		return
	}
	defer s.sess.RemoveClient(cid)

	if !s.isOnline() {
		_, _ = conn.Write(session.OfflineMessage())
	}

	tn := telnet.NewProcessor(func(b []byte) error {
		_, err := conn.Write(b)
		return err
	})
	if _, err := conn.Write(telnet.Greeting()); err != nil {
		log.Debug(tag, "client=%d telnet greeting: %v", cid, err)
		return
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, err := conn.Read(buf)
		if n > 0 {
			data := tn.Process(buf[:n])
			if len(data) > 0 {
				s.sess.EnqueueWrite(session.WriteReq{ClientID: cid, Data: data})
			}
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if err != io.EOF {
				log.Debug(tag, "client=%d read end: %v", cid, err)
			}
			return
		}
	}
}

func (s *Slot) isOnline() bool {
	s.serialMu.Lock()
	defer s.serialMu.Unlock()
	return s.online && s.serial != nil
}

func (s *Slot) serialWriterLoop(ctx context.Context) {
	tag := log.SlotTag(s.Cfg.ID)
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-s.sess.WriteChannel():
			s.serialMu.Lock()
			p := s.serial
			s.serialMu.Unlock()
			if p == nil {
				continue
			}
			if _, err := p.Write(req.Data); err != nil {
				log.Error(tag, "serial write: %v", err)
			} else {
				log.Debug(tag, "client=%d wrote %d bytes", req.ClientID, len(req.Data))
			}
		}
	}
}

func (s *Slot) serialReaderLoop(ctx context.Context) {
	tag := log.SlotTag(s.Cfg.ID)
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.serialMu.Lock()
		p := s.serial
		s.serialMu.Unlock()
		if p == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		n, err := p.Read(buf)
		if n > 0 {
			out := make([]byte, n)
			copy(out, buf[:n])
			s.sess.Broadcast(telnet.Encode(out))
		}
		if err != nil && err != io.EOF {
			log.Debug(tag, "serial read: %v", err)
		}
	}
}

// StatusLine returns a short status summary.
func (s *Slot) StatusLine() string {
	s.serialMu.Lock()
	defer s.serialMu.Unlock()
	on := "NO"
	if s.online {
		on = "YES"
	}
	com := s.comName
	if com == "" {
		com = "-"
	}
	desc := s.Cfg.Label()
	if len(desc) > 16 {
		desc = desc[:16]
	}
	return fmt.Sprintf("%2d  %5d  %-8s %-16s %3s  %d",
		s.Cfg.ID, s.Cfg.TCPPort, com, desc, on, s.sess.ClientCount())
}

// Match resolves the device for this slot from a device list.
func (s *Slot) Match(devices []device.Info) *device.Info {
	return device.MatchSlot(devices, s.Cfg.MatchLocation, s.HubAnchor)
}
