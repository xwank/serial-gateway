package serialio

import (
	"fmt"
	"time"

	"go.bug.st/serial"
	"github.com/local/serial-gateway/internal/config"
)

// Port wraps a host serial port handle.
type Port struct {
	port serial.Port
	name string
}

// Open opens a serial port using slot serial parameters.
func Open(comName string, cfg *config.SlotConfig) (*Port, error) {
	mode := serial.Mode{
		BaudRate: cfg.Baud,
		DataBits: cfg.DataBits,
		Parity:   parseParity(cfg.Parity),
		StopBits: parseStopBits(cfg.StopBits),
	}

	p, err := serial.Open(comName, &mode)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", comName, err)
	}

	_ = p.SetReadTimeout(200 * time.Millisecond)

	return &Port{port: p, name: comName}, nil
}

// Name returns the OS device name.
func (p *Port) Name() string {
	return p.name
}

// Read reads up to len(buf) bytes.
func (p *Port) Read(buf []byte) (int, error) {
	return p.port.Read(buf)
}

// Write writes data to the serial port.
func (p *Port) Write(data []byte) (int, error) {
	return p.port.Write(data)
}

// Close closes the serial port.
func (p *Port) Close() error {
	if p.port == nil {
		return nil
	}
	return p.port.Close()
}

func parseParity(p string) serial.Parity {
	switch p {
	case "E", "e":
		return serial.EvenParity
	case "O", "o":
		return serial.OddParity
	case "M", "m":
		return serial.MarkParity
	case "S", "s":
		return serial.SpaceParity
	default:
		return serial.NoParity
	}
}

func parseStopBits(bits int) serial.StopBits {
	switch bits {
	case 2:
		return serial.TwoStopBits
	case 15:
		return serial.OnePointFiveStopBits
	default:
		return serial.OneStopBit
	}
}
