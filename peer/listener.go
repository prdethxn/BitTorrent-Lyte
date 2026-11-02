package peer

import (
	"fmt"
	"net"
)

// Create and return listening socket
func Listen(address string) (net.Listener, error) {
	socket, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}
	return socket, nil
}

// Accept scoket
func AcceptConnection(listener net.Listener) (net.Conn, error) {
	connection, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("failed to accept connection: %w", err)
	}
	return connection, nil
}
