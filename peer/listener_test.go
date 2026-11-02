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

func TestAcceptConnectionSuccess(t *testing.T) {
	listener, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() returned unexpected error: %v", err)
	}
	defer listener.Close()

	// Connect asynchronously because AcceptConnection blocks.
	dialResult := make(chan error, 1)

	go func() {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err == nil {
			conn.Close()
		}
		dialResult <- err
	}()

	connection, err := AcceptConnection(listener)
	if err != nil {
		t.Fatalf("AcceptConnection() returned unexpected error: %v", err)
	}
	defer connection.Close()

	if connection == nil {
		t.Fatal("AcceptConnection() returned nil connection")
	}

	if err := <-dialResult; err != nil {
		t.Fatalf("client failed to connect: %v", err)
	}
}

func TestAcceptConnectionClosedListener(t *testing.T) {
	listener, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() returned unexpected error: %v", err)
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}

	connection, err := AcceptConnection(listener)

	if err == nil {
		if connection != nil {
			connection.Close()
		}
		t.Fatal("AcceptConnection() expected an error for a closed listener")
	}

	if connection != nil {
		connection.Close()
		t.Fatal("AcceptConnection() returned a connection alongside an error")
	}
}
