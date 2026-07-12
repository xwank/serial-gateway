package telnet

import (
	"bytes"
	"testing"
)

func TestFilterStripsIAC(t *testing.T) {
	in := []byte{0xFF, 0xFB, 0x01, 'h', 'i'}
	out := Filter(in)
	if !bytes.Equal(out, []byte{'h', 'i'}) {
		t.Fatalf("got %v", out)
	}
}

func TestFilterPassthrough(t *testing.T) {
	in := []byte("hello\r\n")
	out := Filter(in)
	if !bytes.Equal(out, in) {
		t.Fatalf("got %v", out)
	}
}

func TestTabPassthrough(t *testing.T) {
	in := []byte{'l', 's', '\t'}
	out := Filter(in)
	if !bytes.Equal(out, in) {
		t.Fatalf("tab must reach serial, got %v", out)
	}
}

func TestProcessorRepliesLinemodeCharacter(t *testing.T) {
	var replies [][]byte
	p := NewProcessor(func(b []byte) error {
		replies = append(replies, append([]byte{}, b...))
		return nil
	})
	user := p.Process([]byte{IAC, DO, OptLinemode})
	if len(user) != 0 {
		t.Fatalf("unexpected user data: %v", user)
	}
	if len(replies) < 1 {
		t.Fatalf("expected negotiation replies, got %v", replies)
	}
	if replies[0][0] != IAC || replies[0][1] != WILL || replies[0][2] != OptLinemode {
		t.Fatalf("expected WILL LINEMODE, got %v", replies[0])
	}
	foundMode := false
	for _, r := range replies {
		if len(r) >= 6 && r[1] == SB && r[2] == OptLinemode && r[3] == LMMode && r[4] == 0 {
			foundMode = true
		}
	}
	if !foundMode {
		t.Fatalf("expected SB LINEMODE MODE 0, got %v", replies)
	}
}

func TestProcessorRejectsClientEcho(t *testing.T) {
	var replies [][]byte
	p := NewProcessor(func(b []byte) error {
		replies = append(replies, append([]byte{}, b...))
		return nil
	})
	_ = p.Process([]byte{IAC, WILL, OptEcho})
	if len(replies) != 1 || replies[0][1] != DONT || replies[0][2] != OptEcho {
		t.Fatalf("expected DONT ECHO, got %v", replies)
	}
}

func TestGreetingDisablesClientEcho(t *testing.T) {
	g := Greeting()
	if !bytes.Contains(g, []byte{IAC, WONT, OptEcho}) {
		t.Fatal("greeting must declare WONT ECHO")
	}
	if !bytes.Contains(g, []byte{IAC, DONT, OptEcho}) {
		t.Fatal("greeting must send DONT ECHO")
	}
	if !bytes.Contains(g, []byte{IAC, DO, OptLinemode}) {
		t.Fatal("greeting must enable linemode negotiation")
	}
}

func TestEncodeEscapesIAC(t *testing.T) {
	in := []byte{'a', IAC, 'b'}
	out := Encode(in)
	want := []byte{'a', IAC, IAC, 'b'}
	if !bytes.Equal(out, want) {
		t.Fatalf("got %v want %v", out, want)
	}
}
