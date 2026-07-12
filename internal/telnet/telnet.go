package telnet

// Telnet protocol bytes (RFC 854).
const (
	IAC  byte = 255
	WILL byte = 251
	WONT byte = 252
	DO   byte = 253
	DONT byte = 254
	SB   byte = 250
	SE   byte = 240
)

// Well-known options.
const (
	OptEcho     byte = 1
	OptSGA      byte = 3 // suppress go ahead
	OptLinemode byte = 34
)

// Linemode subcommands (RFC 1184).
const (
	LMMode byte = 1
)

// Processor strips Telnet negotiation and replies so clients use character mode.
// Serial devices echo locally; the gateway must disable client-side echo and
// negotiate linemode character mode so Tab (0x09) is forwarded immediately.
type Processor struct {
	write func([]byte) error
	buf   []byte
}

// NewProcessor creates a per-connection Telnet handler.
func NewProcessor(write func([]byte) error) *Processor {
	return &Processor{write: write}
}

// Greeting negotiates character-at-a-time mode with remote echo from the serial device.
func Greeting() []byte {
	return []byte{
		IAC, WILL, OptSGA,
		IAC, WONT, OptEcho,
		IAC, DO, OptSGA,
		IAC, DONT, OptEcho,
		IAC, DO, OptLinemode,
		IAC, SB, OptLinemode, LMMode, 0, IAC, SE,
	}
}

// Process consumes input, emits negotiation replies, returns serial-bound user data.
func (p *Processor) Process(data []byte) []byte {
	if len(p.buf) > 0 {
		data = append(append([]byte{}, p.buf...), data...)
		p.buf = nil
	}

	out := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if data[i] != IAC {
			out = append(out, data[i])
			i++
			continue
		}
		if i+1 >= len(data) {
			p.buf = append([]byte{}, data[i:]...)
			break
		}
		cmd := data[i+1]
		switch cmd {
		case IAC:
			out = append(out, IAC)
			i += 2
		case WILL, WONT, DO, DONT:
			if i+2 >= len(data) {
				p.buf = append([]byte{}, data[i:]...)
				i = len(data)
				break
			}
			opt := data[i+2]
			p.reply(cmd, opt)
			i += 3
		case SB:
			opt, payload, n, ok := parseSubnegotiation(data[i:])
			if !ok {
				p.buf = append([]byte{}, data[i:]...)
				i = len(data)
				break
			}
			if opt == OptLinemode && len(payload) >= 1 && payload[0] == LMMode {
				p.replyLinemodeMode(0)
			}
			i += n
		default:
			if i+2 < len(data) {
				i += 3
			} else {
				p.buf = append([]byte{}, data[i:]...)
				i = len(data)
			}
		}
	}
	return out
}

func parseSubnegotiation(data []byte) (opt byte, payload []byte, total int, ok bool) {
	if len(data) < 3 || data[0] != IAC || data[1] != SB {
		return 0, nil, 0, false
	}
	opt = data[2]
	j := 3
	for j < len(data)-1 {
		if data[j] == IAC && data[j+1] == SE {
			return opt, data[3:j], j + 2, true
		}
		j++
	}
	return 0, nil, 0, false
}

func (p *Processor) replyLinemodeMode(mode byte) {
	if p.write == nil {
		return
	}
	_ = p.write([]byte{IAC, SB, OptLinemode, LMMode, mode, IAC, SE})
}

func (p *Processor) reply(cmd, opt byte) {
	if p.write == nil {
		return
	}
	switch cmd {
	case DO:
		switch opt {
		case OptSGA:
			_ = p.write([]byte{IAC, WILL, opt})
		case OptLinemode:
			_ = p.write([]byte{IAC, WILL, opt})
			p.replyLinemodeMode(0)
		case OptEcho:
			_ = p.write([]byte{IAC, WONT, opt})
		default:
			_ = p.write([]byte{IAC, WONT, opt})
		}
	case WILL:
		switch opt {
		case OptSGA:
			_ = p.write([]byte{IAC, DO, opt})
		case OptLinemode:
			_ = p.write([]byte{IAC, DO, opt})
			p.replyLinemodeMode(0)
		case OptEcho:
			_ = p.write([]byte{IAC, DONT, opt})
		default:
			_ = p.write([]byte{IAC, DONT, opt})
		}
	case DONT:
		switch opt {
		case OptSGA:
			_ = p.write([]byte{IAC, WONT, opt})
		case OptEcho:
			_ = p.write([]byte{IAC, WONT, opt})
		default:
			_ = p.write([]byte{IAC, WONT, opt})
		}
	case WONT:
		switch opt {
		case OptEcho:
			_ = p.write([]byte{IAC, DONT, opt})
		}
	}
}

// Encode escapes 0xFF bytes for outbound Telnet data.
func Encode(data []byte) []byte {
	n := 0
	for _, b := range data {
		if b == IAC {
			n++
		}
	}
	if n == 0 {
		out := make([]byte, len(data))
		copy(out, data)
		return out
	}
	out := make([]byte, 0, len(data)+n)
	for _, b := range data {
		if b == IAC {
			out = append(out, IAC, IAC)
			continue
		}
		out = append(out, b)
	}
	return out
}

// Filter removes Telnet IAC sequences without negotiation replies (legacy helper).
func Filter(data []byte) []byte {
	return NewProcessor(nil).Process(data)
}
