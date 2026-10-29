package peer

import (
	"net"
	"testing"
)

func TestListenSuccess(t *testing.T) {
	listener, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() returned unexpected error: %v", err)
	}
	defer listener.Close()

	if listener == nil {
		t.Fatal("Listen() returned nil listener")
	}

	if listener.Addr() == nil {
		t.Fatal("listener has no address")
	}
}

func TestListenInvalidAddress(t *testing.T) {
	listener, err := Listen("invalid-address")
	if err == nil {
		if listener != nil {
			listener.Close()
		}
		t.Fatal("Listen() expected an error for invalid address")
	}

	if listener != nil {
		listener.Close()
		t.Fatal("Listen() returned a listener alongside an error")
	}
}

func TestListenAcceptsTCPConnections(t *testing.T) {
	listener, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() returned unexpected error: %v", err)
	}
	defer listener.Close()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to connect to listener: %v", err)
	}
	defer conn.Close()
}
